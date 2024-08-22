package index

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestMerge(t *testing.T) {
	tmpDir := t.TempDir()
	t1, err := time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse failed with error: %v", err)
	}

	inputs := []map[uint64]streamInfo{
		{

			0:  makeStream("1.2.3.40:1", "105.6.7.8:9", t1.Add(time.Hour*1), []string{"Lorem", "ipsum", "dolor", "sit", "amet,"}),
			1:  makeStream("1.2.30.4:2", "5.106.7.8:8", t1.Add(time.Hour*2), []string{"", "sed", "do", "eiusmod", "tempor"}),
			2:  makeStream("[12::34]:3", "[::1234]:7", t1.Add(time.Hour*3), []string{"magna", "aliqua.", "Ut", "enim", "ad"}),
			10: makeStream("1.20.3.4:4", "5.6.107.8:6", t1.Add(time.Hour*4), []string{"", "exercitation", "ullamco", "laboris"}),
			11: makeStream("10.2.3.4:5", "5.6.7.108:5", t1.Add(time.Hour*5), []string{"commodo", "consequat.", "Duis", "aute"}),
			12: makeStream("[0::34:12]:6", "[0::12:34]:4", t1.Add(time.Hour*6), []string{"", "in", "voluptate", "velit", "esse"}),
		},
		{
			1:  makeStream("1.2.30.4:2", "5.106.7.8:8", t1.Add(time.Hour*2), []string{"", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore", "et", "dolore"}),
			2:  makeStream("[12::34]:3", "[::1234]:7", t1.Add(time.Hour*3), []string{"magna", "aliqua.", "Ut", "enim", "ad", "minim", "veniam,", "quis", "nostrud"}),
			3:  makeStream("1.2.3.40:1", "105.6.7.8:9", t1.Add(time.Hour*4), []string{"Lorem", "ipsum", "dolor", "sit", "amet,", "consectetur", "adipiscing", "elit,"}),
			11: makeStream("10.2.3.4:5", "5.6.7.108:5", t1.Add(time.Hour*5), []string{"commodo", "consequat.", "Duis", "aute", "irure", "dolor", "in", "reprehenderit"}),
			12: makeStream("[0::34:12]:6", "[0::12:34]:4", t1.Add(time.Hour*6), []string{"", "in", "voluptate", "velit", "esse", "cillum", "dolore", "eu", "fugiat"}),
			13: makeStream("1.20.3.4:4", "5.6.107.8:6", t1.Add(time.Hour*7), []string{"", "exercitation", "ullamco", "laboris", "nisi", "ut", "aliquip", "ex", "ea"}),
		},
		{
			20: makeStream("0.0.0.0:0", "0.0.0.0:0", t1, []string{""}),
		},
	}

	indexes := []*Reader(nil)
	for _, streams := range inputs {
		index, err := makeIndex(tmpDir, streams, nil)
		if err != nil {
			t.Errorf("makeIndex failed with error: %v", err)
		}
		indexes = append(indexes, index)
	}

	wantStreams := map[uint64]int{
		0:  0,
		1:  1,
		2:  1,
		3:  1,
		10: 0,
		11: 1,
		12: 1,
		13: 1,
		20: 2,
	}

	merged, err := Merge(tmpDir, indexes)
	if err != nil {
		t.Errorf("Merge failed with error: %v", err)
	}

	if len(merged) != 1 {
		t.Fatalf("Expected 1 merged index, but got %d", len(merged))
	}

	gotJson := map[uint64][]byte{}
	gotData := map[uint64][]Data{}
	err = merged[0].AllStreams(func(s *Stream) error {
		json, err := s.MarshalJSON()
		if err != nil {
			return err
		}
		data, err := s.Data()
		if err != nil {
			return err
		}
		gotJson[s.StreamID] = json
		gotData[s.StreamID] = data
		return nil
	})
	if err != nil {
		t.Errorf("AllStreams failed with error: %v", err)
	}
	if len(gotJson) != len(wantStreams) {
		t.Fatalf("Expected %d streams, but got %d", len(wantStreams), len(gotJson))
	}
	for streamID, i := range wantStreams {
		wantStream, err := indexes[i].StreamByID(streamID)
		if err != nil {
			t.Errorf("StreamByID failed with error: %v", err)
		}
		wantJson, err := wantStream.MarshalJSON()
		if err != nil {
			t.Errorf("MarshalJSON failed with error: %v", err)
		}
		var got, want map[string]interface{}
		if err := json.Unmarshal(wantJson, &want); err != nil {
			t.Errorf("json.Unmarshal failed with error: %v", err)
		}
		if err := json.Unmarshal(gotJson[streamID], &got); err != nil {
			t.Errorf("json.Unmarshal failed with error: %v", err)
		}
		delete(got, "Index")
		delete(want, "Index")
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Stream %d mismatch:\nGot:  %v\nWant: %v", streamID, got, want)
		}
		wantData, err := wantStream.Data()
		if err != nil {
			t.Errorf("Data failed with error: %v", err)
		}
		if !reflect.DeepEqual(gotData[streamID], wantData) {
			t.Errorf("Stream %d data mismatch:\nGot:  %v\nWant: %v", streamID, gotData[streamID], wantData)
		}

	}
	if err := merged[0].Close(); err != nil {
		t.Errorf("Close failed with error: %v", err)
	}
}
