package builder

import (
	"io"
	"path/filepath"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	pcapmetadata "github.com/spq/pkappa2/internal/index/pcapMetadata"
)

func readPackets(pcapDir, pcapFilename string, info *pcapmetadata.PcapInfo) (*pcapmetadata.PcapInfo, []gopacket.Packet, error) {
	handle, err := pcap.OpenOffline(filepath.Join(pcapDir, pcapFilename))
	if err != nil {
		return nil, nil, err
	}
	defer handle.Close()
	src := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := []gopacket.Packet{}
	updateInfo := info == nil
	if updateInfo {
		info = &pcapmetadata.PcapInfo{
			Filename: pcapFilename,
		}
	}
	for packetIndex := uint64(0); ; packetIndex++ {
		p, err := src.NextPacket()
		switch err {
		case io.EOF:
			return info, packets, nil
		case nil:
			md := p.Metadata()
			pcapmetadata.AddPcapMetadata(&md.CaptureInfo, info, packetIndex)
			packets = append(packets, p)
			if updateInfo {
				ts := md.Timestamp
				if info.PacketTimestampMin.IsZero() || info.PacketTimestampMin.After(ts) {
					info.PacketTimestampMin = ts
				}
				if info.PacketTimestampMax.Before(ts) {
					info.PacketTimestampMax = ts
				}
			}
		default:
			return nil, nil, err
		}
	}
}
