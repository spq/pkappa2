package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/bits"
	"os"
	"sort"
	"time"

	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/index/builder"
	pcapmetadata "github.com/spq/pkappa2/internal/index/pcapMetadata"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools"
)

type (
	tag struct {
		definition      string
		referencedTags  []string
		matchingStreams []uint64
		matchingCount   uint
		version         uint
	}
	TagInfo struct {
		Name           string
		Definition     string
		MatchingCount  uint
		IndexesPending uint
		Referenced     bool
	}
	Manager struct {
		StateDir    string
		PcapDir     string
		IndexDir    string
		SnapshotDir string

		jobs              chan func()
		mergeJobRunning   bool
		taggingJobRunning bool
		importJobs        []string

		builder             *builder.Builder
		indexes             []*index.Reader
		indexesVersion      []uint
		currentVersion      uint
		nStreams, nPackets  int
		nUnmergeableIndexes int
		stateFilename       string

		tags map[string]*tag

		usedIndexes map[*index.Reader]uint
	}

	Statistics struct {
		ImportJobCount    int
		IndexCount        int
		IndexLockCount    uint
		PcapCount         int
		StreamCount       int
		PacketCount       int
		MergeJobRunning   bool
		TaggingJobRunning bool
	}

	IndexReleaser []*index.Reader

	stateFile struct {
		Saved time.Time
		Tags  []struct {
			Name       string
			Definition string
		}
	}
)

