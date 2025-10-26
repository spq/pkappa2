package converters

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/tools/bitmask"
)

type (
	cacheFile struct {
		file      *os.File
		cachePath string
		rwmutex   sync.RWMutex
		fileSize  int64
		freeSize  int64
		freeStart int64

		streamInfos map[uint64]streamInfo
	}

	streamInfo struct {
		offset int64
		size   uint64
	}

	// File format:
	// [u64 stream id] [u8 varint chunk sizes] [client data] [server data]
	converterStreamSection struct {
		StreamID uint64
	}

	converterCacheFileHeader struct {
		Magic   [4]byte
		Version uint32
	}
)

const (
	streamHeaderSize    = int64(unsafe.Sizeof(converterStreamSection{}))
	cacheFileHeaderSize = int64(unsafe.Sizeof(converterCacheFileHeader{}))

	// cleanup if at least 16 MiB are free and at least 50%
	cleanupMinFreeSize   = 16 * 1024 * 1024
	cleanupMinFreeFactor = 0.5

	cacheFileMagic   = "P2CC"
	cacheFileVersion = 1
)

func readVarInt(r io.ByteReader) (uint64, int, error) {
	bytes := 0
	result := uint64(0)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		bytes++
		result <<= 7
		result |= uint64(b & 0x7f)
		if b < 0x80 {
			break
		}
	}
	return result, bytes, nil
}

func writeVarInt(writer io.Writer, number uint64) (int, error) {
	buf := [10]byte{}
	for bytesWritten := 1; ; bytesWritten++ {
		buf[len(buf)-bytesWritten] = byte(number) | 0x80
		number >>= 7
		if number == 0 {
			buf[len(buf)-1] &= 0x7f
			return bytesWritten, binary.Write(writer, binary.LittleEndian, buf[len(buf)-bytesWritten:])
		}
	}
}

func readString(r *bufio.Reader) (string, int, error) {
	length, bytes, err := readVarInt(r)
	if err != nil {
		return "", 0, err
	}
	strBytes := make([]byte, length)
	if _, err := io.ReadFull(r, strBytes); err != nil {
		return "", 0, err
	}
	return string(strBytes), bytes + int(length), nil
}

func writeString(writer io.Writer, str string) (int, error) {
	bytesWritten, err := writeVarInt(writer, uint64(len(str)))
	if err != nil {
		return 0, err
	}
	n, err := writer.Write([]byte(str))
	if err != nil {
		return 0, err
	}
	return bytesWritten + n, nil
}

func readVarBytes(r io.ByteReader) ([]byte, int, error) {
	bytes := 0
	result := make([]byte, 0, 1)
	buf := uint16(0)
	bufFilled := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, 0, err
		}
		bytes++
		buf |= uint16(b&0x7f) << bufFilled
		bufFilled += 7
		if bufFilled >= 8 {
			result = append(result, byte(buf))
			buf >>= 8
			bufFilled -= 8
		}
		if b < 0x80 {
			break
		}
	}
	return result, bytes, nil
}

func writeVarBytes(writer io.Writer, data []byte) (int, error) {
	buf := uint16(0)
	bufFilled := 0
	bytesWritten := 0
	for _, b := range data {
		buf |= uint16(b) << bufFilled
		bufFilled += 8
		for {
			if _, err := writer.Write([]byte{byte(0x80 | (buf & 0x7f))}); err != nil {
				return 0, err
			}
			bytesWritten++
			buf >>= 7
			bufFilled -= 7
			if bufFilled <= 7 {
				break
			}
		}
	}
	if bufFilled != 0 || len(data) == 0 {
		if _, err := writer.Write([]byte{byte(buf)}); err != nil {
			return 0, err
		}
		bytesWritten++
	}
	return bytesWritten, nil
}

// skipStream skips a single stream in the given buffer, returning how many bytes were skipped.
func skipStream(buffer *bufio.Reader) (uint64, error) {
	// Read total data size of the stream by adding all chunk sizes up.
	streamSize, dataSize, chunkCount := 0, 0, 0
	for nZeros := 0; nZeros < 2; {
		sz, n, err := readVarInt(buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read size varint: %w", err)
		}
		streamSize += n
		dataSize += int(sz)
		if sz != 0 {
			nZeros = 0
			chunkCount++
		} else {
			nZeros++
		}
	}

	// skip data
	if _, err := buffer.Discard(int(dataSize)); err != nil {
		return 0, fmt.Errorf("failed to discard %d bytes: %w", dataSize, err)
	}
	streamSize += dataSize

	// read times
	for range chunkCount {
		_, n, err := readVarInt(buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read time varint: %w", err)
		}
		streamSize += n
	}

	// read content type
	for {
		chunks, n, err := readVarBytes(buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read content type varbytes: %w", err)
		}
		streamSize += n
		if len(chunks) == 0 {
			break
		}
		// read content type string
		_, n, err = readString(buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read content type string: %w", err)
		}
		streamSize += n
	}
	return uint64(streamSize), nil
}

