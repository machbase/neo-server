package server

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed eula.txt
var eulaTxt string

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

func WrapAssets(path string) http.FileSystem {
	fs := http.Dir(path)
	ret := &wrapFileSystem{base: fs}
	return ret
}

type wrapFileSystem struct {
	base http.FileSystem
}

func (fs *wrapFileSystem) Open(name string) (http.File, error) {
	ret, err := fs.base.Open(name)
	if err == nil {
		return ret, nil
	}
	return fs.base.Open("index.html")
}

//go:embed web/*
var webFs embed.FS

func GetAssets(dir string) http.FileSystem {
	dir = strings.TrimPrefix(strings.TrimSuffix(dir, "/"), "/")
	_, err := fs.Sub(webFs, "web/"+dir)
	if err != nil {
		panic(err)
	}

	return &assetFileSystem{
		StaticFSWrap: StaticFSWrap{
			TrimPrefix:   "",
			Base:         http.FS(webFs),
			FixedModTime: time.Now(),
		},
		prefix: "web/" + dir,
	}
}

type assetFileSystem struct {
	StaticFSWrap
	prefix string
}

func (fs *assetFileSystem) Open(name string) (http.File, error) {
	toks := strings.SplitN(name, "?", 2)
	if len(toks) == 0 {
		return nil, os.ErrNotExist
	}
	name = toks[0]
	if strings.HasSuffix(name, "/") {
		return fs.StaticFSWrap.Open(name)
	} else if isWellKnownFileType(name) {
		return fs.StaticFSWrap.Open(fs.prefix + name)
	} else {
		return fs.StaticFSWrap.Open(fs.prefix + "/index.html")
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
	".webp":  true,
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
