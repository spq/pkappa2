package builder

import (
	"bufio"
	"encoding/binary"
	"math"
	"os"
	"time"
)

type (
	snapshot struct {
		timestamp         time.Time
		referencedPackets map[string][]uint64
		packetCount       uint64
	}

	snapshotHeader struct {
		TimestampSec, TimestampNSec int64
		PacketCount, NumPcaps       uint64
	}

	snapshotEntryHeader struct {
		PacketCount, FilenameLength uint64
	}
)

func loadSnapshots(filename string) ([]*snapshot, error) {
	snapshots := []*snapshot{}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	numSnapshots := uint64(0)
	if err := binary.Read(reader, binary.LittleEndian, &numSnapshots); err != nil {
		return nil, err
	}
	for ; numSnapshots > 0; numSnapshots-- {
		ss := snapshot{}
		header := snapshotHeader{}
		if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
			return nil, err
		}
		ss.timestamp = time.Unix(header.TimestampSec, header.TimestampNSec)
		ss.packetCount = header.PacketCount
		ss.referencedPackets = make(map[string][]uint64, header.NumPcaps)
		for ; header.NumPcaps > 0; header.NumPcaps-- {
			header := snapshotEntryHeader{}
			if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
				return nil, err
			}
			referencedPackets := make([]uint64, header.PacketCount)
			if err := binary.Read(reader, binary.LittleEndian, referencedPackets); err != nil {
				return nil, err
			}
			fn := make([]byte, (header.FilenameLength+7)&^uint64(7))
			if err := binary.Read(reader, binary.LittleEndian, fn); err != nil {
				return nil, err
			}
			ss.referencedPackets[string(fn[:header.FilenameLength])] = referencedPackets
		}
		snapshots = append(snapshots, &ss)
	}
	return snapshots, nil
}

func saveSnapshots(filename string, snapshots []*snapshot) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	numSnapshots := uint64(len(snapshots))
	if err := binary.Write(writer, binary.LittleEndian, &numSnapshots); err != nil {
		return err
	}
	for _, ss := range snapshots {
		header := snapshotHeader{
			TimestampSec:  ss.timestamp.Unix(),
			TimestampNSec: ss.timestamp.UnixNano(),
			PacketCount:   ss.packetCount,
			NumPcaps:      uint64(len(ss.referencedPackets)),
		}
		if err := binary.Write(writer, binary.LittleEndian, &header); err != nil {
			return err
		}
		for fn, rp := range ss.referencedPackets {
			header := snapshotEntryHeader{
				PacketCount:    uint64(len(rp)),
				FilenameLength: uint64(len(fn)),
			}
			if err := binary.Write(writer, binary.LittleEndian, &header); err != nil {
				return err
			}
			if err := binary.Write(writer, binary.LittleEndian, rp); err != nil {
				return err
			}
			for len(fn)%8 != 0 {
				fn += "\x00"
			}
			if _, err := writer.WriteString(fn); err != nil {
				return err
			}
		}
	}
	return writer.Flush()
}

func compactSnapshots(snapshots []*snapshot) []*snapshot {
	minPacketCount := uint64(math.MaxUint64)
	for _, s := range snapshots {
		if minPacketCount > s.packetCount {
			minPacketCount = s.packetCount
		}
	}
	if minPacketCount == 0 {
		return nil
	}
	for i := len(snapshots) - 3; i >= 0; i -= 2 {
		a, b, c := snapshots[i], snapshots[i+1], snapshots[i+2]
		aChunks, bChunks, cChunks := a.packetCount/minPacketCount, b.packetCount/minPacketCount, c.packetCount/minPacketCount
		if aChunks > bChunks || bChunks > cChunks {
			break
		}
		b.packetCount += a.packetCount
		//remove a
		snapshots = append(snapshots[:i], snapshots[i+1:]...)
	}
	return snapshots
}