func NewCacheFile(cachePath string) (*cacheFile, error) {
	file, err := os.OpenFile(cachePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache file: %w", err)
	}

	res := cacheFile{
		file:        file,
		cachePath:   cachePath,
		streamInfos: map[uint64]streamInfo{},
		fileSize:    cacheFileHeaderSize,
		freeStart:   cacheFileHeaderSize,
	}
	buffer := bufio.NewReader(file)

	// read the file header
	fh := converterCacheFileHeader{}
	if err := binary.Read(buffer, binary.LittleEndian, &fh); err != nil {
		if err == io.EOF {
			if err := res.Reset(); err != nil {
				return nil, fmt.Errorf("failed to reset cache file: %w", err)
			}
			return &res, nil
		}
		return nil, fmt.Errorf("failed to read stream header: %w", err)
	}
	if string(fh.Magic[:]) != cacheFileMagic || fh.Version != cacheFileVersion {
		log.Printf("Invalid converter cache file(%q) magic or version: %q/%d, expected %q/%d, resetting file\n",
			res.cachePath, string(fh.Magic[:]), fh.Version, cacheFileMagic, cacheFileVersion)
		if err := res.Reset(); err != nil {
			return nil, fmt.Errorf("failed to reset cache file: %w", err)
		}
		return &res, nil
	}

	// Read all stream ids
	for {
		streamSection := converterStreamSection{}
		if err := binary.Read(buffer, binary.LittleEndian, &streamSection); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read stream header: %w", err)
		}
		res.fileSize += streamHeaderSize

		streamSize, err := skipStream(buffer)
		if err != nil {
			return nil, fmt.Errorf("failed to skip stream data: %w", err)
		}

		if info, ok := res.streamInfos[streamSection.StreamID]; ok {
			if res.freeSize == 0 || res.freeStart > info.offset-streamHeaderSize {
				res.freeStart = info.offset - streamHeaderSize
			}
			res.freeSize += streamHeaderSize + int64(info.size)
		}
		res.streamInfos[streamSection.StreamID] = streamInfo{
			offset: res.fileSize,
			size:   uint64(streamSize),
		}
		res.fileSize += int64(streamSize)
	}
	if res.freeSize == 0 {
		res.freeStart = res.fileSize
	} else {
		if err := res.truncateFile(); err != nil {
			return nil, fmt.Errorf("failed to truncate file: %w", err)
		}
	}

	// Keep the file pointer at the end of the file.
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("failed to seek to end of file: %w", err)
	}

	return &res, nil
}

func (cachefile *cacheFile) Close() error {
	cachefile.rwmutex.Lock()
	// Don't unlock the mutex here, because we don't want to allow any other
	// operations on the file after closing it.

	if err := cachefile.file.Sync(); err != nil {
		return err
	}

	return cachefile.file.Close()
}

func (cachefile *cacheFile) StreamCount() uint64 {
	cachefile.rwmutex.RLock()
	defer cachefile.rwmutex.RUnlock()

	return uint64(len(cachefile.streamInfos))
}

func (cachefile *cacheFile) Reset() error {
	cachefile.rwmutex.Lock()
	defer cachefile.rwmutex.Unlock()

	if _, err := cachefile.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := cachefile.file.Truncate(0); err != nil {
		return err
	}
	// write header
	fh := converterCacheFileHeader{
		Magic:   [4]byte([]byte(cacheFileMagic)),
		Version: cacheFileVersion,
	}
	if _, err := cachefile.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of file: %w", err)
	}
	if err := binary.Write(cachefile.file, binary.LittleEndian, &fh); err != nil {
		return fmt.Errorf("failed to write cache file header: %w", err)
	}
	// Keep the file pointer at the end of the file.
	if _, err := cachefile.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}
	cachefile.streamInfos = map[uint64]streamInfo{}
	cachefile.freeSize = 0
	cachefile.fileSize = cacheFileHeaderSize
	cachefile.freeStart = cacheFileHeaderSize
	return nil
}

func (cachefile *cacheFile) Contains(streamID uint64) bool {
	cachefile.rwmutex.RLock()
	defer cachefile.rwmutex.RUnlock()

	_, ok := cachefile.streamInfos[streamID]
	return ok
}

