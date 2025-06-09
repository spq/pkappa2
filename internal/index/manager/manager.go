package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/index/builder"
	"github.com/spq/pkappa2/internal/index/converters"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools"
	"github.com/spq/pkappa2/internal/tools/bitmask"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
)

const (
	// Request timeout for webhooks
	pcapProcessorWebhookTimeout = time.Second * 5

	pcapOverIPCmdFlush = pcapOverIPCmd(iota)
	pcapOverIPCmdClose
)

type (
	PcapStatistics struct {
		PcapCount         int
		PacketCount       int
		ImportJobCount    int
		IndexCount        int
		StreamCount       int
		StreamRecordCount int
		PacketRecordCount int
	}

	Event struct {
		Type                string
		Tag                 *TagInfo                  `json:",omitempty"`
		Converter           *converters.Statistics    `json:",omitempty"`
		PcapStats           *PcapStatistics           `json:",omitempty"`
		Config              *Config                   `json:",omitempty"`
		Webhooks            *[]string                 `json:",omitempty"`
		PcapOverIPEndpoints *[]PcapOverIPEndpointInfo `json:",omitempty"`
	}

	PcapOverIPEndpointInfo struct {
		Address          string
		LastConnected    int64
		LastDisconnected int64
		ReceivedPackets  uint
	}
	pcapOverIPEndpoint struct {
		PcapOverIPEndpointInfo
		cancel func()
	}
	pcapOverIPPacket struct {
		linkType layers.LinkType
		data     []byte
		ci       gopacket.CaptureInfo
	}
	pcapOverIPCmd byte

	listener struct {
		close  chan struct{}
		active int
	}

	tag struct {
		query.TagDetails
		definition   string
		features     query.FeatureSet
		color        string
		converters   []*converters.CachedConverter
		referencedBy map[string]struct{}
	}
	TagInfo struct {
		Name           string
		Definition     string
		Color          string
		MatchingCount  uint
		UncertainCount uint
		Referenced     bool
		Converters     []string
	}
	Manager struct {
		StateDir     string
		PcapDir      string
		IndexDir     string
		SnapshotDir  string
		ConverterDir string
		WatchDir     string

		jobs                chan func()
		mergeJobRunning     bool
		taggingJobRunning   bool
		converterJobRunning bool
		importJobs          []string

		builder             *builder.Builder
		indexes             []*index.Reader
		nStreamRecords      int
		nPacketRecords      int
		nextStreamID        uint64
		nUnmergeableIndexes int
		stateFilename       string
		allStreams          bitmask.LongBitmask

		updatedStreamsDuringTaggingJob bitmask.LongBitmask
		resetStreamsDuringTaggingJob   bitmask.LongBitmask
		addedStreamsDuringTaggingJob   bitmask.LongBitmask

		streamsToConvert         map[string]*bitmask.LongBitmask
		pcapProcessorWebhookUrls []string
		pcapOverIPEndpoints      []*pcapOverIPEndpoint

		pcapOverIPPackets chan pcapOverIPPacket
		pcapOverIPCmd     chan pcapOverIPCmd

		tags       map[string]*tag
		converters map[string]*converters.CachedConverter

		usedIndexes       map[*index.Reader]uint
		convertersWatcher *fsnotify.Watcher
		pcapsWatcher      *fsnotify.Watcher

		listeners map[chan Event]listener

		config Config
	}

	Statistics struct {
		ImportJobCount      int
		IndexCount          int
		IndexLockCount      uint
		PcapCount           int
		StreamCount         int
		PacketCount         int
		StreamRecordCount   int
		PacketRecordCount   int
		MergeJobRunning     bool
		TaggingJobRunning   bool
		ConverterJobRunning bool
	}

	Config struct {
		AutoInsertLimitToQuery bool
	}

	indexReleaser []*index.Reader

	// TODO: Maybe save md5 of converters to detect changes
	stateFile struct {
		Saved time.Time
		Tags  []struct {
			Name       string
			Definition string
			Matches    []uint64
			Color      string
			Converters []string
		}
		Pcaps                    []*pcapmetadata.PcapInfo
		PcapProcessorWebhookUrls []string
		PcapOverIPEndpoints      []string
		Config                   Config
	}

	updateTagOperationInfo struct {
		markTagAddStreams, markTagDelStreams []uint64
		color, name                          string
		query                                *string
		setConverterNames                    []string
		convertersUpdated                    bool
	}
	UpdateTagOperation func(*updateTagOperationInfo)

	View struct {
		mgr *Manager

		indexes  []*index.Reader
		releaser indexReleaser

		tagDetails    map[string]query.TagDetails
		tagConverters map[string][]string
		converters    map[string]index.ConverterAccess
	}

	StreamContext struct {
		s *index.Stream
		v *View
	}

	streamsOptions struct {
		prefetchTags       []string
		defaultLimit, page uint
		prefetchAllTags    bool
	}
	StreamsOption func(*streamsOptions)
)

func New(pcapDir, indexDir, snapshotDir, stateDir, converterDir, watchDir string) (*Manager, error) {
	ctx := context.Background()
	mgr := Manager{
		PcapDir:      pcapDir,
		IndexDir:     indexDir,
		SnapshotDir:  snapshotDir,
		StateDir:     stateDir,
		ConverterDir: converterDir,
		WatchDir:     watchDir,

		usedIndexes:      make(map[*index.Reader]uint),
		tags:             make(map[string]*tag),
		converters:       make(map[string]*converters.CachedConverter),
		streamsToConvert: make(map[string]*bitmask.LongBitmask),
		jobs:             make(chan func()),
		listeners:        make(map[chan Event]listener),

		config: Config{AutoInsertLimitToQuery: false},
	}

	convertersWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher for converters: %w", err)
	}
	mgr.convertersWatcher = convertersWatcher
	mgr.startMonitoringConverters(convertersWatcher)

	if watchDir != "" {
		pcapsWatcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, fmt.Errorf("failed to create fsnotify watcher for pcaps: %w", err)
		}
		mgr.pcapsWatcher = pcapsWatcher
		mgr.startMonitoringPcaps(pcapsWatcher)
	}

	// Lookup all available converter binaries
	entries, err := os.ReadDir(mgr.ConverterDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read converter directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := mgr.addConverter(filepath.Join(mgr.ConverterDir, entry.Name())); err != nil {
			return nil, fmt.Errorf("failed to add converter %q: %w", entry.Name(), err)
		}
	}

	tools.AssertFolderRWXPermissions("pcap_dir", pcapDir)
	tools.AssertFolderRWXPermissions("index_dir", indexDir)
	tools.AssertFolderRWXPermissions("snapshot_dir", snapshotDir)
	tools.AssertFolderRWXPermissions("state_dir", stateDir)

	// read all existing indexes and load them
	indexFileNames, err := tools.ListFiles(indexDir, "idx")
	if err != nil {
		return nil, err
	}
	for _, fn := range indexFileNames {
		idx, err := index.NewReader(fn)
		if err != nil {
			log.Printf("Unable to load index %q: %v", fn, err)
			continue
		}
		mgr.indexes = append(mgr.indexes, idx)
		mgr.nStreamRecords += idx.StreamCount()
		mgr.nPacketRecords += idx.PacketCount()
		if next := idx.MaxStreamID() + 1; mgr.nextStreamID < next {
			mgr.nextStreamID = next
		}
	}
	mgr.lock(mgr.indexes)

	stateFilenames, err := tools.ListFiles(stateDir, "state.json")
	if err != nil {
		return nil, err
	}
	stateTimestamp := time.Time{}
	cachedKnownPcapData := []*pcapmetadata.PcapInfo(nil)
	if mgr.nextStreamID != 0 {
		mgr.allStreams.Set(uint(mgr.nextStreamID - 1))
		for i := uint64(0); i != mgr.nextStreamID; i++ {
			mgr.allStreams.Set(uint(i))
		}
	}
	var pcapOverIPEndpoints map[string]struct{}
