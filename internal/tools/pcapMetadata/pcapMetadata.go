package pcapmetadata

import (
	"time"

	"github.com/google/gopacket"
)

type (
	PcapInfo struct {
		Filename           string
		Filesize           uint64
		PacketTimestampMin time.Time
		PacketTimestampMax time.Time
		ParseTime          time.Time
		PacketCount        uint
	}

	PcapMetadata struct {
		PcapInfo *PcapInfo
		Index    uint64
	}
)

func AddPcapMetadata(md *gopacket.CaptureInfo, info *PcapInfo, packetIndex uint64) {
	md.AncillaryData = append(md.AncillaryData, &PcapMetadata{info, packetIndex})
}

func FromPacketMetadata(ci *gopacket.CaptureInfo) *PcapMetadata {
	for i := len(ci.AncillaryData) - 1; i >= 0; i-- {
		ad := ci.AncillaryData[i]
		if pmd, ok := ad.(*PcapMetadata); ok {
			return pmd
		}
	}
	return nil
}

func AllFromPacketMetadata(ci *gopacket.CaptureInfo) []*PcapMetadata {
	pmds := []*PcapMetadata(nil)
	for i := len(ci.AncillaryData) - 1; i >= 0; i-- {
		ad := ci.AncillaryData[i]
		if pmd, ok := ad.(*PcapMetadata); ok {
			pmds = append(pmds, pmd)
		}
	}
	return pmds
}
