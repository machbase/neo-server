package engine

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// VirtualFileProperty describes metadata for a virtual file.
type VirtualFileProperty struct {
	CreateTime time.Time
	ModTime    time.Time
	Mode       fs.FileMode
}

// VirtualFS is a programmable in-memory filesystem similar to fstest.MapFS,
// but with mutation APIs for adding and removing entries at runtime.
type VirtualFS struct {
	mu    sync.RWMutex
	files map[string]*virtualFileEntry
}

type virtualFileEntry struct {
	data []byte
	prop VirtualFileProperty
}

var _ fs.FS = (*VirtualFS)(nil)
var _ fs.ReadFileFS = (*VirtualFS)(nil)
var _ fs.ReadDirFS = (*VirtualFS)(nil)
var _ fs.StatFS = (*VirtualFS)(nil)

var defaultTimestamp = time.Unix(1772757478, 0) // 2026-03-06 12:37:58 UTC

func NewVirtualFS() *VirtualFS {
	return &VirtualFS{files: make(map[string]*virtualFileEntry)}
}

type VirtualFileContent []byte

// AddFile adds a virtual file entry with caller-provided content and metadata.
// The content must be []byte or string.
func (vfs *VirtualFS) AddFile(name string, content any, prop VirtualFileProperty) error {
	rel, err := normalizeVirtualPath("create", name, false)
	if err != nil {
		return err
	}

	data, err := toVirtualBytes(content)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.files[rel]; exists {
		return &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}

	// A file cannot be created under an existing file path.
	for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
		if _, exists := vfs.files[parent]; exists {
			return &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
		}
	}

	// A file path cannot replace a virtual directory already implied by children.
	prefix := rel + "/"
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, prefix) {
			return &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
		}
	}

	prop = normalizeVirtualProperty(prop)
	vfs.files[rel] = &virtualFileEntry{data: data, prop: prop}
	return nil
}

// Remove removes a virtual entry.
// If name points to a directory, all descendant files are removed.
func (vfs *VirtualFS) Remove(name string) error {
	rel, err := normalizeVirtualPath("remove", name, true)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if rel == "." {
		if len(vfs.files) == 0 {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
		}
		vfs.files = make(map[string]*virtualFileEntry)
		return nil
	}

	if _, exists := vfs.files[rel]; exists {
		delete(vfs.files, rel)
		return nil
	}

	prefix := rel + "/"
	removed := 0
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, prefix) {
			delete(vfs.files, filePath)
			removed++
		}
	}
	if removed == 0 {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	return nil
}

func (vfs *VirtualFS) Open(name string) (fs.File, error) {
	rel, err := normalizeVirtualPath("open", name, true)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	if entry, ok := vfs.files[rel]; ok {
		info := buildVirtualFileInfo(path.Base(rel), false, int64(len(entry.data)), entry.prop.ModTime, entry.prop.CreateTime, entry.prop.Mode)
		return &virtualOpenFile{reader: bytes.NewReader(entry.data), info: info}, nil
	}

	if !vfs.hasDirLocked(rel) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	entries := vfs.readDirLocked(rel)
	info := buildVirtualFileInfo(dirName(rel), true, 0, time.Time{}, time.Time{}, fs.ModeDir|0755)
	return &virtualOpenDir{info: info, entries: entries}, nil
}

func (vfs *VirtualFS) ReadFile(name string) ([]byte, error) {
	rel, err := normalizeVirtualPath("readfile", name, false)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	entry, ok := vfs.files[rel]
	if !ok {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
	}
	ret := make([]byte, len(entry.data))
	copy(ret, entry.data)
	return ret, nil
}

func (vfs *VirtualFS) Stat(name string) (fs.FileInfo, error) {
	rel, err := normalizeVirtualPath("stat", name, true)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	if entry, ok := vfs.files[rel]; ok {
		return buildVirtualFileInfo(path.Base(rel), false, int64(len(entry.data)), entry.prop.ModTime, entry.prop.CreateTime, entry.prop.Mode), nil
	}
	if vfs.hasDirLocked(rel) {
		return buildVirtualFileInfo(dirName(rel), true, 0, time.Time{}, time.Time{}, fs.ModeDir|0755), nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (vfs *VirtualFS) ReadDir(name string) ([]fs.DirEntry, error) {
	rel, err := normalizeVirtualPath("readdir", name, true)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	if !vfs.hasDirLocked(rel) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return vfs.readDirLocked(rel), nil
}

func (vfs *VirtualFS) hasDirLocked(rel string) bool {
	if rel == "." {
		return true
	}
	prefix := rel + "/"
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, prefix) {
			return true
		}
	}
	return false
}

