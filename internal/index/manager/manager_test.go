package manager

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"github.com/spq/pkappa2/internal/index/converters"
	"github.com/spq/pkappa2/internal/query"
)

type (
	dirs struct {
		base, pcap, index, snapshot, state, converter string
	}
)

var (
	t1 time.Time

	//go:embed testdata/test_converter.py
	converterScript []byte
)

func init() {
	var err error
	t1, err = time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	if err != nil {
		panic(err)
	}
}

func makeTempdirs(t *testing.T) dirs {
	dirs := dirs{
		base: t.TempDir(),
	}
	dirs.pcap = path.Join(dirs.base, "pcap") + "/"
	dirs.index = path.Join(dirs.base, "index") + "/"
	dirs.state = path.Join(dirs.base, "state") + "/"
	dirs.snapshot = path.Join(dirs.base, "snapshot") + "/"
	dirs.converter = path.Join(dirs.base, "converter") + "/"
	for _, p := range []string{dirs.pcap, dirs.index, dirs.snapshot, dirs.state, dirs.converter} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatalf("Mkdir(%q) failed with error: %v", p, err)
		}
	}
	return dirs
}

func addConverter(dirs dirs, name string) {
	if err := os.WriteFile(path.Join(dirs.converter, name), []byte(converterScript), 0775); err != nil {
		panic(err)
	}
}

func makeManager(t *testing.T, dirs dirs) *Manager {
	mgr, err := New(dirs.pcap, dirs.index, dirs.snapshot, dirs.state, dirs.converter)
	if err != nil {
		t.Fatalf("manager.New failed with error: %v", err)
	}
	return mgr
}

func TestEmptyManager(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	if got := mgr.Status(); !reflect.DeepEqual(got, Statistics{}) {
		t.Fatalf("Status() = %v, want {}", got)
	}
	mgr.Close()
}

func makeUDPPacket(client, server string, t time.Time, payload string) pcapOverIPPacket {
	clientAddrPort := netip.MustParseAddrPort(client)
	serverAddrPort := netip.MustParseAddrPort(server)
	if !(clientAddrPort.Addr().Is4() && serverAddrPort.Addr().Is4()) {
		panic("only support v4 for now")
	}
	ip := layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    clientAddrPort.Addr().AsSlice(),
		DstIP:    serverAddrPort.Addr().AsSlice(),
		Protocol: layers.IPProtocolUDP,
	}
	udp := layers.UDP{
		SrcPort: layers.UDPPort(clientAddrPort.Port()),
		DstPort: layers.UDPPort(serverAddrPort.Port()),
	}
	if err := udp.SetNetworkLayerForChecksum(&ip); err != nil {
		panic(err)
	}

	options := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	buffer := gopacket.NewSerializeBuffer()

	err := gopacket.SerializeLayers(buffer, options,
		&ip,
		&udp,
		gopacket.Payload([]byte(payload)),
	)
	if err != nil {
		panic(err)
	}
	data := buffer.Bytes()
	return pcapOverIPPacket{
		linkType: layers.LinkTypeIPv4,
		ci: gopacket.CaptureInfo{
			Timestamp:     t,
			CaptureLength: len(data),
			Length:        0xffff,
		},
		data: data,
	}
}

