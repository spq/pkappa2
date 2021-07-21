package streams

import (
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
	pcapmetadata "github.com/spq/pkappa2/internal/index/pcapMetadata"
)

type (
	TCPStreamData struct {
		Bytes       []byte
		PacketIndex uint64
	}
	TCPStream struct {
		ClientAddr       []byte
		ServerAddr       []byte
		ClientPort       uint16
		ServerPort       uint16
		Packets          []*gopacket.CaptureInfo
		PacketDirections []reassembly.TCPFlowDirection
		Data             []TCPStreamData
		Complete         bool

		tcpstate   *reassembly.TCPSimpleFSM
		optchecker reassembly.TCPOptionCheck
	}
	TCPStreamFactory struct {
		Streams []*TCPStream
	}
	AssemblerContext struct {
		CaptureInfo *gopacket.CaptureInfo
	}
)

const (
	InactivityTimeout = time.Minute * time.Duration(-5)
)

func (ac *AssemblerContext) GetCaptureInfo() gopacket.CaptureInfo {
	return *ac.CaptureInfo
}

func (f *TCPStreamFactory) New(netFlow, tcpFlow gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	toU16 := func(b []byte) uint16 {
		v := uint16(b[0]) << 8
		v |= uint16(b[1])
		return v
	}
	s := &TCPStream{
		ClientAddr: netFlow.Src().Raw(),
		ServerAddr: netFlow.Dst().Raw(),
		ClientPort: toU16(tcpFlow.Src().Raw()),
		ServerPort: toU16(tcpFlow.Dst().Raw()),
		tcpstate: reassembly.NewTCPSimpleFSM(reassembly.TCPSimpleFSMOptions{
			SupportMissingEstablishment: false,
		}),
	}
	f.Streams = append(f.Streams, s)
	return s
}

func (s *TCPStream) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	accept := true
	accept = accept && s.tcpstate.CheckState(tcp, dir)
	accept = accept && (s.optchecker.Accept(tcp, ci, dir, nextSeq, start) == nil)
	// add non-accepted packets, might be interesting when exporting pcaps
	s.Packets = append(s.Packets, ac.(*AssemblerContext).CaptureInfo)
	s.PacketDirections = append(s.PacketDirections, dir)
	return accept
}

func (s *TCPStream) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	length, _ := sg.Lengths()
	if length == 0 {
		return
	}
	ci := sg.CaptureInfo(0)
	pmd := pcapmetadata.FromPacketMetadata(&ci)
	for i := len(s.Packets) - 1; i >= 0; i-- {
		p := s.Packets[i]
		if p.Timestamp != ci.Timestamp {
			continue
		}
		pmd2 := pcapmetadata.FromPacketMetadata(p)
		if pmd.PcapInfo != pmd2.PcapInfo || pmd.Index != pmd2.Index {
			continue
		}
		s.Data = append(s.Data, TCPStreamData{
			Bytes:       sg.Fetch(length),
			PacketIndex: uint64(i),
		})
		return
	}
	s.Data = append(s.Data, TCPStreamData{
		Bytes:       sg.Fetch(length),
		PacketIndex: uint64(len(s.Packets) - 1),
	})
}

func (s *TCPStream) ReassemblyComplete(_ reassembly.AssemblerContext) bool {
	s.Complete = true
	// TODO: figure out what happens if we return true - will we be asked again and can return false then?
	return false
}
