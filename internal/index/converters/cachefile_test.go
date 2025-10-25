package converters

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spq/pkappa2/internal/index"
)

func TestVarIntRoundtrip(t *testing.T) {
	testcases := []uint64{
		0,
		1,
		127,
		128,
		300,
		1<<20 + 123,
		1<<40 + 999,
	}
	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := writeVarInt(&buf, tc); err != nil {
				t.Fatalf("writeVarInt(%d) failed: %v", tc, err)
			}
			r := bufio.NewReader(bytes.NewReader(buf.Bytes()))
			v, _, err := readVarInt(r)
			if err != nil {
				t.Fatalf("readVarInt(%d) failed: %v", tc, err)
			}
			if v != tc {
				t.Fatalf("varint roundtrip mismatch: wrote %d, read %d", tc, v)
			}
		})
	}
}

func TestVarBytesRoundtrip(t *testing.T) {
	testcases := [][]byte{
		{},
		{0x00},
		{0x01, 0x02, 0x03},
		{0xff, 0x80, 0x7f, 0x55, 0x33, 0x11, 0x00, 0xab, 0xcd},
		{1},
		{1, 2},
		{1, 2, 3},
		{1, 2, 3, 4},
		{1, 2, 3, 4, 5},
		{1, 2, 3, 4, 5, 6},
		{1, 2, 3, 4, 5, 6, 7},
		{1, 2, 3, 4, 5, 6, 7, 8},
		{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := writeVarBytes(&buf, tc); err != nil {
				t.Fatalf("writeVarBytes failed: %v", err)
			}
			r := bufio.NewReader(bytes.NewReader(buf.Bytes()))
			out, _, err := readVarBytes(r)
			if err != nil {
				t.Logf("bytes: %v", buf.Bytes())
				t.Fatalf("readVarBytes failed: %v", err)
			}
			if !bytes.Equal(out, tc) {
				t.Fatalf("varbytes roundtrip mismatch: wrote %v, read %v", tc, out)
			}

		})
	}
}

func TestStringRoundtrip(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"The quick brown fox jumps over the lazy dog",
	}
	for _, s := range cases {
		var buf bytes.Buffer
		if _, err := writeString(&buf, s); err != nil {
			t.Fatalf("writeString failed: %v", err)
		}
		r := bufio.NewReader(bytes.NewReader(buf.Bytes()))
		out, _, err := readString(r)
		if err != nil {
			t.Fatalf("readString failed: %v", err)
		}
		if out != s {
			t.Fatalf("string roundtrip mismatch: wrote %q, read %q", s, out)
		}
	}
}

