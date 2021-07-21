package index

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"runtime/debug"
	"sort"
	"time"

	"github.com/google/gopacket/reassembly"
	pcapmetadata "github.com/spq/pkappa2/internal/index/pcapMetadata"
	"github.com/spq/pkappa2/internal/index/streams"
	"github.com/spq/pkappa2/internal/seekbufio"
)

type (
	hostGroup struct {
		hosts    []byte
		hostSize int
	}
	writerImportEntry struct {
		filename string
		offset   uint64
	}
	Writer struct {
		filename   string
		file       *os.File
		buffer     *bufio.Writer
		hostGroups []hostGroup
		imports    map[writerImportEntry]uint32
		packets    []packet
		streams    []stream
		header     fileHeader
	}
)

func (g *hostGroup) add(host []byte) (uint16, bool, bool) {
	if len(g.hosts) == 0 {
		// first host in the group, just add the host
		g.hosts = make([]byte, len(host))
		copy(g.hosts, host)
		g.hostSize = len(host)
		return 0, true, true
	}
	if g.hostSize != len(host) {
		// can't add different size hosts
		return 0, false, false
	}
	for pos := 0; pos < len(g.hosts); pos += g.hostSize {
		if bytes.Equal(g.hosts[pos:][:g.hostSize], host) {
			return uint16(pos / g.hostSize), false, true
		}
	}
	if len(g.hosts) >= math.MaxUint16 {
		return 0, false, false
	}
	g.hosts = append(g.hosts, host...)
	return uint16((len(g.hosts) / g.hostSize) - 1), true, true
}

func (g *hostGroup) pop() {
	g.popN(1)
}

func (g *hostGroup) popN(n int) {
	g.hosts = g.hosts[:len(g.hosts)-n]
}

func (w *Writer) write(what interface{}) error {
	err := binary.Write(w.buffer, binary.LittleEndian, what)
	if err != nil {
		debug.PrintStack()
	}
	return err
}

func (w *Writer) pos() (uint64, error) {
	if err := w.buffer.Flush(); err != nil {
		return 0, err
	}
	pos, err := w.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		debug.PrintStack()
	}
	return uint64(pos), err
}

func (w *Writer) setPos(where *uint64) error {
	pos, err := w.pos()
	if err == nil {
		*where = pos
	}
	return err
}

func (w *Writer) setSectionBegin(section section) error {
	return w.setPos(&w.header.Sections[section].Begin)
}

func (w *Writer) setSectionEnd(section section) error {
	return w.setPos(&w.header.Sections[section].End)
}

func (w *Writer) pad(n uint64) error {
	pos, err := w.pos()
	if err != nil {
		return err
	}
	padding := (n - (pos % n)) % n
	if padding != 0 {
		return w.write(make([]byte, padding))
	}
	return nil
}

func NewWriter(filename string) (*Writer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	w := Writer{
		filename:   filename,
		file:       file,
		buffer:     bufio.NewWriter(file),
		hostGroups: make([]hostGroup, 0),
		imports:    make(map[writerImportEntry]uint32),
	}
	if err := w.write(&w.header); err != nil {
		w.Close()
		return nil, err
	}
	if err := w.setSectionBegin(sectionData); err != nil {
		w.Close()
		return nil, err
	}
	return &w, nil
}

func (w *Writer) Close() error {
	return w.file.Close()
}

func (w *Writer) Filename() string {
	return w.filename
}

