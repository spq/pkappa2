package index

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/reassembly"
	"github.com/spq/pkappa2/internal/index/streams"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
	"golang.org/x/exp/slices"
)

type (
	fakeConverter struct {
		data map[uint64][]string
	}
	streamInfo struct {
		s streams.Stream
		c [][]string
	}
)

func makeStream(client, server string, t1 time.Time, data []string, converterData ...[]string) streamInfo {
	first := reassembly.TCPDirClientToServer
	if len(data) != 0 && data[0] == "" {
		data = data[1:]
		first = first.Reverse()
	}
	clientAddrPort := netip.MustParseAddrPort(client)
	serverAddrPort := netip.MustParseAddrPort(server)
	t2 := t1.Add(time.Second * time.Duration(2+len(data)))
	t3 := t2.Add(time.Minute)

	pcapinfo := &pcapmetadata.PcapInfo{
		Filename: "test.pcap",
		Filesize: 123,

		PacketTimestampMin: t1,
		PacketTimestampMax: t2,

		ParseTime:   t3,
		PacketCount: uint(len(data)) + 2,
	}
	packets := []gopacket.CaptureInfo(nil)
	packetDirections := []reassembly.TCPFlowDirection(nil)
	packets = append(packets, gopacket.CaptureInfo{
		Timestamp:     t1,
		CaptureLength: 123,
		Length:        123,
	})
	packetDirections = append(packetDirections, reassembly.TCPDirClientToServer)
	streamData := []streams.StreamData(nil)
	for i, d := range data {
		packets = append(packets, gopacket.CaptureInfo{
			Timestamp:     t1.Add(time.Second * time.Duration(i+1)),
			CaptureLength: 123,
			Length:        123,
		})
		streamData = append(streamData, streams.StreamData{
			Bytes:       []byte(d),
			PacketIndex: uint64(i + 1),
		})
		packetDirections = append(packetDirections, first)
		first = first.Reverse()
	}
	packets = append(packets, gopacket.CaptureInfo{
		Timestamp:     t2,
		CaptureLength: 123,
		Length:        123,
	})
	packetDirections = append(packetDirections, reassembly.TCPDirClientToServer)
	for i := range packets {
		pcapmetadata.AddPcapMetadata(&packets[i], pcapinfo, uint64(i))
	}
	return streamInfo{
		s: streams.Stream{
			ClientAddr: clientAddrPort.Addr().AsSlice(),
			ServerAddr: serverAddrPort.Addr().AsSlice(),
			ClientPort: clientAddrPort.Port(),
			ServerPort: serverAddrPort.Port(),

			Packets:          packets,
			PacketDirections: packetDirections,

			Data:  streamData,
			Flags: streams.StreamFlagsComplete | streams.StreamFlagsProtocolTCP,
		},
		c: converterData,
	}
}

func makeIndex(tmpDir string, streams map[uint64]streamInfo, converters *map[string]ConverterAccess) (*Reader, error) {
	w, err := NewWriter(tools.MakeFilename(tmpDir, "idx"))
	if err != nil {
		return nil, err
	}
	for streamID, si := range streams {
		ok, err := w.AddStream(&si.s, streamID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("Stream couldn't be added to index")
		}
		for i, d := range streams[streamID].c {
			if d == nil {
				continue
			}
			c := fmt.Sprintf("c%d", i)
			if _, ok := (*converters)[c]; !ok {
				(*converters)[c] = &fakeConverter{
					data: make(map[uint64][]string),
				}
			}
			(*converters)[c].(*fakeConverter).data[streamID] = d
		}
	}
	r, err := w.Finalize()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (c *fakeConverter) Data(stream *Stream, moreDetails bool) (data []Data, clientBytes, serverBytes uint64, wasCached bool, err error) {
	return nil, 0, 0, false, nil
}

func (c *fakeConverter) DataForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, bool, error) {
	const (
		C2S = query.DataRequirementSequenceFlagsDirectionClientToServer / query.DataRequirementSequenceFlagsDirection
		S2C = query.DataRequirementSequenceFlagsDirectionServerToClient / query.DataRequirementSequenceFlagsDirection
	)

	d, ok := c.data[streamID]
	if !ok {
		return [2][]byte{}, [][2]int{}, 0, 0, false, nil
	}
	data := [2][]byte{}
	dataSizes := [][2]int{{}}
	dir := C2S
	for _, s := range d {
		data[dir] = append(data[dir], []byte(s)...)
		dataSizes = append(dataSizes, [2]int{len(data[0]), len(data[1])})
		dir = (C2S ^ S2C) - dir
	}
	return data, dataSizes, 123, 123, true, nil
}