func (vfs *VirtualFS) readDirLocked(rel string) []fs.DirEntry {
	type child struct {
		isDir bool
		entry *virtualFileEntry
	}

	children := map[string]child{}
	prefix := ""
	if rel != "." {
		prefix = rel + "/"
	}

	for filePath, file := range vfs.files {
		if prefix != "" && !strings.HasPrefix(filePath, prefix) {
			continue
		}
		rest := strings.TrimPrefix(filePath, prefix)
		if rest == "" {
			continue
		}
		name, after, found := strings.Cut(rest, "/")
		if found {
			children[name] = child{isDir: true}
			_ = after
			continue
		}
		children[name] = child{isDir: false, entry: file}
	}

	names := make([]string, 0, len(children))
	for name := range children {
		names = append(names, name)
	}
	sort.Strings(names)

	ret := make([]fs.DirEntry, 0, len(names))
	for _, name := range names {
		c := children[name]
		if c.isDir {
			ret = append(ret, &virtualDirEntry{info: buildVirtualFileInfo(name, true, 0, defaultTimestamp, time.Time{}, fs.ModeDir|0755)})
			continue
		}
		ret = append(ret, &virtualDirEntry{info: buildVirtualFileInfo(name, false, int64(len(c.entry.data)), c.entry.prop.ModTime, c.entry.prop.CreateTime, c.entry.prop.Mode)})
	}
	return ret
}

func normalizeVirtualPath(op string, name string, allowRoot bool) (string, error) {
	if name == "" {
		name = "."
	}
	cleaned := path.Clean(strings.TrimPrefix(name, "/"))
	if cleaned == "." {
		if allowRoot {
			return ".", nil
		}
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	if !fs.ValidPath(cleaned) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return cleaned, nil
}

func normalizeVirtualProperty(prop VirtualFileProperty) VirtualFileProperty {
	if prop.ModTime.IsZero() {
		prop.ModTime = defaultTimestamp
	}
	if prop.CreateTime.IsZero() {
		prop.CreateTime = prop.ModTime
	}
	if prop.Mode == 0 {
		prop.Mode = 0644
	}
	prop.Mode &^= fs.ModeDir
	return prop
}

func toVirtualBytes(content any) ([]byte, error) {
	switch v := content.(type) {
	case VirtualFileContent:
		return []byte(v), nil
	case []byte:
		ret := make([]byte, len(v))
		copy(ret, v)
		return ret, nil
	case string:
		return []byte(v), nil
	default:
		return nil, &fs.PathError{Op: "create", Path: "", Err: fs.ErrInvalid}
	}
}

func dirName(rel string) string {
	if rel == "." {
		return "."
	}
	return path.Base(rel)
}

type virtualOpenFile struct {
	reader *bytes.Reader
	info   fs.FileInfo
}

func (f *virtualOpenFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *virtualOpenFile) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *virtualOpenFile) Close() error {
	return nil
}

type virtualOpenDir struct {
	info    fs.FileInfo
	entries []fs.DirEntry
	idx     int
}

func (d *virtualOpenDir) Stat() (fs.FileInfo, error) {
	return d.info, nil
}

func (d *virtualOpenDir) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.Name(), Err: fs.ErrInvalid}
}

func (d *virtualOpenDir) Close() error {
	return nil
}

func (d *virtualOpenDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if d.idx >= len(d.entries) && count > 0 {
		return nil, io.EOF
	}
	if count <= 0 {
		ret := make([]fs.DirEntry, len(d.entries)-d.idx)
		copy(ret, d.entries[d.idx:])
		d.idx = len(d.entries)
		return ret, nil
	}
	end := d.idx + count
	if end > len(d.entries) {
		end = len(d.entries)
	}
	ret := make([]fs.DirEntry, end-d.idx)
	copy(ret, d.entries[d.idx:end])
	d.idx = end
	return ret, nil
}

type virtualDirEntry struct {
	info fs.FileInfo
}

func (d *virtualDirEntry) Name() string {
	return d.info.Name()
}

func (d *virtualDirEntry) IsDir() bool {
	return d.info.IsDir()
}

func (d *virtualDirEntry) Type() fs.FileMode {
	return d.info.Mode().Type()
}

func (d *virtualDirEntry) Info() (fs.FileInfo, error) {
	return d.info, nil
}

type virtualFileInfo struct {
	name       string
	size       int64
	mode       fs.FileMode
	modTime    time.Time
	createTime time.Time
	isDir      bool
}

func buildVirtualFileInfo(name string, isDir bool, size int64, modTime, createTime time.Time, mode fs.FileMode) fs.FileInfo {
	if isDir {
		mode = fs.ModeDir | 0755
	}
	return &virtualFileInfo{
		name:       name,
		size:       size,
		mode:       mode,
		modTime:    modTime,
		createTime: createTime,
		isDir:      isDir,
	}
}

func (i *virtualFileInfo) Name() string {
	return i.name
}

func (i *virtualFileInfo) Size() int64 {
	return i.size
}

func (i *virtualFileInfo) Mode() fs.FileMode {
	return i.mode
}

func (i *virtualFileInfo) ModTime() time.Time {
	return i.modTime
}

func (i *virtualFileInfo) IsDir() bool {
	return i.isDir
}

func (i *virtualFileInfo) Sys() interface{} {
	return VirtualFileProperty{
		CreateTime: i.createTime,
		ModTime:    i.modTime,
		Mode:       i.mode,
	}
}