func New(pcapDir, indexDir, snapshotDir, stateDir string) (*Manager, error) {
	b, err := builder.New(pcapDir, indexDir, snapshotDir)
	if err != nil {
		return nil, err
	}

	mgr := Manager{
		PcapDir:     pcapDir,
		IndexDir:    indexDir,
		SnapshotDir: snapshotDir,
		StateDir:    stateDir,

		builder:        b,
		usedIndexes:    make(map[*index.Reader]uint),
		tags:           make(map[string]*tag),
		currentVersion: 1,
	}

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
		mgr.indexesVersion = append(mgr.indexesVersion, uint(1))
		mgr.nStreams += idx.StreamCount()
		mgr.nPackets += idx.PacketCount()
	}
	mgr.lock(mgr.indexes)

	stateFilenames, err := tools.ListFiles(stateDir, "state.json")
	if err != nil {
		return nil, err
	}
	stateTimestamp := time.Time{}
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
			newTags[t.Name] = &tag{
				definition:     t.Definition,
				referencedTags: q.Conditions.ReferencedTags(),
			}
		}
		cyclingTags := map[string]struct{}{}
		for n, t := range newTags {
			for _, tn := range t.referencedTags {
				if n == tn {
					log.Printf("Invalid tag %q in statefile %q: references itself", n, fn)
					continue nextStateFile
				}
				if _, ok := newTags[tn]; !ok {
					log.Printf("Invalid tag %q in statefile %q: references non-existing tag %q", n, fn, tn)
					continue nextStateFile
				}
			}
			cyclingTags[n] = struct{}{}
		}
	checkCyclingTags:
		for {
		nextCyclingTag:
			for n := range cyclingTags {
				for _, rt := range newTags[n].referencedTags {
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
		mgr.tags = newTags
		mgr.stateFilename = fn
		stateTimestamp = s.Saved
	}

	mgr.jobs = make(chan func())
	go func() {
		for f := range mgr.jobs {
			f()
		}
	}()
	mgr.jobs <- func() {
		mgr.startTaggingJobIfNeeded()
		mgr.startMergeJobIfNeeded()
	}
	return &mgr, nil
}

func (mgr *Manager) saveState() error {
	j := stateFile{
		Saved: time.Now(),
	}
	for n, t := range mgr.tags {
		j.Tags = append(j.Tags, struct {
			Name       string
			Definition string
		}{
			Name:       n,
			Definition: t.definition,
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

func (mgr *Manager) importPcapJob(filename string, existingIndexes []*index.Reader, existingIndexesReleaser IndexReleaser) {
	createdIndexes, err := mgr.builder.FromPcap(mgr.PcapDir, filename, existingIndexes)
	if err != nil {
		log.Printf("importPcapJob(%q) failed: %s", filename, err)
	}
	mgr.jobs <- func() {
		existingIndexesReleaser.release(mgr)
		// add new indexes if some were created
		if len(createdIndexes) > 0 {
			mgr.lock(createdIndexes)
			mgr.currentVersion++
			mgr.indexes = append(mgr.indexes, createdIndexes...)
			for _, idx := range createdIndexes {
				mgr.nStreams += idx.StreamCount()
				mgr.nPackets += idx.PacketCount()
				mgr.indexesVersion = append(mgr.indexesVersion, mgr.currentVersion)
			}
		}
		// remove finished job from queue
		mgr.importJobs = mgr.importJobs[1:]
		// start new import job if there are more queued
		if len(mgr.importJobs) >= 1 {
			idxs, rel := mgr.getIndexesCopy(0, len(mgr.indexes))
			go mgr.importPcapJob(mgr.importJobs[0], idxs, rel)
		}
		mgr.startTaggingJobIfNeeded()
		mgr.startMergeJobIfNeeded()
	}
}

func (mgr *Manager) startMergeJobIfNeeded() {
	if mgr.mergeJobRunning || mgr.taggingJobRunning {
		return
	}
	// only merge if all tags are on the newest version, prioritize updating tags
	for _, t := range mgr.tags {
		if t.version != mgr.currentVersion {
			return
		}
	}
	nStreams := mgr.nStreams
	for i, idx := range mgr.indexes {
		c := idx.StreamCount()
		nStreams -= c
		if i >= mgr.nUnmergeableIndexes && c < nStreams {
			mgr.mergeJobRunning = true
			indexes, indexesReleaser := mgr.getIndexesCopy(i, len(mgr.indexes))
			go mgr.mergeIndexesJob(i, mgr.currentVersion, indexes, indexesReleaser)
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
		if t.version == mgr.currentVersion {
			continue
		}
		for _, tn := range t.referencedTags {
			if mgr.tags[tn].version != mgr.currentVersion {
				continue outer
			}
		}
		referencedTags := make(map[string][]uint64, len(t.referencedTags))
		for _, tn := range t.referencedTags {
			referencedTags[tn] = mgr.tags[tn].matchingStreams
		}
		startIndex := 0
		for i, v := range mgr.indexesVersion {
			if v >= t.version {
				startIndex = i
				break
			}
		}
		mgr.taggingJobRunning = true
		indexes, releaser := mgr.getIndexesCopy(startIndex, len(mgr.indexes))
		go mgr.updateTagJob(n, *t, mgr.currentVersion, referencedTags, indexes, releaser)
		return
	}
}

func (mgr *Manager) mergeIndexesJob(offset int, maxVersion uint, indexes []*index.Reader, releaser IndexReleaser) {
	idxs, err := index.Merge(mgr.IndexDir, indexes)
	if err != nil {
		indexFilenames := []string{}
		for _, i := range indexes {
			indexFilenames = append(indexFilenames, i.Filename())
		}
		log.Printf("mergeIndexesJob(%d, [%q]) failed: %s", offset, indexFilenames, err)
	}
	streamsDiff, packetsDiff := 0, 0
	for _, i := range idxs {
		streamsDiff += i.StreamCount()
		packetsDiff += i.PacketCount()
	}
	for _, i := range indexes {
		streamsDiff -= i.StreamCount()
		packetsDiff -= i.PacketCount()
	}
	newVersions := make([]uint, len(idxs))
	for i := range newVersions {
		newVersions[i] = maxVersion
	}
	mgr.jobs <- func() {
		// replace old indexes if successfully created
		if len(idxs) == 0 || err != nil {
			mgr.nUnmergeableIndexes++
		} else {
			rel := IndexReleaser(mgr.indexes[offset : offset+len(indexes)])
			rel.release(mgr)
			mgr.lock(idxs)
			mgr.indexes = append(mgr.indexes[:offset], append(idxs, mgr.indexes[offset+len(indexes):]...)...)
			mgr.indexesVersion = append(mgr.indexesVersion[:offset], append(newVersions, mgr.indexesVersion[offset+len(indexes):]...)...)
			mgr.nUnmergeableIndexes += len(idxs) - 1
			mgr.nStreams += streamsDiff
			mgr.nPackets += packetsDiff
		}
		mgr.mergeJobRunning = false
		mgr.startMergeJobIfNeeded()
		releaser.release(mgr)
	}
}

func (mgr *Manager) updateTagJob(name string, t tag, newVersion uint, referencedTags map[string][]uint64, indexes []*index.Reader, releaser IndexReleaser) {
	err := func() error {
		q, err := query.Parse(t.definition)
		if err != nil {
			return err
		}
		maxStreamID := uint64(0)
		for _, i := range indexes {
			if maxStreamID < i.MaxStreamID() {
				maxStreamID = i.MaxStreamID()
			}
		}
		streams, _, err := index.SearchStreams(indexes, q.ReferenceTime, q.Conditions, []query.Sorting{{Key: query.SortingKeyID, Dir: query.SortingDirAscending}}, uint(maxStreamID)+1, 0, referencedTags)
		if err != nil {
			return err
		}
		l := (maxStreamID / 64) + 1
		if l < uint64(len(t.matchingStreams)) {
			l = uint64(len(t.matchingStreams))
		}
		newBitset := make([]uint64, l)
		copy(newBitset, t.matchingStreams)
		for _, i := range indexes {
			for id := range i.StreamIDs() {
				newBitset[id/64] &= ^(uint64(1) << (id % 64))
			}
		}
		for _, s := range streams {
			newBitset[s.ID()/64] |= uint64(1) << (s.ID() % 64)
		}
		t.matchingCount = 0
		for _, b := range newBitset {
			t.matchingCount += uint(bits.OnesCount64(b))
		}
		t.matchingStreams = newBitset
		return nil
	}()
	if err != nil {
		log.Printf("updateTagJob failed: %q", err)
		t.matchingCount = 0
		t.matchingStreams = nil
	}
	mgr.jobs <- func() {
		ot, ok := mgr.tags[name]
		if ok && ot.version == t.version && ot.definition == t.definition {
			// only touch the live copy of the tag if the tag was not modified while we were updating it
			t.version = newVersion
			mgr.tags[name] = &t
		}
		mgr.taggingJobRunning = false
		mgr.startTaggingJobIfNeeded()
		mgr.startMergeJobIfNeeded()
		releaser.release(mgr)
	}
}

func (mgr *Manager) ImportPcap(filename string) {
	mgr.jobs <- func() {
		//add job to be processed by importer goroutine
		mgr.importJobs = append(mgr.importJobs, filename)
		//start import job when none running
		if len(mgr.importJobs) == 1 {
			indexes, releaser := mgr.getIndexesCopy(0, len(mgr.indexes))
			go mgr.importPcapJob(filename, indexes, releaser)
		}
	}
}

func (mgr *Manager) getIndexesCopy(start, end int) ([]*index.Reader, IndexReleaser) {
	indexes := append([]*index.Reader(nil), mgr.indexes[start:end]...)
	return indexes, mgr.lock(indexes)
}

func (mgr *Manager) GetIndexes(tags []string) ([]*index.Reader, map[string][]uint64, IndexReleaser, error) {
	type res struct {
		indexes  []*index.Reader
		releaser IndexReleaser
		tags     map[string][]uint64
		err      error
	}
	c := make(chan res)
	mgr.jobs <- func() {
		err := func() error {
			tagMasks := map[string][]uint64{}
			minVersion := uint(math.MaxUint64)
			for _, tn := range tags {
				t, ok := mgr.tags[tn]
				if !ok {
					return fmt.Errorf("tag %q not found", tn)
				}
				if t.version == 0 {
					return fmt.Errorf("tag %q not yet resolved", tn)
				}
				tagMasks[tn] = t.matchingStreams
				// TODO: we should keep old versions of the tags until all are at a common higher version
				// otherwise, we might produce an inconsistent view
				if minVersion > t.version {
					minVersion = t.version
				}
			}
			indexCount := 0
			for _, v := range mgr.indexesVersion {
				if v > minVersion {
					break
				}
				indexCount++
			}
			indexes, releaser := mgr.getIndexesCopy(0, indexCount)
			c <- res{indexes, releaser, tagMasks, nil}
			return nil
		}()
		if err != nil {
			c <- res{err: err}
		}
		close(c)
	}
	r := <-c
	return r.indexes, r.tags, r.releaser, r.err
}

func (mgr *Manager) Stream(streamID uint64) (*index.Stream, IndexReleaser, error) {
	indexes, _, releaser, err := mgr.GetIndexes(nil)
	if err != nil {
		return nil, nil, err
	}
	defer releaser.Release(mgr)
	for i := len(indexes) - 1; i >= 0; i-- {
		idx := indexes[i]
		stream, err := idx.StreamByID(streamID)
		if err != nil {
			return nil, nil, err
		}
		if stream == nil {
			continue
		}
		return stream, releaser.extract(i), nil
	}
	return nil, nil, nil
}

func (mgr *Manager) Status() Statistics {
	c := make(chan Statistics)
	mgr.jobs <- func() {
		locks := uint(0)
		for _, n := range mgr.usedIndexes {
			locks += n
		}
		c <- Statistics{
			IndexCount:        len(mgr.indexes),
			IndexLockCount:    locks,
			PcapCount:         len(mgr.builder.KnownPcaps()),
			ImportJobCount:    len(mgr.importJobs),
			StreamCount:       mgr.nStreams,
			PacketCount:       mgr.nPackets,
			MergeJobRunning:   mgr.mergeJobRunning,
			TaggingJobRunning: mgr.taggingJobRunning,
		}
	}
	res := <-c
	close(c)
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
	}
	res := <-c
	close(c)
	return res
}

func (mgr *Manager) ListTags() []TagInfo {
	c := make(chan []TagInfo)
	mgr.jobs <- func() {
		res := []TagInfo{}
		referencedTag := map[string]struct{}{}
		for _, t := range mgr.tags {
			for _, tn := range t.referencedTags {
				referencedTag[tn] = struct{}{}
			}
		}
		for name, t := range mgr.tags {
			indexesPending := uint(0)
			for _, v := range mgr.indexesVersion {
				if v > t.version {
					indexesPending++
				}
			}
			_, referenced := referencedTag[name]
			res = append(res, TagInfo{
				Name:           name,
				Definition:     t.definition,
				MatchingCount:  t.matchingCount,
				IndexesPending: indexesPending,
				Referenced:     referenced,
			})
		}
		sort.Slice(res, func(i, j int) bool {
			return res[i].Name < res[j].Name
		})
		c <- res
		close(c)
	}
	return <-c
}

func (mgr *Manager) AddTag(name, queryString string) error {
	q, err := query.Parse(queryString)
	if err != nil {
		return err
	}
	if q.Conditions.HasRelativeTimes() {
		return errors.New("relative times not yet supported in tags")
	}
	if len(q.Conditions.SubQueries()) != 1 {
		return errors.New("subqueries not yet supported in tags")
	}
	referencedTags := q.Conditions.ReferencedTags()
	for _, tn := range referencedTags {
		if tn == name {
			return errors.New("self reference not allowed in tags")
		}
	}
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			if _, ok := mgr.tags[name]; ok {
				return errors.New("tag already exists")
			}
			// check if all referenced tags exist
			for _, t := range referencedTags {
				if _, ok := mgr.tags[t]; !ok {
					return fmt.Errorf("unknown referenced tag %q", t)
				}
			}
			mgr.tags[name] = &tag{
				definition:     queryString,
				referencedTags: referencedTags,
			}
			mgr.startTaggingJobIfNeeded()
			return nil
		}()
		c <- err
		close(c)
		//nolint:errcheck
		mgr.saveState()
	}
	return <-c
}

func (mgr *Manager) DelTag(name string) error {
	c := make(chan error)
	mgr.jobs <- func() {
		err := func() error {
			if _, ok := mgr.tags[name]; !ok {
				return fmt.Errorf("unknown tag %q", name)
			}
			for t2name, t2 := range mgr.tags {
				for _, tn := range t2.referencedTags {
					if tn == name {
						return fmt.Errorf("tag %q still references the tag to be deleted", t2name)
					}
				}
			}
			delete(mgr.tags, name)
			return nil
		}()
		c <- err
		close(c)
		//nolint:errcheck
		mgr.saveState()
	}
	return <-c
}

func (mgr *Manager) lock(indexes []*index.Reader) IndexReleaser {
	for _, i := range indexes {
		mgr.usedIndexes[i]++
	}
	return IndexReleaser(append([]*index.Reader(nil), indexes...))
}

func (r *IndexReleaser) extract(i int) IndexReleaser {
	rel := IndexReleaser{(*r)[i]}
	*r = append((*r)[:i], (*r)[i+1:]...)
	return rel
}

// Release all contained indexes
func (r *IndexReleaser) Release(mgr *Manager) {
	mgr.jobs <- func() {
		r.release(mgr)
	}
}

// release all contained indexes from within the mgr goroutine
func (r *IndexReleaser) release(mgr *Manager) {
	for _, i := range *r {
		mgr.usedIndexes[i]--
		if mgr.usedIndexes[i] == 0 {
			delete(mgr.usedIndexes, i)
			i.Close()
			os.Remove(i.Filename())
		}
	}
}