func (w *Writer) AddStream(s *streams.TCPStream, streamID uint64) (bool, error) {
	// check if we can reference the stream.
	if len(w.streams) > math.MaxUint32 {
		return false, nil
	}

	// check if we can reference the first packet, even if we will
	// write more packets, only the first has to be referenced.
	if len(w.packets) > math.MaxUint32 {
		return false, nil
	}

	firstPacketTs := s.Packets[0].Timestamp
	if firstPacketSeconds := uint64(firstPacketTs.Unix()); len(w.packets) == 0 {
		w.header.FirstPacketTime = firstPacketSeconds
	} else if w.header.FirstPacketTime > firstPacketSeconds {
		oldReferenceTime := time.Unix(int64(w.header.FirstPacketTime), 0)
		w.header.FirstPacketTime = firstPacketSeconds
		newReferenceTime := time.Unix(int64(w.header.FirstPacketTime), 0)
		diff := uint64(oldReferenceTime.Sub(newReferenceTime).Nanoseconds())
		for _, oldStream := range w.streams {
			oldStream.FirstPacketTimeNS += diff
			oldStream.LastPacketTimeNS += diff
		}
	}
	referenceTime := time.Unix(int64(w.header.FirstPacketTime), 0)
	lastPacketTs := s.Packets[len(s.Packets)-1].Timestamp

	stream := stream{
		StreamID:          streamID,
		ClientPort:        s.ClientPort,
		ServerPort:        s.ServerPort,
		PacketInfoStart:   uint32(len(w.packets)),
		FirstPacketTimeNS: uint64(firstPacketTs.Sub(referenceTime).Nanoseconds()),
		LastPacketTimeNS:  uint64(lastPacketTs.Sub(referenceTime).Nanoseconds()),
		Flags:             flagsStreamProtocolTCP | flagsStreamSegmentationNone,
	}

	// when we can't add a stream to this writer, we might have
	// to undo some operations, those will be collected here.
	undos := []func(){}
	undo := func() {
		for _, u := range undos {
			u()
		}
	}
	undoable := func(f func()) {
		undos = append(undos, f)
	}

	// try to add the client and server addr to the host groups
	for gID := 0; gID <= len(w.hostGroups); gID++ {
		if gID >= len(w.hostGroups) {
			if len(w.hostGroups) > math.MaxUint16 {
				undo()
				return false, nil
			}
			w.hostGroups = append(w.hostGroups, hostGroup{})
		}
		g := &w.hostGroups[gID]
		cAddrID, added, ok := g.add(s.ClientAddr)
		if !ok {
			continue
		}
		sAddrID, added2, ok := g.add(s.ServerAddr)
		if !ok {
			if added {
				g.pop()
			}
			continue
		}
		if added {
			undoable(g.pop)
		}
		if added2 {
			undoable(g.pop)
		}
		stream.HostGroup = uint16(gID)
		stream.ClientHost = cAddrID
		stream.ServerHost = sAddrID
		break
	}

	// collect new import filenames
	originalImportCount := len(w.imports)
	undoable(func() {
		if originalImportCount == len(w.imports) {
			return
		}
		for e, i := range w.imports {
			if i >= uint32(originalImportCount) {
				delete(w.imports, e)
			}
		}
	})
	for _, p := range s.Packets {
		pmds := pcapmetadata.AllFromPacketMetadata(p)
		for _, pmd := range pmds {
			e := writerImportEntry{
				filename: pmd.PcapInfo.Filename,
				offset:   pmd.Index & (math.MaxUint32 << 32),
			}
			if _, ok := w.imports[e]; !ok {
				if len(w.imports) > math.MaxUint32 {
					undo()
					return false, nil
				}
				w.imports[e] = uint32(len(w.imports))
			}
		}
	}

	// collect the packets and write the data
	if err := w.setPos(&stream.DataStart); err != nil {
		undo()
		return false, err
	}
	stream.DataStart -= w.header.Sections[sectionData].Begin
	undoable(func() {
		w.buffer.Flush()
		//nolint:errcheck
		w.file.Seek(int64(stream.DataStart+w.header.Sections[sectionData].Begin), io.SeekStart)
		w.packets = w.packets[:stream.PacketInfoStart]
	})
	packetToData := map[uint64]int{}
	for i := range s.Data {
		packetToData[s.Data[i].PacketIndex] = i
	}
	lastPacketWithData := len(w.packets)
	for pIndex, p := range s.Packets {
		dir := s.PacketDirections[pIndex]
		pmds := pcapmetadata.AllFromPacketMetadata(p)
		for _, pmd := range pmds {
			flags := uint8(flagsPacketHasNext)
			switch dir {
			case reassembly.TCPDirClientToServer:
				flags |= flagsPacketDirectionClientToServer
			case reassembly.TCPDirServerToClient:
				flags |= flagsPacketDirectionServerToClient
			}
			dataSize := uint64(0)
			if dIndex, ok := packetToData[uint64(pIndex)]; ok {
				dataSize = uint64(len(s.Data[dIndex].Bytes))
			}
			for {
				np := packet{
					ImportID: w.imports[writerImportEntry{
						filename: pmd.PcapInfo.Filename,
						offset:   pmd.Index & (math.MaxUint32 << 32),
					}],
					PacketIndex:        uint32(pmd.Index),
					RelPacketTimeMS:    uint32(p.Timestamp.Sub(s.Packets[0].Timestamp).Microseconds()),
					DataSize:           uint16(dataSize),
					SkipPacketsForData: 0xff,
					Flags:              flags,
				}
				if dataSize > math.MaxUint16 {
					np.DataSize = math.MaxUint16
				}
				if np.DataSize != 0 {
					for ; lastPacketWithData < len(w.packets); lastPacketWithData++ {
						distance := len(w.packets) - lastPacketWithData - 1
						if distance < 0xff {
							w.packets[lastPacketWithData].SkipPacketsForData = uint8(distance)
						}

					}
					lastPacketWithData = len(w.packets)
				}
				w.packets = append(w.packets, np)
				dataSize -= uint64(np.DataSize)
				if dataSize == 0 {
					break
				}
			}
		}
	}
	for ; lastPacketWithData < len(w.packets)-1; lastPacketWithData++ {
		distance := len(w.packets) - lastPacketWithData - 2
		if distance < 0xff {
			w.packets[lastPacketWithData].SkipPacketsForData = uint8(distance)
		}
	}
	for dIndex := range s.Data {
		// TODO: write data from multiple packets for the same
		// direction in one go, don't add data headers in-between.
		d := &s.Data[dIndex]
		dir := s.PacketDirections[d.PacketIndex]
		switch dir {
		case reassembly.TCPDirClientToServer:
			stream.ClientBytes += uint64(len(d.Bytes))
		case reassembly.TCPDirServerToClient:
			stream.ServerBytes += uint64(len(d.Bytes))
		}
		for offset := 0; offset < len(d.Bytes); {
			l := len(d.Bytes) - offset
			if l > math.MaxUint32 {
				l = math.MaxUint32
			}
			header := dataHeader{
				Length: uint32(l),
			}
			lastPacket := dIndex == len(s.Data)-1
			lastChunkOfPacket := offset+l == len(d.Bytes)
			if !(lastPacket && lastChunkOfPacket) {
				header.Flags |= flagsDataHasNext
			}
			switch dir {
			case reassembly.TCPDirServerToClient:
				header.Flags |= flagsDataDirectionServerToClient
			case reassembly.TCPDirClientToServer:
				header.Flags |= flagsDataDirectionClientToServer
			}
			if err := w.write(&header); err != nil {
				undo()
				return false, err
			}
			if err := w.write(d.Bytes[offset : offset+l]); err != nil {
				undo()
				return false, err
			}
			offset += l
		}
	}
	if len(s.Data) == 0 {
		// no data was written, write a fake header with 0 length
		if err := w.write(&dataHeader{}); err != nil {
			undo()
			return false, err
		}
	}
	// drop the has next flag of the last packet
	w.packets[len(w.packets)-1].Flags -= flagsPacketHasNext
	w.streams = append(w.streams, stream)
	return true, nil
}