func TestTags(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	if got, want := mgr.ListTags(), []TagInfo{}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Manager.ListTags() = %v, want %v", got, want)
	}
	testcases := []struct {
		tag         string
		query       string
		expectError bool
	}{
		{
			tag:         "foo",
			query:       "id:1",
			expectError: true,
		},
		{
			tag:   "tag/foo",
			query: "id:1",
		},
		{
			tag:   "service/foo",
			query: "id:1",
		},
		{
			tag:   "mark/foo",
			query: "id:1",
		},
		{
			tag:         "mark/foo",
			query:       "port:1",
			expectError: true,
		},
		{
			tag:   "generated/foo",
			query: "id:1",
		},
		{
			tag:         "tag/foo",
			query:       "foo",
			expectError: true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.tag, func(t *testing.T) {
			err := mgr.AddTag(tc.tag, "blue", tc.query)
			if tc.expectError {
				if err == nil {
					t.Fatalf("Manager.AddTag succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Manager.AddTag failed with error: %v", err)
			}
			if got, want := mgr.ListTags(), []TagInfo{{
				Name:           tc.tag,
				Color:          "blue",
				Definition:     tc.query,
				MatchingCount:  0,
				UncertainCount: 0,
				Referenced:     false,
				Converters:     []string{},
			}}; !reflect.DeepEqual(got, want) {
				t.Fatalf("Manager.ListTags() = %v, want %v", got, want)
			}
			if err := mgr.DelTag(tc.tag); err != nil {
				t.Fatalf("Manager.DelTag failed with error: %v", err)
			}
		})
	}
	if err := mgr.AddTag("service/foo", "red", "cport:2,3"); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	importSomePackets(t, mgr, t1, "tagEvaluated")
	if got := mgr.ListTags()[0]; got.MatchingCount != 2 || got.UncertainCount != 0 {
		t.Fatalf("Manager.ListTags()[0] = %+v, want {MatchingCount: 2, UncertainCount: 0}", got)
	}
	if err := mgr.DelTag("service/foo"); err != nil {
		t.Fatalf("Manager.DelTag failed with error: %v", err)
	}
	if err := mgr.AddTag("mark/foo", "blue", "id:0"); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	if err := mgr.UpdateTag("mark/foo", UpdateTagOperationMarkAddStream([]uint64{2, 3})); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	if got, want := mgr.ListTags()[0].Definition, "id:0,2,3"; got != want {
		t.Fatalf("Manager.ListTags()[0].Definition = %v, want %v", got, want)
	}
	if err := mgr.UpdateTag("mark/foo", UpdateTagOperationMarkDelStream([]uint64{2})); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	if got, want := mgr.ListTags()[0].Definition, "id:0,3"; got != want {
		t.Fatalf("Manager.ListTags()[0].Definition = %v, want %v", got, want)
	}
	if err := mgr.UpdateTag("mark/foo", UpdateTagOperationUpdateName("mark/bar")); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	if got, want := mgr.ListTags()[0].Name, "mark/bar"; got != want {
		t.Fatalf("Manager.ListTags()[0].Name = %v, want %v", got, want)
	}
	if err := mgr.UpdateTag("mark/bar", UpdateTagOperationUpdateColor("red")); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	if got, want := mgr.ListTags()[0].Color, "red"; got != want {
		t.Fatalf("Manager.ListTags()[0].Color = %v, want %v", got, want)
	}
	if err := mgr.AddTag("tag/foo", "blue", "port:123"); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	if err := mgr.AddTag("tag/bar", "blue", "tag:foo"); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	if err := mgr.DelTag("tag/foo"); err == nil {
		t.Fatalf("Manager.DelTag succeeded, want error")
	}
	if err := mgr.UpdateTag("tag/bar", UpdateTagOperationUpdateQuery("port:123")); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	if err := mgr.DelTag("tag/foo"); err != nil {
		t.Fatalf("Manager.DelTag failed with error: %v", err)
	}
	if err := mgr.DelTag("tag/bar"); err != nil {
		t.Fatalf("Manager.DelTag failed with error: %v", err)
	}
	if err := mgr.AddTag("tag/foo", "blue", "tag:foo"); err == nil {
		t.Fatalf("Manager.DelTag succeeded, want error")
	}
}