nextStateFile:
	for _, fn := range stateFilenames {
		f, err := os.Open(fn)
		if err != nil {
			log.Printf("Unable to load state file %q: %v", fn, err)
			continue
		}
		s := stateFile{}
		if err := json.NewDecoder(f).Decode(&s); err != nil {
			log.Printf("Unable to parse state file %q: %v", fn, err)
			continue
		}
		if s.Saved.Before(stateTimestamp) {
			continue
		}
		newTags := make(map[string]*tag, len(s.Tags))
		for _, t := range s.Tags {
			q, err := query.Parse(t.Definition)
			if err != nil {
				log.Printf("Invalid tag %q in statefile %q: %v", t.Name, fn, err)
				continue nextStateFile
			}
			if _, ok := newTags[t.Name]; ok {
				log.Printf("Invalid tag %q in statefile %q: duplicate name", t.Name, fn)
				continue nextStateFile
			}
			matches := bitmask.WrapAsLongBitmask(t.Matches)
			matches.Shrink()
			nt := &tag{
				TagDetails: query.TagDetails{
					Matches:    matches,
					Uncertain:  mgr.allStreams,
					Conditions: q.Conditions,
				},
				definition:   t.Definition,
				features:     q.Conditions.Features(),
				color:        t.Color,
				referencedBy: make(map[string]struct{}),
			}
			if strings.HasPrefix(t.Name, "mark/") || strings.HasPrefix(t.Name, "generated/") {
				ids, ok := q.Conditions.StreamIDs(mgr.nextStreamID)
				if !ok {
					log.Printf("Invalid tag %q in statefile %q: 'mark' or 'generated' tag is malformed", t.Name, fn)
					continue nextStateFile
				}
				nt.Matches = ids
				nt.Uncertain = bitmask.LongBitmask{}
			}
			for _, converterName := range t.Converters {
				converter, ok := mgr.converters[converterName]
				if !ok {
					// TODO: just remove the cache file if any?
					log.Printf("Invalid tag %q in statefile %q: references non-existing converter %q", t.Name, fn, converterName)
					continue
				}
				if err := mgr.attachConverterToTag(nt, t.Name, converter); err != nil {
					log.Printf("Invalid tag %q in statefile %q: Failed to attach converter %q: %v", t.Name, fn, converterName, err)
				}
			}
			newTags[t.Name] = nt
		}
		cyclingTags := map[string]struct{}{}
		for n, t := range newTags {
			for _, tn := range t.referencedTags() {
				if n == tn {
					log.Printf("Invalid tag %q in statefile %q: references itself", n, fn)
					continue nextStateFile
				}
				if _, ok := newTags[tn]; !ok {
					log.Printf("Invalid tag %q in statefile %q: references non-existing tag %q", n, fn, tn)
					continue nextStateFile
				}
				newTags[tn].referencedBy[n] = struct{}{}
			}
			cyclingTags[n] = struct{}{}
		}
	checkCyclingTags:
		for {
		nextCyclingTag:
			for n := range cyclingTags {
				for _, rt := range newTags[n].referencedTags() {
					if _, ok := cyclingTags[rt]; ok {
						continue nextCyclingTag
					}
				}
				delete(cyclingTags, n)
				continue checkCyclingTags
			}
			for n := range cyclingTags {
				log.Printf("Invalid tag %q in statefile %q: contains cycle", n, fn)
				continue nextStateFile
			}
			break
		}
		pcapOverIPEndpointsTemp := map[string]struct{}{}
		for _, v := range s.PcapOverIPEndpoints {
			_, _, err := net.SplitHostPort(v)
			if err != nil {
				log.Printf("Invalid pcap-over-ip host %q in statefile %q: %v", v, fn, err)
				continue nextStateFile
			}
			if _, ok := pcapOverIPEndpointsTemp[v]; ok {
				log.Printf("Invalid pcap-over-ip host %q in statefile %q: duplicate", v, fn)
				continue nextStateFile
			}
			pcapOverIPEndpointsTemp[v] = struct{}{}
		}
		mgr.tags = newTags
		mgr.pcapProcessorWebhookUrls = s.PcapProcessorWebhookUrls
		mgr.stateFilename = fn
		mgr.config = s.Config
		pcapOverIPEndpoints = pcapOverIPEndpointsTemp
		stateTimestamp = s.Saved
		cachedKnownPcapData = s.Pcaps
	}

	mgr.builder, err = builder.New(pcapDir, indexDir, snapshotDir, cachedKnownPcapData)
	if err != nil {
		return nil, err
	}
	if len(mgr.builder.KnownPcaps()) != len(cachedKnownPcapData) {
		if err := mgr.saveState(); err != nil {
			return nil, fmt.Errorf("unable to save state: %w", err)
		}
	}
	mgr.pcapOverIPPackets = make(chan pcapOverIPPacket, 100)
	mgr.pcapOverIPCmd = make(chan pcapOverIPCmd, 1)

	go func() {
		for f := range mgr.jobs {
			f()
		}
	}()
	mgr.jobs <- func() {
		go mgr.pcapOverIPPacketHandler()
		mgr.startTaggingJobIfNeeded()
		mgr.startConverterJobIfNeeded()
		mgr.startMergeJobIfNeeded()
		for a := range pcapOverIPEndpoints {
			mgr.pcapOverIPEndpoints = append(mgr.pcapOverIPEndpoints, mgr.newPcapOverIPEndpoint(ctx, a))
		}
	}
	return &mgr, nil
}

func (t tag) referencedTags() []string {
	m := map[string]struct{}{}
	for _, i := range [2][]string{t.features.MainTags, t.features.SubQueryTags} {
		for _, v := range i {
			m[v] = struct{}{}
		}
	}
	return slices.AppendSeq(make([]string, 0, len(m)), maps.Keys(m))
}

func (t tag) converterNames() []string {
	converterNames := make([]string, len(t.converters))
	for i, converter := range t.converters {
		converterNames[i] = converter.Name()
	}
	return converterNames
}

func (mgr *Manager) Close() {
	if mgr.convertersWatcher != nil {
		if err := mgr.convertersWatcher.Close(); err != nil {
			log.Printf("Failed to close converters watcher: %v", err)
		}
	}
	if mgr.pcapsWatcher != nil {
		if err := mgr.pcapsWatcher.Close(); err != nil {
			log.Printf("Failed to close pcaps watcher: %v", err)
		}
	}
	c := make(chan struct{})
	mgr.jobs <- func() {
		for _, converter := range mgr.converters {
			if err := converter.Close(); err != nil {
				log.Printf("Failed to close converter %q: %v", converter.Name(), err)
			}
		}
		for ch, l := range mgr.listeners {
			if l.active == 0 {
				delete(mgr.listeners, ch)
				close(ch)
			}
			close(l.close)
		}
		for _, e := range mgr.pcapOverIPEndpoints {
			e.cancel()
		}
		mgr.pcapOverIPCmd <- pcapOverIPCmdClose
		close(c)
	}
	<-c
}

func (mgr *Manager) saveState() error {
	j := stateFile{
		Saved:                    time.Now(),
		Pcaps:                    mgr.builder.KnownPcaps(),
		PcapProcessorWebhookUrls: mgr.pcapProcessorWebhookUrls,
		PcapOverIPEndpoints:      make([]string, 0, len(mgr.pcapOverIPEndpoints)),
		Config:                   mgr.config,
	}
	for _, e := range mgr.pcapOverIPEndpoints {
		j.PcapOverIPEndpoints = append(j.PcapOverIPEndpoints, e.Address)
	}
	for n, t := range mgr.tags {
		j.Tags = append(j.Tags, struct {
			Name       string
			Definition string
			Matches    []uint64
			Color      string
			Converters []string
		}{
			Name:       n,
			Definition: t.definition,
			Matches:    t.Matches.Mask(),
			Color:      t.color,
			Converters: t.converterNames(),
		})
	}
	fn := tools.MakeFilename(mgr.StateDir, "state.json")
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(&j); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if mgr.stateFilename != "" {
		if err := os.Remove(mgr.stateFilename); err != nil {
			log.Printf("Unable to delete old statefile %q: %v", mgr.stateFilename, err)
		}
	}
	mgr.stateFilename = fn
	return nil
}

func (mgr *Manager) inheritTagUncertainty() {
	resolvedTags := map[string]struct{}{}
	for len(resolvedTags) != len(mgr.tags) {
	outer:
		for tn, ti := range mgr.tags {
			if _, ok := resolvedTags[tn]; ok {
				continue
			}
			for _, rtn := range ti.referencedTags() {
				if _, ok := resolvedTags[rtn]; !ok {
					continue outer
				}
			}
			resolvedTags[tn] = struct{}{}
			if len(ti.features.MainTags) == 0 && len(ti.features.SubQueryTags) == 0 {
				continue
			}
			fullyInvalidated := false
			for _, rtn := range ti.features.SubQueryTags {
				if !mgr.tags[rtn].Uncertain.IsZero() {
					//TODO: is a matching stream really uncertain?
					ti.Uncertain = mgr.allStreams
					fullyInvalidated = true
					break
				}
			}
			if !fullyInvalidated {
				ti.Uncertain = ti.Uncertain.Copy()
				for _, rtn := range ti.features.MainTags {
					ti.Uncertain.Or(mgr.tags[rtn].Uncertain)
				}
			}
			mgr.tags[tn] = ti
		}
	}
}

func (mgr *Manager) invalidateTags(updatedStreams, resetStreams, addedStreams bitmask.LongBitmask) {
	for tn, ti := range mgr.tags {
		tin := *ti
		if ti.features.SubQueryFeatures != 0 {
			//TODO: is a matching stream really uncertain?
			tin.Uncertain = mgr.allStreams
		} else if ti.features.MainFeatures&^query.FeatureFilterID == 0 {
			continue
		} else {
			tin.Uncertain = ti.Uncertain.Copy()
			tin.Uncertain.Or(addedStreams)
			tin.Uncertain.Or(resetStreams)
			if ti.features.MainFeatures&(query.FeatureFilterData|query.FeatureFilterTimeAbsolute|query.FeatureFilterTimeRelative) != 0 {
				tin.Uncertain.Or(updatedStreams)
			}
		}
		mgr.tags[tn] = &tin
	}
	mgr.inheritTagUncertainty()
}

func (mgr *Manager) importPcapJob(filenames []string, nextStreamID uint64, existingIndexes []*index.Reader, existingIndexesReleaser indexReleaser) {
	processedFiles, usedNewStreamIDs, createdIndexes, updatedStreams, resetStreams, addedStreams, err := mgr.builder.FromPcap(mgr.PcapDir, filenames, existingIndexes)
	if err != nil {
		log.Printf("importPcapJob(%q) failed: %s", filenames, err)
	}
	allStreams := bitmask.LongBitmask{}
	nextStreamID += usedNewStreamIDs
	if nextStreamID != 0 {
		allStreams.Set(uint(nextStreamID - 1))
		for i := uint64(0); i < nextStreamID; i++ {
			allStreams.Set(uint(i))
		}
	}
	newStreamCount := 0
	newPacketCount := 0
	for _, idx := range createdIndexes {
		newStreamCount += idx.StreamCount()
		newPacketCount += idx.PacketCount()
	}
	mgr.jobs <- func() {
		mgr.allStreams = allStreams
		existingIndexesReleaser.release(mgr)
		// add new indexes if some were created
		if len(createdIndexes) > 0 {
			mgr.indexes = append(mgr.indexes, createdIndexes...)
			mgr.nStreamRecords += newStreamCount
			mgr.nPacketRecords += newPacketCount
			mgr.nextStreamID = nextStreamID
			mgr.lock(createdIndexes)
			mgr.updatedStreamsDuringTaggingJob.Or(*updatedStreams)
			mgr.resetStreamsDuringTaggingJob.Or(*resetStreams)
			mgr.addedStreamsDuringTaggingJob.Or(*addedStreams)
			mgr.invalidateTags(*updatedStreams, *resetStreams, *addedStreams)
			mgr.invalidateConverters(updatedStreams)
		}
		// remove finished job from queue
		mgr.importJobs = mgr.importJobs[processedFiles:]
		// start new import job if there are more queued
		if len(mgr.importJobs) >= 1 {
			idxs, rel := mgr.getIndexesCopy(0)
			go mgr.importPcapJob(mgr.importJobs[:], mgr.nextStreamID, idxs, rel)
		} else {
			mgr.pcapOverIPCmd <- pcapOverIPCmdFlush
		}
		mgr.startTaggingJobIfNeeded()
		mgr.startConverterJobIfNeeded()
		mgr.startMergeJobIfNeeded()
		if err := mgr.saveState(); err != nil {
			log.Printf("importPcapJob(%q) failed to save state file: %s", filenames, err)
		}
		mgr.event(Event{
			Type: "pcapProcessed",
			PcapStats: &PcapStatistics{
				PcapCount:         len(mgr.builder.KnownPcaps()),
				ImportJobCount:    len(mgr.importJobs),
				StreamCount:       int(mgr.nextStreamID),
				PacketCount:       int(mgr.builder.PacketCount()),
				IndexCount:        len(mgr.indexes),
				StreamRecordCount: mgr.nStreamRecords,
				PacketRecordCount: mgr.nPacketRecords,
			},
		})
		mgr.triggerPcapProcessedWebhooks(filenames[:processedFiles])
	}
}

