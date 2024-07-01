package index

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"runtime/debug"
	"sort"
	"time"
	"unsafe"
)

type (
	readerHostGroup struct {
		hosts     []byte
		hostSize  int
		hostCount int
	}
	readerImportEntry struct {
		filename          string
		packetIndexOffset uint64
	}
	Reader struct {
		filename   string
		file       *os.File
		size       int64
		header     fileHeader
		imports    []readerImportEntry
		hostGroups []readerHostGroup

		ReferenceTime time.Time
		packetID,
		firstPacketTimeNS,
		lastPacketTimeNS struct {
			min, max uint64
		}

		containedStreamIds map[uint64]uint32
	}

	Stream struct {
		stream
		r     *Reader
		index uint32
	}
	Direction int
	Packet    struct {
		Timestamp    time.Time
		PcapFilename string
		PcapIndex    uint64
		Direction    Direction
	}
	Data struct {
		Direction   Direction
		Content     []byte
		Time        time.Time
		ContentType string `json:",omitempty"`
	}
)

const (
	DirectionClientToServer Direction = 0
	DirectionServerToClient Direction = 1

	ChunkSplitThreshold = 50 * time.Millisecond
)

func (dir Direction) Reverse() Direction {
	return dir ^ DirectionClientToServer ^ DirectionServerToClient
}

func (hg *readerHostGroup) get(id uint16) net.IP {
	return net.IP(hg.hosts[hg.hostSize*int(id):][:hg.hostSize])
}

func (r *Reader) Filename() string {
	return r.filename
}

func (r *Reader) calculateOffset(section section, objectSize, index int) int64 {
	return int64(r.header.Sections[section].Begin) + int64(objectSize*index)
}

func (r *Reader) readAt(offset int64, d interface{}) error {
	s := io.NewSectionReader(r.file, offset, r.size-offset)
	err := binary.Read(s, binary.LittleEndian, d)
	if err != nil {
		debug.PrintStack()
	}
	return err
}

var isLittleEndian bool

func init() {
	isLittleEndian = binary.NativeEndian.Uint16([]byte("AB")) == binary.LittleEndian.Uint16([]byte("AB"))
}

func (r *Reader) streamByIndex(index uint32) (*stream, error) {
	obj := stream{}
	var err error
	var d interface{}
	if isLittleEndian {
		d = (*[unsafe.Sizeof(obj)]byte)(unsafe.Pointer(&obj))
	} else {
		d = obj
	}
	err = r.readAt(r.calculateOffset(sectionStreams, int(unsafe.Sizeof(obj)), int(index)), d)
	return &obj, err
}

func (r *Reader) packetByIndex(index uint64) (*packet, error) {
	obj := packet{}
	var err error
	var d interface{}
	if isLittleEndian {
		d = (*[unsafe.Sizeof(obj)]byte)(unsafe.Pointer(&obj))
	} else {
		d = obj
	}
	err = r.readAt(r.calculateOffset(sectionPackets, int(unsafe.Sizeof(obj)), int(index)), d)
	return &obj, err
}

func (r *Reader) readLookup(lookup section, index int) (uint32, error) {
	streamIndex := uint32(0)
	err := r.readAt(r.calculateOffset(lookup, 4, index), &streamIndex)
	return streamIndex, err
}

func (r *Reader) readObjects(section section, d interface{}) error {
	return r.readAt(r.calculateOffset(section, 0, 0), d)
}

func (r *Reader) objectCount(section section, objectSize int) int {
	return int(r.header.Sections[section].size()) / objectSize
}

func (r *Reader) Close() error {
	return r.file.Close()
}

