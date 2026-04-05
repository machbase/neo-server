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
	dirs  map[string]VirtualFileProperty
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
var _ chmodFS = (*VirtualFS)(nil)
var _ chownFS = (*VirtualFS)(nil)

var defaultTimestamp = time.Unix(1772757478, 0) // 2026-03-06 12:37:58 UTC

func NewVirtualFS() *VirtualFS {
	return &VirtualFS{
		dirs:  make(map[string]VirtualFileProperty),
		files: make(map[string]*virtualFileEntry),
	}
}

func (vfs *VirtualFS) Clone() *VirtualFS {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	clone := NewVirtualFS()
	for dirPath, prop := range vfs.dirs {
		clone.dirs[dirPath] = prop
	}
	for filePath, entry := range vfs.files {
		clone.files[filePath] = &virtualFileEntry{
			data: cloneVirtualBytes(entry.data),
			prop: entry.prop,
		}
	}
	return clone
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

	if _, exists := vfs.dirs[rel]; exists {
		return &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}
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
	for dirPath := range vfs.dirs {
		if dirPath == rel || strings.HasPrefix(dirPath, prefix) {
			return &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
		}
	}
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, prefix) {
			return &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
		}
	}

	prop = normalizeVirtualProperty(prop)
	vfs.ensureParentDirsLocked(rel, prop.ModTime)
	vfs.files[rel] = &virtualFileEntry{data: data, prop: prop}
	return nil
}

func (vfs *VirtualFS) WriteFile(name string, data []byte) error {
	rel, err := normalizeVirtualPath("writefile", name, false)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.dirs[rel]; exists {
		return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrInvalid}
	}
	for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
		if _, exists := vfs.files[parent]; exists {
			return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrInvalid}
		}
	}
	stamp := defaultTimestamp
	mode := fs.FileMode(0)
	if entry, exists := vfs.files[rel]; exists {
		stamp = entry.prop.CreateTime
		mode = entry.prop.Mode
	}
	prop := normalizeVirtualProperty(VirtualFileProperty{CreateTime: stamp, Mode: mode})
	vfs.ensureParentDirsLocked(rel, prop.ModTime)
	vfs.files[rel] = &virtualFileEntry{data: cloneVirtualBytes(data), prop: prop}
	return nil
}

func (vfs *VirtualFS) Chmod(name string, mode uint32) error {
	rel, err := normalizeVirtualPath("chmod", name, true)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if rel == "." {
		return nil
	}
	if entry, exists := vfs.files[rel]; exists {
		entry.prop.Mode = normalizeVirtualProperty(VirtualFileProperty{Mode: fs.FileMode(mode)}).Mode
		entry.prop.ModTime = defaultTimestamp
		return nil
	}
	if prop, exists := vfs.dirs[rel]; exists {
		prop.Mode = fs.FileMode(mode)
		prop.ModTime = defaultTimestamp
		vfs.dirs[rel] = normalizeVirtualProperty(prop)
		return nil
	}
	if vfs.hasDirLocked(rel) {
		vfs.dirs[rel] = normalizeVirtualProperty(VirtualFileProperty{
			CreateTime: defaultTimestamp,
			ModTime:    defaultTimestamp,
			Mode:       fs.FileMode(mode),
		})
		return nil
	}
	return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
}

func (vfs *VirtualFS) Chown(name string, uid, gid int) error {
	rel, err := normalizeVirtualPath("chown", name, true)
	if err != nil {
		return err
	}
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()
	if rel == "." || vfs.hasDirLocked(rel) {
		return nil
	}
	if _, exists := vfs.files[rel]; exists {
		return nil
	}
	return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrNotExist}
}

func (vfs *VirtualFS) AppendFile(name string, data []byte) error {
	rel, err := normalizeVirtualPath("appendfile", name, false)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.dirs[rel]; exists {
		return &fs.PathError{Op: "appendfile", Path: name, Err: fs.ErrInvalid}
	}
	for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
		if _, exists := vfs.files[parent]; exists {
			return &fs.PathError{Op: "appendfile", Path: name, Err: fs.ErrInvalid}
		}
	}
	if entry, exists := vfs.files[rel]; exists {
		entry.data = append(entry.data, data...)
		entry.prop.ModTime = defaultTimestamp
		vfs.touchParentDirsLocked(rel, entry.prop.ModTime)
		return nil
	}
	prop := normalizeVirtualProperty(VirtualFileProperty{})
	vfs.ensureParentDirsLocked(rel, prop.ModTime)
	vfs.files[rel] = &virtualFileEntry{data: cloneVirtualBytes(data), prop: prop}
	return nil
}