func (mgr *Manager) startMergeJobIfNeeded() {
	if mgr.mergeJobRunning || mgr.taggingJobRunning || mgr.converterJobRunning {
		return
	}
	// only merge if all tags are on the newest version, prioritize updating tags
	for _, t := range mgr.tags {
		if !t.Uncertain.IsZero() {
			return
		}
	}
	nStreams := mgr.nStreamRecords
	for i, idx := range mgr.indexes {
		c := idx.StreamCount()
		nStreams -= c
		if i >= mgr.nUnmergeableIndexes && c < nStreams {
			mgr.mergeJobRunning = true
			indexes, indexesReleaser := mgr.getIndexesCopy(i)
			go mgr.mergeIndexesJob(i, indexes, indexesReleaser)
			return
		}
	}
}

func (mgr *Manager) startTaggingJobIfNeeded() {
	if mgr.taggingJobRunning {
		return
	}
outer:
	for n, t := range mgr.tags {
		if t.Uncertain.IsZero() {
			continue
		}
		for _, tn := range t.referencedTags() {
			if !mgr.tags[tn].Uncertain.IsZero() {
				continue outer
			}
		}
		tagDetails := make(map[string]query.TagDetails)
		for _, tn := range t.referencedTags() {
			tagDetails[tn] = mgr.tags[tn].TagDetails
		}
		mgr.updatedStreamsDuringTaggingJob = bitmask.LongBitmask{}
		mgr.resetStreamsDuringTaggingJob = bitmask.LongBitmask{}
		mgr.addedStreamsDuringTaggingJob = bitmask.LongBitmask{}
		mgr.taggingJobRunning = true
		indexes, releaser := mgr.getIndexesCopy(0)
		converters := make(map[string]index.ConverterAccess)
		for converterName, converter := range mgr.converters {
			converters[converterName] = converter
		}
		go mgr.updateTagJob(n, *t, tagDetails, converters, indexes, releaser)
		return
	}
}

func (mgr *Manager) mergeIndexesJob(offset int, indexes []*index.Reader, releaser indexReleaser) {
	mergedIndexes, err := index.Merge(mgr.IndexDir, indexes)
	if err != nil {
		indexFilenames := []string{}
		for _, i := range indexes {
			indexFilenames = append(indexFilenames, i.Filename())
		}
		log.Printf("mergeIndexesJob(%d, [%q]) failed: %s", offset, indexFilenames, err)
	}
	streamsDiff, packetsDiff := 0, 0
	for _, idx := range mergedIndexes {
		streamsDiff += idx.StreamCount()
		packetsDiff += idx.PacketCount()
	}
	for _, idx := range indexes {
		streamsDiff -= idx.StreamCount()
		packetsDiff -= idx.PacketCount()
	}
	mgr.jobs <- func() {
		// replace old indexes if successfully created
		if len(mergedIndexes) == 0 || err != nil {
			mgr.nUnmergeableIndexes++
		} else {
			rel := indexReleaser(mgr.indexes[offset : offset+len(indexes)])
			rel.release(mgr)
			mgr.lock(mergedIndexes)
			mgr.indexes = append(mgr.indexes[:offset], append(mergedIndexes, mgr.indexes[offset+len(indexes):]...)...)
			mgr.nUnmergeableIndexes += len(mergedIndexes) - 1
			mgr.nStreamRecords += streamsDiff
			mgr.nPacketRecords += packetsDiff
		}
		mgr.mergeJobRunning = false
		mgr.startMergeJobIfNeeded()
		releaser.release(mgr)
		mgr.event(Event{
			Type: "indexesMerged",
			PcapStats: &PcapStatistics{
				PcapCount:         len(mgr.builder.KnownPcaps()),
				ImportJobCount:    len(mgr.importJobs),
				StreamCount:       int(mgr.nextStreamID),
				PacketCount:       int(mgr.builder.PacketCount()),
				IndexCount:        len(mgr.indexes),
				StreamRecordCount: mgr.nStreamRecords,
				PacketRecordCount: mgr.nPacketRecords,
			},
		})
	}
}

func (mgr *Manager) updateTagJob(name string, t tag, tagDetails map[string]query.TagDetails, converters map[string]index.ConverterAccess, indexes []*index.Reader, releaser indexReleaser) {
	err := func() error {
		q, err := query.Parse(t.definition)
		if err != nil {
			return err
		}
		streams, _, _, err := index.SearchStreams(context.Background(), indexes, &t.Uncertain, q.ReferenceTime, q.Conditions, nil, []query.Sorting{{Key: query.SortingKeyID, Dir: query.SortingDirAscending}}, 0, 0, tagDetails, converters, false)
		if err != nil {
			return err
		}
		t.Matches = t.Matches.Copy()
		t.Matches.Sub(t.Uncertain)
		for _, s := range streams {
			t.Matches.Set(uint(s.ID()))
		}
		return nil
	}()
	if err != nil {
		log.Printf("updateTagJob failed: %q", err)
		t.Matches = bitmask.LongBitmask{}
	}
	t.Uncertain = bitmask.LongBitmask{}
	mgr.jobs <- func() {
		// don't touch the tag if it was modified
		if ot, ok := mgr.tags[name]; ok && ot.definition == t.definition {
			t.color = ot.color
			t.converters = ot.converters
			t.referencedBy = ot.referencedBy
			for _, converter := range t.converters {
				mgr.streamsToConvert[converter.Name()].Or(t.Matches)
			}
			mgr.tags[name] = &t
			if !(mgr.updatedStreamsDuringTaggingJob.IsZero() && mgr.resetStreamsDuringTaggingJob.IsZero() && mgr.addedStreamsDuringTaggingJob.IsZero()) {
				mgr.invalidateTags(mgr.updatedStreamsDuringTaggingJob, mgr.resetStreamsDuringTaggingJob, mgr.addedStreamsDuringTaggingJob)
			}
			if err := mgr.saveState(); err != nil {
				log.Printf("updateTagJob failed, unable to save state: %q", err)
			}
		}
		mgr.taggingJobRunning = false
		mgr.startTaggingJobIfNeeded()
		mgr.startConverterJobIfNeeded()
		mgr.startMergeJobIfNeeded()
		releaser.release(mgr)
		mgr.event(Event{
			Type: "tagEvaluated",
			Tag:  makeTagInfo(name, &t),
		})
	}
}

func (mgr *Manager) ImportPcaps(filenames []string) {
	if len(filenames) == 0 {
		return
	}
	mgr.jobs <- func() {
		//add job to be processed by importer goroutine
		mgr.importJobs = append(mgr.importJobs, filenames...)
		//start import job when none running
		if len(mgr.importJobs) == len(filenames) {
			indexes, releaser := mgr.getIndexesCopy(0)
			go mgr.importPcapJob(mgr.importJobs[:len(filenames)], mgr.nextStreamID, indexes, releaser)
		}
		mgr.event(Event{
			Type: "pcapArrived",
		})
	}
}

func (mgr *Manager) getIndexesCopy(start int) ([]*index.Reader, indexReleaser) {
	indexes := append([]*index.Reader(nil), mgr.indexes[start:]...)
	return indexes, mgr.lock(indexes)
}

func (mgr *Manager) SetConfig(config Config) error {
	c := make(chan error)
	mgr.jobs <- func() {
		mgr.config = config

		mgr.event(Event{
			Type:   "configUpdated",
			Config: &config,
		})
		c <- mgr.saveState()
		close(c)
	}
	return <-c
}

func (mgr *Manager) Config() Config {
	c := make(chan Config)
	mgr.jobs <- func() {
		c <- mgr.config
		close(c)
	}
	return <-c
}

func (mgr *Manager) Status() Statistics {
	c := make(chan Statistics)
	mgr.jobs <- func() {
		locks := uint(0)
		for _, n := range mgr.usedIndexes {
			locks += n
		}
		c <- Statistics{
			IndexCount:          len(mgr.indexes),
			IndexLockCount:      locks,
			PcapCount:           len(mgr.builder.KnownPcaps()),
			ImportJobCount:      len(mgr.importJobs),
			StreamRecordCount:   mgr.nStreamRecords,
			PacketRecordCount:   mgr.nPacketRecords,
			StreamCount:         int(mgr.nextStreamID),
			PacketCount:         int(mgr.builder.PacketCount()),
			MergeJobRunning:     mgr.mergeJobRunning,
			TaggingJobRunning:   mgr.taggingJobRunning,
			ConverterJobRunning: mgr.converterJobRunning,
		}
		close(c)
	}
	res := <-c
	return res
}

func (mgr *Manager) KnownPcaps() []pcapmetadata.PcapInfo {
	c := make(chan []pcapmetadata.PcapInfo)
	mgr.jobs <- func() {
		r := []pcapmetadata.PcapInfo{}
		for _, p := range mgr.builder.KnownPcaps() {
			r = append(r, *p)
		}
		c <- r
		close(c)
	}
	res := <-c
	return res
}

func makeTagInfo(name string, t *tag) *TagInfo {
	m := t.Matches.Copy()
	m.Sub(t.Uncertain)
	definition := t.definition
	if _, _, mark := parseTagName(name); mark {
		definition = "..."
	}
	return &TagInfo{
		Name:           name,
		Definition:     definition,
		Color:          t.color,
		MatchingCount:  uint(m.OnesCount()),
		UncertainCount: uint(t.Uncertain.OnesCount()),
		Referenced:     len(t.referencedBy) != 0,
		Converters:     t.converterNames(),
	}
}

func (mgr *Manager) ListTags() []TagInfo {
	c := make(chan []TagInfo)
	mgr.jobs <- func() {
		res := []TagInfo{}
		for name, t := range mgr.tags {
			res = append(res, *makeTagInfo(name, t))
		}
		sort.Slice(res, func(i, j int) bool {
			return res[i].Name < res[j].Name
		})
		c <- res
		close(c)
	}
	return <-c
}