func NewReader(filename string) (*Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	r := Reader{
		filename:           filename,
		file:               file,
		size:               int64(unsafe.Sizeof(fileHeader{})),
		containedStreamIds: make(map[uint64]uint32),
	}

	if err := func() error {
		// read header
		if err := r.readAt(0, &r.header); err != nil {
			return err
		}
		if string(r.header.Magic[:]) != fileMagic {
			return fmt.Errorf("wrong magic: %q, expected %q", string(r.header.Magic[:]), fileMagic)
		}
		for _, s := range r.header.Sections {
			if uint64(r.size) < s.End {
				r.size = int64(s.End)
			}
		}

		// read imports
		importFilenames := make([]byte, r.header.Sections[sectionImportFilenames].size())
		if err := r.readObjects(sectionImportFilenames, importFilenames); err != nil {
			return err
		}
		importEntries := make([]importEntry, r.header.Sections[sectionImports].size()/int64(unsafe.Sizeof(importEntry{})))
		if err := r.readObjects(sectionImports, importEntries); err != nil {
			return err
		}
		for _, ie := range importEntries {
			null := bytes.IndexByte(importFilenames[ie.Filename:], 0)
			fn := string(importFilenames[ie.Filename : int(ie.Filename)+null])
			r.imports = append(r.imports, readerImportEntry{
				filename:          fn,
				packetIndexOffset: ie.PacketIndexOffset,
			})
		}

		// read hosts
		v4hosts := make([]byte, r.header.Sections[sectionV4Hosts].size())
		if err := r.readObjects(sectionV4Hosts, v4hosts); err != nil {
			return err
		}
		v6hosts := make([]byte, r.header.Sections[sectionV6Hosts].size())
		if err := r.readObjects(sectionV6Hosts, v6hosts); err != nil {
			return err
		}
		hostGroups := make([]hostGroupEntry, r.header.Sections[sectionHostGroups].size()/int64(unsafe.Sizeof(hostGroupEntry{})))
		if err := r.readObjects(sectionHostGroups, hostGroups); err != nil {
			return err
		}
		for _, hg := range hostGroups {
			hosts := []byte(nil)
			hostSize := 0
			switch hg.Flags & flagsHostGroupIPVersion {
			case flagsHostGroupIP4:
				hosts = v4hosts
				hostSize = 4
			case flagsHostGroupIP6:
				hosts = v6hosts
				hostSize = 16
			}
			hostCount := int(hg.Count) + 1
			hosts = hosts[hg.Start:][:hostSize*hostCount]
			r.hostGroups = append(r.hostGroups, readerHostGroup{
				hostCount: hostCount,
				hostSize:  hostSize,
				hosts:     hosts,
			})
		}

		// get times
		r.ReferenceTime = time.Unix(int64(r.header.FirstPacketTime), 0)
		if s, err := r.minStream(sectionStreamsByFirstPacketTime); err != nil {
			return err
		} else {
			r.firstPacketTimeNS.min = s.FirstPacketTimeNS
		}
		if s, err := r.maxStream(sectionStreamsByFirstPacketTime); err != nil {
			return err
		} else {
			r.firstPacketTimeNS.max = s.FirstPacketTimeNS
		}
		if s, err := r.minStream(sectionStreamsByLastPacketTime); err != nil {
			return err
		} else {
			r.lastPacketTimeNS.min = s.LastPacketTimeNS
		}
		if s, err := r.maxStream(sectionStreamsByLastPacketTime); err != nil {
			return err
		} else {
			r.lastPacketTimeNS.max = s.LastPacketTimeNS
		}

		//read all stream id's
		// TODO: optimize
		r.packetID.min = math.MaxUint64
		r.packetID.max = 0
		for i, n := 0, r.StreamCount(); i < n; i++ {
			s, err := r.streamByIndex(uint32(i))
			if err != nil {
				return err
			}
			if r.packetID.min > s.StreamID {
				r.packetID.min = s.StreamID
			}
			if r.packetID.max < s.StreamID {
				r.packetID.max = s.StreamID
			}
			r.containedStreamIds[s.StreamID] = uint32(i)
		}
		return nil
	}(); err != nil {
		r.Close()
		return nil, err
	}

	return &r, nil
}

func (r *Reader) StreamCount() int {
	return r.objectCount(sectionStreams, int(unsafe.Sizeof(stream{})))
}

func (r *Reader) PacketCount() int {
	return r.objectCount(sectionPackets, int(unsafe.Sizeof(packet{})))
}

