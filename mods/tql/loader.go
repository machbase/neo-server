package tql

import (
	"expvar"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

var instance *loader

func Init() {
	instance = &loader{}
	instance.vap = NewMemoryFS("/web/api/tql-assets/")
	go instance.vap.Start()
}

func Deinit() {
	if instance.vap != nil {
		instance.vap.Stop()
	}
	instance = nil
}

func HttpFileSystem() http.FileSystem {
	return instance.vap
}

func AssetProvider() VolatileAssetsProvider {
	return instance.vap
}

type VolatileAssetsProvider interface {
	VolatileFilePrefix() string
	VolatileFileWrite(name string, val []byte, deadline time.Time) fs.File
}

type Loader interface {
	Load(path string) (*Script, error)
}

type loader struct {
	vap *MemoryFS
}

func NewLoader() Loader {
	return instance
}

func (ld *loader) Load(path string) (*Script, error) {
	var ret *Script
	fsmgr := ssfs.Default()
	ent, err := fsmgr.Get("/" + strings.TrimPrefix(path, "/"))
	if err != nil || ent.IsDir {
		return nil, fmt.Errorf("not found '%s'", path)
	}
	ret = &Script{
		path0:   filepath.ToSlash(path),
		content: ent.Content,
		vap:     ld.vap,
	}
	return ret, nil
}

type Script struct {
	path0   string
	content []byte
	vap     VolatileAssetsProvider
}

func (sc *Script) String() string {
	return fmt.Sprintf("path: %s", sc.path0)
}

type MemoryFS struct {
	Prefix   string
	list     map[string]*MemoryFile
	listLock sync.Mutex
	stop     chan bool
}

func NewMemoryFS(prefix string) *MemoryFS {
	ret := &MemoryFS{
		Prefix: prefix,
		list:   map[string]*MemoryFile{},
		stop:   make(chan bool),
	}
	expvar.Publish("machbase:memoryfs:count", expvar.Func(ret.statzCount))
	expvar.Publish("machbase:memoryfs:total_size", expvar.Func(ret.statzTotalSize))
	return ret
}

var _ VolatileAssetsProvider = (*MemoryFS)(nil)
var _ http.FileSystem = (*MemoryFS)(nil)

func (fs *MemoryFS) Start() {
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

func (fs *MemoryFS) statzCount() any { return len(fs.list) }

func (fs *MemoryFS) statzTotalSize() any {
	var total int64
	fs.listLock.Lock()
	for _, v := range fs.list {
		total += int64(len(v.data))
	}
	fs.listLock.Unlock()
	return total
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
