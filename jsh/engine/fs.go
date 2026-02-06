package engine

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// FS allows mounting multiple fs.FS at different paths
type FS struct {
	mounts map[string]fs.FS
	fds    map[int]*os.File
	nextFD int
	fdMu   sync.Mutex
}

var _ fs.FS = (*FS)(nil)
var _ fs.ReadDirFS = (*FS)(nil)
var _ fs.ReadFileFS = (*FS)(nil)

// NewFS creates a new MountFS
func NewFS() *FS {
	return &FS{
		mounts: make(map[string]fs.FS),
		fds:    make(map[int]*os.File),
		nextFD: 3, // Start from 3 (0, 1, 2 are stdin, stdout, stderr)
	}
}

// Mount mounts an fs.FS at a given virtual path
// Returns error if mountPoint is invalid or already exists
func (m *FS) Mount(mountPoint string, filesystem fs.FS) error {
	if filesystem == nil {
		return fs.ErrInvalid
	}

	mountPoint = CleanPath(mountPoint)

	// Check for conflicting mounts
	for existing := range m.mounts {
		if mountPoint == existing {
			return fs.ErrExist
		}
		// Check if new mount would shadow existing mount
		if mountPoint != "/" && strings.HasPrefix(existing, mountPoint+"/") {
			return fs.ErrExist
		}
		// Check if existing mount would shadow new mount
		if existing != "/" && strings.HasPrefix(mountPoint, existing+"/") {
			return fs.ErrExist
		}
	}

	m.mounts[mountPoint] = filesystem
	return nil
}

// Unmount removes a mounted filesystem at the given path
func (m *FS) Unmount(mountPoint string) error {
	mountPoint = CleanPath(mountPoint)

	if _, ok := m.mounts[mountPoint]; !ok {
		return fs.ErrNotExist
	}

	delete(m.mounts, mountPoint)
	return nil
}

// Mounts returns a list of all mount points
func (m *FS) Mounts() []string {
	mounts := make([]string, 0, len(m.mounts))
	for mountPoint := range m.mounts {
		mounts = append(mounts, mountPoint)
	}
	sort.Strings(mounts)
	return mounts
}

// bestMatch finds the best matching mounted fs.FS for the given path
func (m *FS) bestMatch(name string) (fs.FS, string) {
	name = CleanPath(name)
	// Find the longest matching mount point
	var bestMatch string
	var bestFS fs.FS

	for mountPoint, filesystem := range m.mounts {
		if mountPoint == "/" {
			// Root mount matches everything
			if bestMatch == "" {
				bestMatch = "/"
				bestFS = filesystem
			}
			continue
		}
		if name == mountPoint || strings.HasPrefix(name, mountPoint+"/") {
			if len(mountPoint) > len(bestMatch) {
				bestMatch = mountPoint
				bestFS = filesystem
			}
		}
	}

	return bestFS, bestMatch
}

// getRelativePath converts an absolute path to a relative path within a mounted filesystem
func getRelativePath(name, bestMatch string) string {
	relPath := strings.TrimPrefix(name, bestMatch)
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "" {
		return "."
	}
	return relPath
}

// getOSPath attempts to get the OS path from a filesystem (if it's os.DirFS)
func getOSPath(filesystem fs.FS, relPath string) (string, error) {
	// os.DirFS is a string type, so we can use reflection to check
	if reflect.TypeOf(filesystem).Kind() == reflect.String {
		return filepath.Join(fmt.Sprintf("%v", filesystem), relPath), nil
	}
	return "", fs.ErrPermission
}

// performOSOperation is a helper for operations that require OS filesystem access
func (m *FS) performOSOperation(name string, operation func(string) error) error {
	name = CleanPath(name)
	bestFS, bestMatch := m.bestMatch(name)
	if bestFS == nil {
		return fs.ErrNotExist
	}

	relPath := getRelativePath(name, bestMatch)
	target, err := getOSPath(bestFS, relPath)
	if err != nil {
		return err
	}

	if err := operation(target); err != nil {
		return fs.ErrInvalid
	}
	return nil
}