func (vfs *VirtualFS) Mkdir(name string) error {
	rel, err := normalizeVirtualPath("mkdir", name, false)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.files[rel]; exists {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}
	for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
		if _, exists := vfs.files[parent]; exists {
			return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrInvalid}
		}
	}
	vfs.ensureDirLocked(rel, defaultTimestamp)
	return nil
}

func (vfs *VirtualFS) Rename(oldName, newName string) error {
	oldRel, err := normalizeVirtualPath("rename", oldName, false)
	if err != nil {
		return err
	}
	newRel, err := normalizeVirtualPath("rename", newName, false)
	if err != nil {
		return err
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if oldRel == newRel {
		if _, ok := vfs.files[oldRel]; ok {
			return nil
		}
		if _, ok := vfs.dirs[oldRel]; ok || vfs.hasDirLocked(oldRel) {
			return nil
		}
		return &fs.PathError{Op: "rename", Path: oldName, Err: fs.ErrNotExist}
	}
	if _, exists := vfs.files[newRel]; exists {
		return &fs.PathError{Op: "rename", Path: newName, Err: fs.ErrExist}
	}
	if _, exists := vfs.dirs[newRel]; exists {
		return &fs.PathError{Op: "rename", Path: newName, Err: fs.ErrExist}
	}
	if vfs.hasDescendantLocked(newRel) {
		return &fs.PathError{Op: "rename", Path: newName, Err: fs.ErrExist}
	}
	for parent := path.Dir(newRel); parent != "."; parent = path.Dir(parent) {
		if _, exists := vfs.files[parent]; exists {
			return &fs.PathError{Op: "rename", Path: newName, Err: fs.ErrInvalid}
		}
	}
	if parent := path.Dir(newRel); parent != "." && !vfs.hasDirLocked(parent) {
		return &fs.PathError{Op: "rename", Path: newName, Err: fs.ErrNotExist}
	}

	if entry, exists := vfs.files[oldRel]; exists {
		delete(vfs.files, oldRel)
		vfs.ensureParentDirsLocked(newRel, entry.prop.ModTime)
		vfs.files[newRel] = entry
		vfs.cleanupParentDirsLocked(path.Dir(oldRel))
		return nil
	}
	if _, exists := vfs.dirs[oldRel]; !exists && !vfs.hasDescendantLocked(oldRel) {
		return &fs.PathError{Op: "rename", Path: oldName, Err: fs.ErrNotExist}
	}

	vfs.ensureParentDirsLocked(newRel, defaultTimestamp)
	oldPrefix := oldRel + "/"
	newPrefix := newRel + "/"

	newDirs := make(map[string]VirtualFileProperty)
	for dirPath, prop := range vfs.dirs {
		switch {
		case dirPath == oldRel:
			newDirs[newRel] = prop
		case strings.HasPrefix(dirPath, oldPrefix):
			newDirs[newPrefix+strings.TrimPrefix(dirPath, oldPrefix)] = prop
		}
	}
	if prop, exists := vfs.dirs[oldRel]; exists {
		newDirs[newRel] = prop
	}
	if len(newDirs) == 0 {
		newDirs[newRel] = normalizeVirtualProperty(VirtualFileProperty{Mode: fs.ModeDir | 0755})
	}

	newFiles := make(map[string]*virtualFileEntry)
	for filePath, entry := range vfs.files {
		if strings.HasPrefix(filePath, oldPrefix) {
			newFiles[newPrefix+strings.TrimPrefix(filePath, oldPrefix)] = entry
		}
	}

	for dirPath := range vfs.dirs {
		if dirPath == oldRel || strings.HasPrefix(dirPath, oldPrefix) {
			delete(vfs.dirs, dirPath)
		}
	}
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, oldPrefix) {
			delete(vfs.files, filePath)
		}
	}
	for dirPath, prop := range newDirs {
		vfs.dirs[dirPath] = prop
	}
	for filePath, entry := range newFiles {
		vfs.files[filePath] = entry
	}
	vfs.cleanupParentDirsLocked(path.Dir(oldRel))
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
		if len(vfs.files) == 0 && len(vfs.dirs) == 0 {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
		}
		vfs.dirs = make(map[string]VirtualFileProperty)
		vfs.files = make(map[string]*virtualFileEntry)
		return nil
	}

	if _, exists := vfs.files[rel]; exists {
		delete(vfs.files, rel)
		vfs.cleanupParentDirsLocked(path.Dir(rel))
		return nil
	}

	prefix := rel + "/"
	removed := 0
	if _, exists := vfs.dirs[rel]; exists {
		delete(vfs.dirs, rel)
		removed++
	}
	for dirPath := range vfs.dirs {
		if strings.HasPrefix(dirPath, prefix) {
			delete(vfs.dirs, dirPath)
			removed++
		}
	}
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, prefix) {
			delete(vfs.files, filePath)
			removed++
		}
	}
	if removed == 0 {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	vfs.cleanupParentDirsLocked(path.Dir(rel))
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
	if prop, ok := vfs.dirPropLocked(rel); ok {
		return buildVirtualFileInfo(dirName(rel), true, 0, prop.ModTime, prop.CreateTime, prop.Mode), nil
	}
	if vfs.hasDirLocked(rel) {
		return buildVirtualFileInfo(dirName(rel), true, 0, defaultTimestamp, defaultTimestamp, fs.ModeDir|0755), nil
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
	if _, exists := vfs.dirs[rel]; exists {
		return true
	}
	prefix := rel + "/"
	for dirPath := range vfs.dirs {
		if strings.HasPrefix(dirPath, prefix) {
			return true
		}
	}
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
	for dirPath, prop := range vfs.dirs {
		if prefix != "" && !strings.HasPrefix(dirPath, prefix) {
			continue
		}
		rest := strings.TrimPrefix(dirPath, prefix)
		if rest == "" {
			continue
		}
		name, _, found := strings.Cut(rest, "/")
		if found {
			children[name] = child{isDir: true}
			continue
		}
		children[name] = child{isDir: true, entry: &virtualFileEntry{prop: normalizeVirtualProperty(prop)}}
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
			prop := normalizeVirtualProperty(VirtualFileProperty{Mode: fs.ModeDir | 0755})
			if c.entry != nil {
				prop = normalizeVirtualProperty(c.entry.prop)
			}
			ret = append(ret, &virtualDirEntry{info: buildVirtualFileInfo(name, true, 0, prop.ModTime, prop.CreateTime, prop.Mode)})
			continue
		}
		ret = append(ret, &virtualDirEntry{info: buildVirtualFileInfo(name, false, int64(len(c.entry.data)), c.entry.prop.ModTime, c.entry.prop.CreateTime, c.entry.prop.Mode)})
	}
	return ret
}

