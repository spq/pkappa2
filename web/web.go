package web

import (
	"embed"
	"io/fs"
	"os"
	"path"
)

//go:embed dist/*
var assets embed.FS

type FS struct{}

func (*FS) Open(name string) (fs.File, error) {
	name = path.Join("dist", name)
	f, err := assets.Open(name)
	if err != nil && os.IsNotExist(err) {
		f, err = assets.Open("dist/index.html")
	}
	return f, err
}