func (r *Reader) minStream(lookup section) (*stream, error) {
	i, err := r.readLookup(lookup, 0)
	if err != nil {
		return nil, err
	}
	return r.streamByIndex(i)
}

func (r *Reader) maxStream(lookup section) (*stream, error) {
	i, err := r.readLookup(lookup, r.StreamCount()-1)
	if err != nil {
		return nil, err
	}
	return r.streamByIndex(i)
}

func (r *Reader) MinStreamID() uint64 {
	return r.packetID.min
}

func (r *Reader) MaxStreamID() uint64 {
	return r.packetID.max
}

func (r *Reader) StreamIDs() map[uint64]uint32 {
	return r.containedStreamIds
}

func (s stream) wrap(r *Reader, idx uint32) (*Stream, error) {
	return &Stream{
		stream: s,
		index:  idx,
		r:      r,
	}, nil
}

func (r *Reader) StreamByID(streamID uint64) (*Stream, error) {
	if streamID < r.MinStreamID() || streamID > r.MaxStreamID() {
		return nil, nil
	}
	streamIndex, ok := r.containedStreamIds[streamID]
	if !ok {
		return nil, nil
	}
	s, err := r.streamByIndex(streamIndex)
	if err != nil {
		return nil, err
	}
	return s.wrap(r, streamIndex)
}

func (r *Reader) streamIndexByLookup(section section, f func(s *stream) (bool, error)) (uint32, bool, error) {
	var firstError error
	idx := sort.Search(r.StreamCount(), func(i int) bool {
		if firstError != nil {
			return false
		}
		streamIndex, err := r.readLookup(section, i)
		if err != nil {
			firstError = err
			return false
		}
		s, err := r.streamByIndex(streamIndex)
		if err != nil {
			firstError = err
			return false
		}
		res, err := f(s)
		if err != nil {
			firstError = err
			return false
		}
		return res
	})
	if firstError != nil {
		return 0, false, firstError
	}
	if idx >= r.StreamCount() {
		return 0, false, nil
	}
	streamIndex, err := r.readLookup(section, idx)
	if err != nil {
		return 0, false, err
	}
	return streamIndex, true, firstError
}

func (r *Reader) StreamByFirstPacketSource(pcapFilename string, packetIndex uint64) (*Stream, error) {
	firstPacketSource := func(s *stream) (string, uint64, error) {
		p, err := r.packetByIndex(uint64(s.PacketInfoStart))
		if err != nil {
			return "", 0, err
		}
		imp := r.imports[p.ImportID]
		return imp.filename, imp.packetIndexOffset + uint64(p.PacketIndex), nil
	}
	streamIndex, streamFound, err := r.streamIndexByLookup(sectionStreamsByFirstPacketSource, func(s *stream) (bool, error) {
		fn, idx, err := firstPacketSource(s)
		if err != nil {
			return false, err
		}
		if fn != pcapFilename {
			return pcapFilename <= fn, nil
		}
		return packetIndex <= idx, nil
	})
	if err != nil {
		return nil, err
	}
	if !streamFound {
		return nil, nil
	}
	s, err := r.streamByIndex(streamIndex)
	if err != nil {
		return nil, err
	}
	fn, idx, err := firstPacketSource(s)
	if err != nil {
		return nil, err
	}
	if fn != pcapFilename || idx != packetIndex {
		return nil, nil
	}
	return s.wrap(r, streamIndex)
}

func (s *Stream) ID() uint64 {
	return s.StreamID
}

func (s *Stream) Index() uint32 {
	return s.index
}

func (s *Stream) ClientHostIP() string {
	return s.r.hostGroups[s.HostGroup].get(s.ClientHost).String()
}

func (s *Stream) ServerHostIP() string {
	return s.r.hostGroups[s.HostGroup].get(s.ServerHost).String()
}

func (s *Stream) Protocol() string {
	protocols := map[uint16]string{
		flagsStreamProtocolOther: "Other",
		flagsStreamProtocolTCP:   "TCP",
		flagsStreamProtocolUDP:   "UDP",
		flagsStreamProtocolSCTP:  "SCTP",
	}
	return protocols[s.Flags&flagsStreamProtocol]
}