func TestManagerRestartKeepsState(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	if err := mgr.AddTag("tag/foo", "red", "port:123"); err != nil {
		mgr.Close()
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	if err := mgr.AddTag("mark/foo", "red", "id:-1"); err != nil {
		mgr.Close()
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	mgr.Close()
	mgr = makeManager(t, dirs)
	if got, want := mgr.ListTags(), []TagInfo{
		{
			Name:           "mark/foo",
			Color:          "red",
			Definition:     "id:-1",
			MatchingCount:  0,
			UncertainCount: 0,
			Referenced:     false,
			Converters:     []string{},
		},
		{
			Name:           "tag/foo",
			Color:          "red",
			Definition:     "port:123",
			MatchingCount:  0,
			UncertainCount: 0,
			Referenced:     false,
			Converters:     []string{},
		},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Manager.ListTags() = %v, want %v", got, want)
	}
	defer mgr.Close()
}

func TestManagerPcapOverIP(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	if err := mgr.AddPcapOverIPEndpoint("foo"); err == nil {
		t.Fatalf("Manager.AddPcapOverIPEndpoint succeeded, want error")
	}
	if err := mgr.DelPcapOverIPEndpoint("foo"); err == nil {
		t.Fatalf("Manager.DelPcapOverIPEndpoint succeeded, want error")
	}
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenTCP failed with error: %v", err)
	}
	events, eventsCloser := mgr.Listen()
	if err := mgr.AddPcapOverIPEndpoint(listener.Addr().String()); err != nil {
		t.Fatalf("Manager.AddPcapOverIPEndpoint failed: %v", err)
	}
	if err := mgr.AddPcapOverIPEndpoint(listener.Addr().String()); err == nil {
		t.Fatalf("Manager.AddPcapOverIPEndpoint succeeded, want error")
	}
	conn, err := listener.AcceptTCP()
	if err != nil {
		t.Fatalf("listener.AcceptTCP failed with error: %v", err)
	}
	defer conn.Close()
	pkt := makeUDPPacket("1.2.3.4:1234", "5.6.7.8:5678", time.Now(), "foo9001")
	wr := pcapgo.NewWriter(conn)
	if err := wr.WriteFileHeader(0xffff, pkt.linkType); err != nil {
		t.Fatalf("pcapgo.NewWriter failed with error: %v", err)
	}
	if err := wr.WritePacket(pkt.ci, pkt.data); err != nil {
		t.Fatalf("pcapgo.Writer.WritePacket failed with error: %v", err)
	}
	waitForEvent(t, events, eventsCloser, "pcapProcessed")
	if got := mgr.ListPcapOverIPEndpoints(); len(got) != 1 || got[0].ReceivedPackets != 1 || got[0].LastConnected == 0 {
		t.Fatalf("Manager.ListPcapOverIPEndpoints() = %v, want [{ReceivedPackets:1, LastConnected:non-zero}]", got)
	}
	v := mgr.GetView()
	c, err := v.Stream(0)
	if err != nil {
		t.Fatalf("View.Stream failed with error: %v", err)
	}
	d, err := c.Data("")
	if err != nil {
		t.Fatalf("Stream.Data failed with error: %v", err)
	}
	if len(d) != 1 || string(d[0].Content) != "foo9001" {
		t.Fatalf("Stream.Data = %v, want [{Content:foo}]", d)
	}
	if got := mgr.KnownPcaps(); len(got) != 1 || got[0].PacketCount != 1 {
		t.Fatalf("Manager.KnownPcaps() = %+v, want [{PacketCount:1}]", got)
	}
	defer v.Release()
	if err := mgr.DelPcapOverIPEndpoint(listener.Addr().String()); err != nil {
		t.Fatalf("Manager.DelPcapOverIPEndpoint failed: %v", err)
	}
}

func importSomePackets(t *testing.T, mgr *Manager, t1 time.Time, eventType string) {
	pcaps, err := writePcaps(mgr.PcapDir, []pcapOverIPPacket{
		makeUDPPacket("1.2.3.4:1", "4.3.2.1:4321", t1.Add(time.Second*0), "foo"),
		makeUDPPacket("1.2.3.4:2", "4.3.2.1:4321", t1.Add(time.Second*1), "bar"),
		makeUDPPacket("1.2.3.4:3", "4.3.2.1:4321", t1.Add(time.Second*2), "baz"),
		makeUDPPacket("1.2.3.4:4", "4.3.2.1:4321", t1.Add(time.Second*3), "qux"),
	})
	if err != nil {
		t.Fatalf("writePcaps failed with error: %v", err)
	}
	if eventType != "" {
		events, eventCloser := mgr.Listen()
		mgr.ImportPcaps(pcaps)
		waitForEvent(t, events, eventCloser, eventType)
	} else {
		mgr.ImportPcaps(pcaps)
	}
}

func TestWebhooks(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	if got := mgr.ListPcapProcessorWebhooks(); len(got) != 0 {
		t.Fatalf("Manager.ListPcapProcessorWebhooks() = %v, want []", got)
	}
	receivedPcaps := make(chan []string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			if _, err := rw.Write([]byte(err.Error())); err != nil {
				panic(err)
			}
			return
		}
		res := []string(nil)
		if err := json.Unmarshal(data, &res); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			if _, err := rw.Write([]byte(err.Error())); err != nil {
				panic(err)
			}
			return
		}
		if _, err := rw.Write([]byte("ok")); err != nil {
			panic(err)
		}
		receivedPcaps <- res
		close(receivedPcaps)
	}))
	defer server.Close()

	if err := mgr.AddPcapProcessorWebhook(server.URL); err != nil {
		t.Fatalf("Manager.AddPcapProcessorWebhook failed with error: %v", err)
	}
	if err := mgr.AddPcapProcessorWebhook(server.URL); err == nil {
		t.Fatalf("Manager.AddPcapProcessorWebhook succeeded, want error")
	}
	if err := mgr.DelPcapProcessorWebhook("foo"); err == nil {
		t.Fatalf("Manager.DelPcapProcessorWebhook succeeded, want error")
	}
	if got, want := mgr.ListPcapProcessorWebhooks(), []string{server.URL}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Manager.ListPcapProcessorWebhooks() = %v, want %v", got, want)
	}
	importSomePackets(t, mgr, t1, "")
	if got, want := <-receivedPcaps, []string{path.Join(mgr.PcapDir, mgr.KnownPcaps()[0].Filename)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("receivedPcaps = %v, want %v", got, want)
	}
	if got, want := mgr.ListPcapProcessorWebhooks(), []string{server.URL}; !slices.Equal(got, want) {
		t.Fatalf("Manager.ListPcapProcessorWebhooks() = %v, want %v", got, want)
	}
	if err := mgr.DelPcapProcessorWebhook(server.URL); err != nil {
		t.Fatalf("Manager.DelPcapProcessorWebhook failed with error: %v", err)
	}
	if got := mgr.ListPcapProcessorWebhooks(); len(got) != 0 {
		t.Fatalf("Manager.ListPcapProcessorWebhooks() = %v, want []", got)
	}
}