func (vfs *VirtualFS) dirPropLocked(rel string) (VirtualFileProperty, bool) {
	if rel == "." {
		return normalizeVirtualProperty(VirtualFileProperty{Mode: fs.ModeDir | 0755}), true
	}
	prop, ok := vfs.dirs[rel]
	if !ok {
		return VirtualFileProperty{}, false
	}
	return normalizeVirtualProperty(prop), true
}

func (vfs *VirtualFS) hasDescendantLocked(rel string) bool {
	prefix := rel + "/"
	for dirPath := range vfs.dirs {
		if strings.HasPrefix(dirPath, prefix) {
			return true
		}
	}
	for filePath := range vfs.files {
		if strings.HasPrefix(filePath, prefix) {
			return true
		}
	}
	return false
}

func (vfs *VirtualFS) ensureParentDirsLocked(rel string, stamp time.Time) {
	parent := path.Dir(rel)
	if parent == "." {
		return
	}
	vfs.ensureDirLocked(parent, stamp)
}

func (vfs *VirtualFS) ensureDirLocked(rel string, stamp time.Time) {
	if rel == "." {
		return
	}
	if _, exists := vfs.files[rel]; exists {
		return
	}
	vfs.ensureDirLocked(path.Dir(rel), stamp)
	if _, exists := vfs.dirs[rel]; !exists {
		vfs.dirs[rel] = normalizeVirtualProperty(VirtualFileProperty{
			CreateTime: stamp,
			ModTime:    stamp,
			Mode:       fs.ModeDir | 0755,
		})
	}
}

func (vfs *VirtualFS) touchParentDirsLocked(rel string, stamp time.Time) {
	for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
		prop, exists := vfs.dirs[parent]
		if !exists {
			continue
		}
		prop.ModTime = stamp
		vfs.dirs[parent] = normalizeVirtualProperty(prop)
	}
}

func (vfs *VirtualFS) cleanupParentDirsLocked(rel string) {
	for rel != "." {
		if _, exists := vfs.dirs[rel]; !exists {
			rel = path.Dir(rel)
			continue
		}
		if vfs.hasDescendantLocked(rel) {
			break
		}
		delete(vfs.dirs, rel)
		rel = path.Dir(rel)
	}
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
		return cloneVirtualBytes(v), nil
	case string:
		return []byte(v), nil
	default:
		return nil, &fs.PathError{Op: "create", Path: "", Err: fs.ErrInvalid}
	}
}

func cloneVirtualBytes(src []byte) []byte {
	ret := make([]byte, len(src))
	copy(ret, src)
	return ret
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
		perm := mode.Perm()
		if perm == 0 {
			perm = 0755
		}
		mode = fs.ModeDir | perm
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