func (s *Stream) Packets() ([]Packet, error) {
	packets := []Packet{}
	lastImportID, lastPacketIndex := -1, -1
	dir := map[uint8]Direction{
		flagsPacketDirectionClientToServer: DirectionClientToServer,
		flagsPacketDirectionServerToClient: DirectionServerToClient,
	}
	refTime := s.FirstPacket()
	lastRelPacketTimeMS := uint32(0)
	for i := uint64(s.PacketInfoStart); ; i++ {
		p, err := s.r.packetByIndex(i)
		if err != nil {
			return nil, err
		}
		if int(p.ImportID) != lastImportID || int(p.PacketIndex) != lastPacketIndex {
			lastImportID = int(p.ImportID)
			lastPacketIndex = int(p.PacketIndex)
			imp := s.r.imports[p.ImportID]
			if p.RelPacketTimeMS < lastRelPacketTimeMS {
				refTime = refTime.Add(time.Microsecond << 32)
			}
			lastRelPacketTimeMS = p.RelPacketTimeMS
			packets = append(packets, Packet{
				PcapFilename: imp.filename,
				PcapIndex:    imp.packetIndexOffset + uint64(p.PacketIndex),
				Direction:    dir[p.Flags&flagsPacketDirection],
				Timestamp:    refTime.Add(time.Duration(p.RelPacketTimeMS) * time.Microsecond),
			})
		}
		if p.Flags&flagsPacketHasNext == 0 {
			break
		}
	}
	return packets, nil
}

func (s *Stream) Data() ([]Data, error) {
	off := int64(s.PacketInfoStart) * int64(unsafe.Sizeof(packet{}))
	sr := io.NewSectionReader(s.r.file, int64(s.r.header.Sections[sectionPackets].Begin)+off, s.r.header.Sections[sectionPackets].size()-off)
	br := bufio.NewReader(sr)
	p := packet{}
	refTime := s.FirstPacket()
	type packetTime struct {
		ts time.Time
		sz uint64
	}
	expectWraps := (time.Duration(s.LastPacketTimeNS-s.FirstPacketTimeNS)*time.Nanosecond + time.Microsecond) / (time.Microsecond << 32)
	packetTimes := [2][]packetTime{nil, nil}
	lastRelPacketTimeMS := uint32(0)
	prevTs := time.Time{}
	prevDir := uint8(0)
	for {
		if err := binary.Read(br, binary.LittleEndian, &p); err != nil {
			return nil, err
		}
		if expectWraps != 0 {
			if p.RelPacketTimeMS < lastRelPacketTimeMS {
				refTime = refTime.Add(time.Microsecond << 32)
				expectWraps--
			}
			lastRelPacketTimeMS = p.RelPacketTimeMS
		}
		if p.DataSize != 0 {
			ts := refTime.Add(time.Duration(p.RelPacketTimeMS) * time.Microsecond)
			dir := ((p.Flags & flagsPacketDirection) / flagsPacketDirection) ^ uint8(DirectionClientToServer) ^ (flagsPacketDirectionClientToServer / flagsPacketDirection)
			ci := &packetTimes[dir]
			if len(*ci) != 0 && dir == prevDir && ts.Sub(prevTs) < ChunkSplitThreshold {
				(*ci)[len(*ci)-1].sz += uint64(p.DataSize)
			} else {
				*ci = append(*ci, packetTime{ts, uint64(p.DataSize)})
			}
			prevTs = ts
			prevDir = dir
		}
		if (p.Flags & flagsPacketHasNext) == 0 {
			break
		}
		if p.SkipPacketsForData != 0 && expectWraps == 0 {
			if _, err := br.Discard(int(p.SkipPacketsForData) * int(unsafe.Sizeof(packet{}))); err != nil {
				return nil, err
			}
		}
	}
	data := []Data{}
	sr = io.NewSectionReader(s.r.file, int64(s.r.header.Sections[sectionData].Begin+s.DataStart), s.r.header.Sections[sectionData].size()-int64(s.DataStart))
	br = bufio.NewReader(sr)

	content := [2][]byte{}
	content[DirectionClientToServer] = make([]byte, s.ClientBytes)
	content[DirectionServerToClient] = make([]byte, s.ServerBytes)
	if err := binary.Read(br, binary.LittleEndian, content[DirectionClientToServer]); err != nil {
		return nil, err
	}
	if err := binary.Read(br, binary.LittleEndian, content[DirectionServerToClient]); err != nil {
		return nil, err
	}

	position := [2]uint64{}
	for dir := DirectionClientToServer; ; dir = dir.Reverse() {
		if position[DirectionClientToServer] == s.ClientBytes && position[DirectionServerToClient] == s.ServerBytes {
			break
		}
		sz := uint64(0)
		for {
			b, err := br.ReadByte()
			if err != nil {
				return nil, err
			}
			sz <<= 7
			sz |= uint64(b & 0x7f)
			if b < 0x80 {
				break
			}
		}
		if sz == 0 {
			continue
		}
		pt := &packetTimes[dir][0]
		for {
			szCur := sz
			if szCur > pt.sz {
				szCur = pt.sz
			}
			data = append(data, Data{
				Direction: dir,
				Content:   content[dir][position[dir]:][:szCur],
				Time:      pt.ts.UTC(),
			})
			position[dir] += szCur
			pt.sz -= szCur
			sz -= szCur
			if sz == 0 {
				break
			}
			if pt.sz == 0 {
				packetTimes[dir] = packetTimes[dir][1:]
				pt = &packetTimes[dir][0]
			}
		}
		if pt.sz == 0 {
			packetTimes[dir] = packetTimes[dir][1:]
		}
	}
	return data, nil
}

