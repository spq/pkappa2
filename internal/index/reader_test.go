package index

import (
	"fmt"
	"testing"
	"time"

	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
)

func TestReader(t *testing.T) {
	tmpDir := t.TempDir()
	t1, err := time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse failed with error: %v", err)
	}
	streams := map[uint64]streamInfo{}
	for i := 0; i < 10; i++ {
		streams[uint64(i*100)] = makeStream("1.2.3.4:1234", "4.3.2.1:4321", t1.Add(time.Hour*time.Duration(i)), []string{fmt.Sprintf("foo%d", i)})
	}
	idx, err := makeIndex(tmpDir, streams, nil)
	if err != nil {
		t.Fatalf("makeIndex failed: %v", err)
	}
	if got, want := idx.PacketCount(), 30; got != want {
		t.Fatalf("Reader.PacketCount() = %v, want %v", got, want)
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
		if s1.index != streamIndex {
			t.Fatalf("streamIndex mismatch: %v != %v", s1.index, streamIndex)
		}
		s2, err := idx.streamByIndex(streamIndex)
		if err != nil {
			t.Fatalf("Reader.streamByIndex failed with error: %v", err)
		}
		if s2.StreamID != streamID {
			t.Fatalf("streamID mismatch: %v != %v", s2.StreamID, streamID)
		}
		s3, err := idx.StreamByFirstPacketSource(streams[streamID].s.Packets[0].AncillaryData[0].(*pcapmetadata.PcapMetadata).PcapInfo.Filename, 0)
		if err != nil {
			t.Fatalf("Reader.StreamByFirstPacketSource failed with error: %v", err)
		}
		if s3.index != streamIndex {
			t.Fatalf("streamIndex mismatch: %v != %v", s3.index, streamIndex)
		}
		packets, err := s1.Packets()
		if err != nil {
			t.Fatalf("Stream.Packets failed with error: %v", err)
		}
		if len(packets) != 3 {
			t.Fatalf("len(Stream.Packets()) = %v, want 3", len(packets))
		}
	}
}
