package udpreassembly

import (
	"bytes"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/reassembly"
	"github.com/spq/pkappa2/internal/index/streams"
)

type (
	connection struct {
		lastActivity time.Time
		stream       *streams.Stream
	}
	Assembler struct {
		factory     *streams.StreamFactory
		connections map[uint64][]connection
	}
)

func NewAssembler(factory *streams.StreamFactory) *Assembler {
	return &Assembler{
		factory:     factory,
		connections: make(map[uint64][]connection),
	}
}

func (a *Assembler) FlushCloseOlderThan(t time.Time) {
	for h, cs := range a.connections {
		nDeleted := 0
		for i, c := range cs {
			if c.lastActivity.Before(t) {
				c.stream.ReassemblyComplete(nil)
				nDeleted++
				continue
			}
			if nDeleted != 0 {
				cs[i-nDeleted] = c
			}
		}
		switch nDeleted {
		case 0:
		case len(cs):
			delete(a.connections, h)
		default:
			a.connections[h] = cs[:len(cs)-nDeleted]
		}
	}
}

func (a *Assembler) AssembleWithContext(netFlow gopacket.Flow, u *layers.UDP, ac reassembly.AssemblerContext) {
	toU16 := func(b []byte) uint16 {
		v := uint16(b[0]) << 8
		v |= uint16(b[1])
		return v
	}
	f := u.TransportFlow()
	ah, ap, bh, bp := netFlow.Src(), toU16(f.Src().Raw()), netFlow.Dst(), toU16(f.Dst().Raw())

	// search connection
	hash := ah.FastHash() ^ bh.FastHash() ^ uint64(ap) ^ uint64(bp)
	stream := (*streams.Stream)(nil)
	dir := reassembly.TCPDirClientToServer
	cs, ok := a.connections[hash]
	if ok {
		ok = false
		for i, c := range cs {
			aIsClient := bytes.Equal(c.stream.ClientAddr, ah.Raw()) && c.stream.ClientPort == ap
			aIsServer := bytes.Equal(c.stream.ServerAddr, ah.Raw()) && c.stream.ServerPort == ap
			bIsClient := bytes.Equal(c.stream.ClientAddr, bh.Raw()) && c.stream.ClientPort == bp
			bIsServer := bytes.Equal(c.stream.ServerAddr, bh.Raw()) && c.stream.ServerPort == bp
			isC2S := aIsClient && bIsServer
			isS2C := bIsClient && aIsServer
			if isC2S == isS2C {
				continue
			}
			ok = true
			stream = c.stream
			if aIsServer {
				dir = reassembly.TCPDirServerToClient
			}
			// register activity in connection
			cs[i].lastActivity = ac.GetCaptureInfo().Timestamp
			break
		}
	}
	if !ok {
		// create new connection if none found
		stream = a.factory.NewUDP(netFlow, f)
		a.connections[hash] = append(cs, connection{
			lastActivity: ac.GetCaptureInfo().Timestamp,
			stream:       stream,
		})
	}
	// add data to connection
	stream.AddUDPPacket(dir, u.Payload, ac)
}
