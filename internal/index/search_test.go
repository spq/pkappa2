package index

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/reassembly"
	"github.com/spq/pkappa2/internal/index/streams"
	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
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

var (
	t1 time.Time
)

func init() {
	var err error
	t1, err = time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	if err != nil {
		panic(err)
	}
}

func makeStream(client, server string, t time.Time, data []string, converterData ...[]string) streamInfo {
	first := reassembly.TCPDirClientToServer
	if len(data) != 0 && data[0] == "" {
		data = data[1:]
		first = first.Reverse()
	}
	clientAddrPort := netip.MustParseAddrPort(client)
	serverAddrPort := netip.MustParseAddrPort(server)
	t2 := t.Add(time.Second * time.Duration(2+len(data)))
	t3 := t2.Add(time.Minute)

	pcapinfo := &pcapmetadata.PcapInfo{
		Filename: fmt.Sprintf("%s_%s_%d.pcap", client, server, t.UnixNano()),
		Filesize: 123,

		PacketTimestampMin: t,
		PacketTimestampMax: t2,

		ParseTime:   t3,
		PacketCount: uint(len(data)) + 2,
	}
	packets := []gopacket.CaptureInfo(nil)
	packetDirections := []reassembly.TCPFlowDirection(nil)
	packets = append(packets, gopacket.CaptureInfo{
		Timestamp:     t,
		CaptureLength: 123,
		Length:        123,
	})
	packetDirections = append(packetDirections, reassembly.TCPDirClientToServer)
	streamData := []streams.StreamData(nil)
	for i, d := range data {
		packets = append(packets, gopacket.CaptureInfo{
			Timestamp:     t.Add(time.Second * time.Duration(i+1)),
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
			"show issue #53 fixed",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo"}, []string{"bar"}, []string{"baz"}),
			},
			"-cdata:bar",
			[]uint64{},
		},
		{
			"a subquery with data query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle1"}),
				makeStream("192.168.0.100:234", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle2"}),
				makeStream("192.168.0.100:345", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle0 needle1 needle2"}),
			},
			"@sub:cdata.none:\"(?P<var>needle[0-9])\" cdata.none:@sub:var@ id:@sub:id@+1:",
			[]uint64{2},
		},
		{
			"a subquery with negated data query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.100:234", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.100:345", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle2"}),
				makeStream("192.168.0.100:456", "192.168.0.1:80", t1.Add(time.Hour*4), []string{"needle0 needle1 needle2"}),
				makeStream("192.168.0.100:567", "192.168.0.1:80", t1.Add(time.Hour*5), []string{"needle0 missing1 needle2"}),
			},
			"@sub:id:0,1,2 @sub:cdata:\"(?P<var>needle[0-9])\" -cdata:@sub:var@ id:3,4",
			[]uint64{4},
		},
		{
			"a subquery with data query and converters",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle0 missing1"}, []string{"missing0 needle1"}),
			},
			"@sub:id:0,1 @sub:cdata:\"(?P<var>needle[0-9])\" cdata:@sub:var@ id:2",
			[]uint64{2},
		},
		{
			"a subquery with inverted data query and converters",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle0 missing1"}, []string{"missing0 needle1"}),
			},
			"@sub:id:0,1 @sub:cdata:\"(?P<var>needle[0-9])\" -cdata:@sub:var@ id:2",
			[]uint64{},
		},
		{
			"a subquery with data query and converters matching only one subquery stream in one converter",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"foo"}, []string{"needle0"}),
			},
			"@sub:id:0,1 @sub:cdata:\"(?P<var>needle[0-9])\" cdata:@sub:var@ id:2",
			[]uint64{2},
		},
		{
			"test sequence of data connected using then",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle1", "needle2", "needle3"}),
				makeStream("192.168.0.100:234", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle2", "needle3", "needle1"}),
			},
			"cdata:needle1 then sdata:needle2",
			[]uint64{0},
		},
		{
			"test protocol:tcp query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
			},
			"protocol:tcp",
			[]uint64{0},
		},
		{
			"test protocol:udp query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
			},
			"protocol:udp",
			[]uint64{},
		},
		{
			"test ftime query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.101:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.102:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle2"}),
			},
			fmt.Sprintf(`ftime:"%s"`, t1.Add(time.Hour*2).Local().Format("2006-01-02 1504")),
			[]uint64{1},
		},
		{
			"test ltime query",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.101:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.102:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle2"}),
			},
			fmt.Sprintf(`ltime:":%s"`, t1.Add(time.Hour*2).Local().Format("2006-01-02 1504")),
			[]uint64{0},
		},
		{
			"sort by id",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle2"}),
			},
			"sort:id",
			[]uint64{0, 1, 2},
		},
		{
			"sort by -id",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle2"}),
			},
			"sort:-id",
			[]uint64{2, 1, 0},
		},
		{
			"sort by ftime",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle1"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"needle2"}),
			},
			"sort:ftime",
			[]uint64{1, 0, 2},
		},
		{
			"sort by ltime",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"needle0"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle1", "foo"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"needle2"}),
			},
			"sort:ltime",
			[]uint64{2, 1, 0},
		},
		{
			"sort by cbytes",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"AA"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"AAA"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"A"}),
			},
			"sort:cbytes",
			[]uint64{2, 0, 1},
		},
		{
			"sort by sbytes",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo", "A"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"foo", "AAA"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"foo", "AA"}),
			},
			"sort:sbytes",
			[]uint64{0, 2, 1},
		},
		{
			"sort by cport",
			[]streamInfo{
				makeStream("192.168.0.100:2", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("192.168.0.100:1", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("192.168.0.100:3", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"sort:cport",
			[]uint64{1, 0, 2},
		},
		{
			"sort by sport",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:3", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("192.168.0.100:123", "192.168.0.1:1", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("192.168.0.100:123", "192.168.0.1:2", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"sort:sport",
			[]uint64{1, 2, 0},
		},
		{
			"sort by chost",
			[]streamInfo{
				makeStream("192.168.0.102:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("192.168.0.101:123", "192.168.0.1:80", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"sort:chost",
			[]uint64{2, 1, 0},
		},
		{
			"sort by shost",
			[]streamInfo{
				makeStream("192.168.0.100:123", "192.168.0.1:80", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("192.168.0.100:123", "192.168.0.2:80", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("192.168.0.100:123", "192.168.0.3:80", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"sort:shost",
			[]uint64{0, 1, 2},
		},
		{
			"sort by multiple",
			[]streamInfo{
				makeStream("192.168.0.100:2", "192.168.0.1:1", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("192.168.0.100:1", "192.168.0.1:1", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("192.168.0.100:1", "192.168.0.1:2", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"sort:cport,sport",
			[]uint64{1, 2, 0},
		},
		{
			"impossible filter",
			[]streamInfo{
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*1), []string{"foo"}),
			},
			"id:123",
			nil,
		},
		{
			"partially impossible filter",
			[]streamInfo{
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*1), []string{"foo"}),
			},
			"id:123 or id::100",
			[]uint64{0},
		},
		{
			"sort with allowed early exit",
			[]streamInfo{
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"sort:id limit:2",
			[]uint64{0, 1},
		},
		{
			"sort with allowed early exit",
			[]streamInfo{
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*1), []string{"foo"}),
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*2), []string{"foo"}),
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*3), []string{"foo"}),
			},
			"id:0:2 sort:id limit:2",
			[]uint64{0, 1},
		},
		{
			"impossible filter supporting lookup",
			[]streamInfo{
				makeStream("1.2.3.4:1234", "1.2.3.4:1234", t1.Add(time.Hour*1), []string{"foo"}),
			},
			"id:123: limit:2",
			nil,
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
			if err != nil {
				t.Fatalf("Error creating index: %v", err)
			}
			t.Logf("using query %q", tc.query)
			q, err := query.Parse(tc.query)
			if err != nil {
				t.Errorf("Error parsing query: %v", err)
			}
			l := uint(100)
			if q.Limit != nil {
				l = *q.Limit
			}
			results, _, _, err := SearchStreams(context.Background(), []*Reader{r}, nil, q.ReferenceTime, q.Conditions, q.Grouping, q.Sorting, l, 0, nil, converters, false)
			if err != nil {
				t.Fatalf("Error searching streams: %v", err)
			}
			got := []uint64(nil)
			for _, s := range results {
				got = append(got, s.StreamID)
			}
			if !slices.Equal(got, tc.expected) {
				t.Errorf("Unexpected streams: %v, want: %v", got, tc.expected)
			}
		})
	}
}

func TestSearch(t *testing.T) {

}