func (cachefile *cacheFile) Data(stream *index.Stream) ([]index.Data, uint64, uint64, error) {
	return cachefile.data(stream.ID(), stream.FirstPacket())
}

func (cachefile *cacheFile) data(streamID uint64, firstPacketTime time.Time) ([]index.Data, uint64, uint64, error) {
	cachefile.rwmutex.RLock()
	defer cachefile.rwmutex.RUnlock()

	info, ok := cachefile.streamInfos[streamID]
	if !ok {
		return nil, 0, 0, nil
	}

	buffer := bufio.NewReader(io.NewSectionReader(cachefile.file, info.offset, int64(info.size)))
	data := []index.Data{}

	type sizeAndDirection struct {
		Size      uint64
		Direction index.Direction
	}
	// Read chunk sizes
	dataSizes := []sizeAndDirection{}
	prevWasZero := false
	direction := index.DirectionClientToServer
	bytes := [2]uint64{0, 0}
	for {
		sz, _, err := readVarInt(buffer)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read size varint: %w", err)
		}
		if sz == 0 && prevWasZero {
			break
		}
		dataSizes = append(dataSizes, sizeAndDirection{Direction: direction, Size: sz})
		prevWasZero = sz == 0
		bytes[direction] += sz
		direction = direction.Reverse()
	}
	dataSizes = dataSizes[:len(dataSizes)-1] // Remove the last zero size chunk

	// Read data
	clientData := make([]byte, bytes[index.DirectionClientToServer])
	if _, err := io.ReadFull(buffer, clientData); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to read client data: %w", err)
	}
	serverData := make([]byte, bytes[index.DirectionServerToClient])
	if _, err := io.ReadFull(buffer, serverData); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to read server data: %w", err)
	}

	// Split data into chunks and read times
	lastTime := firstPacketTime
	for _, ds := range dataSizes {
		if ds.Size == 0 {
			continue
		}
		var bytes []byte
		if ds.Direction == index.DirectionClientToServer {
			bytes = clientData[:ds.Size]
			clientData = clientData[ds.Size:]
		} else {
			bytes = serverData[:ds.Size]
			serverData = serverData[ds.Size:]
		}

		relMS, _, err := readVarInt(buffer)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read time varint: %w", err)
		}
		lastTime = lastTime.Add(time.Duration(relMS) * time.Microsecond)
		data = append(data, index.Data{
			Direction: ds.Direction,
			Content:   bytes,
			Time:      lastTime.UTC(),
		})
	}

	// Read content types and enrich the data
	for {
		chunks, _, err := readVarBytes(buffer)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read content type varbytes: %w", err)
		}
		if len(chunks) == 0 {
			break
		}
		ct, _, err := readString(buffer)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read content type string: %w", err)
		}
		for i, b := range chunks {
			bit := i * 8
			for b != 0 {
				if (b & 1) != 0 {
					if bit >= len(data) {
						return nil, 0, 0, fmt.Errorf("content type bitmask out of range")
					}
					data[bit].ContentType = ct
				}
				bit++
				b >>= 1
			}
		}
	}
	return data, bytes[index.DirectionClientToServer], bytes[index.DirectionServerToClient], nil
}

func (cachefile *cacheFile) DataForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, bool, error) {
	cachefile.rwmutex.RLock()
	defer cachefile.rwmutex.RUnlock()

	info, ok := cachefile.streamInfos[streamID]
	if !ok {
		return [2][]byte{}, [][2]int{}, 0, 0, false, nil
	}
	buffer := bufio.NewReader(io.NewSectionReader(cachefile.file, info.offset, int64(info.size)))

	// Read chunk sizes
	dataSizes := [][2]int{{}}
	prevWasZero := false
	direction := index.DirectionClientToServer
	clientBytes := uint64(0)
	serverBytes := uint64(0)
	for {
		last := dataSizes[len(dataSizes)-1]
		sz, _, err := readVarInt(buffer)
		if err != nil {
			return [2][]byte{}, [][2]int{}, 0, 0, true, fmt.Errorf("failed to read size varint: %w", err)
		}
		if sz == 0 {
			if prevWasZero {
				break
			} else {
				prevWasZero = true
				direction = direction.Reverse()
				continue
			}
		}
		new := [2]int{
			last[0],
			last[1],
		}
		new[direction] += int(sz)
		dataSizes = append(dataSizes, new)
		prevWasZero = false
		if direction == index.DirectionClientToServer {
			clientBytes += sz
		} else {
			serverBytes += sz
		}
		direction = direction.Reverse()
	}

	// Read data
	clientData := make([]byte, clientBytes)
	if _, err := io.ReadFull(buffer, clientData); err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, true, fmt.Errorf("failed to read client data: %w", err)
	}
	serverData := make([]byte, serverBytes)
	if _, err := io.ReadFull(buffer, serverData); err != nil {
		return [2][]byte{}, [][2]int{}, 0, 0, true, fmt.Errorf("failed to read server data: %w", err)
	}
	return [2][]byte{clientData, serverData}, dataSizes, clientBytes, serverBytes, true, nil
}

