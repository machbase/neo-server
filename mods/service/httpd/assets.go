package httpd

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/tql"
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

type MemoryFS struct {
	Prefix   string
	list     map[string]*MemoryFile
	listLock sync.Mutex
	stop     chan bool
}

var _ tql.VolatileAssetsProvider = &MemoryFS{}

func (fs *MemoryFS) Start() {
	fs.stop = make(chan bool)
	fs.list = map[string]*MemoryFile{}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			fs.listLock.Lock()
			for k, v := range fs.list {
				if v.deadline.Before(now) {
					delete(fs.list, k)
				}
			}
			fs.listLock.Unlock()
		case <-fs.stop:
			return
		}
	}
}

func (fs *MemoryFS) Stop() {
	fs.stop <- true
}

func (fs *MemoryFS) Open(name string) (http.File, error) {
	fs.listLock.Lock()
	defer fs.listLock.Unlock()
	if f, ok := fs.list[name]; ok {
		if time.Now().Before(f.deadline) {
			return f.Clone(), nil
		}
	}
	return nil, os.ErrNotExist
}

func (fs *MemoryFS) VolatileFilePrefix() string {
	return fs.Prefix
}

func (fs *MemoryFS) VolatileFileWrite(name string, val []byte, deadline time.Time) fs.File {
	ret := &MemoryFile{
		Name:     name,
		deadline: deadline,
		at:       0,
		data:     val,
		fs:       fs,
	}
	fs.listLock.Lock()
	fs.list[name] = ret
	fs.listLock.Unlock()
	return ret
}

func (fs *MemoryFS) Statz() map[string]any {
	fs.listLock.Lock()
	total := int64(0)
	count := len(fs.list)
	for _, v := range fs.list {
		total += int64(len(v.data))
	}
	fs.listLock.Unlock()

	return map[string]any{
		"count":      count,
		"total_size": total,
	}
}

type MemoryFile struct {
	Name     string
	deadline time.Time
	fs       *MemoryFS
	at       int64
	data     []byte
}

func (f *MemoryFile) Clone() *MemoryFile {
	return &MemoryFile{
		Name:     f.Name,
		deadline: f.deadline,
		fs:       f.fs,
		at:       0,
		data:     f.data,
	}
}

func (f *MemoryFile) Close() error {
	return nil
}

func (f *MemoryFile) Stat() (os.FileInfo, error) {
	return &memoryFileInfo{f}, nil
}

func (f *MemoryFile) Readdir(count int) ([]os.FileInfo, error) {
	f.fs.listLock.Lock()
	defer f.fs.listLock.Unlock()
	ret := make([]os.FileInfo, len(f.fs.list))
	i := 0
	for _, file := range f.fs.list {
		ret[i], _ = file.Stat()
		i++
	}
	return ret, nil
}

func (f *MemoryFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		f.at = offset
	case 1:
		f.at += offset
	case 2:
		f.at = int64(len(f.data)) + offset
	}
	return f.at, nil
}

func (f *MemoryFile) Read(b []byte) (int, error) {
	i := 0
	for f.at < int64(len(f.data)) && i < len(b) {
		b[i] = f.data[f.at]
		i++
		f.at++
	}
	return i, nil
}

type memoryFileInfo struct {
	file *MemoryFile
}

func (fi *memoryFileInfo) Name() string       { return fi.file.Name }
func (fi *memoryFileInfo) Size() int64        { return int64(len(fi.file.data)) }
func (fi *memoryFileInfo) Mode() os.FileMode  { return os.ModeTemporary }
func (fi *memoryFileInfo) ModTime() time.Time { return time.Now() }
func (fi *memoryFileInfo) IsDir() bool        { return false }
func (fi *memoryFileInfo) Sys() any           { return nil }