func TestManagerMerging(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	events, eventsCloser := mgr.Listen()
	defer eventsCloser()
	for i := 0; i < 10; i++ {
		pcaps, err := writePcaps(mgr.PcapDir, []pcapOverIPPacket{
			makeUDPPacket(fmt.Sprintf("9.0.0.%d:123", i), "2.3.4.5:9001", t1.Add(time.Second*time.Duration(i)), "foo"),
		})
		if err != nil {
			t.Fatalf("writePcaps failed with error: %v", err)
		}
		mgr.ImportPcaps(pcaps)
		waitForEvent(t, events, func() {}, "pcapProcessed")
	}
	waitForEvent(t, events, eventsCloser, "indexesMerged")
}

func TestManagerView(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	importSomePackets(t, mgr, t1, "pcapProcessed")
	if err := mgr.AddTag("tag/foo", "red", ""); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	view := mgr.GetView()
	defer view.Release()
	if err := view.AllStreams(context.Background(), func(sc StreamContext) error {
		return nil
	}, PrefetchAllTags()); err != nil {
		t.Fatalf("View.AllStreams failed with error: %v", err)
	}
	rt, err := view.ReferenceTime()
	if err != nil {
		t.Fatalf("View.ReferenceTime failed with error: %v", err)
	}
	if rt.UTC() != t1.UTC() {
		t.Fatalf("View.ReferenceTime = %v, want %v", rt, t1)
	}
	sc, err := view.Stream(0)
	if err != nil {
		t.Fatalf("View.Stream failed with error: %v", err)
	}
	if got := sc.Stream().ClientPort; got != 1 {
		t.Fatalf("StreamContext.Stream().ClientPort = %v, want 1", got)
	}
	if got, err := sc.HasTag("foo"); err != nil || got {
		t.Fatalf("StreamContext.HasTag(\"foo\") = %v, %v, want false, nil", got, err)
	}
	if got, err := sc.AllTags(); err != nil || len(got) != 1 || got[0] != "tag/foo" {
		t.Fatalf("StreamContext.AllTags() = %v, %v, want [], nil", got, err)
	}
	if _, err := sc.Data("bar"); err == nil {
		t.Fatalf("StreamContext.Data(\"bar\") succeeded, want error")
	}
	if got, err := sc.Data(""); err != nil || len(got) != 1 || string(got[0].Content) != "foo" {
		t.Fatalf("StreamContext.Data(\"\") = %+v, %v, want [{Content:foo}], nil", got, err)
	}
	if got, err := sc.AllConverters(); err != nil || len(got) != 0 {
		t.Fatalf("StreamContext.AllConverters() = %v, %v, want [], nil", got, err)
	}
	q, err := query.Parse("")
	if err != nil {
		t.Fatalf("query.Parse failed: %v", err)
	}
	if err := mgr.AddTag("tag/bar", "red", ""); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	if m, n, err := view.SearchStreams(context.Background(), q, func(StreamContext) error {
		return nil
	}, Limit(1, 1), PrefetchAllTags()); err != nil || n != 1 || !m {
		t.Fatalf("View.SearchStreams() = %v, %v, %v, want true, 1, nil", m, n, err)
	}
}