func parseTagName(fullName string) (typ, name string, isMark bool) {
	ok := false
	typ, name, ok = strings.Cut(fullName, "/")
	if !ok {
		return "", "", false
	}
	isMark = typ == "mark" || typ == "generated"
	if typ != "tag" && typ != "service" && !isMark {
		return "", "", false
	}
	return
}

func (mgr *Manager) AddTag(name, color, queryString string) error {
	typ, sub, isMark := parseTagName(name)
	if typ == "" {
		return errors.New("invalid tag name (need a 'tag/', 'service/', 'mark/' or 'generated/' prefix)")
	}
	if sub == "" {
		return errors.New("invalid tag name (prefix only not allowed)")
	}
	q, err := query.Parse(queryString)
	if err != nil {
		return err
	}
	features := q.Conditions.Features()
	if (features.MainFeatures|features.SubQueryFeatures)&query.FeatureFilterTimeRelative != 0 {
		return errors.New("relative times not yet supported in tags")
	}
	if q.Grouping != nil {
		return errors.New("grouping not allowed in tags")
	}
	nt := &tag{
		TagDetails: query.TagDetails{
			Conditions: q.Conditions,
		},
		definition:   queryString,
		features:     features,
		color:        color,
		referencedBy: make(map[string]struct{}),
	}
	for _, tn := range nt.referencedTags() {
		if tn == name {
			return errors.New("self reference not allowed in tags")
		}
	}
	if isMark {
		if _, ok := q.Conditions.StreamIDs(0); !ok {
			return errors.New("tags of type `mark` have to only contain an `id` filter")
		}
	}
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			if _, ok := mgr.tags[name]; ok {
				return errors.New("tag already exists")
			}
			// check if all referenced tags exist
			for _, t := range nt.referencedTags() {
				if _, ok := mgr.tags[t]; !ok {
					return fmt.Errorf("unknown referenced tag %q", t)
				}
			}
			mgr.tags[name] = nt
			if isMark {
				nt.Matches, _ = q.Conditions.StreamIDs(mgr.nextStreamID)
			} else {
				nt.Uncertain = mgr.allStreams
				mgr.startTaggingJobIfNeeded()
			}
			mgr.event(Event{
				Type: "tagAdded",
				Tag:  makeTagInfo(name, nt),
			})
			for _, tn := range nt.referencedTags() {
				t := mgr.tags[tn]
				t.referencedBy[name] = struct{}{}
				if len(t.referencedBy) == 1 {
					mgr.event(Event{
						Type: "tagUpdated",
						Tag:  makeTagInfo(tn, t),
					})
				}
			}
			return mgr.saveState()
		}()
		c <- err
		close(c)
	}
	return <-c
}

func (mgr *Manager) DelTag(name string) error {
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			tag, ok := mgr.tags[name]
			if !ok {
				return fmt.Errorf("unknown tag %q", name)
			}
			if len(tag.referencedBy) != 0 {
				return fmt.Errorf("tag %q still references the tag to be deleted", slices.AppendSeq(make([]string, 0, len(tag.referencedBy)), maps.Keys(tag.referencedBy))[0])
			}
			// remove converter results of attached converters from cache
			if len(tag.converters) > 0 {
				for _, converter := range tag.converters {
					if err := mgr.detachConverterFromTag(tag, name, converter); err != nil {
						return err
					}
				}
			}
			delete(mgr.tags, name)
			mgr.event(Event{
				Type: "tagDeleted",
				Tag: &TagInfo{
					Name:       name,
					Converters: []string{},
				},
			})
			for _, tn := range tag.referencedTags() {
				t := mgr.tags[tn]
				delete(t.referencedBy, name)
				if len(t.referencedBy) != 0 {
					continue
				}
				mgr.event(Event{
					Type: "tagUpdated",
					Tag:  makeTagInfo(tn, t),
				})
			}
			return mgr.saveState()
		}()
		c <- err
		close(c)
	}
	return <-c
}

func UpdateTagOperationMarkAddStream(streams []uint64) UpdateTagOperation {
	s := make([]uint64, 0, len(streams))
	s = append(s, streams...)
	return func(i *updateTagOperationInfo) {
		i.markTagAddStreams = s
	}
}

func UpdateTagOperationMarkDelStream(streams []uint64) UpdateTagOperation {
	s := make([]uint64, 0, len(streams))
	s = append(s, streams...)
	return func(i *updateTagOperationInfo) {
		i.markTagDelStreams = s
	}
}

func UpdateTagOperationUpdateColor(color string) UpdateTagOperation {
	return func(i *updateTagOperationInfo) {
		i.color = color
	}
}

func UpdateTagOperationUpdateQuery(query string) UpdateTagOperation {
	return func(i *updateTagOperationInfo) {
		i.query = &query
	}
}

func UpdateTagOperationUpdateName(name string) UpdateTagOperation {
	return func(i *updateTagOperationInfo) {
		i.name = name
	}
}

func UpdateTagOperationSetConverter(converterNames []string) UpdateTagOperation {
	return func(i *updateTagOperationInfo) {
		i.setConverterNames = converterNames
		i.convertersUpdated = true
	}
}

func (mgr *Manager) UpdateTag(name string, operation UpdateTagOperation) error {
	info := updateTagOperationInfo{convertersUpdated: false}
	operation(&info)
	maxUsedStreamID := uint64(0)
	if len(info.markTagAddStreams) != 0 || len(info.markTagDelStreams) != 0 {
		if !(strings.HasPrefix(name, "mark/") || strings.HasPrefix(name, "generated/")) {
			return fmt.Errorf("tag %q is not of type 'mark' or 'generated'", name)
		}
		for _, s := range info.markTagAddStreams {
			if maxUsedStreamID <= s {
				maxUsedStreamID = s + 1
			}
		}
		for _, s := range info.markTagDelStreams {
			if maxUsedStreamID <= s {
				maxUsedStreamID = s + 1
			}
		}
		if maxUsedStreamID == 0 {
			// no operation
			return nil
		}
		maxUsedStreamID--
	}
	var newTag *tag
	if info.query != nil {
		q, err := query.Parse(*info.query)
		if err != nil {
			return err
		}
		features := q.Conditions.Features()
		if (features.MainFeatures|features.SubQueryFeatures)&query.FeatureFilterTimeRelative != 0 {
			return errors.New("relative times not yet supported in tags")
		}
		if q.Grouping != nil {
			return errors.New("grouping not allowed in tags")
		}
		newTag = &tag{
			TagDetails: query.TagDetails{
				Conditions: q.Conditions,
			},
			definition: *info.query,
			features:   features,
		}
		for _, tn := range newTag.referencedTags() {
			if tn == name {
				return errors.New("self reference not allowed in tags")
			}
		}
		if strings.HasPrefix(name, "mark/") {
			if _, ok := q.Conditions.StreamIDs(0); !ok {
				return errors.New("tags of type `mark` have to only contain an `id` filter")
			}
		}
	}
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			tag, ok := mgr.tags[name]
			if !ok {
				return fmt.Errorf("unknown tag %q", name)
			}
			if info.color != "" {
				tag.color = info.color
			}
			if newTag != nil {
				newTag.color = tag.color
				newTag.converters = tag.converters
				newTag.referencedBy = tag.referencedBy
				newTag.Uncertain = mgr.allStreams
				onlyBefore := map[string]struct{}{}
				onlyAfter := map[string]struct{}{}
				for _, rtn := range tag.referencedTags() {
					onlyBefore[rtn] = struct{}{}
				}
				for _, rtn := range newTag.referencedTags() {
					if _, ok := onlyBefore[rtn]; ok {
						delete(onlyBefore, rtn)
					} else {
						onlyAfter[rtn] = struct{}{}
					}
				}
				for rtn := range onlyBefore {
					rt := mgr.tags[rtn]
					delete(rt.referencedBy, name)
					if len(rt.referencedBy) == 0 {
						mgr.event(Event{
							Type: "tagUpdated",
							Tag:  makeTagInfo(rtn, rt),
						})
					}
				}
				for rtn := range onlyAfter {
					rt := mgr.tags[rtn]
					rt.referencedBy[name] = struct{}{}
					if len(rt.referencedBy) == 1 {
						mgr.event(Event{
							Type: "tagUpdated",
							Tag:  makeTagInfo(rtn, rt),
						})
					}
				}
				tag = newTag
				mgr.tags[name] = tag
				mgr.inheritTagUncertainty()
				mgr.startTaggingJobIfNeeded()
				mgr.startConverterJobIfNeeded()
			}
			if info.convertersUpdated {
				// detach deselected converters from tag
				for _, converter := range tag.converters {
					if slices.Contains(info.setConverterNames, converter.Name()) {
						continue
					}
					if err := mgr.detachConverterFromTag(tag, name, converter); err != nil {
						return fmt.Errorf("failed to detach converter %q from tag %q: %w", converter.Name(), name, err)
					}
				}
				// attach new converters to tag
				converterNames := tag.converterNames()
				for _, converterName := range info.setConverterNames {
					if slices.Contains(converterNames, converterName) {
						continue
					}
					if converter, ok := mgr.converters[converterName]; !ok {
						return fmt.Errorf("unknown converter %q", converterName)
					} else {
						if err := mgr.attachConverterToTag(tag, name, converter); err != nil {
							return fmt.Errorf("failed to attach converter %q to tag %q: %w", converterName, name, err)
						}
					}
				}
				mgr.startConverterJobIfNeeded()
			}
			if maxUsedStreamID != 0 {
				if maxUsedStreamID >= mgr.nextStreamID {
					return fmt.Errorf("unknown stream id %d", maxUsedStreamID)
				}
				newTag := *tag
				newTag.Matches = tag.Matches.Copy()
				newTag.Uncertain = tag.Uncertain.Copy()
				// update mark streamid tag matches without parsing the definition again
				// this is a bit hacky but it is much faster than parsing the definition of long mark tags again
				if len(info.markTagAddStreams) != 0 {
					b := strings.Builder{}
					b.WriteString("id:")
					for _, s := range info.markTagAddStreams {
						if newTag.Matches.IsSet(uint(s)) {
							continue
						}
						newTag.Matches.Set(uint(s))
						newTag.Uncertain.Set(uint(s))
						b.WriteString(fmt.Sprintf("%d,", s))

						for _, converter := range newTag.converters {
							mgr.streamsToConvert[converter.Name()].Set(uint(s))
						}
					}
					if b.Len() != len("id:") {
						markQuery := b.String()
						markQuery = markQuery[:len(markQuery)-1]
						if q, err := query.Parse(markQuery); err == nil {
							newTag.Conditions = newTag.Conditions.Or(q.Conditions)
						}
						if newTag.definition == "id:-1" {
							newTag.definition = markQuery
						} else {
							newTag.definition = fmt.Sprintf("%s,%s", newTag.definition, markQuery[3:])
						}
					}
				}
				if len(info.markTagDelStreams) != 0 {
					for _, s := range info.markTagDelStreams {
						if !newTag.Matches.IsSet(uint(s)) {
							continue
						}
						newTag.Matches.Unset(uint(s))
						newTag.Uncertain.Set(uint(s))
						// TODO: invalidate converter cache for this stream
					}
					b := strings.Builder{}
					b.WriteString("id:")
					for i := uint(0); newTag.Matches.Next(&i); i++ {
						b.WriteString(fmt.Sprintf("%d,", i))
					}
					if b.Len() == len("id:") {
						newTag.definition = "id:-1"
						newTag.Conditions = nil
					} else {
						markQuery := b.String()
						markQuery = markQuery[:len(markQuery)-1]
						if q, err := query.Parse(markQuery); err == nil {
							newTag.Conditions = q.Conditions
							newTag.definition = markQuery
						}
					}
				}
				tag = &newTag
				mgr.tags[name] = tag
				mgr.inheritTagUncertainty()
				mgr.tags[name].Uncertain = bitmask.LongBitmask{}
				mgr.startTaggingJobIfNeeded()
				mgr.startConverterJobIfNeeded()
			}
			if info.name != "" {
				oldTyp, _, _ := parseTagName(name)
				newTyp, newSub, _ := parseTagName(info.name)
				if newTyp != oldTyp {
					return errors.New("invalid tag name (can't change type of tag)")
				}
				if newSub == "" {
					return errors.New("invalid tag name (prefix only not allowed)")
				}
				if _, ok := mgr.tags[info.name]; ok {
					return fmt.Errorf("tag %q already exists", info.name)
				}
				if len(tag.referencedBy) != 0 {
					return fmt.Errorf("tag %q still references the tag to be renamed", slices.AppendSeq(make([]string, 0, len(tag.referencedBy)), maps.Keys(tag.referencedBy))[0])
				}
				delete(mgr.tags, name)
				mgr.tags[info.name] = tag
				for _, rtn := range tag.referencedTags() {
					rt := mgr.tags[rtn]
					delete(rt.referencedBy, name)
					rt.referencedBy[info.name] = struct{}{}
				}

				mgr.event(Event{
					Type: "tagDeleted",
					Tag: &TagInfo{
						Name:       name,
						Converters: []string{},
					},
				})
				mgr.event(Event{
					Type: "tagAdded",
					Tag:  makeTagInfo(info.name, tag),
				})
			} else {
				mgr.event(Event{
					Type: "tagUpdated",
					Tag:  makeTagInfo(name, tag),
				})
			}
			return mgr.saveState()
		}()
		c <- err
		close(c)
	}
	return <-c
}

