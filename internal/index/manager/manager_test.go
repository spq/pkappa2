package manager

import (
	"net/netip"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type (
	dirs struct {
		base, pcap, index, snapshot, state, converter string
	}
)

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
	udp.SetNetworkLayerForChecksum(&ip)

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
	t1, err := time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse failed with error: %v", err)
	}
	pcaps, err := writePcaps(mgr.PcapDir, []pcapOverIPPacket{
		makeUDPPacket("1.2.3.4:1", "4.3.2.1:4321", t1.Add(time.Second*1), "foo"),
		makeUDPPacket("1.2.3.4:2", "4.3.2.1:4321", t1.Add(time.Second*2), "bar"),
		makeUDPPacket("1.2.3.4:3", "4.3.2.1:4321", t1.Add(time.Second*3), "baz"),
		makeUDPPacket("1.2.3.4:4", "4.3.2.1:4321", t1.Add(time.Second*4), "qux"),
	})
	if err != nil {
		t.Fatalf("writePcaps failed with error: %v", err)
	}
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
			mgr.DelTag(tc.tag)
		})
	}
	events, eventCloser := mgr.Listen()
	mgr.ImportPcaps(pcaps)
	for e := range events {
		if e.Type == "pcapProcessed" {
			eventCloser()
			break
		}
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
}
