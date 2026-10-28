package server

import (
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util/glob"
)

// WorkspaceFileSystem provides server file operations over the JSH filesystem.
// Configure mounts before publishing it to concurrent readers.
type WorkspaceFileSystem struct {
	fs      *engine.FS
	mounts  map[string]*WorkspaceBaseDir // Configuration metadata; fs selects mounts.
	ignores map[string]bool
}

var _ tql.FileSystem = (*WorkspaceFileSystem)(nil)

type WorkspaceBaseDir struct {
	abspath    string
	mountPoint string
	readOnly   bool
}

func (bd *WorkspaceBaseDir) ReadOnly() bool {
	return bd.readOnly
}

func normalizeMountPoint(mntPoint string) string {
	return cleanVirtualPath(mntPoint)
}

func NewWorkspaceFileSystem(baseDirs []string) (*WorkspaceFileSystem, error) {
	ret := &WorkspaceFileSystem{
		fs:     engine.NewFS(),
		mounts: map[string]*WorkspaceBaseDir{},
	}
	ret.ignores = map[string]bool{
		".git":          true,
		"machbase_home": true,
		"node_modules":  true,
		".pnp":          true,
		".DS_Store":     true,
		"neow.app":      true, // it can be shown as as a directory on macOS
	}
	for _, dir := range baseDirs {
		mntPoint := ""
		if strings.Contains(dir, "=") {
			parts := strings.SplitN(dir, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid base dir: %s", dir)
			}
			mntPoint = parts[0]
			dir = parts[1]
		} else {
			mntPoint = filepath.Base(dir)
		}
		if err := ret.Mount(mntPoint, dir, false); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (service *WorkspaceFileSystem) ListMounts() []string {
	ret := []string{}
	for _, bd := range service.mounts {
		ret = append(ret, bd.mountPoint)
	}
	slices.Sort(ret)
	return ret
}

func (service *WorkspaceFileSystem) Mount(mntPoint, path string, readOnly bool) error {
	mntPoint = normalizeMountPoint(mntPoint)
	abspath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	workPoint := WorkPath(mntPoint)
	// Keep unavailable sources configured, as before. Runtime mount arguments
	// omit them with a startup warning rather than rejecting the configuration.
	if err := service.fs.MountStrict(workPoint, os.DirFS(abspath)); err != nil {
		return service.workMountError(MountSpec{MountPoint: mntPoint, Dir: abspath}, err)
	}
	service.mounts[workPoint] = &WorkspaceBaseDir{abspath: abspath, mountPoint: mntPoint, readOnly: readOnly}
	return nil
}

func (service *WorkspaceFileSystem) Unmount(mntPoint string) error {
	mntPoint = normalizeMountPoint(mntPoint)
	if mntPoint == "/" {
		return fmt.Errorf("unable to unmount root directory")
	}
	workPoint := WorkPath(mntPoint)
	if _, exists := service.mounts[workPoint]; !exists {
		return nil
	}
	if err := service.fs.Unmount(workPoint); err != nil {
		return err
	}
	delete(service.mounts, workPoint)
	return nil
}

// cleanVirtualPath converts the given path into an absolute, slash separated
// virtual path without "." or ".." elements, so it can not escape the mounts.
func cleanVirtualPath(p string) string {
	return pathpkg.Clean("/" + filepath.ToSlash(p))
}

func (service *WorkspaceFileSystem) protectsMount(path string) bool {
	return service.fs.IsMountPoint(WorkPath(path))
}

func (service *WorkspaceFileSystem) FindBaseDir(path string) *WorkspaceBaseDir {
	mountPoint, _, _, err := service.fs.ResolveHostMount(WorkPath(path))
	if err != nil {
		return nil
	}
	return service.mounts[mountPoint]
}

// returns WorkspaceBaseDir and real path of the given path
func (service *WorkspaceFileSystem) FindRealPath(path string) (*WorkspaceRealPath, error) {
	mountPoint, relPath, absPath, err := service.fs.ResolveHostMount(WorkPath(path))
	if err != nil {
		return nil, err
	}
	bd := service.mounts[mountPoint]
	if bd == nil {
		return nil, os.ErrNotExist
	}
	if relPath == "." {
		relPath = ""
	}
	readOnly := bd.readOnly && relPath == ""

	return &WorkspaceRealPath{
		BaseDir:      bd,
		RelativePath: relPath,
		AbsPath:      absPath,
		ReadOnly:     readOnly,
	}, nil
}

// IsMountPoint reports whether the real path is a mount point directory itself.
func (rp *WorkspaceRealPath) IsMountPoint() bool {
	return rp.RelativePath == ""
}

type WorkspaceRealPath struct {
	BaseDir      *WorkspaceBaseDir
	RelativePath string
	AbsPath      string
	ReadOnly     bool
}

type WorkspaceEntry struct {
	IsDir    bool                 `json:"isDir"`
	Name     string               `json:"name"`
	ReadOnly bool                 `json:"readOnly,omitempty"`
	Content  []byte               `json:"content,omitempty"`  // file content, if the entry is FILE
	Children []*WorkspaceSubEntry `json:"children,omitempty"` // entry of sub files and dirs, if the entry is DIR
	abspath  string               `json:"-"`
	GitUrl   string               `json:"gitUrl,omitempty"`
	GitClone bool                 `json:"gitClone"`
}

type WorkspaceSubEntry struct {
	IsDir              bool   `json:"isDir"`
	Name               string `json:"name"`
	ReadOnly           bool   `json:"readOnly,omitempty"`
	Type               string `json:"type"`
	Size               int64  `json:"size,omitempty"`
	LastModifiedMillis int64  `json:"lastModifiedUnixMillis"`
	GitUrl             string `json:"gitUrl,omitempty"`
	GitClone           bool   `json:"gitClone"`
	Virtual            bool   `json:"virtual"`
}

type WorkspaceSubEntryFilter func(*WorkspaceSubEntry) bool

// returns os.ErrNotExists if not found the path
func (service *WorkspaceFileSystem) Get(path string) (*WorkspaceEntry, error) {
	return service.GetFilter(path, nil)
}

func (service *WorkspaceFileSystem) GetGlob(path string, pattern string) (*WorkspaceEntry, error) {
	path = filepath.Clean(path)
	return service.GetFilter(path, func(ent *WorkspaceSubEntry) bool {
		if ent.IsDir {
			return true
		}
		ok, err := glob.Match(pattern, ent.Name)
		if err != nil {
			return false
		}
		return ok
	})
}

func (service *WorkspaceFileSystem) GetFilter(path string, filter WorkspaceSubEntryFilter) (*WorkspaceEntry, error) {
	return service.getEntry(path, filter)
}

func (service *WorkspaceFileSystem) RealPath(path string) (string, error) {
	workPath := WorkPath(path)
	if _, err := service.fs.Stat(workPath); err != nil {
		return "", err
	}
	virtual, err := service.fs.IsVirtualDirectory(workPath)
	if err != nil {
		return "", err
	}
	if virtual {
		return "", os.ErrNotExist
	}
	return service.fs.ResolveHostPath(workPath)
}

func (service *WorkspaceFileSystem) OpenFile(path string) (io.ReadCloser, error) {
	if service == nil {
		return nil, os.ErrNotExist
	}
	return service.fs.Open(WorkPath(path))
}

func (service *WorkspaceFileSystem) Stat(path string) (os.FileInfo, error) {
	if service == nil {
		return nil, os.ErrNotExist
	}
	return service.fs.Stat(WorkPath(path))
}

func (service *WorkspaceFileSystem) getEntry(path string, filter WorkspaceSubEntryFilter) (*WorkspaceEntry, error) {
	path = cleanVirtualPath(path)
	workPath := WorkPath(path)
	stat, err := service.fs.Stat(workPath)
	if err != nil {
		return nil, err
	}
	isVirtualDir, err := service.fs.IsVirtualDirectory(workPath)
	if err != nil {
		return nil, err
	}
	hostPath := ""
	if !isVirtualDir {
		hostPath, err = service.fs.ResolveHostPath(workPath)
		if err != nil {
			return nil, err
		}
	}
	filename := pathpkg.Base(path)

	if stat.IsDir() {
		ret := &WorkspaceEntry{
			IsDir:    true,
			Name:     filename,
			ReadOnly: isVirtualDir,
			abspath:  hostPath,
		}
		if !isVirtualDir {
			if url, isGitClone := service.isGitClone(hostPath); isGitClone {
				ret.GitClone = true
				ret.GitUrl = url
			}
		}
		entries, err := service.fs.ReadDir(workPath)
		if err != nil {
			return nil, err
		}
		for _, ent := range entries {
			if ent.Name() == "." || ent.Name() == ".." || service.ignores[ent.Name()] {
				continue
			}
			childPath := pathpkg.Join(workPath, ent.Name())
			if service.fs.IsMountPoint(childPath) {
				ret.Children = append(ret.Children, &WorkspaceSubEntry{
					IsDir:    true,
					Name:     ent.Name(),
					ReadOnly: isVirtualDir,
					Type:     "dir",
				})
				continue
			}
			if strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			nfo, err := ent.Info()
			if err != nil {
				return nil, err
			}
			entType := "dir"
			if !nfo.IsDir() {
				entType = filepath.Ext(ent.Name())
			}
			subEnt := &WorkspaceSubEntry{
				IsDir:              nfo.IsDir(),
				Name:               ent.Name(),
				ReadOnly:           false,
				Type:               entType,
				Size:               nfo.Size(),
				LastModifiedMillis: nfo.ModTime().UnixMilli(),
			}
			if filter != nil {
				if !filter(subEnt) {
					continue
				}
			}
			if nfo.IsDir() {
				childHostPath, err := service.fs.ResolveHostPath(childPath)
				if err != nil {
					return nil, err
				}
				if url, isGitClone := service.isGitClone(childHostPath); isGitClone {
					subEnt.GitClone = true
					subEnt.GitUrl = url
				}
			}
			ret.Children = append(ret.Children, subEnt)
		}
		sort.Slice(ret.Children, func(i, j int) bool {
			// sort, directory first
			if ret.Children[i].IsDir && ret.Children[j].IsDir {
				return ret.Children[i].Name < ret.Children[j].Name
			} else if ret.Children[i].IsDir && !ret.Children[j].IsDir {
				return true
			} else if !ret.Children[i].IsDir && ret.Children[j].IsDir {
				return false
			} else {
				return ret.Children[i].Name < ret.Children[j].Name
			}
		})
		return ret, nil
	} else {
		ret := &WorkspaceEntry{
			IsDir:    false,
			Name:     filename,
			ReadOnly: false,
			abspath:  hostPath,
		}
		content, err := service.fs.ReadFile(workPath)
		if err != nil {
			return nil, err
		}
		ret.Content = content
		return ret, nil
	}
}

func (service *WorkspaceFileSystem) MkDir(path string) (*WorkspaceEntry, error) {
	rp, err := service.FindRealPath(path)
	if err != nil {
		return nil, err
	}
	if rp.ReadOnly {
		return nil, fmt.Errorf("unable to create directory, %s is read-only", path)
	}
	if err := service.fs.Mkdir(WorkPath(path)); err != nil {
		return nil, err
	}
	return service.Get(path)
}

func (service *WorkspaceFileSystem) Remove(path string) error {
	rp, err := service.FindRealPath(path)
	if err != nil {
		return err
	}
	if rp.ReadOnly {
		return fmt.Errorf("unable to remove, %s is read-only", path)
	}
	if service.protectsMount(path) {
		return fmt.Errorf("unable to remove, %s is a mount point or contains a mount point", path)
	}
	return service.fs.Remove(WorkPath(path))
}

func (service *WorkspaceFileSystem) RemoveRecursive(path string) error {
	rp, err := service.FindRealPath(path)
	if err != nil {
		return err
	}
	if rp.ReadOnly {
		return fmt.Errorf("unable to remove, %s is read-only", path)
	}
	if service.protectsMount(path) {
		return fmt.Errorf("unable to remove, %s is a mount point or contains a mount point", path)
	}
	return service.fs.RemoveAll(WorkPath(path))
}

// Rename moves path to dest, refusing an existing destination and mount points.
// Paths in different mounts are moved with os.Rename, so it succeeds when both
// host directories are on the same device. Otherwise the original OS error
// (e.g. EXDEV) is returned unwrapped; there is no copy-and-delete fallback.
func (service *WorkspaceFileSystem) Rename(path string, dest string) error {
	rp, err := service.FindRealPath(path)
	if err != nil {
		return err
	}
	if rp.ReadOnly {
		return fmt.Errorf("unable to rename, %s is read-only", path)
	}
	if service.protectsMount(path) {
		return fmt.Errorf("unable to rename, %s is a mount point or contains a mount point", path)
	}
	rp2, err := service.FindRealPath(dest)
	if err != nil {
		return err
	}
	if rp2.ReadOnly {
		return fmt.Errorf("unable to rename, %s is read-only", dest)
	}
	if service.protectsMount(dest) {
		return fmt.Errorf("unable to rename, %s is a mount point or contains a mount point", dest)
	}
	_, err = service.fs.Stat(WorkPath(path))
	if err != nil {
		return fmt.Errorf("unable to access %s, %s", path, err.Error())
	}
	_, err = service.fs.Stat(WorkPath(dest))
	if err == nil {
		return fmt.Errorf("%q already exists", dest)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return service.fs.RenameHost(WorkPath(path), WorkPath(dest))
}

func (service *WorkspaceFileSystem) Set(path string, content []byte) error {
	_, err := service.FindRealPath(path)
	if err != nil {
		return err
	}
	stat, err := service.fs.Stat(WorkPath(path))
	if err == nil && stat.IsDir() {
		return fmt.Errorf("unable to write, %s is directory", path)
	}
	return service.fs.WriteFile(WorkPath(path), content)
}