func (mgr *Manager) lock(indexes []*index.Reader) indexReleaser {
	for _, i := range indexes {
		mgr.usedIndexes[i]++
	}
	return indexReleaser(append([]*index.Reader(nil), indexes...))
}

// release all contained indexes from within the mgr goroutine
func (r *indexReleaser) release(mgr *Manager) {
	for _, i := range *r {
		mgr.usedIndexes[i]--
		if mgr.usedIndexes[i] == 0 {
			delete(mgr.usedIndexes, i)
			i.Close()
			os.Remove(i.Filename())
		}
	}
}

func (mgr *Manager) startConverterJobIfNeeded() {
	if mgr.converterJobRunning {
		return
	}
	activeConverters := []*converters.CachedConverter(nil)
	streamsToConvert := []*bitmask.LongBitmask(nil)

	// TODO: split this into smaller chunks so that we can abort long running jobs
	//       when a converter gets detached from a tag while it is running
	for converterName, converter := range mgr.converters {
		streams := mgr.streamsToConvert[converterName]
		if streams.IsZero() {
			continue
		}
		mgr.streamsToConvert[converterName] = &bitmask.LongBitmask{}
		streamsToConvert = append(streamsToConvert, streams)
		activeConverters = append(activeConverters, converter)
	}
	if len(activeConverters) == 0 {
		return
	}
	indexes, releaser := mgr.getIndexesCopy(0)
	go mgr.convertStreamJob(activeConverters, streamsToConvert, indexes, releaser)
	mgr.converterJobRunning = true
}

func (mgr *Manager) convertStreamJob(allConverters []*converters.CachedConverter, allStreamIDs []*bitmask.LongBitmask, indexes []*index.Reader, releaser indexReleaser) {
	type job struct {
		streamID  uint64
		converter int
	}
	jobs := []job(nil)
	for i, streamIDs := range allStreamIDs {
		for streamID := uint(0); streamIDs.Next(&streamID); streamID++ {
			jobs = append(jobs, job{uint64(streamID), i})
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		a, b := jobs[i], jobs[j]
		if a.streamID != b.streamID {
			return a.streamID < b.streamID
		}
		// "randomize" the order of the converters
		offset := int(a.streamID)
		return (a.converter+offset)%len(allConverters) < b.converter
	})

	freeJobsGlobal := 0
	freeJobs := []int(nil)
	for _, converter := range allConverters {
		maxProcessCount := converter.MaxProcessCount()
		freeJobs = append(freeJobs, maxProcessCount)
		freeJobsGlobal += maxProcessCount
	}
	if numCPUs := runtime.NumCPU(); freeJobsGlobal > numCPUs {
		freeJobsGlobal = numCPUs
	}
	maxJobsGlobal := freeJobsGlobal
	type result struct {
		job job
		err error
	}
	alreadyCached := errors.New("alreadyCached")
	results := make(chan result, freeJobsGlobal)
	failedJobs := make(map[job]struct{})
	for jobIDs := []int(nil); len(jobs) != 0 || freeJobsGlobal != maxJobsGlobal; {
		jobIDs = jobIDs[:0]
		for i, job := range jobs {
			if freeJobs[job.converter] == 0 {
				continue
			}
			jobIDs = append(jobIDs, i)

			freeJobs[job.converter]--
			freeJobsGlobal--
			if freeJobsGlobal == 0 {
				break
			}
		}
		for numDeleted, jobID := range jobIDs {
			jobID -= numDeleted
			job := jobs[jobID]
			jobs = append(jobs[:jobID], jobs[jobID+1:]...)

			// Convert the stream
			go func() {
				converter := allConverters[job.converter]
				if converter.Contains(job.streamID) {
					results <- result{job, alreadyCached}
					return
				}
				for idxIdx := len(indexes) - 1; idxIdx >= 0; idxIdx-- {
					index := indexes[idxIdx]

					// Load the stream from the index
					stream, err := index.StreamByID(job.streamID)
					if err != nil {
						results <- result{job, err}
						return
					}
					// The stream isn't in this index file
					if stream == nil {
						continue
					}
					_, _, _, _, err = converter.Data(stream, false)
					results <- result{job, err}
					return
				}
			}()
		}

		handleResult := func(res result) {
			freeJobs[res.job.converter]++
			freeJobsGlobal++
			switch res.err {
			case nil:
				return
			default:
				log.Printf("Error converting stream %d with converter %q: %v", res.job.streamID, allConverters[res.job.converter].Name(), res.err)
				if _, ok := failedJobs[res.job]; !ok {
					failedJobs[res.job] = struct{}{}
					jobs = append(jobs, res.job)
					return
				}
				log.Printf("Discarding conversion of stream %d with converter %q because it failed twice", res.job.streamID, allConverters[res.job.converter].Name())
				fallthrough
			case alreadyCached:
				allStreamIDs[res.job.converter].Unset(uint(res.job.streamID))
			}
		}
		handleResult(<-results)
	outer:
		for {
			select {
			case res := <-results:
				handleResult(res)
			default:
				break outer
			}
		}
	}

	mgr.jobs <- func() {
		mgr.converterJobRunning = false

		for i, converter := range allConverters {
			// The converter was removed while we were running.
			// Discard the result.
			if _, ok := mgr.converters[converter.Name()]; !ok {
				if err := converter.Reset(); err != nil {
					log.Printf("error while resetting converter %q after discarding results: %v", converter.Name(), err)
				}
				continue
			}

			// Mark the converted streams as uncertain on all tags using a data: filter
			// The tag could match on the converted data now.
			for _, tag := range mgr.tags {
				// TODO: Only tag again if the tag matches converted data
				if tag.features.MainFeatures&query.FeatureFilterData == 0 && tag.features.SubQueryFeatures&query.FeatureFilterData == 0 {
					continue
				}
				tag.Uncertain.Or(*allStreamIDs[i])
			}
			mgr.updatedStreamsDuringTaggingJob.Or(*allStreamIDs[i])
			mgr.event(Event{
				Type:      "converterCompleted",
				Converter: converter.Statistics(),
			})
		}
		mgr.inheritTagUncertainty()
		mgr.startTaggingJobIfNeeded()
		mgr.startConverterJobIfNeeded()
		releaser.release(mgr)
	}
}

func (mgr *Manager) invalidateConverters(updatedStreams *bitmask.LongBitmask) {
	for _, converter := range mgr.converters {
		invalidatedStreams := converter.InvalidateChangedStreams(updatedStreams)
		mgr.streamsToConvert[converter.Name()].Or(invalidatedStreams)
	}
}