func (cachefile *cacheFile) truncateFile() error {
	// cleanup the file by skipping all old streams
	if _, err := cachefile.file.Seek(cachefile.freeStart, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to free start: %w", err)
	}

	reader := bufio.NewReader(io.NewSectionReader(cachefile.file, cachefile.freeStart, cachefile.fileSize-cachefile.freeStart))
	writer := bufio.NewWriter(cachefile.file)

	newFilesize := cachefile.freeStart
	header := converterStreamSection{}
	for oldFileOffset := cachefile.freeStart; ; {
		if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream header: %w", err)
		}
		oldFileOffset += streamHeaderSize
		// only copy the stream if we have the metadata for it
		if info, ok := cachefile.streamInfos[header.StreamID]; ok && info.offset == oldFileOffset {
			if err := binary.Write(writer, binary.LittleEndian, header); err != nil {
				return fmt.Errorf("failed to write stream header: %w", err)
			}
			if _, err := io.CopyN(writer, reader, int64(info.size)); err != nil {
				return fmt.Errorf("failed to copy stream data: %w", err)
			}
			oldFileOffset += int64(info.size)
			info.offset = newFilesize + streamHeaderSize
			cachefile.streamInfos[header.StreamID] = info
			newFilesize += streamHeaderSize + int64(info.size)
			continue
		}
		// skip the stream
		n, err := skipStream(reader)
		if err != nil {
			return fmt.Errorf("failed to skip stream: %w", err)
		}
		oldFileOffset += int64(n)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	cachefile.fileSize = newFilesize
	if _, err := cachefile.file.Seek(cachefile.fileSize, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}
	if err := cachefile.file.Truncate(cachefile.fileSize); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}
	cachefile.freeSize = 0
	cachefile.freeStart = cachefile.fileSize
	return nil
}

func (cachefile *cacheFile) SetData(stream *index.Stream, convertedPackets []index.Data) error {
	return cachefile.setData(stream.ID(), stream.FirstPacket(), convertedPackets)
}

