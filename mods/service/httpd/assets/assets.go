package assets

import (
	"embed"
	"io/fs"
	"net/http"
	"time"
)

//go:embed favicon.ico
var favicon []byte

//go:embed apple-touch-icon.png
var appleTouchIcon []byte

//go:embed apple-touch-icon-precomposed.png
var appleTouchIconPrecomposed []byte

//go:embed echarts/*
var echartsDir embed.FS

func EchartsDir() http.FileSystem {
	return &staticFSWrap{
		base:         http.FS(echartsDir),
		fixedModTime: time.Now(),
	}
}

type staticFSWrap struct {
	base         http.FileSystem
	fixedModTime time.Time
}

type staticFile struct {
	http.File
	modTime time.Time
}

func (fsw *staticFSWrap) Open(name string) (http.File, error) {
	f, err := fsw.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &staticFile{f, fsw.fixedModTime}, nil
}

func (f *staticFile) Stat() (fs.FileInfo, error) {
	stat, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &staticStat{stat, f.modTime}, nil
}

func (f *staticFile) ModTime() time.Time {
	return f.modTime
}

type staticStat struct {
	fs.FileInfo
	modTime time.Time
}

func (stat *staticStat) ModTime() time.Time {
	return stat.modTime
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.RequestURI {
	case "/favicon.ico":
		w.Header().Set("Content-Type", "image/x-icon")
		w.WriteHeader(http.StatusOK)
		w.Write(favicon)
	case "/apple-touch-icon.png":
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(appleTouchIcon)
	case "/apple-touch-icon-precomposed.png":
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(appleTouchIconPrecomposed)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
