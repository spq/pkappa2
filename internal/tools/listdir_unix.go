//go:build unix
// +build unix

package tools

import (
	"golang.org/x/sys/unix"
	"log"
)

func AssertFolderRWXPermissions(name, dir string) {
	err := unix.Access(dir, unix.R_OK|unix.W_OK|unix.X_OK)
	if err != nil {
		log.Fatalf("%s %s has too strict permissions. Need rwx.", name, dir)
	}
}

func IsFileExecutable(name string) bool {
	if err := unix.Access(path, unix.X_OK); err != nil {
		return false
	}
	return true
}