func (cachefile *cacheFile) setData(streamID uint64, streamTime time.Time, convertedPackets []index.Data) error {
	cachefile.rwmutex.Lock()
	defer cachefile.rwmutex.Unlock()

	if cachefile.freeSize >= cleanupMinFreeSize && cachefile.freeSize >= int64(float64(cachefile.fileSize)*cleanupMinFreeFactor) {
		if err := cachefile.truncateFile(); err != nil {
			return fmt.Errorf("failed to truncate file: %w", err)
		}
	}

	writer := bufio.NewWriter(cachefile.file)
	// Write stream header
	streamSection := converterStreamSection{
		StreamID: streamID,
	}
	if err := binary.Write(writer, binary.LittleEndian, &streamSection); err != nil {
		return fmt.Errorf("failed to write stream header: %w", err)
	}

	streamSize := uint64(0)
	for pIndex, wantDir := 0, index.DirectionClientToServer; pIndex < len(convertedPackets); {
		convertedPacket := convertedPackets[pIndex]
		dir := convertedPacket.Direction
		// Write a length of 0 if the server sent the first packet.
		if dir != wantDir {
			if err := writer.WriteByte(0); err != nil {
				return fmt.Errorf("failed to write zero length for first packet: %w", err)
			}
			streamSize++
			wantDir = wantDir.Reverse()
		}
		bytesWritten, err := writeVarInt(writer, uint64(len(convertedPacket.Content)))
		if err != nil {
			return fmt.Errorf("failed to write chunk size: %w", err)
		}
		streamSize += uint64(bytesWritten)

		wantDir = wantDir.Reverse()
		pIndex++
	}
	// Append two lengths of 0 to indicate the end of the chunk sizes
	if err := binary.Write(writer, binary.LittleEndian, []byte{0, 0}); err != nil {
		// TODO: The cache file is corrupt now. We should probably delete it.
		return fmt.Errorf("failed to write end of chunk sizes: %w", err)
	}
	streamSize += 2

	// Write chunk data
	for _, direction := range []index.Direction{index.DirectionClientToServer, index.DirectionServerToClient} {
		for _, convertedPacket := range convertedPackets {
			if convertedPacket.Direction != direction {
				continue
			}
			if err := binary.Write(writer, binary.LittleEndian, convertedPacket.Content); err != nil {
				return fmt.Errorf("failed to write packet content: %w", err)
			}
			streamSize += uint64(len(convertedPacket.Content))
		}
	}

	// Write times and collect content types
	lastTime := streamTime
	contentTypes := map[string][]byte{}
	for i, convertedPacket := range convertedPackets {
		relTime := convertedPacket.Time.Sub(lastTime)
		bytesWritten, err := writeVarInt(writer, uint64(relTime.Microseconds()))
		if err != nil {
			return fmt.Errorf("failed to write relative packet time: %w", err)
		}
		streamSize += uint64(bytesWritten)
		lastTime = lastTime.Add(relTime)

		ct := convertedPacket.ContentType
		if ct == "" {
			continue
		}
		bm := contentTypes[ct]
		for i >= len(bm)*8 {
			bm = append(bm, 0)
		}
		bm[i/8] |= 1 << (i & 7)
		contentTypes[ct] = bm
	}

	// Write content types
	for contentType, chunks := range contentTypes {
		// Write chunk bitmask
		bytesWritten, err := writeVarBytes(writer, chunks)
		if err != nil {
			return fmt.Errorf("failed to write content type chunk bitmask: %w", err)
		}
		streamSize += uint64(bytesWritten)
		// Write content type string
		bytesWritten, err = writeString(writer, contentType)
		if err != nil {
			return fmt.Errorf("failed to write content type string: %w", err)
		}
		streamSize += uint64(bytesWritten)
	}
	// Write ending zero chunk bitmask
	if err := writer.WriteByte(0); err != nil {
		return fmt.Errorf("failed to write ending zero content type chunk bitmask: %w", err)
	}
	streamSize++

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	// Remember where to look for this stream.
	cachefile.streamInfos[streamID] = streamInfo{
		offset: cachefile.fileSize + streamHeaderSize,
		size:   streamSize,
	}

	if cachefile.freeStart == cachefile.fileSize {
		cachefile.freeStart += streamHeaderSize + int64(streamSize)
	}
	cachefile.fileSize += streamHeaderSize + int64(streamSize)

	return nil
}

func (cachefile *cacheFile) InvalidateChangedStreams(streams *bitmask.LongBitmask) bitmask.LongBitmask {
	invalidatedStreams := bitmask.LongBitmask{}

	cachefile.rwmutex.Lock()
	defer cachefile.rwmutex.Unlock()

	// see which of the streams are in the cache
	for streamID := uint(0); streams.Next(&streamID); streamID++ {
		// delete the stream from the in-memory index
		// it will be re-added when the stream is converted again
		if info, ok := cachefile.streamInfos[uint64(streamID)]; ok {
			cachefile.freeSize += int64(info.size) + streamHeaderSize
			if cachefile.freeStart > info.offset-streamHeaderSize {
				cachefile.freeStart = info.offset - streamHeaderSize
			}
			delete(cachefile.streamInfos, uint64(streamID))
			invalidatedStreams.Set(streamID)
		}
	}

	return invalidatedStreams
}

// func (writer *writer) invalidateStream(stream *index.Stream) error {

// 	offset, ok := writer.cache.containedStreamIds[stream.ID()]
// 	if !ok {
// 		return nil
// 	}

// 	if err := writer.buffer.Flush(); err != nil {
// 		return err
// 	}
// 	if _, err := writer.file.Seek(offset, io.SeekStart); err != nil {
// 		return err
// 	}

// 	// Find stream in file and replace streamid with InvalidStreamID
// 	streamSection := converterStreamSection{}
// 	if err := binary.Read(writer.file, binary.LittleEndian, &streamSection); err != nil {
// 		return err
// 	}
// 	// Should never happen
// 	if streamSection.StreamID != stream.ID() {
// 		return fmt.Errorf("stream id mismatch during invalidation: %d != %d, offset %d", streamSection.StreamID, stream.ID(), offset)
// 	}

// 	streamSection.StreamID = InvalidStreamID
// 	if _, err := writer.file.Seek(-int64(unsafe.Sizeof(streamSection)), io.SeekCurrent); err != nil {
// 		return err
// 	}
// 	if err := binary.Write(writer.file, binary.LittleEndian, streamSection); err != nil {
// 		return err
// 	}

// 	delete(writer.cache.containedStreamIds, stream.ID())
// 	return nil
// }
