package tools

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func ListFiles(dir, extension string) ([]string, error) {
	fs, err := ioutil.ReadDir(dir)
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

func AssertFolderRWXPermissions(name, dir string) {
	fi, err := os.Lstat(dir)
	if err != nil {
		log.Fatal(err)
	}
	if fi.Mode().Perm()&0750 != 0750 {
		log.Fatalf("%s %s has too strict permissions: %#o. Need rwx.", name, dir, fi.Mode().Perm())
	}
}
