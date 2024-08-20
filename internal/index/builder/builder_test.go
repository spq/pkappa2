package builder

import (
	"path"
	"reflect"
	"testing"
	"time"
)

var (
	t1 time.Time
)

func init() {
	var err error
	t1, err = time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	if err != nil {
		panic(err)
	}
}

func TestSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name     string
		snapshot []*snapshot
	}{
		{
			name:     "empty",
			snapshot: []*snapshot{},
		},
		{
			name:     "nil",
			snapshot: nil,
		},
		{
			name: "single",
			snapshot: []*snapshot{
				{
					timestamp:         t1,
					referencedPackets: map[string][]uint64{"a": {1, 2, 3}},
					chunkCount:        42,
				},
			},
		},
		{
			name: "multiple",
			snapshot: []*snapshot{
				{
					timestamp:         t1,
					referencedPackets: map[string][]uint64{"a": {1, 2, 3}},
					chunkCount:        42,
				},
				{
					timestamp:         t1.Add(time.Hour),
					referencedPackets: map[string][]uint64{"b": {4, 5, 6}},
					chunkCount:        43,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn := path.Join(t.TempDir(), "test.snap")
			if err := saveSnapshots(fn, tc.snapshot); err != nil {
				t.Fatalf("saveSnapshots failed: %v", err)
			}
			got, err := loadSnapshots(fn)
			if err != nil {
				t.Fatalf("loadSnapshots failed: %v", err)
			}
			if len(got) != len(tc.snapshot) {
				t.Fatalf("len(got)=%d, want %d", len(got), len(tc.snapshot))
			}
			for i, want := range tc.snapshot {
				got := *got[i]
				if got.timestamp.UTC() != want.timestamp.UTC() {
					t.Errorf("got=%v, want %v", got.timestamp, want.timestamp)
				}
				if got.chunkCount != want.chunkCount {
					t.Errorf("got=%v, want %v", got.chunkCount, want.chunkCount)
				}
				if !reflect.DeepEqual(got.referencedPackets, want.referencedPackets) {
					t.Errorf("got=%v, want %v", got.referencedPackets, want.referencedPackets)
				}
			}
		})
	}
}
