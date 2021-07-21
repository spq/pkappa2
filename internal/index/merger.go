package index

import (
	"log"
	"os"

	"github.com/spq/pkappa2/internal/tools"
)

func Merge(indexDir string, indexes []*Reader) ([]*Reader, error) {
	ws := []*Writer{}
	rs := []*Reader{}
	err := func() error {
		for idxIdx := len(indexes); idxIdx > 0; {
			idxIdx--
			idx := indexes[idxIdx]
			for wIdx := 0; wIdx <= len(ws); wIdx++ {
				if wIdx == len(ws) {
					w, err := NewWriter(tools.MakeFilename(indexDir, "idx"))
					if err != nil {
						return err
					}
					ws = append(ws, w)
				}
				w := ws[wIdx]

				added, err := w.AddIndex(idx)
				if err != nil {
					return err
				}
				if added {
					break
				}
			}
		}
		for _, w := range ws {
			r, err := w.Finalize()
			if err != nil {
				return err
			}
			rs = append(rs, r)
		}
		return nil
	}()
	if err != nil {
		for _, r := range rs {
			r.Close()
		}
		for _, w := range ws {
			w.Close()
			os.Remove(w.filename)
		}
		return nil, err
	}
	inputFiles := []string{}
	outputFiles := []string{}
	for _, i := range indexes {
		inputFiles = append(inputFiles, i.filename)
	}
	for _, i := range rs {
		outputFiles = append(outputFiles, i.filename)
	}
	log.Printf("merged indexes %q into %q\n", inputFiles, outputFiles)
	return rs, nil
}
