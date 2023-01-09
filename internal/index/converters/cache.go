package converters

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"unsafe"

	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/tools/bitmask"
)

type (
	CachedConverter struct {
		converter *Converter

		cachePath string
		reader    *reader
		writer    *writer
		rwmutex   sync.RWMutex

		// Fast lookup if given streamid is already in the cache.
		availableStreams bitmask.LongBitmask
		// Map of streamid to file offset in the cache file.
		containedStreamIds map[uint64]int64
	}

	writer struct {
		cache  *CachedConverter
		file   *os.File
		buffer *bufio.Writer
	}

	reader struct {
		cache *CachedConverter
		file  *os.File
	}

	// File format
	converterStreamSection struct {
		StreamID uint64
		DataSize uint64
	}
)

const (
	InvalidStreamID = ^uint64(0)
)

func NewCache(converterName, executablePath, indexCachePath string) (*CachedConverter, error) {
	cache := CachedConverter{
		converter:          New(converterName, executablePath),
		containedStreamIds: make(map[uint64]int64),
	}

	filename := fmt.Sprintf("converterindex-%s.cidx", cache.Name())
	cache.cachePath = filepath.Join(indexCachePath, filename)

	// Load existing streamids from cache file.
	if _, err := os.Stat(cache.cachePath); err == nil {
		reader, err := cache.newReader()
		if err != nil {
			return nil, err
		}
		cache.reader = reader
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return &cache, nil
}

func (cache *CachedConverter) Name() string {
	return cache.converter.Name()
}

func (cache *CachedConverter) HasStream(streamID uint64) bool {
	cache.rwmutex.RLock()
	defer cache.rwmutex.RUnlock()
	return cache.availableStreams.IsSet(uint(streamID))
}

func (cache *CachedConverter) Reset() {
	// Stop all converter processes.
	cache.converter.Reset()

	// Remove the cache file.
	cache.rwmutex.Lock()
	defer cache.rwmutex.Unlock()
	if cache.reader != nil {
		cache.reader.file.Close()
		cache.reader = nil
	}
	if cache.writer != nil {
		cache.writer.file.Close()
		cache.writer = nil
	}
	os.Remove(cache.cachePath)

	// Reset the cache index.
	cache.availableStreams = bitmask.LongBitmask{}
	cache.containedStreamIds = make(map[uint64]int64)
}

func (cache *CachedConverter) Data(stream *index.Stream) (data []index.Data, clientBytes, serverBytes uint64, err error) {
	// See if the stream data is cached already.
	data, clientBytes, serverBytes, err = cache.cachedData(stream)
	if err != nil {
		return nil, 0, 0, err
	}
	if data != nil {
		return data, clientBytes, serverBytes, nil
	}

	// Convert the stream if it's not in the cache.
	convertedPackets, clientBytes, serverBytes, err := cache.converter.Data(stream)
	if err != nil {
		return nil, 0, 0, err
	}

	// Save it to the cache.
	if err := cache.appendStream(stream, convertedPackets, clientBytes+serverBytes); err != nil {
		return nil, 0, 0, err
	}
	return convertedPackets, clientBytes, serverBytes, nil
}

func (cache *CachedConverter) cachedData(stream *index.Stream) (data []index.Data, clientBytes, serverBytes uint64, err error) {
	cache.rwmutex.RLock()
	defer cache.rwmutex.RUnlock()
	if cache.HasStream(stream.ID()) {
		return cache.reader.readStream(stream.ID())
	}
	return nil, 0, 0, nil
}

func (cache *CachedConverter) appendStream(stream *index.Stream, convertedPackets []index.Data, dataSize uint64) error {
	cache.rwmutex.Lock()
	defer cache.rwmutex.Unlock()
	if cache.writer == nil {
		writer, err := cache.newWriter()
		if err != nil {
			return err
		}
		cache.writer = writer
	}

	if cache.HasStream(stream.ID()) {
		if err := cache.writer.invalidateStream(stream); err != nil {
			return err
		}
	}
	if err := cache.writer.appendStream(stream, convertedPackets, dataSize); err != nil {
		return err
	}
	return nil
}

func (cache *CachedConverter) newReader() (*reader, error) {
	file, err := os.Open(cache.cachePath)
	if err != nil {
		return nil, err
	}
	reader := &reader{
		cache: cache,
		file:  file,
	}

	// Read all stream ids
	buffer := bufio.NewReader(file)
	pos := int64(0)
	for {
		streamSection := converterStreamSection{}
		if err := binary.Read(buffer, binary.LittleEndian, &streamSection); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		pos += int64(unsafe.Sizeof(streamSection))
		if streamSection.StreamID != InvalidStreamID {
			reader.cache.availableStreams.Set(uint(streamSection.StreamID))
			reader.cache.containedStreamIds[streamSection.StreamID] = pos
		}
		if _, err := buffer.Discard(int(streamSection.DataSize)); err != nil {
			return nil, err
		}
		pos += int64(streamSection.DataSize)
	}
	return reader, nil
}

func (reader *reader) readStream(streamID uint64) ([]index.Data, uint64, uint64, error) {
	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	reader.cache.rwmutex.RLock()
	defer reader.cache.rwmutex.RUnlock()

	pos, ok := reader.cache.containedStreamIds[streamID]
	if !ok {
		return nil, 0, 0, fmt.Errorf("stream %d not found in %s", streamID, reader.cache.cachePath)
	}

	if _, err := reader.file.Seek(int64(pos), io.SeekStart); err != nil {
		return nil, 0, 0, err
	}

	buffer := bufio.NewReader(reader.file)
	data := []index.Data{}

	type sizeAndDirection struct {
		Size      uint64
		Direction index.Direction
	}
	// Read chunk sizes
	dataSizes := []sizeAndDirection{}
	prevWasZero := false
	direction := index.DirectionClientToServer
	clientBytes := uint64(0)
	serverBytes := uint64(0)
	for {
		sz := uint64(0)
		for {
			b, err := buffer.ReadByte()
			if err != nil {
				return nil, 0, 0, err
			}
			sz <<= 7
			sz |= uint64(b & 0x7f)
			if b < 0x80 {
				break
			}
		}
		if sz == 0 && prevWasZero {
			break
		}
		dataSizes = append(dataSizes, sizeAndDirection{Direction: direction, Size: sz})
		prevWasZero = sz == 0
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
		return nil, 0, 0, err
	}
	serverData := make([]byte, serverBytes)
	if _, err := io.ReadFull(buffer, serverData); err != nil {
		return nil, 0, 0, err
	}

	// Split data into chunks
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
		data = append(data, index.Data{
			Direction: ds.Direction,
			Content:   bytes,
		})
	}
	return data, clientBytes, serverBytes, nil
}

