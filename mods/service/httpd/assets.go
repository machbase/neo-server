package httpd

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed assets/*
var assetsDir embed.FS

func AssetsDir() http.FileSystem {
	return &StaticFSWrap{
		TrimPrefix:      "/web/",
		PrependRealPath: "/assets/",
		Base:            http.FS(assetsDir),
		FixedModTime:    time.Now(),
	}
}

type StaticFSWrap struct {
	TrimPrefix      string
	PrependRealPath string
	Base            http.FileSystem
	FixedModTime    time.Time
}

type staticFile struct {
	http.File
	modTime time.Time
}

func (fsw *StaticFSWrap) Open(name string) (http.File, error) {
	if !strings.HasPrefix(name, fsw.TrimPrefix) {
		name = strings.TrimSuffix(fsw.TrimPrefix, "/") + "/" + strings.TrimPrefix(name, "/")
	}
	f, err := fsw.Base.Open(fsw.PrependRealPath + strings.TrimPrefix(name, fsw.TrimPrefix))
	if err != nil {
		return nil, err
	}
	return &staticFile{File: f, modTime: fsw.FixedModTime}, nil
}

func (f *staticFile) Stat() (fs.FileInfo, error) {
	stat, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &staticStat{FileInfo: stat, modTime: f.modTime}, nil
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
