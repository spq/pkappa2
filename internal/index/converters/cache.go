package converters

import (
	"fmt"
	"path/filepath"

	"github.com/spq/pkappa2/internal/index"
)

type (
	CachedConverter struct {
		converter *Converter
		cacheFile *cacheFile
	}
	Statistics struct {
		Name              string
		CachedStreamCount uint64
		Processes         []ProcessStats
	}
)

func NewCache(converterName, executablePath, indexCachePath string) (*CachedConverter, error) {
	filename := fmt.Sprintf("converterindex-%s.cidx", converterName)
	cachePath := filepath.Join(indexCachePath, filename)

	cacheFile, err := NewCacheFile(cachePath)
	if err != nil {
		return nil, err
	}

	return &CachedConverter{
		converter: New(converterName, executablePath),
		cacheFile: cacheFile,
	}, nil
}

func (cache *CachedConverter) Close() error {
	return cache.cacheFile.Close()
}

func (cache *CachedConverter) Name() string {
	return cache.converter.Name()
}

func (cache *CachedConverter) Statistics() *Statistics {
	return &Statistics{
		Name:              cache.converter.Name(),
		CachedStreamCount: cache.cacheFile.StreamCount(),
		Processes:         cache.converter.ProcessStats(),
	}
}

func (cache *CachedConverter) Stderrs() [][]string {
	return cache.converter.Stderrs()
}

func (cache *CachedConverter) Reset() error {
	// Stop all converter processes.
	cache.converter.Reset()

	// Remove the cache file.
	return cache.cacheFile.Reset()
}

func (cache *CachedConverter) Contains(streamID uint64) bool {
	return cache.cacheFile.Contains(streamID)
}

func (cache *CachedConverter) Data(stream *index.Stream) (data []index.Data, clientBytes, serverBytes uint64, err error) {
	// See if the stream data is cached already.
	data, clientBytes, serverBytes, err = cache.cacheFile.Data(stream.ID())
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
	if err := cache.cacheFile.SetData(stream.ID(), convertedPackets); err != nil {
		return nil, 0, 0, err
	}
	return convertedPackets, clientBytes, serverBytes, nil
}

func (cache *CachedConverter) DataForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, error) {
	return cache.cacheFile.DataForSearch(streamID)
}