func (cache *CachedConverter) newWriter() (*writer, error) {
	file, err := os.OpenFile(cache.cachePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &writer{
		cache:  cache,
		file:   file,
		buffer: bufio.NewWriter(file),
	}, nil
}

func (writer *writer) write(what interface{}) error {
	err := binary.Write(writer.buffer, binary.LittleEndian, what)
	if err != nil {
		debug.PrintStack()
	}
	return err
}

func (writer *writer) invalidateStream(stream *index.Stream) error {

	offset, ok := writer.cache.containedStreamIds[stream.ID()]
	if !ok {
		return nil
	}

	if err := writer.buffer.Flush(); err != nil {
		return err
	}
	if _, err := writer.file.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	// Find stream in file and replace streamid with InvalidStreamID
	streamSection := converterStreamSection{}
	if err := binary.Read(writer.file, binary.LittleEndian, &streamSection); err != nil {
		return err
	}
	// Should never happen
	if streamSection.StreamID != stream.ID() {
		return fmt.Errorf("stream id mismatch during invalidation: %d != %d, offset %d", streamSection.StreamID, stream.ID(), offset)
	}

	streamSection.StreamID = InvalidStreamID
	if _, err := writer.file.Seek(-int64(unsafe.Sizeof(streamSection)), io.SeekCurrent); err != nil {
		return err
	}
	if err := binary.Write(writer.file, binary.LittleEndian, streamSection); err != nil {
		return err
	}

	delete(writer.cache.containedStreamIds, stream.ID())
	return nil
}

func (writer *writer) appendStream(stream *index.Stream, convertedPackets []index.Data, dataSize uint64) error {
	offset, err := writer.file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// [u64 stream id] [u64 data size] [u8 varint chunk sizes] [client data] [server data]
	segmentation := []byte(nil)
	buf := [10]byte{}
	for pIndex, wantDir := 0, index.DirectionClientToServer; pIndex < len(convertedPackets); {
		// TODO: Merge packets with the same direction. Do we even want to allow converters to change the direction?
		convertedPacket := convertedPackets[pIndex]
		sz := len(convertedPacket.Content)
		dir := convertedPacket.Direction
		// Write a length of 0 if the server sent the first packet.
		if dir != wantDir {
			segmentation = append(segmentation, 0)
			wantDir = wantDir.Reverse()
		}
		pos := len(buf)
		flag := byte(0)
		for {
			pos--
			buf[pos] = byte(sz&0x7f) | flag
			flag = 0x80
			sz >>= 7
			if sz == 0 {
				break
			}
		}
		segmentation = append(segmentation, buf[pos:]...)
		wantDir = wantDir.Reverse()
		pIndex++
	}
	// Append two lengths of 0 to indicate the end of the chunk sizes
	segmentation = append(segmentation, 0, 0)

	// Write stream section
	streamSection := converterStreamSection{
		StreamID: stream.ID(),
		DataSize: dataSize + uint64(len(segmentation)),
	}
	if err := writer.write(streamSection); err != nil {
		return err
	}
	// Write chunk sizes
	if err := writer.write(segmentation); err != nil {
		// TODO: The cache file is corrupt now. We should probably delete it.
		return err
	}

	// Write chunk data
	for _, direction := range []index.Direction{index.DirectionClientToServer, index.DirectionServerToClient} {
		for _, convertedPacket := range convertedPackets {
			if convertedPacket.Direction != direction {
				continue
			}
			if err := writer.write(convertedPacket.Content); err != nil {
				// TODO: The cache file is corrupt now. We should probably delete it.
				return err
			}
		}
	}

	if err := writer.buffer.Flush(); err != nil {
		return err
	}

	// Remember where to look for this stream.
	writer.cache.availableStreams.Set(uint(stream.ID()))
	writer.cache.containedStreamIds[stream.ID()] = offset

	// Create a reader if this is the first time writing to the cache file.
	if writer.cache.reader == nil {
		reader, err := writer.cache.newReader()
		if err != nil {
			return err
		}
		writer.cache.reader = reader
	}

	return nil
}
