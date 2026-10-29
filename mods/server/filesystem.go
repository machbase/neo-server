package server

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
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
	mounts  map[string]MountSpec // Configuration metadata keyed by JSH mount point; fs selects mounts.
	ignores map[string]bool
}

var _ tql.FileSystem = (*WorkspaceFileSystem)(nil)

// NewWorkspaceFileSystem mounts the canonical "mountPoint=dir" entries
// (see MountSpec.String and NormalizeFileDirs) under WorkMountPoint.
// A bare "dir" entry is mounted at its base name.
func NewWorkspaceFileSystem(fileDirs []string) (*WorkspaceFileSystem, error) {
	ret := &WorkspaceFileSystem{
		fs:     engine.NewFS(),
		mounts: map[string]MountSpec{},
	}
	ret.ignores = map[string]bool{
		".git":          true,
		"machbase_home": true,
		"node_modules":  true,
		".pnp":          true,
		".DS_Store":     true,
		"neow.app":      true, // it can be shown as as a directory on macOS
	}
	for _, entry := range fileDirs {
		spec, err := parseCanonicalMountSpec(entry)
		if err != nil {
			return nil, err
		}
		if err := ret.mount(spec); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (service *WorkspaceFileSystem) mount(spec MountSpec) error {
	workPoint := spec.WorkMountPoint()
	// Keep unavailable sources configured, as before. Runtime mount arguments
	// omit them with a startup warning rather than rejecting the configuration.
	if err := service.fs.MountStrict(workPoint, os.DirFS(spec.Dir)); err != nil {
		return service.workMountError(spec, err)
	}
	service.mounts[workPoint] = spec
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

// FindRealPath returns the mount and the host path of the given path.
// The path does not need to exist, but it must belong to a host mount.
func (service *WorkspaceFileSystem) FindRealPath(path string) (*WorkspaceRealPath, error) {
	mountPoint, relPath, absPath, err := service.fs.ResolveHostMount(WorkPath(path))
	if err != nil {
		return nil, err
	}
	spec, ok := service.mounts[mountPoint]
	if !ok {
		return nil, os.ErrNotExist
	}
	if relPath == "." {
		relPath = ""
	}
	return &WorkspaceRealPath{
		Mount:        spec,
		RelativePath: relPath,
		AbsPath:      absPath,
	}, nil
}

type WorkspaceRealPath struct {
	Mount        MountSpec
	RelativePath string
	AbsPath      string
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

// GlobFilter returns a listing filter that keeps directories and the files
// whose name matches the glob pattern.
func GlobFilter(pattern string) WorkspaceSubEntryFilter {
	return func(ent *WorkspaceSubEntry) bool {
		if ent.IsDir {
			return true
		}
		ok, err := glob.Match(pattern, ent.Name)
		return err == nil && ok
	}
}

// Get returns the file content or the directory listing of the path.
// It returns os.ErrNotExist if the path is not found.
func (service *WorkspaceFileSystem) Get(path string) (*WorkspaceEntry, error) {
	return service.GetFilter(path, nil)
}

// Open opens the file of the path for streaming.
func (service *WorkspaceFileSystem) Open(path string) (fs.File, error) {
	if service == nil {
		return nil, os.ErrNotExist
	}
	return service.fs.Open(WorkPath(path))
}

func (service *WorkspaceFileSystem) OpenFile(path string) (io.ReadCloser, error) {
	return service.Open(path)
}

func (service *WorkspaceFileSystem) Stat(path string) (os.FileInfo, error) {
	if service == nil {
		return nil, os.ErrNotExist
	}
	return service.fs.Stat(WorkPath(path))
}

// GetFilter is Get with a filter applied to the directory listing.
func (service *WorkspaceFileSystem) GetFilter(path string, filter WorkspaceSubEntryFilter) (*WorkspaceEntry, error) {
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
	if _, err := service.FindRealPath(path); err != nil {
		return nil, err
	}
	if err := service.fs.Mkdir(WorkPath(path)); err != nil {
		return nil, err
	}
	return service.Get(path)
}

func (service *WorkspaceFileSystem) Remove(path string) error {
	if _, err := service.FindRealPath(path); err != nil {
		return err
	}
	if service.protectsMount(path) {
		return fmt.Errorf("unable to remove, %s is a mount point or contains a mount point", path)
	}
	return service.fs.Remove(WorkPath(path))
}

func (service *WorkspaceFileSystem) RemoveRecursive(path string) error {
	if _, err := service.FindRealPath(path); err != nil {
		return err
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
	if _, err := service.FindRealPath(path); err != nil {
		return err
	}
	if service.protectsMount(path) {
		return fmt.Errorf("unable to rename, %s is a mount point or contains a mount point", path)
	}
	if _, err := service.FindRealPath(dest); err != nil {
		return err
	}
	if service.protectsMount(dest) {
		return fmt.Errorf("unable to rename, %s is a mount point or contains a mount point", dest)
	}
	if _, err := service.fs.Stat(WorkPath(path)); err != nil {
		return fmt.Errorf("unable to access %s, %s", path, err.Error())
	}
	_, err := service.fs.Stat(WorkPath(dest))
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
