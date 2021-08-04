package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/index/manager"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/web"
)

var (
	baseDir     = flag.String("base_dir", "/tmp", "All paths are relative to this path")
	pcapDir     = flag.String("pcap_dir", "", "Path where pcaps will be stored")
	indexDir    = flag.String("index_dir", "", "Path where indexes will be stored")
	snapshotDir = flag.String("snapshot_dir", "", "Path where snapshots will be stored")
	stateDir    = flag.String("state_dir", "", "Path where state files will be stored")

	userPassword = flag.String("user_password", "", "HTTP auth password for users")
	pcapPassword = flag.String("pcap_password", "", "HTTP auth password for pcaps")

	listenAddress = flag.String("address", ":8080", "Listen address")
)

func main() {
	flag.Parse()

	mgr, err := manager.New(
		filepath.Join(*baseDir, *pcapDir),
		filepath.Join(*baseDir, *indexDir),
		filepath.Join(*baseDir, *snapshotDir),
		filepath.Join(*baseDir, *stateDir),
	)
	if err != nil {
		log.Fatalf("manager.New failed: %v", err)
	}
	var server *http.Server

	r := chi.NewRouter()
	r.Use(middleware.SetHeader("Access-Control-Allow-Origin", "*"))
	r.Use(middleware.SetHeader("Access-Control-Allow-Methods", "*"))
	/*
		r.Options(`/*`, func(w http.ResponseWriter, r *http.Request) {
			for k, v := range headers {
				w.Header().Set(k, v)
			}
		})
	*/
	checkBasicAuth := func(password string) func(http.Handler) http.Handler {
		if password == "" {
			return func(h http.Handler) http.Handler {
				return h
			}
		}
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, pass, ok := r.BasicAuth()
				if ok && pass == password {
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Add("WWW-Authenticate", `Basic realm="realm"`)
				w.WriteHeader(http.StatusUnauthorized)
			})
		}
	}
	rUser := r.With(checkBasicAuth(*userPassword))
	rPcap := r.With(checkBasicAuth(*pcapPassword))

	rPcap.Post("/upload/{filename:.+[.]pcap}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		if filename != filepath.Base(filename) {
			http.Error(w, "Invalid filename", http.StatusBadRequest)
			return
		}
		fullFilename := filepath.Join(*baseDir, *pcapDir, filename)

		dst, err := os.OpenFile(fullFilename, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			http.Error(w, "File already exists", http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(dst, r.Body); err != nil {
			http.Error(w, fmt.Sprintf("Error while storing file: %s", err), http.StatusInternalServerError)
			dst.Close()
			os.Remove(fullFilename)
			return
		}
		if err := dst.Close(); err != nil {
			http.Error(w, fmt.Sprintf("Error while storing file: %s", err), http.StatusInternalServerError)
			os.Remove(fullFilename)
			return
		}
		mgr.ImportPcap(filename)
		http.Error(w, "OK", http.StatusOK)
	})
	rUser.Mount("/debug", middleware.Profiler())
	rUser.Get("/api/status.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(mgr.Status()); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
		}
	})
	rUser.Get("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(mgr.ListTags()); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
		}
	})
	rUser.Delete("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		s := r.URL.Query()["name"]
		if len(s) != 1 {
			http.Error(w, "`name` parameter missing", http.StatusBadRequest)
			return
		}
		if err := mgr.DelTag(s[0]); err != nil {
			http.Error(w, fmt.Sprintf("delete failed: %v", err), http.StatusBadRequest)
			return
		}
	})
	rUser.Put("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		s := r.URL.Query()["name"]
		if len(s) != 1 {
			http.Error(w, "`name` parameter missing", http.StatusBadRequest)
			return
		}
		if len(s[0]) < 1 {
			http.Error(w, "`name` parameter empty", http.StatusBadRequest)
			return
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := mgr.AddTag(s[0], string(body)); err != nil {
			http.Error(w, fmt.Sprintf("add failed: %v", err), http.StatusBadRequest)
			return
		}
	})
	rUser.Get(`/api/download/{stream:\d+}.pcap`, func(w http.ResponseWriter, r *http.Request) {
		streamIDStr := chi.URLParam(r, "stream")
		streamID, err := strconv.ParseUint(streamIDStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid stream id %q failed: %v", streamIDStr, err), http.StatusBadRequest)
			return
		}
		stream, streamReleaser, err := mgr.Stream(streamID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Stream(%d) failed: %v", streamID, err), http.StatusInternalServerError)
			return
		}
		defer streamReleaser.Release(mgr)
		packets, err := stream.Packets()
		if err != nil {
			http.Error(w, fmt.Sprintf("Stream(%d).Packets() failed: %v", streamID, err), http.StatusInternalServerError)
			return
		}
		knownPcaps := map[string]time.Time{}
		for _, kp := range mgr.KnownPcaps() {
			knownPcaps[kp.Filename] = kp.PacketTimestampMin
		}
		pcapFiles := map[string][]uint64{}
		for _, p := range packets {
			if _, ok := knownPcaps[p.PcapFilename]; !ok {
				http.Error(w, fmt.Sprintf("Unknown pcap %q referenced", p.PcapFilename), http.StatusInternalServerError)
				return
			}
			pcapFiles[p.PcapFilename] = append(pcapFiles[p.PcapFilename], p.PcapIndex)
		}
		usedPcapFiles := []string{}
		for fn, packetIndexes := range pcapFiles {
			sort.Slice(packetIndexes, func(i, j int) bool {
				return packetIndexes[i] < packetIndexes[j]
			})
			usedPcapFiles = append(usedPcapFiles, fn)
		}
		sort.Slice(usedPcapFiles, func(i, j int) bool {
			return knownPcaps[usedPcapFiles[i]].Before(knownPcaps[usedPcapFiles[j]])
		})
		w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
		pcapProducer := pcapgo.NewWriterNanos(w)
		for i, fn := range usedPcapFiles {
			handle, err := pcap.OpenOffline(filepath.Join(mgr.PcapDir, fn))
			if err != nil {
				http.Error(w, fmt.Sprintf("OpenOffline failed: %v", err), http.StatusInternalServerError)
				return
			}
			defer handle.Close()
			if i == 0 {
				if err := pcapProducer.WriteFileHeader(uint32(handle.SnapLen()), handle.LinkType()); err != nil {
					http.Error(w, fmt.Sprintf("WriteFileHeader failed: %v", err), http.StatusInternalServerError)
					return
				}
			}
			pos := uint64(0)
			for _, p := range pcapFiles[fn] {
				for {
					data, ci, err := handle.ReadPacketData()
					if err != nil {
						http.Error(w, fmt.Sprintf("ReadPacketData failed: %v", err), http.StatusInternalServerError)
						return
					}
					pos++
					if p != pos-1 {
						continue
					}
					if err := pcapProducer.WritePacket(ci, data); err != nil {
						http.Error(w, fmt.Sprintf("WritePacket failed: %v", err), http.StatusInternalServerError)
						return
					}
					break
				}
			}
		}
	})
	rUser.Get(`/api/stream/{stream:\d+}.json`, func(w http.ResponseWriter, r *http.Request) {
		streamIDStr := chi.URLParam(r, "stream")
		streamID, err := strconv.ParseUint(streamIDStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid stream id %q failed: %v", streamIDStr, err), http.StatusBadRequest)
			return
		}
		stream, streamReleaser, err := mgr.Stream(streamID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Stream(%d) failed: %v", streamID, err), http.StatusInternalServerError)
			return
		}
		if stream == nil {
			http.Error(w, fmt.Sprintf("stream %d not found", streamID), http.StatusNotFound)
			return
		}
		defer streamReleaser.Release(mgr)
		data, err := stream.Data()
		if err != nil {
			http.Error(w, fmt.Sprintf("Stream(%d).Data() failed: %v", streamID, err), http.StatusInternalServerError)
			return
		}
		response := struct {
			Stream *index.Stream
			Data   []index.Data
		}{
			Stream: stream,
			Data:   data,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
			return
		}
	})
	rUser.Post("/api/search.json", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		qq, err := query.Parse(string(body))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			response := struct {
				Error string
			}{
				Error: err.Error(),
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}
		page := uint(0)
		if s := r.URL.Query()["page"]; len(s) == 1 {
			n, err := strconv.ParseUint(s[0], 10, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid page %q: %v", s[0], err), http.StatusBadRequest)
				return
			}
			page = uint(n)
		}

		response := struct {
			Debug       []string
			Results     []*index.Stream
			MoreResults bool
		}{
			Debug:   qq.Debug,
			Results: []*index.Stream{},
		}
		indexes, tags, indexesReleaser, err := mgr.GetIndexes(qq.Conditions.ReferencedTags())
		if err != nil {
			http.Error(w, fmt.Sprintf("GetIndexes failed: %v", err), http.StatusInternalServerError)
			return
		}
		defer indexesReleaser.Release(mgr)
		res, hasMore, err := index.SearchStreams(indexes, 0, qq.ReferenceTime, qq.Conditions, qq.Sorting, qq.Limit, page*qq.Limit, tags)
		if err != nil {
			http.Error(w, fmt.Sprintf("SearchStreams failed: %v", err), http.StatusInternalServerError)
			return
		}
		response.Results = append(response.Results, res...)
		response.MoreResults = hasMore
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
			return
		}
	})
	rUser.Get("/*", http.FileServer(http.FS(&web.FS{})).ServeHTTP)

	server = &http.Server{
		Addr:    *listenAddress,
		Handler: r,
	}
	log.Println("Ready to serve...")
	if err := server.ListenAndServe(); err != nil {
		log.Printf("ListenAndServe failed: %v", err)
	}
}
