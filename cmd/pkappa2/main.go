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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/index/manager"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools/bitmask"
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
		res, hasMore, err := index.SearchStreams(indexes, 0, qq.ReferenceTime, qq.Conditions, qq.Grouping, qq.Sorting, qq.Limit, page*qq.Limit, tags)
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
	rUser.Get("/api/graph.json", func(w http.ResponseWriter, r *http.Request) {
		var min, max time.Time
		delta := 1 * time.Minute
		if s := r.URL.Query()["delta"]; len(s) == 1 {
			d, err := time.ParseDuration(s[0])
			if err != nil || d <= 0 {
				http.Error(w, fmt.Sprintf("Invalid delta %q: %v", s[0], err), http.StatusBadRequest)
				return
			}
			delta = d
		}
		if s := r.URL.Query()["min"]; len(s) == 1 {
			t, err := time.Parse("1", s[0])
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid min time %q: %v", s[0], err), http.StatusBadRequest)
				return
			}
			min = t.Truncate(delta)
		}
		if s := r.URL.Query()["max"]; len(s) == 1 {
			t, err := time.Parse("1", s[0])
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid max time %q: %v", s[0], err), http.StatusBadRequest)
				return
			}
			max = t.Truncate(delta)
		}
		type (
			Aspect uint8
		)
		const (
			AspectAnchor          Aspect = 0b0001
			AspectAnchorFirst     Aspect = 0b0000
			AspectAnchorLast      Aspect = 0b0001
			AspectType            Aspect = 0b1110
			AspectTypeConnections Aspect = 0b0000
			AspectTypeDuration    Aspect = 0b0010
			AspectTypeBytes       Aspect = 0b0100
			AspectTypeClientBytes Aspect = 0b0110
			AspectTypeServerBytes Aspect = 0b1000
		)
		aspects := []Aspect(nil)
		for _, a := range r.URL.Query()["aspect"] {
			if !func() bool {
				as := strings.Split(a, "@")
				if len(as) != 1 && len(as) != 2 {
					return false
				}
				aspect := Aspect(0)
				if v, ok := map[string]Aspect{
					"connections": AspectTypeConnections,
					"duration":    AspectTypeDuration,
					"bytes":       AspectTypeBytes,
					"cbytes":      AspectTypeClientBytes,
					"sbytes":      AspectTypeServerBytes,
				}[as[0]]; ok {
					aspect |= v
				} else {
					return false
				}
				if len(as) == 2 {
					if v, ok := map[string]Aspect{
						"first": AspectAnchorFirst,
						"last":  AspectAnchorLast,
					}[as[1]]; ok {
						aspect |= v
					} else {
						return false
					}
				}
				aspects = append(aspects, aspect)
				return true
			}() {
				http.Error(w, fmt.Sprintf("Invalid aspect %q: %v", a, err), http.StatusBadRequest)
				return
			}
		}
		if len(aspects) == 0 {
			aspects = []Aspect{AspectAnchorFirst | AspectTypeConnections}
		}
		sort.Slice(aspects, func(i, j int) bool {
			a, b := aspects[i], aspects[j]
			if (a^b)&AspectAnchor != 0 {
				return a&AspectAnchor < b&AspectAnchor
			}
			return a < b
		})

		indexes, tags, releaser, err := mgr.GetIndexes(r.URL.Query()["tag"])
		if err != nil {
			http.Error(w, fmt.Sprintf("GetIndexes failed: %v", err), http.StatusInternalServerError)
			return
		}
		defer releaser.Release(mgr)

		referenceTime := time.Time{}
		for _, idx := range indexes {
			if referenceTime.IsZero() || referenceTime.After(idx.ReferenceTime) {
				referenceTime = idx.ReferenceTime
			}
		}

		type (
			tagInfo struct {
				name string
				mask bitmask.LongBitmask
				used map[int]int
			}
		)
		tagInfos := []tagInfo(nil)
		for tn, tm := range tags {
			tagInfos = append(tagInfos, tagInfo{
				name: tn,
				mask: tm,
				used: make(map[int]int),
			})
		}
		type tagGroup struct {
			extends    int
			extendedBy string
			counts     map[time.Duration][]uint64
		}
		tagGroups := []tagGroup{{}}
		for i := len(indexes); i > 0; i-- {
			idx := indexes[i-1]
			if err := idx.AllStreams(func(s *index.Stream) error {
				for _, idx2 := range indexes[i:] {
					if _, ok := idx2.StreamIDs()[s.ID()]; ok {
						return nil
					}
				}
				tagGroupId := 0
				for _, ti := range tagInfos {
					if !ti.mask.IsSet(uint(s.ID())) {
						continue
					}
					newTagGroupId, ok := ti.used[tagGroupId]
					if !ok {
						newTagGroupId = len(tagGroups)
						tagGroups = append(tagGroups, tagGroup{
							extends:    tagGroupId,
							extendedBy: ti.name,
						})
						ti.used[tagGroupId] = newTagGroupId
					}
					tagGroupId = newTagGroupId
				}
				tagGroup := &tagGroups[tagGroupId]
				if tagGroup.counts == nil {
					tagGroup.counts = make(map[time.Duration][]uint64)
				}
				var t time.Time
				skip := false
				countsEntry := []uint64(nil)
				countsKey := time.Duration(0)
				for i, a := range aspects {
					if i == 0 || (aspects[i-1]^a)&AspectAnchor != 0 {
						if i != 0 {
							tagGroup.counts[countsKey] = countsEntry
						}
						switch a & AspectAnchor {
						case AspectAnchorFirst:
							t = s.FirstPacket()
						case AspectAnchorLast:
							t = s.LastPacket()
						}
						t = t.Truncate(delta)
						if skip = (!min.IsZero() && min.After(t)) || (!max.IsZero() && max.Before(t)); skip {
							continue
						}
						ok := false
						countsKey = t.Sub(referenceTime)
						if countsEntry, ok = tagGroup.counts[countsKey]; !ok {
							countsEntry = make([]uint64, len(aspects))
						}
					} else if skip {
						continue
					}

					d := uint64(0)
					switch a & AspectType {
					case AspectTypeConnections:
						d = 1
					case AspectTypeBytes:
						d = s.ClientBytes + s.ServerBytes
					case AspectTypeClientBytes:
						d = s.ClientBytes
					case AspectTypeServerBytes:
						d = s.ServerBytes
					case AspectTypeDuration:
						d = s.LastPacketTimeNS - s.FirstPacketTimeNS
					}
					countsEntry[i] += d
				}
				tagGroup.counts[countsKey] = countsEntry
				return nil
			}); err != nil {
				http.Error(w, fmt.Sprintf("GetIndexes failed: %v", err), http.StatusInternalServerError)
				return
			}
		}
		response := struct {
			Min, Max time.Time
			Delta    time.Duration
			Aspects  []string
			Data     []struct {
				Tags []string
				Data [][]uint64
			}
		}{}
		response.Delta = delta
		for _, a := range aspects {
			response.Aspects = append(response.Aspects, fmt.Sprintf("%s@%s", map[Aspect]string{
				AspectTypeConnections: "connections",
				AspectTypeDuration:    "duration",
				AspectTypeBytes:       "bytes",
				AspectTypeClientBytes: "cbytes",
				AspectTypeServerBytes: "sbytes",
			}[(a&AspectType)], []string{
				"first", "last",
			}[(a&AspectAnchor)/AspectAnchor]))
		}
		for _, tg := range tagGroups {
			for d := range tg.counts {
				t := referenceTime.Add(d)
				if response.Min.IsZero() || response.Min.After(t) {
					response.Min = t
				}
				if response.Max.IsZero() || response.Max.Before(t) {
					response.Max = t
				}
			}
		}
		for tagGroupId := range tagGroups {
			tg := &tagGroups[tagGroupId]
			data := [][]uint64(nil)
			for d, v := range tg.counts {
				t := referenceTime.Add(d).Sub(response.Min) / delta
				data = append(data, append([]uint64{uint64(t)}, v...))
			}
			sort.Slice(data, func(i, j int) bool {
				return data[i][0] < data[j][0]
			})
			tagsList := []string{}
			for tagGroupId != 0 {
				tagGroupId = tg.extends
				tagsList = append(tagsList, tg.extendedBy)
				tg = &tagGroups[tagGroupId]
			}

			response.Data = append(response.Data, struct {
				Tags []string
				Data [][]uint64
			}{
				Tags: tagsList,
				Data: data,
			})
		}

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
