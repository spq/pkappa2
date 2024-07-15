package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"github.com/gorilla/websocket"
	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/index/manager"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools"
	"github.com/spq/pkappa2/web"
)

const (
	// WebSocket timings
	// Time allowed to write a message to the client.
	writeWait = 5 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var (
	baseDir      = flag.String("base_dir", os.TempDir(), "All paths are relative to this path")
	pcapDir      = flag.String("pcap_dir", "", "Path where pcaps will be stored")
	indexDir     = flag.String("index_dir", "", "Path where indexes will be stored")
	snapshotDir  = flag.String("snapshot_dir", "", "Path where snapshots will be stored")
	stateDir     = flag.String("state_dir", "", "Path where state files will be stored")
	converterDir = flag.String("converter_dir", "./converters", "Path where converter executables are searched")

	userPassword = flag.String("user_password", "", "HTTP auth password for users")
	pcapPassword = flag.String("pcap_password", "", "HTTP auth password for pcaps")

	listenAddress = flag.String("address", ":8080", "Listen address")

	startupCpuprofile = flag.String("startup_cpuprofile", "", "write cpu profile to file")
)

func main() {
	flag.Parse()

	if *startupCpuprofile != "" {
		f, err := os.Create(*startupCpuprofile)
		if err != nil {
			log.Fatalf("Failed to create profile folder %s: %v", *startupCpuprofile, err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("Failed to start CPU profile: %v", err)
		}
	}

	tools.AssertFolderRWXPermissions("base_dir", *baseDir)

	mgr, err := manager.New(
		filepath.Join(*baseDir, *pcapDir),
		filepath.Join(*baseDir, *indexDir),
		filepath.Join(*baseDir, *snapshotDir),
		filepath.Join(*baseDir, *stateDir),
		*converterDir,
	)
	if err != nil {
		log.Fatalf("manager.New failed: %v", err)
	}
	defer mgr.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		log.Println("Interrupt received. Cleaning up...")
		mgr.Close()
		os.Exit(1)
	}()

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

	rPcap.Post("/upload/{filename:.+[.]pcap[ng]?}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		if filename != filepath.Base(filename) {
			http.Error(w, "Invalid filename", http.StatusBadRequest)
			return
		}
		tools.AssertFolderRWXPermissions("pcap_dir", filepath.Join(*baseDir, *pcapDir))
		fullFilename := filepath.Join(*baseDir, *pcapDir, filename)

		dst, err := os.OpenFile(fullFilename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		if err != nil {
			http.Error(w, fmt.Sprintf("File already exists: %v", err), http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(dst, r.Body); err != nil {
			http.Error(w, fmt.Sprintf("Error while storing file: %v", err), http.StatusInternalServerError)
			if err := dst.Close(); err != nil {
				log.Printf("Failed to close new pcap file in upload: %v", err)
			}
			if err := os.Remove(fullFilename); err != nil {
				log.Printf("Failed to remove empty new pcap file in upload: %v", err)
			}
			return
		}
		if err := dst.Close(); err != nil {
			http.Error(w, fmt.Sprintf("Error while storing file: %s", err), http.StatusInternalServerError)
			if err := os.Remove(fullFilename); err != nil {
				log.Printf("Failed to remove new pcap file in upload after failed save: %v", err)
			}
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
	rUser.Get("/api/pcaps.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(mgr.KnownPcaps()); err != nil {
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
		n := r.URL.Query()["name"]
		if len(n) != 1 || n[0] == "" {
			http.Error(w, "`name` parameter missing or empty", http.StatusBadRequest)
			return
		}
		c := r.URL.Query()["color"]
		if len(c) != 1 || c[0] == "" {
			http.Error(w, "`color` parameter missing or empty", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := mgr.AddTag(n[0], c[0], string(body)); err != nil {
			http.Error(w, fmt.Sprintf("add failed: %v", err), http.StatusBadRequest)
			return
		}
	})
	rUser.Patch("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query()["name"]
		if len(n) != 1 || n[0] == "" {
			http.Error(w, "`name` parameter missing or empty", http.StatusBadRequest)
			return
		}
		m := r.URL.Query()["method"]
		if len(m) != 1 || m[0] == "" {
			http.Error(w, "`method` parameter missing or empty", http.StatusBadRequest)
			return
		}

		operation := manager.UpdateTagOperation(nil)
		streamMarkMethod := manager.UpdateTagOperationMarkAddStream
		switch m[0] {
		case "mark_del":
			streamMarkMethod = manager.UpdateTagOperationMarkDelStream
			fallthrough
		case "mark_add":
			s := r.URL.Query()["stream"]
			if len(s) == 0 {
				http.Error(w, "`stream` parameter missing", http.StatusBadRequest)
				return
			}
			streams := make([]uint64, 0, len(s))
			for _, n := range s {
				v, err := strconv.ParseUint(n, 10, 64)
				if err != nil {
					http.Error(w, fmt.Sprintf("invalid value for `stream` parameter: %q", n), http.StatusBadRequest)
					return
				}
				streams = append(streams, v)
			}
			operation = streamMarkMethod(streams)
		case "change_color":
			c := r.URL.Query()["color"]
			if len(c) != 1 || c[0] == "" {
				http.Error(w, "`color` parameter missing or empty", http.StatusBadRequest)
				return
			}
			operation = manager.UpdateTagOperationUpdateColor(c[0])
		case "converter_set":
			c := r.URL.Query()["converters"]
			operation = manager.UpdateTagOperationSetConverter(c)
		default:
			http.Error(w, fmt.Sprintf("unknown `method`: %q", m[0]), http.StatusBadRequest)
			return
		}
		if err := mgr.UpdateTag(n[0], operation); err != nil {
			http.Error(w, fmt.Sprintf("update failed: %v", err), http.StatusBadRequest)
			return
		}
	})
	rUser.Get("/api/converters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(mgr.ListConverters()); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
		}
	})
	rUser.Get(`/api/converters/stderr/{name:.+}/{pid:\d+}`, func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		pidStr := chi.URLParam(r, "pid")
		pid, err := strconv.ParseInt(pidStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid process id %q failed: %v", pidStr, err), http.StatusBadRequest)
			return
		}
		converterDetails, err := mgr.ConverterStderr(name, int(pid))
		if err != nil {
			http.Error(w, fmt.Sprintf("get converter stderr failed: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(converterDetails); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
		}
	})
	rUser.Delete("/api/converters/{name:.+}", func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if err := mgr.ResetConverter(name); err != nil {
			http.Error(w, fmt.Sprintf("reset failed: %v", err), http.StatusBadRequest)
		}
	})
	rUser.Get(`/api/download/{stream:\d+}.pcap`, func(w http.ResponseWriter, r *http.Request) {
		streamIDStr := chi.URLParam(r, "stream")
		streamID, err := strconv.ParseUint(streamIDStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid stream id %q failed: %v", streamIDStr, err), http.StatusBadRequest)
			return
		}
		v := mgr.GetView()
		defer v.Release()
		streamContext, err := v.Stream(streamID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Stream(%d) failed: %v", streamID, err), http.StatusInternalServerError)
			return
		}
		if streamContext.Stream() == nil {
			http.Error(w, fmt.Sprintf("Stream(%d) not found", streamID), http.StatusNotFound)
			return
		}
		packets, err := streamContext.Stream().Packets()
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
		v := mgr.GetView()
		defer v.Release()
		streamContext, err := v.Stream(streamID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Stream(%d) failed: %v", streamID, err), http.StatusInternalServerError)
			return
		}
		if streamContext.Stream() == nil {
			http.Error(w, fmt.Sprintf("stream %d not found", streamID), http.StatusNotFound)
			return
		}
		converter := "auto"
		if f := r.URL.Query()["converter"]; len(f) == 1 {
			converter = f[0]
		}
		converters, err := streamContext.AllConverters()
		if err != nil {
			http.Error(w, fmt.Sprintf("AllConverters() failed: %v", err), http.StatusInternalServerError)
			return
		}
		if converter == "auto" {
			if len(converters) == 1 {
				converter = converters[0]
			} else {
				converter = ""
			}
		} else if converter == "none" {
			converter = ""
		} else {
			if !strings.HasPrefix(converter, "converter:") {
				http.Error(w, fmt.Sprintf("invalid converter %q", converter), http.StatusBadRequest)
				return
			}
			converter = converter[len("converter:"):]
		}
		data, err := streamContext.Data(converter)
		if err != nil {
			http.Error(w, fmt.Sprintf("Data(%q) failed: %v", converter, err), http.StatusInternalServerError)
			return
		}
		tags, err := streamContext.AllTags()
		if err != nil {
			http.Error(w, fmt.Sprintf("AllTags() failed: %v", err), http.StatusInternalServerError)
			return
		}
		// TODO: Send correct ClientBytes and ServerBytes when sending converter output.
		response := struct {
			Stream          *index.Stream
			Data            []index.Data
			Tags            []string
			Converters      []string
			ActiveConverter string
		}{
			Stream:          streamContext.Stream(),
			Data:            data,
			Tags:            tags,
			Converters:      converters,
			ActiveConverter: converter,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
			return
		}
	})
	rUser.Post("/api/search.json", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
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
			Debug   []string
			Results []struct {
				Stream *index.Stream
				Tags   []string
			}
			Offset      uint
			MoreResults bool
		}{
			Debug: qq.Debug,
			Results: []struct {
				Stream *index.Stream
				Tags   []string
			}{},
		}
		v := mgr.GetView()
		defer v.Release()
		hasMore, offset, err := v.SearchStreams(r.Context(), qq, func(c manager.StreamContext) error {
			tags, err := c.AllTags()
			if err != nil {
				return err
			}
			response.Results = append(response.Results, struct {
				Stream *index.Stream
				Tags   []string
			}{
				Stream: c.Stream(),
				Tags:   tags,
			})
			return nil
		}, manager.Limit(100, page), manager.PrefetchAllTags())
		if err != nil {
			http.Error(w, fmt.Sprintf("SearchStreams failed: %v", err), http.StatusInternalServerError)
			return
		}
		response.MoreResults = hasMore
		response.Offset = offset
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
			return
		}
	})
	rUser.Get("/api/graph.json", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
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
		filter := (*query.Query)(nil)
		if qs := r.URL.Query()["query"]; len(qs) == 1 {
			q, err := query.Parse(qs[0])
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid query %q: %v", qs[0], err), http.StatusBadRequest)
				return
			}
			if q.Grouping != nil {
				http.Error(w, fmt.Sprintf("Invalid query %q: grouping not supported", qs[0]), http.StatusBadRequest)
				return
			}
			filter = q
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

		groupingTags := r.URL.Query()["tag"]

		v := mgr.GetView()
		defer v.Release()

		referenceTime, err := v.ReferenceTime()
		if err != nil {
			http.Error(w, fmt.Sprintf("ReferenceTime failed: %v", err), http.StatusInternalServerError)
			return
		}

		type (
			tagInfo struct {
				name string
				used map[int]int
			}
		)
		tagInfos := []tagInfo(nil)
		for _, tn := range groupingTags {
			tagInfos = append(tagInfos, tagInfo{
				name: tn,
				used: make(map[int]int),
			})
		}
		type tagGroup struct {
			extends    int
			extendedBy string
			counts     map[time.Duration][]uint64
		}
		tagGroups := []tagGroup{{}}
		handleStream := func(c manager.StreamContext) error {
			tagGroupId := 0
			for _, ti := range tagInfos {
				hasTag, err := c.HasTag(ti.name)
				if err != nil {
					return err
				}
				if !hasTag {
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
			s := c.Stream()
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
		}

		if filter != nil {
			_, _, err := v.SearchStreams(ctx, filter, handleStream, manager.PrefetchTags(groupingTags))
			if err != nil {
				http.Error(w, fmt.Sprintf("SearchStreams failed: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			err := v.AllStreams(ctx, handleStream, manager.PrefetchTags(groupingTags))
			if err != nil {
				http.Error(w, fmt.Sprintf("AllStreams failed: %v", err), http.StatusInternalServerError)
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
	rUser.Get("/api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(mgr.ListPcapProcessorWebhooks()); err != nil {
			http.Error(w, fmt.Sprintf("Encode failed: %v", err), http.StatusInternalServerError)
		}
	})
	rUser.Delete("/api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query()["url"]
		if len(u) != 1 || u[0] == "" {
			http.Error(w, "`url` parameter missing", http.StatusBadRequest)
			return
		}
		if err := mgr.DelPcapProcessorWebhook(u[0]); err != nil {
			http.Error(w, fmt.Sprintf("delete failed: %v", err), http.StatusBadRequest)
			return
		}
	})
	rUser.Put("/api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query()["url"]
		if len(u) != 1 || u[0] == "" {
			http.Error(w, "`url` parameter missing or empty", http.StatusBadRequest)
			return
		}
		if err := mgr.AddPcapProcessorWebhook(u[0]); err != nil {
			http.Error(w, fmt.Sprintf("add failed: %v", err), http.StatusBadRequest)
			return
		}
	})
	rUser.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket Upgrade failed: %v", err)
			return
		}
		defer c.Close()
		log.Printf("WebSocket Client %q connected", c.RemoteAddr().String())

		ch, closer := mgr.Listen()
		defer closer()

		// Read from websocket to process control messages
		clientClosed := make(chan struct{})
		go func() {
			c.SetReadLimit(512)
			if err := c.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
				log.Printf("WebSocket SetReadDeadline failed: %v", err)
				close(clientClosed)
				return
			}
			c.SetPongHandler(func(string) error {
				if err := c.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
					log.Printf("WebSocket SetReadDeadline failed: %v", err)
					return err
				}
				return nil
			})
			for {
				_, _, err := c.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						log.Printf("WebSocket ReadMessage failed: %v", err)
					}
					close(clientClosed)
					return
				}
			}
		}()
		// Write to websocket to send updates
		pingTicker := time.NewTicker(pingPeriod)
		defer pingTicker.Stop()

	outer:
		for {
			select {
			case msg := <-ch:
				if err := c.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
					log.Printf("WebSocket SetWriteDeadline failed: %v", err)
					break outer
				}
				if err := c.WriteJSON(msg); err != nil {
					log.Printf("WebSocket WriteJSON failed: %v", err)
					break outer
				}
			case <-pingTicker.C:
				if err := c.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
					log.Printf("WebSocket SetWriteDeadline failed: %v", err)
					break outer
				}
				if err := c.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					log.Printf("WebSocket Ping failed: %v", err)
					break outer
				}
			case <-clientClosed:
				break outer
			}
		}
		log.Printf("WebSocket Client %q disconnected", c.RemoteAddr().String())
	})

	server := &http.Server{
		Addr:    *listenAddress,
		Handler: r,
	}
	log.Println("Ready to serve...")

	if *startupCpuprofile != "" {
		pprof.StopCPUProfile()
		log.Printf("CPU profile written to %s", *startupCpuprofile)
	}

	if err := server.ListenAndServe(); err != nil {
		log.Printf("ListenAndServe failed: %v", err)
	}
}
