package builder

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
)

type (
	Packet struct {
		p       gopacket.Packet
		ci      gopacket.CaptureInfo
		data    []byte
		decoder gopacket.Decoder
	}
)

func (p *Packet) Parsed() gopacket.Packet {
	if p.p == nil {
		p.p = gopacket.NewPacket(p.data, p.decoder, gopacket.NoCopy)
		md := p.p.Metadata()
		md.CaptureInfo = p.ci
		md.Truncated = md.Truncated || p.ci.CaptureLength < p.ci.Length
	}
	return p.p
}

func (p *Packet) Timestamp() time.Time {
	return p.ci.Timestamp
}

func (p *Packet) CaptureInfo() *gopacket.CaptureInfo {
	return &p.ci
}

func readPackets(pcapDir, pcapFilename string, info *pcapmetadata.PcapInfo) (*pcapmetadata.PcapInfo, []Packet, error) {
	updateInfo := info == nil
	if updateInfo {
		info = &pcapmetadata.PcapInfo{
			Filename:  pcapFilename,
			ParseTime: time.Now(),
		}
		if s, err := os.Stat(filepath.Join(pcapDir, pcapFilename)); err != nil {
			return nil, nil, err
		} else {
			info.Filesize = uint64(s.Size())
		}
	}
	handle, err := pcap.OpenOffline(filepath.Join(pcapDir, pcapFilename))
	if err != nil {
		return nil, nil, err
	}
	defer handle.Close()
	packets := []Packet(nil)
	var decoder gopacket.Decoder
	switch lt := handle.LinkType(); lt {
	case layers.LinkTypeIPv4:
		decoder = layers.LayerTypeIPv4
	case layers.LinkTypeIPv6:
		decoder = layers.LayerTypeIPv6
	default:
		decoder = lt
	}
	for packetIndex := uint64(0); ; packetIndex++ {
		data, ci, err := handle.ReadPacketData()
		switch err {
		case io.EOF:
			return info, packets, nil
		case nil:
		default:
			return nil, nil, err
		}
		if updateInfo {
			ts := ci.Timestamp
			if info.PacketTimestampMin.IsZero() || info.PacketTimestampMin.After(ts) {
				info.PacketTimestampMin = ts
			}
			if info.PacketTimestampMax.Before(ts) {
				info.PacketTimestampMax = ts
			}
			info.PacketCount++
		}
		pcapmetadata.AddPcapMetadata(&ci, info, packetIndex)
		packets = append(packets, Packet{
			decoder: decoder,
			data:    data,
			ci:      ci,
		})
	}
}