func (s *Stream) FirstPacket() time.Time {
	return s.r.ReferenceTime.Add(time.Duration(s.FirstPacketTimeNS) * time.Nanosecond)
}

func (s *Stream) LastPacket() time.Time {
	return s.r.ReferenceTime.Add(time.Duration(s.LastPacketTimeNS) * time.Nanosecond)
}

func (s *Stream) Reader() *Reader {
	return s.r
}

func (s *Stream) MarshalJSON() ([]byte, error) {
	type SideInfo struct {
		Host  string
		Port  uint16
		Bytes uint64
	}
	return json.Marshal(struct {
		ID                      uint64
		Protocol                string
		Client, Server          SideInfo
		FirstPacket, LastPacket time.Time
		Index                   string
	}{
		ID:          s.ID(),
		FirstPacket: s.FirstPacket().Local(),
		LastPacket:  s.LastPacket().Local(),
		Client: SideInfo{
			Host:  s.r.hostGroups[s.HostGroup].get(s.ClientHost).String(),
			Port:  s.ClientPort,
			Bytes: s.ClientBytes,
		},
		Server: SideInfo{
			Host:  s.r.hostGroups[s.HostGroup].get(s.ServerHost).String(),
			Port:  s.ServerPort,
			Bytes: s.ServerBytes,
		},
		Protocol: s.Protocol(),
		Index:    s.r.filename,
	})
}

func (r *Reader) AllStreams(handler func(*Stream) error) error {
	for i := 0; i < r.StreamCount(); i++ {
		s, err := r.streamByIndex(uint32(i))
		if err != nil {
			return err
		}
		ws, err := s.wrap(r, uint32(i))
		if err != nil {
			return err
		}
		if err := handler(ws); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) sectionReader(section section) *io.SectionReader {
	s := r.header.Sections[section]
	return io.NewSectionReader(r.file, int64(s.Begin), s.size())
}

func (d *Data) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return json.Marshal(struct {
			Direction   Direction
			Content     []byte
			ContentType string `json:",omitempty"`
		}{Direction: d.Direction, Content: d.Content, ContentType: d.ContentType})
	} else {
		return json.Marshal(struct {
			Direction   Direction
			Content     []byte
			Time        time.Time
			ContentType string `json:",omitempty"`
		}{Direction: d.Direction, Content: d.Content, Time: d.Time, ContentType: d.ContentType})
	}
}