func (w *Writer) Finalize() (*Reader, error) {
	if err := w.setSectionEnd(sectionData); err != nil {
		w.Close()
		return nil, err
	}
	if err := w.pad(8); err != nil {
		w.Close()
		return nil, err
	}

	writeSection := func(section section, f func() error) error {
		if err := w.setSectionBegin(section); err != nil {
			w.Close()
			return err
		}
		if err := f(); err != nil {
			w.Close()
			return err
		}
		if err := w.setSectionEnd(section); err != nil {
			w.Close()
			return err
		}
		if err := w.pad(8); err != nil {
			w.Close()
			return err
		}
		return nil
	}

	importFilenams := []byte{}
	importFilenameOffsets := map[string]uint64{}
	importRecords := make([]importEntry, len(w.imports))
	for e, impPos := range w.imports {
		fnPos, ok := importFilenameOffsets[e.filename]
		if !ok {
			fnPos = uint64(len(importFilenams))
			importFilenameOffsets[e.filename] = fnPos
			importFilenams = append(importFilenams, []byte(e.filename)...)
			importFilenams = append(importFilenams, 0)
		}
		importRecords[impPos] = importEntry{
			Filename:          fnPos,
			PacketIndexOffset: e.offset,
		}
	}
	// write import filenames
	if err := writeSection(sectionImportFilenames, func() error {
		return w.write(importFilenams)
	}); err != nil {
		return nil, err
	}

	// write imports
	if err := writeSection(sectionImports, func() error {
		return w.write(importRecords)
	}); err != nil {
		return nil, err
	}

	// write packet infos
	if err := writeSection(sectionPackets, func() error {
		for _, p := range w.packets {
			if err := w.write(&p); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// write v4 hosts
	if err := writeSection(sectionV4Hosts, func() error {
		for _, hg := range w.hostGroups {
			if hg.hostSize != 4 {
				continue
			}
			if err := w.write(hg.hosts); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// write v6 hosts
	if err := writeSection(sectionV6Hosts, func() error {
		for _, hg := range w.hostGroups {
			if hg.hostSize != 16 {
				continue
			}
			if err := w.write(hg.hosts); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// write host groups
	if err := writeSection(sectionHostGroups, func() error {
		v4offset, v6offset := 0, 0
		for _, hg := range w.hostGroups {
			flags := uint16(0)
			offset := 0
			if hg.hostSize == 16 {
				flags |= flagsHostGroupIP6
				offset = v6offset
				v6offset += len(hg.hosts) / hg.hostSize
			} else {
				flags |= flagsHostGroupIP4
				offset = v4offset
				v4offset += len(hg.hosts) / hg.hostSize
			}
			entry := hostGroupEntry{
				Start: uint32(offset),
				Count: uint16((len(hg.hosts) / hg.hostSize) - 1),
				Flags: flags,
			}
			if err := w.write(&entry); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// write streams
	if err := writeSection(sectionStreams, func() error {
		for _, s := range w.streams {
			if err := w.write(&s); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	//write lookups
	writeLookup := func(section section, less func(a, b *stream) bool) error {
		l := make([]uint32, len(w.streams))
		for i := range l {
			l[i] = uint32(i)
		}
		sort.Slice(l, func(a, b int) bool {
			return less(&w.streams[l[a]], &w.streams[l[b]])
		})
		return writeSection(section, func() error {
			return w.write(l)
		})
	}
	if err := writeLookup(sectionStreamsByStreamID, func(a, b *stream) bool {
		return a.StreamID < b.StreamID
	}); err != nil {
		return nil, err
	}
	importEntries := make([]writerImportEntry, len(w.imports))
	for e, id := range w.imports {
		importEntries[id] = e
	}
	if err := writeLookup(sectionStreamsByFirstPacketSource, func(a, b *stream) bool {
		ap, bp := &w.packets[a.PacketInfoStart], &w.packets[b.PacketInfoStart]
		if ap.ImportID == bp.ImportID {
			return ap.PacketIndex < bp.PacketIndex
		}
		aie, bie := &importEntries[ap.ImportID], &importEntries[bp.ImportID]
		if aie.filename != bie.filename {
			return aie.filename < bie.filename
		}
		return aie.offset+uint64(ap.PacketIndex) < bie.offset+uint64(bp.PacketIndex)
	}); err != nil {
		return nil, err
	}
	if err := writeLookup(sectionStreamsByFirstPacketTime, func(a, b *stream) bool {
		return a.FirstPacketTimeNS < b.FirstPacketTimeNS
	}); err != nil {
		return nil, err
	}
	if err := writeLookup(sectionStreamsByLastPacketTime, func(a, b *stream) bool {
		return a.LastPacketTimeNS < b.LastPacketTimeNS
	}); err != nil {
		return nil, err
	}

	if err := w.buffer.Flush(); err != nil {
		w.Close()
		return nil, err
	}

	// update header
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		w.Close()
		return nil, err
	}
	copy(w.header.Magic[:], []byte(fileMagic))
	if err := w.write(&w.header); err != nil {
		w.Close()
		return nil, err
	}
	if err := w.buffer.Flush(); err != nil {
		w.Close()
		return nil, err
	}

	// write everything to the file and close it
	if err := w.Close(); err != nil {
		return nil, err
	}

	return NewReader(w.filename)
}

func (w *Writer) AddIndex(r *Reader) (bool, error) {
	// when we can't add a stream to this writer, we might have
	// to undo some operations, those will be collected here.
	undos := []func(){}
	undo := func() {
		for _, u := range undos {
			u()
		}
	}
	undoable := func(f func()) {
		undos = append(undos, f)
	}

	/*
		w.packets
		w.streams
		w.header
	*/

	// merge imports
	importRemap := []uint32{}
	importCountBefore := len(w.imports)
	undoable(func() {
		for imp, idx := range w.imports {
			if int(idx) >= importCountBefore {
				delete(w.imports, imp)
			}
		}
	})
	for _, i := range r.imports {
		k := writerImportEntry{
			filename: i.filename,
			offset:   i.packetIndexOffset,
		}
		newIndex, ok := w.imports[k]
		if !ok {
			if len(w.imports) > math.MaxUint32 {
				undo()
				return false, nil
			}
			newIndex = uint32(len(w.imports))
			w.imports[k] = newIndex
		}
		importRemap = append(importRemap, newIndex)
	}

	// merge host groups
	type hgRemap struct {
		nAdded         int
		hostGroupRemap uint16
		hostRemap      []uint16
	}
	hgRemapper := []hgRemap{}
	hgCountBefore := len(w.hostGroups)
	undoable(func() {
		for _, m := range hgRemapper {
			w.hostGroups[m.hostGroupRemap].popN(m.nAdded)
		}
		w.hostGroups = w.hostGroups[:hgCountBefore]
	})
	for _, rhg := range r.hostGroups {
		for whgIdx := 0; whgIdx <= len(w.hostGroups); {
			remap := hgRemap{
				hostGroupRemap: uint16(whgIdx),
			}
			if whgIdx == len(w.hostGroups) {
				if len(w.hostGroups) > math.MaxUint16 {
					undo()
					return false, nil
				}
				w.hostGroups = append(w.hostGroups, hostGroup{
					hostSize: rhg.hostSize,
					hosts:    rhg.hosts,
				})
				remap.hostRemap = make([]uint16, 0, rhg.hostCount)
				for h := 0; h < rhg.hostCount; h++ {
					remap.hostRemap = append(remap.hostRemap, uint16(h))
				}
			} else {
				whg := &w.hostGroups[whgIdx]
				failed := false
				nAdded := 0
				for h := 0; h < rhg.hostCount; h++ {
					newIndex, added, ok := whg.add(rhg.get(uint16(h)))
					if !ok {
						failed = true
						break
					}
					remap.hostRemap = append(remap.hostRemap, newIndex)
					if added {
						nAdded++
					}
				}
				if failed {
					whg.popN(nAdded)
					continue
				}
			}
			hgRemapper = append(hgRemapper, remap)
			break
		}
	}

	// build a list of stream id's added to the writer
	existingStreamIDs := map[uint64]struct{}{}
	for _, s := range w.streams {
		existingStreamIDs[s.StreamID] = struct{}{}
	}

	// merge streams tigether with data and packets
	streamCountBefore := len(w.streams)
	packetCountBefore := len(w.packets)
	dataPosBefore := uint64(0)
	if err := w.setPos(&dataPosBefore); err != nil {
		undo()
		return false, err
	}
	undoable(func() {
		w.streams = w.streams[:streamCountBefore]
		w.packets = w.packets[:packetCountBefore]
		w.buffer.Flush()
		//nolint:errcheck
		w.file.Seek(int64(dataPosBefore), io.SeekStart)
	})
	sr := io.NewSectionReader(r.file, int64(r.header.Sections[sectionData].Begin), r.header.Sections[sectionData].size())
	br := seekbufio.NewSeekableBufferReader(sr)
	minFirstPacketTimeNS := uint64(math.MaxUint64)
	for sIdx, sCount := 0, r.StreamCount(); sIdx < sCount; sIdx++ {
		s, err := r.streamByIndex(uint32(sIdx))
		if err != nil {
			return false, err
		}
		if _, ok := existingStreamIDs[s.StreamID]; ok {
			continue
		}
		if len(w.streams) > math.MaxUint32 || len(w.packets) > math.MaxUint32 {
			undo()
			return false, nil
		}
		newStream := *s
		hgr := &hgRemapper[newStream.HostGroup]
		newStream.HostGroup = hgr.hostGroupRemap
		newStream.ClientHost = hgr.hostRemap[newStream.ClientHost]
		newStream.ServerHost = hgr.hostRemap[newStream.ServerHost]
		newStream.PacketInfoStart = uint32(len(w.packets))
		for pIdx := uint64(s.PacketInfoStart); ; pIdx++ {
			p, err := r.packetByIndex(pIdx)
			if err != nil {
				return false, err
			}
			newPacket := *p
			newPacket.ImportID = importRemap[newPacket.ImportID]
			w.packets = append(w.packets, newPacket)
			if newPacket.Flags&flagsPacketHasNext == 0 {
				break
			}
		}

		if err := w.setPos(&newStream.DataStart); err != nil {
			undo()
			return false, err
		}
		newStream.DataStart -= w.header.Sections[sectionData].Begin

		if _, err := br.Seek(int64(s.DataStart), io.SeekStart); err != nil {
			undo()
			return false, err
		}

		for {
			h := dataHeader{}
			if err := binary.Read(br, binary.LittleEndian, &h); err != nil {
				undo()
				return false, err
			}
			if err := w.write(h); err != nil {
				undo()
				return false, err
			}
			if _, err := io.CopyN(w.buffer, br, int64(h.Length)); err != nil {
				undo()
				return false, err
			}
			if h.Flags&flagsDataHasNext == 0 {
				break
			}
		}

		if minFirstPacketTimeNS > newStream.FirstPacketTimeNS {
			minFirstPacketTimeNS = newStream.FirstPacketTimeNS
		}
		w.streams = append(w.streams, newStream)
	}

	if len(w.streams) == streamCountBefore {
		// no new streams, all kept the same
		undo()
		return true, nil
	}

	newFirstPacketTimeS := uint64(time.Unix(int64(r.header.FirstPacketTime), 0).Add(time.Nanosecond * time.Duration(minFirstPacketTimeNS)).Unix())
	if streamCountBefore != 0 && newFirstPacketTimeS > w.header.FirstPacketTime {
		newFirstPacketTimeS = w.header.FirstPacketTime
	}
	newTimeDiffNS := (r.header.FirstPacketTime - newFirstPacketTimeS) * uint64(time.Second/time.Nanosecond)
	oldTimeDiffNS := (w.header.FirstPacketTime - newFirstPacketTimeS) * uint64(time.Second/time.Nanosecond)
	if oldTimeDiffNS != 0 {
		for sIdx := range w.streams[:streamCountBefore] {
			s := &w.streams[sIdx]
			s.FirstPacketTimeNS += oldTimeDiffNS
			s.LastPacketTimeNS += oldTimeDiffNS
		}
	}
	if newTimeDiffNS != 0 {
		for sIdx := range w.streams[streamCountBefore:] {
			s := &w.streams[sIdx]
			s.FirstPacketTimeNS += newTimeDiffNS
			s.LastPacketTimeNS += newTimeDiffNS
		}
	}
	w.header.FirstPacketTime = newFirstPacketTimeS
	return true, nil
}