func waitForEvent(t *testing.T, listener <-chan Event, listenerCloser func(), eventType string) {
	for e := range listener {
		t.Logf("event: %+v\n", e)
		if e.Type == eventType {
			break
		}
	}
	if listenerCloser != nil {
		listenerCloser()
	}
}

func TestConverters(t *testing.T) {
	dirs := makeTempdirs(t)
	addConverter(dirs, "foo")
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	if got := mgr.ListConverters(); len(got) != 1 || got[0].Name != "foo" {
		gotReadable := []converters.Statistics(nil)
		for _, s := range got {
			gotReadable = append(gotReadable, *s)
		}
		t.Fatalf("Manager.ListConverters() = %v, want [{Name:foo}]", gotReadable)
	}
	listener, listenerCloser := mgr.Listen()
	addConverter(dirs, "bar")
	waitForEvent(t, listener, listenerCloser, "converterAdded")
	if got := mgr.ListConverters(); len(got) != 2 || got[0].Name != "bar" || got[1].Name != "foo" {
		gotReadable := []converters.Statistics(nil)
		for _, s := range got {
			gotReadable = append(gotReadable, *s)
		}
		t.Fatalf("Manager.ListConverters() = %v, want [{Name:bar}, {Name:foo}]", gotReadable)
	}
	if err := mgr.ResetConverter("foo"); err != nil {
		t.Fatalf("Manager.ResetConverter failed with error: %v", err)
	}
	if err := mgr.ResetConverter("baz"); err == nil {
		t.Fatalf("Manager.ResetConverter succeeded, want error")
	}
	listener, listenerCloser = mgr.Listen()
	defer listenerCloser()
	if err := os.Remove(path.Join(dirs.converter, "bar")); err != nil {
		t.Fatalf("os.Remove failed with error: %v", err)
	}
	waitForEvent(t, listener, nil, "converterDeleted")
	if got := mgr.ListConverters(); len(got) != 1 || got[0].Name != "foo" {
		gotReadable := []converters.Statistics(nil)
		for _, s := range got {
			gotReadable = append(gotReadable, *s)
		}
		t.Fatalf("Manager.ListConverters() = %v, want [{Name:foo}]", gotReadable)
	}
	importSomePackets(t, mgr, t1, "pcapProcessed")
	if err := mgr.AddTag("tag/foo", "red", ""); err != nil {
		t.Fatalf("Manager.AddTag failed with error: %v", err)
	}
	if err := mgr.UpdateTag("tag/foo", UpdateTagOperationSetConverter([]string{"foo"})); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	waitForEvent(t, listener, nil, "converterCompleted")
	view := mgr.GetView()
	defer view.Release()
	if err := view.AllStreams(context.Background(), func(sc StreamContext) error {
		data, err := sc.Data("foo")
		if err != nil {
			return err
		}
		if len(data) != 1 || !strings.Contains(string(data[0].Content), fmt.Sprintf("\"StreamID\": %d", sc.Stream().ID())) {
			t.Log(string(data[0].Content))
			return fmt.Errorf("StreamContext.Data(\"foo\") = %v, want [{Content:foo}]", data)
		}
		if got, err := sc.AllConverters(); err != nil || len(got) != 1 || got[0] != "foo" {
			t.Fatalf("StreamContext.AllConverters returned: %v, %v, want [foo], nil", got, err)
		}
		return nil
	}, PrefetchTags([]string{"tag/foo"})); err != nil {
		t.Fatalf("View.AllStreams failed with error: %v", err)
	}
	if err := mgr.UpdateTag("tag/foo", UpdateTagOperationSetConverter(nil)); err != nil {
		t.Fatalf("Manager.UpdateTag failed with error: %v", err)
	}
	if got := mgr.ListTags(); len(got) != 1 || len(got[0].Converters) != 0 {
		t.Fatalf("ListTags returned %v, want [{Converters: []}]", got)
	}
}
