package streams

import (
	"flag"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
)

var (
	checkTCPState   = flag.Bool("tcp_check_state", true, "enable checking of tcp state")
	checkTCPOptions = flag.Bool("tcp_check_options", false, "enable checking of tcp options")
)

type (
	StreamFlags uint8

	StreamData struct {
		Bytes       []byte
		PacketIndex uint64
	}

	Stream struct {
		ClientAddr       []byte
		ServerAddr       []byte
		ClientPort       uint16
		ServerPort       uint16
		Packets          []*gopacket.CaptureInfo
		PacketDirections []reassembly.TCPFlowDirection
		Data             []StreamData
		Flags            StreamFlags

		tcpstate      *reassembly.TCPSimpleFSM
		tcpoptchecker reassembly.TCPOptionCheck
	}
	StreamFactory struct {
		Streams []*Stream
	}
	AssemblerContext struct {
		CaptureInfo *gopacket.CaptureInfo
	}
)

const (
	InactivityTimeout = time.Minute * time.Duration(-5)

	StreamFlagsComplete    StreamFlags = 1
	StreamFlagsProtocol    StreamFlags = 2
	StreamFlagsProtocolTCP StreamFlags = 0
	StreamFlagsProtocolUDP StreamFlags = 2
)

func (ac *AssemblerContext) GetCaptureInfo() gopacket.CaptureInfo {
	return *ac.CaptureInfo
}

func (f *StreamFactory) NewUDP(netFlow, udpFlow gopacket.Flow) *Stream {
	toU16 := func(b []byte) uint16 {
		v := uint16(b[0]) << 8
		v |= uint16(b[1])
		return v
	}
	s := &Stream{
		ClientAddr: netFlow.Src().Raw(),
		ServerAddr: netFlow.Dst().Raw(),
		ClientPort: toU16(udpFlow.Src().Raw()),
		ServerPort: toU16(udpFlow.Dst().Raw()),
		Flags:      StreamFlagsProtocolUDP,
	}
	f.Streams = append(f.Streams, s)
	return s
}

func (f *StreamFactory) New(netFlow, tcpFlow gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	toU16 := func(b []byte) uint16 {
		v := uint16(b[0]) << 8
		v |= uint16(b[1])
		return v
	}
	s := &Stream{
		ClientAddr: netFlow.Src().Raw(),
		ServerAddr: netFlow.Dst().Raw(),
		ClientPort: toU16(tcpFlow.Src().Raw()),
		ServerPort: toU16(tcpFlow.Dst().Raw()),
		Flags:      StreamFlagsProtocolTCP,
		tcpstate: reassembly.NewTCPSimpleFSM(reassembly.TCPSimpleFSMOptions{
			SupportMissingEstablishment: false,
		}),
		tcpoptchecker: reassembly.NewTCPOptionCheck(),
	}
	f.Streams = append(f.Streams, s)
	return s
}

func (s *Stream) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	// add non-accepted packets, might be interesting when exporting pcaps
	s.Packets = append(s.Packets, ac.(*AssemblerContext).CaptureInfo)
	s.PacketDirections = append(s.PacketDirections, dir)

	if *checkTCPState {
		if !s.tcpstate.CheckState(tcp, dir) {
			return false
		}
	}
	if *checkTCPOptions {
		if err := s.tcpoptchecker.Accept(tcp, ci, dir, nextSeq, start); err != nil {
			return false
		}
	}
	return true
}

func (s *Stream) AddUDPPacket(dir reassembly.TCPFlowDirection, data []byte, ac reassembly.AssemblerContext) {
	s.Packets = append(s.Packets, ac.(*AssemblerContext).CaptureInfo)
	s.PacketDirections = append(s.PacketDirections, dir)
	length := len(data)
	if length == 0 {
		return
	}
	ci := ac.GetCaptureInfo()
	pmd := pcapmetadata.FromPacketMetadata(&ci)
	for i := len(s.Packets) - 1; ; i-- {
		p := s.Packets[i]
		if p.Timestamp != ci.Timestamp {
			continue
		}
		pmd2 := pcapmetadata.FromPacketMetadata(p)
		if pmd.PcapInfo != pmd2.PcapInfo || pmd.Index != pmd2.Index {
			continue
		}
		s.Data = append(s.Data, StreamData{
			Bytes:       data,
			PacketIndex: uint64(i),
		})
		return
	}
}

func (s *Stream) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	length, _ := sg.Lengths()
	if length == 0 {
		return
	}
	ci := sg.CaptureInfo(0)
	pmd := pcapmetadata.FromPacketMetadata(&ci)
	for i := len(s.Packets) - 1; ; i-- {
		p := s.Packets[i]
		if p.Timestamp != ci.Timestamp {
			continue
		}
		pmd2 := pcapmetadata.FromPacketMetadata(p)
		if pmd.PcapInfo != pmd2.PcapInfo || pmd.Index != pmd2.Index {
			continue
		}
		s.Data = append(s.Data, StreamData{
			Bytes:       sg.Fetch(length),
			PacketIndex: uint64(i),
		})
		return
	}
}

func (s *Stream) ReassemblyComplete(_ reassembly.AssemblerContext) bool {
	s.Flags |= StreamFlagsComplete
	// TODO: figure out what happens if we return true - will we be asked again and can return false then?
	return false
}