func (mgr *Manager) startMonitoringConverters(watcher *fsnotify.Watcher) {
	go func() {
		var (
			// Wait 500ms for new events; each new event resets the timer.
			waitFor = 500 * time.Millisecond

			// Keep track of the timers, as path → timer.
			mu     sync.Mutex
			timers = make(map[string]*time.Timer)
		)
		for {
			select {
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)

			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)

				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					mgr.jobs <- func() {
						if err := mgr.removeConverter(event.Name); err != nil {
							log.Printf("error while removing converter: %v", err)
						}
						name := strings.TrimSuffix(filepath.Base(event.Name), filepath.Ext(event.Name))
						mgr.event(Event{
							Type: "converterDeleted",
							Converter: &converters.Statistics{
								Name:      name,
								Processes: []converters.ProcessStats{},
							},
						})
					}
				}

				if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) && !event.Has(fsnotify.Chmod) {
					continue
				}

				mu.Lock()
				timer, ok := timers[event.Name]
				mu.Unlock()

				// No timer yet, so create one.
				if !ok {
					timer = time.AfterFunc(math.MaxInt64, func() {
						mu.Lock()
						delete(timers, event.Name)
						mu.Unlock()

						mgr.jobs <- func() {
							if event.Has(fsnotify.Create) {
								fileInfo, err := os.Stat(event.Name)
								if err != nil || fileInfo.IsDir() {
									return
								}
								if err := mgr.addConverter(event.Name); err != nil {
									log.Printf("error while adding converter: %v", err)
								}
								name := strings.TrimSuffix(filepath.Base(event.Name), filepath.Ext(event.Name))
								converter := mgr.converters[name]
								mgr.event(Event{
									Type:      "converterAdded",
									Converter: converter.Statistics(),
								})
							}
							if event.Has(fsnotify.Chmod) {
								fileInfo, err := os.Stat(event.Name)
								if err != nil || fileInfo.IsDir() {
									return
								}
								if err := mgr.restartConverterProcess(event.Name); err != nil {
									log.Printf("error while restarting converter: %v", err)
								}
							}
							if event.Has(fsnotify.Write) {
								fileInfo, err := os.Stat(event.Name)
								if err != nil || fileInfo.IsDir() {
									return
								}
								if err := mgr.restartConverterProcess(event.Name); err != nil {
									log.Printf("error while restarting converter: %v", err)
								}
							}
						}
					})
					timer.Stop()

					mu.Lock()
					timers[event.Name] = timer
					mu.Unlock()
				}

				// Reset the timer for this path, so it will start again.
				timer.Reset(waitFor)
			}
		}
	}()

	err := watcher.Add(mgr.ConverterDir)
	if err != nil {
		log.Fatal(fmt.Errorf("error while adding converter dir to watcher %v: %w", mgr.ConverterDir, err))
	}
}

func (mgr *Manager) startMonitoringPcaps(watcher *fsnotify.Watcher) {
	go func() {
		var (
			waitFor = 500 * time.Millisecond
			mu      sync.Mutex
			timers  = make(map[string]*time.Timer)
		)
		for {
			select {
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)

			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)

				if !(event.Has(fsnotify.Create|fsnotify.Write|fsnotify.Chmod) && strings.HasPrefix(filepath.Ext(event.Name), ".pcap")) {
					continue
				}

				mu.Lock()
				timer, ok := timers[event.Name]
				mu.Unlock()

				if !ok {
					timer = time.AfterFunc(math.MaxInt64, func() {
						fileInfo, err := os.Stat(event.Name)
						if err != nil || fileInfo.IsDir() {
							return
						}

						src, err := os.Open(event.Name)
						if err != nil {
							log.Printf("error while opening new pcap: %v", err)
							return
						}
						defer src.Close()

						dstFilename := filepath.Join(mgr.PcapDir, fileInfo.Name())
						dst, err := os.OpenFile(dstFilename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
						if err != nil {
							log.Printf("error while copying new pcap to PcapDir: %v", err)
							return
						}

						if _, err := io.Copy(dst, src); err != nil {
							log.Printf("error while copying new pcap to PcapDir: %v", err)
							if err := dst.Close(); err != nil {
								log.Printf("error while closing empty new pcap: %v", err)
							}
							if err := os.Remove(dstFilename); err != nil {
								log.Printf("error while removing empty new pcap file: %v", err)
							}
							return
						}
						if err := dst.Close(); err != nil {
							log.Printf("error while closing new pcap: %v", err)
							if err := os.Remove(dstFilename); err != nil {
								log.Printf("error while removing new pcap file after failed save: %v", err)
							}
							return
						}
						mgr.ImportPcaps([]string{fileInfo.Name()})

						mu.Lock()
						delete(timers, event.Name)
						mu.Unlock()
					})
					timer.Stop()

					mu.Lock()
					timers[event.Name] = timer
					mu.Unlock()
				}
				timer.Reset(waitFor)
			}
		}
	}()
	err := watcher.Add(mgr.WatchDir)
	if err != nil {
		log.Fatal(fmt.Errorf("error while adding pcaps dir to watcher %v: %w", mgr.WatchDir, err))
	}
}

func (mgr *Manager) addConverter(path string) error {
	// TODO: Do we want to check this now or when we start the converter?
	if !tools.IsFileExecutable(path) {
		return fmt.Errorf("error: converter %s is not executable", path)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, ok := mgr.converters[name]; ok {
		return fmt.Errorf("error: converter %s already exists", name)
	}
	if name == "none" {
		return fmt.Errorf("error: converter %s is reserved", name)
	}
	// Converter names have to be plain ascii so we can use them in the query language easily.
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(name) {
		return fmt.Errorf("error: converter %s has to be alphanumeric", name)
	}

	converter, err := converters.NewCache(name, path, mgr.IndexDir)
	if err != nil {
		return fmt.Errorf("error: failed to create converter %s: %w", name, err)
	}
	mgr.converters[name] = converter
	mgr.streamsToConvert[name] = &bitmask.LongBitmask{}
	return nil
}

func (mgr *Manager) removeConverter(path string) error {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	converter, ok := mgr.converters[name]
	if !ok {
		return fmt.Errorf("error: converter %s does not exist", name)
	}

	// remove converter from all tags
	for tagName, tag := range mgr.tags {
		if err := mgr.detachConverterFromTag(tag, tagName, converter); err != nil {
			return err
		}
	}

	// Stop the process if it is running and delete the cache file.
	if err := converter.Reset(); err != nil {
		return err
	}

	delete(mgr.converters, name)
	delete(mgr.streamsToConvert, name)
	return mgr.saveState()
}

func (mgr *Manager) restartConverterProcess(path string) error {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	converter, ok := mgr.converters[name]
	if !ok {
		if err := mgr.addConverter(path); err != nil {
			return err
		}
		converter = mgr.converters[name]
		mgr.event(Event{
			Type:      "converterAdded",
			Converter: converter.Statistics(),
		})
	}
	// Stop the process if it is running and restart it
	if err := converter.Reset(); err != nil {
		return err
	}

	// run the converter on all streams that match the tags it is attached to again
	for _, tag := range mgr.tags {
		if slices.Contains(tag.converters, converter) {
			mgr.streamsToConvert[name].Or(tag.Matches)
		}
	}
	mgr.startConverterJobIfNeeded()

	mgr.event(Event{
		Type:      "converterRestarted",
		Converter: converter.Statistics(),
	})
	return nil
}

func (mgr *Manager) attachConverterToTag(tag *tag, tagName string, converter *converters.CachedConverter) error {
	// check if converter already exists
	if slices.Contains(tag.converters, converter) {
		return nil
	}
	// assert low complexity of this tag's query
	// cannot attach converter to tag which references other tags or matches on stream data
	// because we don't want to recursively trigger converters
	// TODO: we could allow data queries if they only reference the stream's own plain data
	if tag.features.MainFeatures&query.FeatureFilterData != 0 || tag.features.SubQueryFeatures&query.FeatureFilterData != 0 || len(tag.features.MainTags) > 0 || len(tag.features.SubQueryTags) > 0 {
		return fmt.Errorf("error: cannot attach converter to tag %s because it's query is too complex", tagName)
	}

	tag.converters = append(tag.converters, converter)
	mgr.streamsToConvert[converter.Name()].Or(tag.Matches)
	mgr.event(Event{
		Type: "tagUpdated",
		Tag:  makeTagInfo(tagName, tag),
	})
	return nil
}

func (mgr *Manager) detachConverterFromTag(tag *tag, tagName string, converter *converters.CachedConverter) error {
	for i, c := range tag.converters {
		if c == converter {
			tag.converters = append(tag.converters[:i], tag.converters[i+1:]...)
			break
		}
	}
	mgr.event(Event{
		Type: "tagUpdated",
		Tag:  makeTagInfo(tagName, tag),
	})
	// delete/invalidate converter results for all matching streams now
	// but only if they aren't matches of other tags the converter is attached to.
	matchingStreams := bitmask.LongBitmask{}
	for _, t := range mgr.tags {
		if t == tag {
			continue
		}
		if slices.Contains(t.converters, converter) {
			matchingStreams.Or(t.Matches)
		}
	}

	// only delete results for streams that are not matched by other tags
	onlyThisTag := tag.Matches.Copy()
	onlyThisTag.Sub(matchingStreams)
	mgr.streamsToConvert[converter.Name()].Sub(onlyThisTag)
	// TODO: invalidate all streams in the cache that are only matched by this tag.

	if matchingStreams.IsZero() {
		// no other tags use this converter, delete all results
		if err := converter.Reset(); err != nil {
			return err
		}
	}
	return nil
}

func (mgr *Manager) ResetConverter(converterName string) error {
	c := make(chan error)
	mgr.jobs <- func() {
		c <- mgr.restartConverterProcess(converterName)
		close(c)
	}
	return <-c
}

func (mgr *Manager) ListConverters() []*converters.Statistics {
	c := make(chan []*converters.Statistics)
	mgr.jobs <- func() {
		stats := make([]*converters.Statistics, 0, len(mgr.converters))
		for _, converter := range mgr.converters {
			stats = append(stats, converter.Statistics())
		}
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Name < stats[j].Name
		})
		c <- stats
		close(c)
	}
	return <-c
}

func (mgr *Manager) ConverterStderr(converterName string, pid int) (*converters.ProcessStderr, error) {
	c := make(chan *converters.ProcessStderr)
	mgr.jobs <- func() {
		converter, ok := mgr.converters[converterName]
		if !ok {
			c <- nil
			close(c)
			return
		}
		c <- converter.Stderr(pid)
		close(c)
	}
	stderr := <-c
	if stderr == nil {
		return nil, fmt.Errorf("error: converter %s or process with pid %d does not exist", converterName, pid)
	}
	return stderr, nil
}

func (mgr *Manager) ListPcapProcessorWebhooks() []string {
	c := make(chan []string)
	mgr.jobs <- func() {
		if mgr.pcapProcessorWebhookUrls == nil {
			c <- []string{}
		} else {
			c <- mgr.pcapProcessorWebhookUrls
		}
		close(c)
	}
	return <-c
}