// Open implements fs.FS
func (m *FS) Open(name string) (fs.File, error) {
	name = CleanPath(name)
	// Validate path: skip leading / for fs.ValidPath check
	validPath := strings.TrimPrefix(name, "/")
	if validPath == "" {
		validPath = "."
	}
	if !fs.ValidPath(validPath) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Find the longest matching mount point
	bestFS, bestMatch := m.bestMatch(name)

	if bestFS == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	return bestFS.Open(getRelativePath(name, bestMatch))
}

func (m *FS) CleanPath(name string) string {
	return CleanPath(name)
}

func (m *FS) Stat(name string) (fs.FileInfo, error) {
	name = CleanPath(name)
	f, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func (m *FS) CountLines(name string) (int, error) {
	name = CleanPath(name)
	f, err := m.Open(name)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Use large buffer for optimal performance
	const bufferSize = 64 * 1024 // 64KB buffer
	buffer := make([]byte, bufferSize)
	count := 0

	for {
		n, err := f.Read(buffer)
		if n > 0 {
			// Count newline characters in the buffer
			for i := 0; i < n; i++ {
				if buffer[i] == '\n' {
					count++
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return count, err
		}
	}
	return count, nil
}

func (m *FS) ReadFile(name string) ([]byte, error) {
	name = CleanPath(name)
	f, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// Mkdir creates a directory at the specified path
func (m *FS) Mkdir(name string) error {
	return m.performOSOperation(name, func(target string) error {
		return os.MkdirAll(target, 0755)
	})
}

// Rmdir removes a directory at the specified path
func (m *FS) Rmdir(name string) error {
	return m.performOSOperation(name, os.Remove)
}

// Remove removes a file at the specified path
func (m *FS) Remove(name string) error {
	return m.performOSOperation(name, os.Remove)
}

// Rename renames a file or directory from oldName to newName
func (m *FS) Rename(oldName, newName string) error {
	oldName = CleanPath(oldName)
	newName = CleanPath(newName)

	// Find the longest matching mount point for oldName
	oldFS, oldMatch := m.bestMatch(oldName)
	if oldFS == nil {
		return fs.ErrNotExist
	}

	// Find the longest matching mount point for newName
	newFS, newMatch := m.bestMatch(newName)
	if newFS == nil {
		return fs.ErrNotExist
	}

	// Ensure both paths are on the same filesystem
	if oldFS != newFS {
		return fs.ErrInvalid
	}

	oldRelPath := getRelativePath(oldName, oldMatch)
	newRelPath := getRelativePath(newName, newMatch)

	oldTarget, err := getOSPath(oldFS, oldRelPath)
	if err != nil {
		return err
	}

	newTarget, err := getOSPath(newFS, newRelPath)
	if err != nil {
		return err
	}

	if err := os.Rename(oldTarget, newTarget); err != nil {
		return fs.ErrInvalid
	}
	return nil
}

// WriteFile writes data to a file at the specified path
func (m *FS) WriteFile(name string, data []byte) error {
	return m.performOSOperation(name, func(target string) error {
		return os.WriteFile(target, data, 0644)
	})
}

// AppendFile appends data to a file at the specified path
// Creates the file if it does not exist
func (m *FS) AppendFile(name string, data []byte) error {
	return m.performOSOperation(name, func(target string) error {
		f, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(data)
		return err
	})
}

// Chmod changes the permission mode of a file or directory at the specified path
func (m *FS) Chmod(name string, mode uint32) error {
	return m.performOSOperation(name, func(target string) error {
		return os.Chmod(target, fs.FileMode(mode))
	})
}

// Chown changes the uid and gid of a file or directory at the specified path
func (m *FS) Chown(name string, uid, gid int) error {
	return m.performOSOperation(name, func(target string) error {
		return os.Chown(target, uid, gid)
	})
}

// Symlink creates a symbolic link from newName to oldName
func (m *FS) Symlink(oldName, newName string) error {
	newName = CleanPath(newName)

	newFS, newMatch := m.bestMatch(newName)
	if newFS == nil {
		return fs.ErrNotExist
	}

	relPath := getRelativePath(newName, newMatch)
	target, err := getOSPath(newFS, relPath)
	if err != nil {
		return err
	}

	return os.Symlink(oldName, target)
}

// Readlink reads the target of a symbolic link
func (m *FS) Readlink(name string) (string, error) {
	name = CleanPath(name)
	bestFS, bestMatch := m.bestMatch(name)
	if bestFS == nil {
		return "", fs.ErrNotExist
	}

	relPath := getRelativePath(name, bestMatch)
	target, err := getOSPath(bestFS, relPath)
	if err != nil {
		return "", err
	}

	return os.Readlink(target)
}

// OpenFD opens a file and returns a file descriptor
func (m *FS) OpenFD(name string, flags int, mode uint32) (int, error) {
	name = CleanPath(name)
	bestFS, bestMatch := m.bestMatch(name)
	if bestFS == nil {
		return -1, fs.ErrNotExist
	}

	relPath := getRelativePath(name, bestMatch)
	target, err := getOSPath(bestFS, relPath)
	if err != nil {
		return -1, err
	}

	file, err := os.OpenFile(target, flags, fs.FileMode(mode))
	if err != nil {
		return -1, err
	}

	m.fdMu.Lock()
	defer m.fdMu.Unlock()

	fd := m.nextFD
	m.nextFD++
	m.fds[fd] = file

	return fd, nil
}

// CloseFD closes a file descriptor
func (m *FS) CloseFD(fd int) error {
	m.fdMu.Lock()
	defer m.fdMu.Unlock()

	file, ok := m.fds[fd]
	if !ok {
		return fs.ErrInvalid
	}

	delete(m.fds, fd)
	return file.Close()
}

func (m *FS) HostWriterFD(fd int) (io.Writer, error) {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()
	if !ok {
		return nil, fs.ErrInvalid
	}
	return file, nil
}

func (m *FS) HostReaderFD(fd int) (io.Reader, error) {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()
	if !ok {
		return nil, fs.ErrInvalid
	}
	return file, nil
}

// ReadFD reads from a file descriptor into the provided buffer at the specified offset
// The buffer must be pre-allocated by the caller
// Reads up to length bytes into buffer[offset:offset+length]
// Returns the number of bytes read
func (m *FS) ReadFD(fd int, buffer []byte, offset int, length int) (int, error) {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()

	if !ok {
		return 0, fs.ErrInvalid
	}

	if offset < 0 || length < 0 || offset+length > len(buffer) {
		return 0, fs.ErrInvalid
	}

	n, err := file.Read(buffer[offset : offset+length])
	if err != nil && err != io.EOF {
		return 0, err
	}

	return n, nil
}

// WriteFD writes data to a file descriptor
func (m *FS) WriteFD(fd int, data []byte) (int, error) {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()

	if !ok {
		return 0, fs.ErrInvalid
	}

	return file.Write(data)
}

// FstatFD gets file info for a file descriptor
func (m *FS) FstatFD(fd int) (fs.FileInfo, error) {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()

	if !ok {
		return nil, fs.ErrInvalid
	}

	return file.Stat()
}

// FchmodFD changes the mode of a file descriptor
func (m *FS) FchmodFD(fd int, mode uint32) error {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()

	if !ok {
		return fs.ErrInvalid
	}

	return file.Chmod(fs.FileMode(mode))
}

// FchownFD changes the owner of a file descriptor
func (m *FS) FchownFD(fd int, uid, gid int) error {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()

	if !ok {
		return fs.ErrInvalid
	}

	return file.Chown(uid, gid)
}

// FsyncFD synchronizes a file descriptor
func (m *FS) FsyncFD(fd int) error {
	m.fdMu.Lock()
	file, ok := m.fds[fd]
	m.fdMu.Unlock()

	if !ok {
		return fs.ErrInvalid
	}

	return file.Sync()
}

// ReadDir implements fs.ReadDirFS
func (m *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = CleanPath(name)
	// Validate path: skip leading / for fs.ValidPath check
	validPath := strings.TrimPrefix(name, "/")
	if validPath == "" {
		validPath = "."
	}
	if !fs.ValidPath(validPath) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	// Find the longest matching mount point
	bestFS, bestMatch := m.bestMatch(name)

	if bestFS == nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	relPath := getRelativePath(name, bestMatch)

	// Read base directory entries
	var entries []fs.DirEntry
	if readDirFS, ok := bestFS.(fs.ReadDirFS); ok {
		var err error
		entries, err = readDirFS.ReadDir(relPath)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		entries, err = fs.ReadDir(bestFS, relPath)
		if err != nil {
			return nil, err
		}
	}

	// Get current directory info for "." entry
	var currentInfo fs.FileInfo
	if f, err := bestFS.Open(relPath); err == nil {
		currentInfo, _ = f.Stat()
		f.Close()
	}

	// Get parent directory info for ".." entry
	var parentInfo fs.FileInfo
	parentRelPath := relPath
	if relPath == "." {
		// Already at the root of this mount, use root info for parent
		if f, err := bestFS.Open("."); err == nil {
			parentInfo, _ = f.Stat()
			f.Close()
		}
	} else {
		// Get parent directory
		lastSlash := strings.LastIndex(relPath, "/")
		if lastSlash > 0 {
			parentRelPath = relPath[:lastSlash]
		} else {
			parentRelPath = "."
		}
		if f, err := bestFS.Open(parentRelPath); err == nil {
			parentInfo, _ = f.Stat()
			f.Close()
		}
	}

	// Add . and .. entries with real directory info
	dotEntries := []fs.DirEntry{
		&dotDirEntry{name: ".", isDir: true, info: currentInfo},
		&dotDirEntry{name: "..", isDir: true, info: parentInfo},
	}
	entries = append(dotEntries, entries...)

	// Add mounted directories as entries
	for mountPoint := range m.mounts {
		// Skip the root mount
		if mountPoint == "/" {
			continue
		}

		// Check if this mount point is a direct child of the current directory
		// For example, if name is "/" and mountPoint is "/bin", add "bin"
		// If name is "/usr" and mountPoint is "/usr/local", add "local"
		if strings.HasPrefix(mountPoint, name+"/") || (name == "/" && mountPoint != "/") {
			relativePath := strings.TrimPrefix(mountPoint, name)
			relativePath = strings.TrimPrefix(relativePath, "/")

			// Only include direct children (not nested)
			if !strings.Contains(relativePath, "/") && relativePath != "" {
				// Check if this entry already exists in the base entries
				exists := false
				for _, entry := range entries {
					if entry.Name() == relativePath {
						exists = true
						break
					}
				}
				if !exists {
					entries = append(entries, &dotDirEntry{name: relativePath, isDir: true})
				}
			}
		}
	}

	return entries, nil
}

// dotDirEntry implements fs.DirEntry for . and .. and mount points
type dotDirEntry struct {
	name  string
	isDir bool
	info  fs.FileInfo // underlying file info, may be nil
}

func (d *dotDirEntry) Name() string {
	return d.name
}

func (d *dotDirEntry) IsDir() bool {
	return d.isDir
}

func (d *dotDirEntry) Type() fs.FileMode {
	if d.isDir {
		return fs.ModeDir
	}
	return 0
}

func (d *dotDirEntry) Info() (fs.FileInfo, error) {
	if d.info != nil {
		return &dotFileInfo{
			name:    d.name,
			isDir:   d.isDir,
			size:    d.info.Size(),
			modTime: d.info.ModTime(),
		}, nil
	}
	return &dotFileInfo{name: d.name, isDir: d.isDir}, nil
}

// dotFileInfo implements fs.FileInfo for . and .. and mount points
type dotFileInfo struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}

func (d *dotFileInfo) Name() string {
	return d.name
}

func (d *dotFileInfo) Size() int64 {
	return d.size
}

func (d *dotFileInfo) Mode() fs.FileMode {
	if d.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}

func (d *dotFileInfo) ModTime() time.Time {
	return d.modTime
}

func (d *dotFileInfo) IsDir() bool {
	return d.isDir
}

func (d *dotFileInfo) Sys() interface{} {
	return nil
}

func (jr *JSRuntime) Filesystem(vm *goja.Runtime, module *goja.Object) {
	exports := module.Get("exports").(*goja.Object)

	exports.Set("resolvePath", func(path string) string { return jr.Env.ResolvePath(path) })
	exports.Set("resolveAbsPath", func(path string) string { return jr.Env.ResolveAbsPath(path) })
	exports.Set("readFile", func(path string) ([]byte, error) { return jr.filesystem.ReadFile(path) })
	exports.Set("writeFile", func(path string, data []byte) error { return jr.filesystem.WriteFile(path, data) })
	exports.Set("appendFile", func(path string, data []byte) error { return jr.filesystem.AppendFile(path, data) })
	exports.Set("stat", func(path string) (fs.FileInfo, error) { return jr.filesystem.Stat(path) })
	exports.Set("mkdir", func(path string) error { return jr.filesystem.Mkdir(path) })
	exports.Set("rmdir", func(path string) error { return jr.filesystem.Rmdir(path) })
	exports.Set("remove", func(path string) error { return jr.filesystem.Remove(path) })
	exports.Set("rename", func(oldPath, newPath string) error { return jr.filesystem.Rename(oldPath, newPath) })
	exports.Set("readDir", func(path string) ([]fs.DirEntry, error) { return jr.filesystem.ReadDir(path) })
	exports.Set("chmod", func(path string, mode uint32) error { return jr.filesystem.Chmod(path, mode) })
	exports.Set("chown", func(path string, uid, gid int) error { return jr.filesystem.Chown(path, uid, gid) })
	exports.Set("symlink", func(oldName, newName string) error { return jr.filesystem.Symlink(oldName, newName) })
	exports.Set("readlink", func(path string) (string, error) { return jr.filesystem.Readlink(path) })

	// File descriptor operations
	exports.Set("open", func(path string, flags int, mode uint32) (int, error) {
		return jr.filesystem.OpenFD(path, flags, mode)
	})
	exports.Set("close", func(fd int) error { return jr.filesystem.CloseFD(fd) })
	exports.Set("hostWriter", func(fd int) (io.Writer, error) { return jr.filesystem.HostWriterFD(fd) })
	exports.Set("hostReader", func(fd int) (io.Reader, error) { return jr.filesystem.HostReaderFD(fd) })
	exports.Set("read", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 4 {
			panic(vm.NewTypeError("read requires 4 arguments: fd, buffer, offset, length"))
		}
		fd := int(call.Arguments[0].ToInteger())
		bufferObj := call.Arguments[1].Export()
		offset := int(call.Arguments[2].ToInteger())
		length := int(call.Arguments[3].ToInteger())

		buffer, ok := bufferObj.([]byte)
		if !ok {
			panic(vm.NewTypeError("buffer argument must be a Uint8Array"))
		}
		// Read data from file
		bytesRead, err := jr.filesystem.ReadFD(fd, buffer, offset, length)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		return vm.ToValue(bytesRead)
	})
	exports.Set("write", func(fd int, data []byte) (int, error) {
		return jr.filesystem.WriteFD(fd, data)
	})
	exports.Set("fstat", func(fd int) (fs.FileInfo, error) { return jr.filesystem.FstatFD(fd) })
	exports.Set("fchmod", func(fd int, mode uint32) error { return jr.filesystem.FchmodFD(fd, mode) })
	exports.Set("fchown", func(fd int, uid, gid int) error { return jr.filesystem.FchownFD(fd, uid, gid) })
	exports.Set("fsync", func(fd int) error { return jr.filesystem.FsyncFD(fd) })
	exports.Set("countLines", func(path string) (int, error) { return jr.filesystem.CountLines(path) })

	// Export OS-specific file constants from Go
	// These values are platform-specific and must come from the os package
	exports.Set("O_RDONLY", os.O_RDONLY)
	exports.Set("O_WRONLY", os.O_WRONLY)
	exports.Set("O_RDWR", os.O_RDWR)
	exports.Set("O_CREAT", os.O_CREATE)
	exports.Set("O_EXCL", os.O_EXCL)
	exports.Set("O_TRUNC", os.O_TRUNC)
	exports.Set("O_APPEND", os.O_APPEND)
}
