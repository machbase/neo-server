package httpd

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed web/*
var webFs embed.FS

func GetAssets(dir string) http.FileSystem {
	dir = strings.TrimPrefix(strings.TrimSuffix(dir, "/"), "/")
	subfs, err := fs.Sub(webFs, "web/"+dir)
	if err != nil {
		panic(err)
	}

	return &assetFileSystem{
		FileSystem: http.FS(subfs),
	}
}

type assetFileSystem struct {
	http.FileSystem
	Prefix string
}

func (fs *assetFileSystem) Open(name string) (http.File, error) {
	if strings.HasSuffix(name, "/") {
		return fs.FileSystem.Open(name)
	} else if isWellKnownFileType(name) {
		return fs.FileSystem.Open(name)
	} else {
		return fs.FileSystem.Open("index.html")
	}
}

var wellknowns = map[string]bool{
	".css":   true,
	".gif":   true,
	".html":  true,
	".htm":   true,
	".ico":   true,
	".jpg":   true,
	".jpeg":  true,
	".json":  true,
	".js":    true,
	".map":   true,
	".png":   true,
	".svg":   true,
	".ttf":   true,
	".txt":   true,
	".yaml":  true,
	".yml":   true,
	".woff":  true,
	".woff2": true,
}

func isWellKnownFileType(name string) bool {
	ext := filepath.Ext(name)
	if len(ext) == 0 {
		return false
	}

	if _, ok := wellknowns[strings.ToLower(ext)]; ok {
		return true
	}
	return false
}