func (mgr *Manager) AddPcapProcessorWebhook(url string) error {
	c := make(chan error)
	mgr.jobs <- func() {
		for _, u := range mgr.pcapProcessorWebhookUrls {
			if u == url {
				c <- fmt.Errorf("error: url %q already exists", url)
				close(c)
				return
			}
		}
		mgr.pcapProcessorWebhookUrls = append(mgr.pcapProcessorWebhookUrls, url)
		mgr.event(Event{
			Type:     "webhooksUpdated",
			Webhooks: &mgr.pcapProcessorWebhookUrls,
		})
		c <- mgr.saveState()
		close(c)
	}
	return <-c
}

func (mgr *Manager) DelPcapProcessorWebhook(url string) error {
	c := make(chan error)
	mgr.jobs <- func() {
		for i, u := range mgr.pcapProcessorWebhookUrls {
			if u == url {
				mgr.pcapProcessorWebhookUrls = append(mgr.pcapProcessorWebhookUrls[:i], mgr.pcapProcessorWebhookUrls[i+1:]...)
				mgr.event(Event{
					Type:     "webhooksUpdated",
					Webhooks: &mgr.pcapProcessorWebhookUrls,
				})
				c <- mgr.saveState()
				close(c)
				return
			}
		}
		c <- fmt.Errorf("error: url %q does not exist", url)
		close(c)
	}
	return <-c
}

func (mgr *Manager) triggerPcapProcessedWebhooks(filenames []string) {
	var absFilenames []string
	for _, filename := range filenames {
		absFilename, err := filepath.Abs(filepath.Join(mgr.PcapDir, filename))
		if err != nil {
			log.Printf("error: pcap webhook failed to get absolute path of %q: %v\n", filename, err)
			continue
		}
		absFilenames = append(absFilenames, absFilename)
	}
	jsonBody, err := json.Marshal(absFilenames)
	if err != nil {
		log.Printf("error: webhook body json encode failed: %v\n", err)
		return
	}
	for _, webhookUrl := range mgr.pcapProcessorWebhookUrls {
		go triggerPcapProcessedWebhook(webhookUrl, jsonBody)
	}
}

func triggerPcapProcessedWebhook(webhookUrl string, jsonBody []byte) {
	err := func() error {
		bodyReader := bytes.NewReader(jsonBody)

		ctx, cncl := context.WithTimeout(context.Background(), pcapProcessorWebhookTimeout)
		defer cncl()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookUrl, bodyReader)
		if err != nil {
			return fmt.Errorf("failed to create webhook request for processed pcap: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to making webhook request for processed pcap: %w", err)
		}

		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("webhook request for processed pcap failed: %q", res.Status)
		}
		return nil
	}()
	if err != nil {
		log.Printf("webhook error: %v\n", err)
	}
}

func writePcaps(pcapDir string, packets []pcapOverIPPacket) ([]string, error) {
	filenames := []string(nil)
	handledLinkTypes := map[layers.LinkType]struct{}{}
	for len(packets) != 0 {
		lt := packets[0].linkType
		fnPartial := tools.MakeFilename("", "pcap")
		fnFull := filepath.Join(pcapDir, fnPartial)
		f, err := os.Create(fnFull)
		if err != nil {
			return filenames, err
		}
		defer func() {
			if f != nil {
				if err := f.Close(); err != nil {
					log.Printf("error closing file %q: %v", fnFull, err)
				}
			}
			if fnFull != "" {
				log.Printf("removing file %q because of a previous error", fnFull)
				if err := os.Remove(fnFull); err != nil {
					log.Printf("error removing file %q: %v", fnFull, err)
				}
			}
		}()
		w, err := pcapgo.NewNgWriter(f, lt)
		if err != nil {
			return filenames, err
		}
		nextStart := 0
		for i, packet := range packets {
			if packet.linkType != lt {
				if nextStart == 0 {
					if _, ok := handledLinkTypes[lt]; !ok {
						nextStart = i
					}
				}
				continue
			}
			if err := w.WritePacket(packet.ci, packet.data); err != nil {
				return filenames, err
			}
		}
		if err := w.Flush(); err != nil {
			return filenames, err
		}
		if f, err = nil, f.Close(); err != nil {
			return filenames, err
		}
		filenames = append(filenames, fnPartial)
		fnFull = ""
		if nextStart == 0 {
			break
		}
		packets = packets[nextStart:]
		handledLinkTypes[lt] = struct{}{}
	}
	return filenames, nil
}

func (mgr *Manager) pcapOverIPPacketHandler() {
	packets := []pcapOverIPPacket(nil)
	queue := false
	for {
		select {
		case packet := <-mgr.pcapOverIPPackets:
			packets = append(packets, packet)
			if queue {
				continue
			}
			queue = true

		case cmd := <-mgr.pcapOverIPCmd:
			switch cmd {
			case pcapOverIPCmdClose:
				return
			case pcapOverIPCmdFlush:
				if len(packets) == 0 {
					queue = false
					continue
				}
			}
		}
		go func(packets []pcapOverIPPacket) {
			filenames, err := writePcaps(mgr.PcapDir, packets)
			if err != nil {
				log.Printf("error writing PCAP-over-IP packets: %v", err)
			}
			if len(filenames) != 0 {
				mgr.ImportPcaps(filenames)
			}
		}(packets)
		packets = nil
	}
}

func (mgr *Manager) newPcapOverIPEndpoint(ctx context.Context, address string) *pcapOverIPEndpoint {
	ctx, cancel := context.WithCancel(ctx)
	endpoint := &pcapOverIPEndpoint{
		PcapOverIPEndpointInfo: PcapOverIPEndpointInfo{
			Address: address,
		},
		cancel: cancel,
	}
	go func() {
		for {
			func() {
				d := net.Dialer{}
				c, err := d.DialContext(ctx, "tcp", endpoint.Address)
				if err != nil {
					log.Printf("Can't connect to PCAP-over-IP endpoint %q: %v\n", endpoint.Address, err)
					return
				}
				conn := c.(*net.TCPConn)
				file, err := conn.File()
				if err != nil {
					conn.Close()
					log.Printf("Can't get file descriptor of PCAP-over-IP endpoint %q: %v\n", endpoint.Address, err)
					return
				}
				ctx, innerCancel := context.WithCancel(ctx)
				go func() {
					<-ctx.Done()
					_ = conn.CloseRead()
					_ = conn.CloseWrite()
					conn.Close()
					file.Close()
				}()
				defer innerCancel()
				handle, err := pcap.OpenOfflineFile(file)
				if err != nil {
					log.Printf("Can't open file descriptor of PCAP-over-IP endpoint %q: %v\n", endpoint.Address, err)
					return
				}
				defer handle.Close()
				lt := handle.LinkType()
				sl := handle.SnapLen()
				log.Printf("Connection to PCAP-over-IP endpoint %q established (using linkType %s and snaplen %d)\n", endpoint.Address, lt.String(), sl)

				endpoint.LastConnected = time.Now().UnixNano()
				for {
					data, ci, err := handle.ReadPacketData()
					if err != nil {
						log.Printf("Error reading packet from PCAP-over-IP endpoint %q: %v\n", endpoint.Address, err)
						return
					}
					mgr.pcapOverIPPackets <- pcapOverIPPacket{lt, data, ci}
					endpoint.ReceivedPackets++
				}
			}()
			if endpoint.LastDisconnected <= endpoint.LastConnected {
				endpoint.LastDisconnected = time.Now().UnixNano()
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	return endpoint
}

func (mgr *Manager) ListPcapOverIPEndpoints() []PcapOverIPEndpointInfo {
	c := make(chan []PcapOverIPEndpointInfo)
	mgr.jobs <- func() {
		endpoints := make([]PcapOverIPEndpointInfo, 0, len(mgr.pcapOverIPEndpoints))
		for _, e := range mgr.pcapOverIPEndpoints {
			endpoints = append(endpoints, e.PcapOverIPEndpointInfo)
		}
		c <- endpoints
		close(c)
	}
	return <-c
}

func (mgr *Manager) AddPcapOverIPEndpoint(address string) error {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return err
	}
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			for _, e := range mgr.pcapOverIPEndpoints {
				if e.Address == address {
					return fmt.Errorf("error: address %q already exists", address)
				}
			}
			mgr.pcapOverIPEndpoints = append(mgr.pcapOverIPEndpoints, mgr.newPcapOverIPEndpoint(context.Background(), address))
			endpoints := make([]PcapOverIPEndpointInfo, 0, len(mgr.pcapOverIPEndpoints))
			for _, e := range mgr.pcapOverIPEndpoints {
				endpoints = append(endpoints, e.PcapOverIPEndpointInfo)
			}
			mgr.event(Event{
				Type:                "pcapOverIPEndpointsUpdated",
				PcapOverIPEndpoints: &endpoints,
			})
			return mgr.saveState()
		}()
		c <- err
		close(c)
	}
	return <-c
}

func (mgr *Manager) DelPcapOverIPEndpoint(address string) error {
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			toDelete := slices.IndexFunc(mgr.pcapOverIPEndpoints, func(e *pcapOverIPEndpoint) bool {
				return e.Address == address
			})
			if toDelete == -1 {
				return fmt.Errorf("error: address %q doesn't exist", address)
			}
			mgr.pcapOverIPEndpoints[toDelete].cancel()
			mgr.pcapOverIPEndpoints = slices.Delete(mgr.pcapOverIPEndpoints, toDelete, toDelete+1)
			endpoints := make([]PcapOverIPEndpointInfo, 0, len(mgr.pcapOverIPEndpoints))
			for _, e := range mgr.pcapOverIPEndpoints {
				endpoints = append(endpoints, e.PcapOverIPEndpointInfo)
			}
			mgr.event(Event{
				Type:                "pcapOverIPEndpointsUpdated",
				PcapOverIPEndpoints: &endpoints,
			})
			return mgr.saveState()
		}()
		c <- err
		close(c)
	}
	return <-c
}

func (mgr *Manager) GetView() View {
	return View{mgr: mgr}
}

func (v *View) fetch() error {
	if len(v.indexes) != 0 {
		return nil
	}
	v.tagDetails = make(map[string]query.TagDetails)
	v.tagConverters = make(map[string][]string)
	v.converters = make(map[string]index.ConverterAccess)
	c := make(chan error)
	v.mgr.jobs <- func() {
		v.indexes, v.releaser = v.mgr.getIndexesCopy(0)
		for tn, ti := range v.mgr.tags {
			v.tagDetails[tn] = ti.TagDetails
			for _, c := range ti.converters {
				v.tagConverters[tn] = append(v.tagConverters[tn], c.Name())
			}
		}
		for converterName, converter := range v.mgr.converters {
			v.converters[converterName] = converter
		}
		c <- nil
		close(c)
	}
	return <-c
}

