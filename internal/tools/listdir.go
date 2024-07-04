package tools

import (
	"os"
	"path/filepath"
	"strings"
)

func ListFiles(dir, extension string) ([]string, error) {
	fs, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	res := []string{}
	for _, f := range fs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), "."+extension) {
			continue
		}
		res = append(res, filepath.Join(dir, f.Name()))
	}
	return res, nil
}
