package index

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/reassembly"
	"github.com/spq/pkappa2/internal/index/streams"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
)

func TestReader(t *testing.T) {
	tmpDir := t.TempDir()
	streams := map[uint64]streamInfo{}
	for i := 0; i < 10; i++ {
		streams[uint64(i*100)] = makeStream("1.2.3.4:1234", "4.3.2.1:4321", t1.Add(time.Hour*time.Duration(i)), []string{fmt.Sprintf("foo%d", i)})
	}
	idx, err := makeIndex(tmpDir, streams, nil)
	if err != nil {
		t.Fatalf("makeIndex failed: %v", err)
	}
	if got, want := idx.PacketCount(), 30; got != want {
		t.Errorf("Reader.PacketCount() = %v, want %v", got, want)
	}
	gotStreams := idx.StreamIDs()
	if len(gotStreams) != len(streams) {
		t.Fatalf("len(Reader.StreamIDs()) = %v, want %v", len(gotStreams), len(streams))
	}
	for streamID, streamIndex := range gotStreams {
		s1, err := idx.StreamByID(streamID)
		if err != nil {
			t.Fatalf("Reader.StreamByID failed with error: %v", err)
		}
		if s1.Index() != streamIndex {
			t.Errorf("streamIndex mismatch: %v != %v", s1.index, streamIndex)
		}
		if s1.Reader() != idx {
			t.Error("Stream.Reader() returned unexpected value")
		}
		s2, err := idx.streamByIndex(streamIndex)
		if err != nil {
			t.Fatalf("Reader.streamByIndex failed with error: %v", err)
		}
		if s2.StreamID != streamID {
			t.Errorf("streamID mismatch: %v != %v", s2.StreamID, streamID)
		}
		s3, err := idx.StreamByFirstPacketSource(streams[streamID].s.Packets[0].AncillaryData[0].(*pcapmetadata.PcapMetadata).PcapInfo.Filename, 0)
		if err != nil {
			t.Fatalf("Reader.StreamByFirstPacketSource failed with error: %v", err)
		}
		if s3.index != streamIndex {
			t.Errorf("streamIndex mismatch: %v != %v", s3.index, streamIndex)
		}
		packets, err := s1.Packets()
		if err != nil {
			t.Fatalf("Stream.Packets failed with error: %v", err)
		}
		if len(packets) != 3 {
			t.Errorf("len(Stream.Packets()) = %v, want 3", len(packets))
		}
		if got, want := s1.FirstPacket(), t1.Add(time.Hour*time.Duration(streamID/100)).UTC(); !got.Equal(want) {
			t.Errorf("Stream[%d].FirstPacket() = %v, want %v", streamID, got, want)
		}
		if got, want := s1.LastPacket(), t1.Add(time.Hour*time.Duration(streamID/100)+time.Second*3).UTC(); !got.Equal(want) {
			t.Errorf("Stream[%d].LastPacket() = %v, want %v", streamID, got, want)
		}
		if got, want := s1.ClientHostIP(), "1.2.3.4"; got != want {
			t.Errorf("Stream[%d].ClientHostIP() = %v, want %v", streamID, got, want)
		}
		if got, want := s1.ServerHostIP(), "4.3.2.1"; got != want {
			t.Errorf("Stream[%d].ServerHostIP() = %v, want %v", streamID, got, want)
		}
	}
}

func TestLongPackets(t *testing.T) {
	tmpDir := t.TempDir()
	pi := pcapmetadata.PcapInfo{
		Filename:           "foo.pcap",
		Filesize:           123,
		PacketTimestampMin: t1,
		PacketTimestampMax: t1.Add(19 * 4 * time.Minute),
		ParseTime:          t1.Add(2 * time.Hour),
		PacketCount:        3,
	}
	s := streams.Stream{
		ClientAddr: []byte("AAAA"),
		ServerAddr: []byte("BBBB"),
		ClientPort: 123,
		ServerPort: 456,
		Flags:      streams.StreamFlagsProtocolTCP | streams.StreamFlagsComplete,
	}
	for i := 0; i < 40; i++ {
		s.Packets = append(s.Packets, gopacket.CaptureInfo{
			CaptureLength: 123,
			Length:        123,
			Timestamp:     t1.Add(time.Duration(i) * 2 * time.Minute),
			AncillaryData: []interface{}{
				&pcapmetadata.PcapMetadata{
					PcapInfo: &pi,
					Index:    uint64(i),
				},
			},
		})
		if i%2 == 0 {
			b := []byte("A")
			if i == 6 {
				b = make([]byte, 1<<17)
				b[0] = 'A'
				b[len(b)-1] = 'A'
			}
			s.Data = append(s.Data, streams.StreamData{
				Bytes:       b,
				PacketIndex: uint64(i),
			})
		}
		s.PacketDirections = append(s.PacketDirections, reassembly.TCPDirClientToServer)
		s.PacketDirections = append(s.PacketDirections, reassembly.TCPDirClientToServer)
	}
	streams := map[uint64]streamInfo{
		0: {s: s},
	}
	idx, err := makeIndex(tmpDir, streams, nil)
	if err != nil {
		t.Fatalf("makeIndex failed: %v", err)
	}
	var data []Data
	var packets []Packet
	if err := idx.AllStreams(func(s *Stream) error {
		d, err := s.Data()
		if err != nil {
			return err
		}
		p, err := s.Packets()
		if err != nil {
			return err
		}
		data = d
		packets = p
		return nil
	}); err != nil {
		t.Fatalf("Reader.AllStreams failed with error: %v", err)
	}
	if len(data) != 20 {
		t.Fatalf("len(data) = %v, want 20", len(data))
	}
	if len(packets) != 40 {
		t.Fatalf("len(packets) = %v, want 40", len(packets))
	}
	for i := 0; i < 20; i++ {
		wantData := []byte("A")
		if i == 3 {
			wantData = make([]byte, 1<<17)
			wantData[0] = 'A'
			wantData[len(wantData)-1] = 'A'
		}
		if got := data[i].Content; !bytes.Equal(got, wantData) {
			t.Errorf("data[%d].Content = %v, want %v", i, got, wantData)
		}
		if got, want := data[i].Direction, DirectionClientToServer; got != want {
			t.Errorf("data[%d].Direction = %v, want %v", i, got, want)
		}
	}
	for i := 0; i < 40; i++ {
		if got, want := packets[i].PcapIndex, uint64(i); got != want {
			t.Errorf("packets[%d].PcapIndex = %v, want %v", i, got, want)
		}
		if got, want := packets[i].PcapFilename, "foo.pcap"; got != want {
			t.Errorf("packets[%d].PcapFilename = %v, want %v", i, got, want)
		}
		if got, want := packets[i].Direction, DirectionClientToServer; got != want {
			t.Errorf("packets[%d].Direction = %v, want %v", i, got, want)
		}
		if got, want := packets[i].Timestamp, t1.Add(time.Duration(i)*2*time.Minute); !got.Equal(want) {
			t.Errorf("packets[%d].Timestamp = %v, want %v", i, got, want)
		}
	}
}