func (v *View) Release() {
	if len(v.releaser) != 0 {
		v.mgr.jobs <- func() {
			v.releaser.release(v.mgr)
		}
	}
}

func PrefetchTags(tags []string) StreamsOption {
	return func(o *streamsOptions) {
		o.prefetchTags = append(o.prefetchTags, tags...)
	}
}

func PrefetchAllTags() StreamsOption {
	return func(o *streamsOptions) {
		o.prefetchAllTags = true
	}
}

func Limit(defaultLimit, page uint) StreamsOption {
	return func(o *streamsOptions) {
		o.defaultLimit = defaultLimit
		o.page = page
	}
}

func (v *View) prefetchTags(ctx context.Context, tags []string, bm bitmask.LongBitmask) error {
	if len(tags) == 0 {
		return nil
	}
	uncertainTags := map[string]bitmask.LongBitmask{}
	addTag := (func(string, bitmask.LongBitmask))(nil)
	addTag = func(tn string, streams bitmask.LongBitmask) {
		ti := v.tagDetails[tn]
		if ti.Uncertain.IsZero() {
			return
		}
		uncertain := ti.Uncertain
		if !streams.IsZero() {
			uncertain = uncertain.Copy()
			uncertain.And(streams)
			if uncertain.IsZero() {
				return
			}
		}
		if u, ok := uncertainTags[tn]; ok {
			tmp := uncertain.Copy()
			tmp.Sub(u)
			if tmp.IsZero() {
				return
			}
			tmp.Or(u)
			uncertain = tmp
		}
		uncertainTags[tn] = uncertain
		f := ti.Conditions.Features()
		for _, tn := range f.SubQueryTags {
			addTag(tn, bitmask.LongBitmask{})
		}
		for _, tn := range f.MainTags {
			addTag(tn, uncertain)
		}
	}
	for _, tn := range tags {
		if _, ok := v.tagDetails[tn]; !ok {
			return fmt.Errorf("tag %q not defined", tn)
		}
		addTag(tn, bm)
	}
	for len(uncertainTags) != 0 {
	outer:
		for tn, uncertain := range uncertainTags {
			ti := v.tagDetails[tn]
			f := ti.Conditions.Features()
			for _, rtn := range f.MainTags {
				if _, ok := uncertainTags[rtn]; ok {
					continue outer
				}
			}
			for _, rtn := range f.SubQueryTags {
				if _, ok := uncertainTags[rtn]; ok {
					continue outer
				}
			}
			matches, _, _, err := index.SearchStreams(ctx, v.indexes, &uncertain, time.Time{}, ti.Conditions, nil, []query.Sorting{{Key: query.SortingKeyID, Dir: query.SortingDirAscending}}, 0, 0, v.tagDetails, v.converters, false)
			if err != nil {
				return err
			}
			ti.Uncertain = ti.Uncertain.Copy()
			ti.Uncertain.Sub(uncertain)
			ti.Matches = ti.Matches.Copy()
			ti.Matches.Sub(uncertain)
			for _, s := range matches {
				ti.Matches.Set(uint(s.StreamID))
			}
			v.tagDetails[tn] = ti
			delete(uncertainTags, tn)
		}
	}
	return nil
}

func (v *View) AllStreams(ctx context.Context, f func(StreamContext) error, options ...StreamsOption) error {
	opts := streamsOptions{}
	for _, o := range options {
		o(&opts)
	}
	if opts.defaultLimit != 0 || opts.page != 0 {
		return errors.New("Limit not supported for AllStreams")
	}
	if err := v.fetch(); err != nil {
		return err
	}
	if opts.prefetchAllTags {
		for tn := range v.tagDetails {
			opts.prefetchTags = append(opts.prefetchTags, tn)
		}
	}
	if err := v.prefetchTags(ctx, opts.prefetchTags, bitmask.LongBitmask{}); err != nil {
		return err
	}
	for i := len(v.indexes); i > 0; i-- {
		idx := v.indexes[i-1]
		if err := idx.AllStreams(func(s *index.Stream) error {
			for _, idx2 := range v.indexes[i:] {
				if _, ok := idx2.StreamIDs()[s.ID()]; ok {
					return nil
				}
			}
			return f(StreamContext{
				s: s,
				v: v,
			})
		}); err != nil {
			return err
		}
	}
	return nil
}

func (v *View) SearchStreams(ctx context.Context, filter *query.Query, f func(StreamContext) error, options ...StreamsOption) (bool, uint, *index.DataRegexes, error) {
	opts := streamsOptions{}
	for _, o := range options {
		o(&opts)
	}
	if err := v.fetch(); err != nil {
		return false, 0, nil, err
	}
	if opts.prefetchAllTags {
		for tn := range v.tagDetails {
			opts.prefetchTags = append(opts.prefetchTags, tn)
		}
	}
	limit := opts.defaultLimit
	if filter.Limit != nil {
		limit = *filter.Limit
	}
	offset := opts.page * limit
	res, hasMore, dataRegexes, err := index.SearchStreams(ctx, v.indexes, nil, filter.ReferenceTime, filter.Conditions, filter.Grouping, filter.Sorting, limit, offset, v.tagDetails, v.converters, true)
	if err != nil {
		return false, 0, nil, err
	}
	if len(res) == 0 {
		return hasMore, offset, dataRegexes, nil
	}
	if len(opts.prefetchTags) != 0 {
		searchedStreams := bitmask.LongBitmask{}
		for _, s := range res {
			searchedStreams.Set(uint(s.StreamID))
		}
		if err := v.prefetchTags(ctx, opts.prefetchTags, searchedStreams); err != nil {
			return false, 0, nil, err
		}
	}
	for _, s := range res {
		if err := f(StreamContext{
			s: s,
			v: v,
		}); err != nil {
			return false, 0, nil, err
		}
	}
	return hasMore, offset, dataRegexes, nil
}

func (v *View) ReferenceTime() (time.Time, error) {
	if err := v.fetch(); err != nil {
		return time.Time{}, err
	}
	referenceTime := time.Time{}
	for _, idx := range v.indexes {
		if referenceTime.IsZero() || referenceTime.After(idx.ReferenceTime) {
			referenceTime = idx.ReferenceTime
		}
	}
	return referenceTime, nil
}

func (v *View) Stream(streamID uint64) (StreamContext, error) {
	if err := v.fetch(); err != nil {
		return StreamContext{}, err
	}
	for i := len(v.indexes) - 1; i >= 0; i-- {
		idx := v.indexes[i]
		stream, err := idx.StreamByID(streamID)
		if err != nil {
			return StreamContext{}, err
		}
		if stream == nil {
			continue
		}
		return StreamContext{
			s: stream,
			v: v,
		}, nil
	}
	return StreamContext{}, nil
}

func (c StreamContext) Stream() *index.Stream {
	return c.s
}

func (c StreamContext) Data(converterName string) ([]index.Data, error) {
	if c.Stream() == nil {
		return nil, fmt.Errorf("stream not found")
	}
	if converterName == "" {
		return c.Stream().Data()
	}
	converter, ok := c.v.converters[converterName]
	if !ok {
		return nil, fmt.Errorf("invalid converter %q", converterName)
	}
	data, _, _, wasCached, err := converter.Data(c.Stream(), true)
	// only send event if the data wasn't cached before
	if err == nil && !wasCached {
		c.v.mgr.jobs <- func() {
			converter, ok := c.v.mgr.converters[converterName]
			if ok {
				c.v.mgr.event(Event{
					Type:      "converterCompleted",
					Converter: converter.Statistics(),
				})
			}
		}
	}
	return data, err
}

func (c StreamContext) HasTag(name string) (bool, error) {
	if c.v == nil {
		return false, fmt.Errorf("no view")
	}
	td := c.v.tagDetails[name]
	if !td.Uncertain.IsSet(uint(c.s.ID())) {
		return td.Matches.IsSet(uint(c.s.ID())), nil
	}
	//TODO: figure out if the uncertain tag matches
	return false, nil
}

func (c StreamContext) AllTags() ([]string, error) {
	if c.v == nil {
		return nil, fmt.Errorf("no view")
	}
	tags := []string{}
	for tn, td := range c.v.tagDetails {
		if !td.Uncertain.IsSet(uint(c.s.ID())) {
			if td.Matches.IsSet(uint(c.s.ID())) {
				tags = append(tags, tn)
			}
			continue
		}
		//TODO: figure out if the uncertain tag matches
	}
	sort.Strings(tags)
	return tags, nil
}

func (c StreamContext) AllConverters() ([]string, error) {
	if c.v == nil {
		return nil, fmt.Errorf("no view")
	}
	converters := []string{}
	for tn, cns := range c.v.tagConverters {
		ok, err := c.HasTag(tn)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for _, cn := range cns {
			if !slices.Contains(converters, cn) {
				converters = append(converters, cn)
			}
		}
	}
	sort.Strings(converters)
	return converters, nil
}

func (mgr *Manager) event(e Event) {
	for ch, l := range mgr.listeners {
		if l.active == 0 {
			select {
			case ch <- e:
				continue
			default:
			}
		} else {
			select {
			case <-l.close:
				continue
			default:
			}
		}
		l.active++
		mgr.listeners[ch] = l
		go func(ch chan Event, cl chan struct{}) {
			select {
			case ch <- e:
				mgr.jobs <- func() {
					l := mgr.listeners[ch]
					l.active--
					mgr.listeners[ch] = l
				}
			case <-cl:
				mgr.jobs <- func() {
					l := mgr.listeners[ch]
					if l.active == 1 {
						delete(mgr.listeners, ch)
						close(ch)
					} else {
						l.active--
						mgr.listeners[ch] = l
					}
				}
			}
		}(ch, l.close)
	}
}

func (mgr *Manager) Listen() (chan Event, func()) {
	ch := make(chan Event)
	mgr.jobs <- func() {
		mgr.listeners[ch] = listener{
			close: make(chan struct{}),
		}
	}
	return ch, func() {
		mgr.jobs <- func() {
			l, ok := mgr.listeners[ch]
			if !ok {
				return
			}
			if l.active == 0 {
				delete(mgr.listeners, ch)
				close(ch)
			}
			close(l.close)
		}
	}
}
