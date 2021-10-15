package index

type (
	section byte
)

const (
	sectionData section = iota
	sectionPackets
	sectionV6Hosts
	sectionV4Hosts
	sectionHostGroups
	sectionImports
	sectionImportFilenames
	sectionStreams
	sectionStreamsByStreamID
	sectionStreamsByFirstPacketSource
	sectionStreamsByFirstPacketTime
	sectionStreamsByLastPacketTime
	sectionsCount int = iota
)

type (
	fileHeaderSection struct {
		Begin uint64
		End   uint64
	}
	fileHeader struct {
		Magic           [16]byte
		FirstPacketTime uint64
		Sections        [sectionsCount]fileHeaderSection
	}
	hostGroupEntry struct {
		Start uint32
		Count uint16 // add 1: 0 means 1, 0xffff means 0x10000
		Flags uint16
	}
	importEntry struct {
		Filename          uint64
		PacketIndexOffset uint64
	}
	packet struct {
		RelPacketTimeMS    uint32
		ImportID           uint32
		PacketIndex        uint32
		DataSize           uint16
		SkipPacketsForData uint8 //how many of the next packets have no data and have a follow up packet, 255 means 255+
		Flags              uint8
	}
	stream struct {
		StreamID               uint64
		FirstPacketTimeNS      uint64
		LastPacketTimeNS       uint64
		DataStart              uint64
		ClientBytes            uint64
		ServerBytes            uint64
		PacketInfoStart        uint32
		Flags                  uint16
		HostGroup              uint16
		ClientHost, ServerHost uint16
		ClientPort, ServerPort uint16
	}
)

const (
	fileMagic = "pkappa2index\x00\x00\x00\x02"

	flagsHostGroupIPVersion = 0b1
	flagsHostGroupIP4       = 0b0
	flagsHostGroupIP6       = 0b1

	flagsPacketHasNext                 = 0b01
	flagsPacketDirection               = 0b10
	flagsPacketDirectionClientToServer = 0b00
	flagsPacketDirectionServerToClient = 0b10

	flagsStreamProtocol         = 0b011
	flagsStreamProtocolOther    = 0b000
	flagsStreamProtocolTCP      = 0b001
	flagsStreamProtocolUDP      = 0b010
	flagsStreamProtocolSCTP     = 0b011
	flagsStreamSegmentation     = 0b100
	flagsStreamSegmentationNone = 0b000
	flagsStreamSegmentationHTTP = 0b100
)

func (fhs fileHeaderSection) size() int64 {
	return int64(fhs.End - fhs.Begin)
}