func TestCachefile(t *testing.T) {
	// Create a new cache file
	cacheFilePath := fmt.Sprintf("%s/test.cache", t.TempDir())

	cf, err := NewCacheFile(cacheFilePath)
	if err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}
	t.Cleanup(func() {
		cf.Close()
	})

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Write and read back a simple stream
	packets := []index.Data{
		{
			Direction: index.DirectionClientToServer,
			Content:   []byte("1"),
			Time:      t1,
		},
		{
			Direction: index.DirectionClientToServer,
			Content:   []byte("2"),
			Time:      t1,
		},
		{
			Direction:   index.DirectionServerToClient,
			Content:     []byte("3"),
			Time:        t1.Add(1 * time.Second),
			ContentType: "foo",
		},

		{
			Direction: index.DirectionClientToServer,
			Content:   []byte("4"),
			Time:      t1.Add(2 * time.Second),
		},
		{
			Direction: index.DirectionClientToServer,
			Content:   []byte("5"),
			Time:      t1.Add(2 * time.Second),
		},
		{
			Direction:   index.DirectionServerToClient,
			Content:     []byte("6"),
			Time:        t1.Add(2 * time.Second),
			ContentType: "bar",
		},

		{
			Direction: index.DirectionClientToServer,
			Content:   []byte("7"),
			Time:      t1.Add(3 * time.Second),
		},
		{
			Direction: index.DirectionClientToServer,
			Content:   []byte("8"),
			Time:      t1.Add(3 * time.Second),
		},
		{
			Direction:   index.DirectionServerToClient,
			Content:     []byte("9"),
			Time:        t1.Add(4 * time.Second),
			ContentType: "foo",
		},
	}
	if err := cf.setData(123, t1, packets); err != nil {
		t.Fatalf("failed to write stream: %v", err)
	}
	if cf.streamInfos[123].offset != 16 {
		t.Fatalf("stream offset = %d, want 16", cf.streamInfos[123].offset)
	}

	check := func() {
		got, bytesDirectionClientToServer, bytesDirectionServerToClient, err := cf.data(123, t1)
		if err != nil {
			t.Fatalf("failed to read stream: %v", err)
		}
		if len(got) != len(packets) {
			t.Fatalf("read back %d packets, want %d", len(got), len(packets))
		}
		for i := range got {
			if got[i].Direction != packets[i].Direction {
				t.Errorf("packet %d: direction = %v, want %v", i, got[i].Direction, packets[i].Direction)
			}
			if !bytes.Equal(got[i].Content, packets[i].Content) {
				t.Errorf("packet %d: content = %v, want %v", i, got[i].Content, packets[i].Content)
			}
			if !got[i].Time.Equal(packets[i].Time) {
				t.Errorf("packet %d: time = %v, want %v", i, got[i].Time, packets[i].Time)
			}
			if got[i].ContentType != packets[i].ContentType {
				t.Errorf("packet %d: content type = %v, want %v", i, got[i].ContentType, packets[i].ContentType)
			}
		}
		var wantBytesDirectionClientToServer, wantBytesDirectionServerToClient uint64
		for _, p := range packets {
			if p.Direction == index.DirectionClientToServer {
				wantBytesDirectionClientToServer += uint64(len(p.Content))
			} else {
				wantBytesDirectionServerToClient += uint64(len(p.Content))
			}
		}
		if bytesDirectionClientToServer != wantBytesDirectionClientToServer {
			t.Errorf("bytesDirectionClientToServer = %d, want %d", bytesDirectionClientToServer, wantBytesDirectionClientToServer)
		}
		if bytesDirectionServerToClient != wantBytesDirectionServerToClient {
			t.Errorf("bytesDirectionServerToClient = %d, want %d", bytesDirectionServerToClient, wantBytesDirectionServerToClient)
		}
		data, lengths, clientBytes, serverBytes, present, err := cf.DataForSearch(123)
		if err != nil {
			t.Fatalf("DataForSearch failed: %v", err)
		}
		if !present {
			t.Fatalf("DataForSearch: stream not present")
		}
		if clientBytes != bytesDirectionClientToServer {
			t.Errorf("DataForSearch: clientBytes = %d, want %d", clientBytes, bytesDirectionClientToServer)
		}
		if serverBytes != bytesDirectionServerToClient {
			t.Errorf("DataForSearch: serverBytes = %d, want %d", serverBytes, bytesDirectionServerToClient)
		}
		if got, want := string(data[index.DirectionClientToServer]), "124578"; got != want {
			t.Errorf("DataForSearch: data[%d] = %q, want %q", index.DirectionClientToServer, got, want)
		}
		if got, want := string(data[index.DirectionServerToClient]), "369"; got != want {
			t.Errorf("DataForSearch: data[%d] = %q, want %q", index.DirectionServerToClient, got, want)
		}
		if len(lengths) != 10 {
			t.Fatalf("DataForSearch: lengths length = %d, want 10", len(lengths))
		}
		for i, v := range lengths {
			wa, wb := i-(i/3), i/3
			ga, gb := v[0], v[1]
			if ga != wa || gb != wb {
				t.Errorf("DataForSearch: lengths[%d] = [%d, %d], want [%d, %d]", i, ga, gb, wa, wb)
			}
		}
	}
	check()

	// Reopen the cache file and check again
	wantFileSize := cf.streamInfos[123].offset + int64(cf.streamInfos[123].size)
	cf.Close()

	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	t.Logf("Cache content: %q", string(data))
	if int64(len(data)) != wantFileSize {
		t.Fatalf("cache file size = %d, want %d", len(data), wantFileSize)
	}

	cf, err = NewCacheFile(cacheFilePath)
	if err != nil {
		t.Fatalf("failed to reopen cache file: %v", err)
	}
	if cf.streamInfos[123].offset != 16 {
		t.Fatalf("after reopen, stream offset = %d, want 16", cf.streamInfos[123].offset)
	}
	if cf.streamInfos[123].size != uint64(wantFileSize-16) {
		t.Fatalf("after reopen, stream size = %d, want %d", cf.streamInfos[123].size, wantFileSize-16)
	}
	check()

	// Re-add the same stream again and re-open the file
	if err := cf.setData(123, t1, packets); err != nil {
		t.Fatalf("failed to re-write stream: %v", err)
	}
	if cf.streamInfos[123].offset != int64(wantFileSize)+8 {
		t.Fatalf("after re-adding, stream offset = %d, want %d", cf.streamInfos[123].offset, wantFileSize+8)
	}
	cf.Close()

	data2, err := os.ReadFile(cacheFilePath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	t.Logf("Cache content: %q", string(data2))
	if int64(len(data2)) != wantFileSize*2-8 {
		t.Fatalf("cache file size = %d, want %d", len(data2), wantFileSize*2-8)
	}

	cf, err = NewCacheFile(cacheFilePath)
	if err != nil {
		t.Fatalf("failed to reopen cache file after re-adding stream: %v", err)
	}
	if cf.streamInfos[123].offset != 16 {
		t.Fatalf("after re-adding, stream offset = %d, want %d", cf.streamInfos[123].offset, 16)
	}
	check()
}