func TestSearchStreams(t *testing.T) {
	tmpDir := t.TempDir()
	t1, err := time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse failed with error: %v", err)
	}
	testCases := []struct {
		name     string
		streams  []streamInfo
		query    string
		expected []uint64
	}{
		{
			"simple query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
			},
			"sport:80",
			[]uint64{0},
		},
		{
			"simple query with no results",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
			},
			"sport:456",
			[]uint64{},
		},
		{
			"chost query",
			[]streamInfo{
				makeStream("192.168.0.1:123", "192.168.0.100:80", t1.Add(time.Hour), []string{"hello", "world"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
				makeStream("192.168.0.1:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
			},
			"chost:192.168.0.100",
			[]uint64{1},
		},
		{
			"shost query",
			[]streamInfo{
				makeStream("192.168.0.1:123", "192.168.0.100:80", t1.Add(time.Hour), []string{"hello", "world"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
				makeStream("192.168.0.1:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
			},
			"shost:192.168.0.100",
			[]uint64{0},
		},
		{
			"cdata query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"hello", "world"}),
			},
			"cdata:hello",
			[]uint64{0},
		},
		{
			"sdata query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"foo", "bar needle bar", "baz"}),
			},
			"sdata:ne*dle",
			[]uint64{0},
		},
		{
			"negated data query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"foo", "bar needle bar", "baz"}),
			},
			"-sdata:ne*dle",
			[]uint64{},
		},
		{
			"negated data query with explicit none converter",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour), []string{"foo", "bar needle bar", "baz"}),
			},
			"-sdata.none:ne*dle",
			[]uint64{},
		},
		{
			"data query using a specific converter",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo", "bar", "baz"}, []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"foo", "bar", "baz"}, nil, []string{"needle2"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"foo", "bar", "baz"}, nil, nil, []string{"needle3"}),
			},
			"cdata.c1:ne*dle",
			[]uint64{1},
		},
		{
			"search for data only found in converter without searching in converter",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo", "bar", "baz"}, []string{"needle", "needle"}),
			},
			"cdata.none:ne*dle",
			[]uint64{},
		},
		{
			"show issue #53 (stream should not be returned but it is)",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo"}, []string{"bar"}, []string{"baz"}),
			},
			"-cdata:bar",
			[]uint64{0},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			converters := map[string]ConverterAccess{
				"dummy": &fakeConverter{},
			}
			streamsMap := make(map[uint64]streamInfo)
			for i, s := range tc.streams {
				streamsMap[uint64(i)] = s
			}
			r, err := makeIndex(tmpDir, streamsMap, &converters)
			q, err := query.Parse(tc.query)
			if err != nil {
				t.Errorf("Error parsing query: %v", err)
			}
			results, _, err := SearchStreams(context.Background(), []*Reader{r}, nil, q.ReferenceTime, q.Conditions, q.Grouping, q.Sorting, 100, 0, nil, converters)
			if err != nil {
				t.Errorf("Error searching streams: %v", err)
			}
			got := []uint64(nil)
			for _, s := range results {
				got = append(got, s.StreamID)
			}
			if !slices.Equal(got, tc.expected) {
				t.Errorf("Unexpected streams: %v", got)
			}
		})
	}
}
