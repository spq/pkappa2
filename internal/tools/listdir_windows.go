//go:build windows
// +build windows

package tools

import (
	"log"
	"os"
)

func AssertFolderRWXPermissions(name, dir string) {
	fi, err := os.Stat(dir)
	if err != nil {
		log.Fatalf("%s %s doesn't exist: %v", name, dir, err)
	}
	mode := fi.Mode()
	if !mode.IsDir() {
		log.Fatalf("%s %s is not a folder", name, dir)
	}
	if uint(mode)&0777 == 0 {
		log.Fatalf("%s %s has too strict permissions. Need rwx has %q", name, dir, mode.Perm())
	}
}

func IsFileExecutable(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		return false
	}
	mode := fi.Mode()
	if !mode.IsRegular() {
		return false
	}
	//if uint(mode)&0111 == 0 {
	//	return false
	//}
	return true
}
