package tools

import (
	"golang.org/x/sys/unix"
	"io/ioutil"
	"log"
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
	err := unix.Access(dir, unix.R_OK|unix.W_OK|unix.X_OK)
	if err != nil {
		log.Fatalf("%s %s has too strict permissions. Need rwx.", name, dir)
	}
}
