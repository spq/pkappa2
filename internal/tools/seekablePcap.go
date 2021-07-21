package tools

import (
	"math"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type (
	SeekablePcapHolder struct {
		filename    string
		handle      *pcap.Handle
		source      *gopacket.PacketSource
		packetIndex uint64
	}
)

func NewSeekablePcapHolder(filename string) *SeekablePcapHolder {
	return &SeekablePcapHolder{
		filename:    filename,
		packetIndex: math.MaxUint32,
	}
}

func (s *SeekablePcapHolder) Close() {
	if s.handle != nil {
		s.handle.Close()
		s.handle = nil
	}
}

func (s *SeekablePcapHolder) Packet(packetIndex uint64) (gopacket.Packet, error) {
	if s.packetIndex > packetIndex {
		s.Close()
		handle, err := pcap.OpenOffline(s.filename)
		if err != nil {
			return nil, err
		}
		s.handle = handle
		s.source = gopacket.NewPacketSource(handle, handle.LinkType())
		s.packetIndex = 0
	}
	for s.packetIndex < packetIndex {
		_, err := s.source.NextPacket()
		if err != nil {
			return nil, err
		}
		s.packetIndex++
	}
	pkt, err := s.source.NextPacket()
	if err != nil {
		return nil, err
	}
	s.packetIndex++
	return pkt, nil
}
