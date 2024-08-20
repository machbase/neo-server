package ssfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/machbase/neo-server/mods/util/glob"
)

// Server Side File System
type SSFS struct {
	bases       []*BaseDir
	mountLock   sync.RWMutex
	ignores     map[string]bool
	virtualDirs map[string][]string
}

var defaultFs *SSFS

func SetDefault(fs *SSFS) {
	defaultFs = fs
}

func Default() *SSFS {
	return defaultFs
}

type BaseDir struct {
	//	name       string
	abspath    string
	mountPoint string
	readOnly   bool
}

func (bd *BaseDir) ReadOnly() bool {
	return bd.readOnly
}

func (bd *BaseDir) RealPath(path string) string {
	path = filepath.Join(bd.abspath, strings.TrimPrefix(path, bd.mountPoint))
	return filepath.FromSlash(path)
}

func (bd *BaseDir) NormalizePath(path string) string {
	return filepath.Join(bd.mountPoint, strings.TrimPrefix(path, bd.mountPoint))
}

func normalizeMountPoint(mntPoint string) string {
	return "/" + strings.TrimPrefix(strings.TrimSuffix(mntPoint, "/"), "/")
}

func NewServerSideFileSystem(baseDirs []string) (*SSFS, error) {
	ret := &SSFS{
		virtualDirs: map[string][]string{},
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
		mntPoint = normalizeMountPoint(mntPoint)

		abspath, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		bd := &BaseDir{
			abspath:    abspath,
			mountPoint: mntPoint,
			readOnly:   false,
		}
		if err := ret.mountBaseDir(bd); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (ssfs *SSFS) ListMounts() []string {
	ssfs.mountLock.RLock()
	defer ssfs.mountLock.RUnlock()

	ret := []string{}
	for _, bd := range ssfs.bases {
		ret = append(ret, bd.mountPoint)
	}
	slices.Sort(ret)
	return ret
}

func (ssfs *SSFS) Mount(mntPoint, path string, readOnly bool) error {
	ssfs.mountLock.Lock()
	defer ssfs.mountLock.Unlock()

	mntPoint = normalizeMountPoint(mntPoint)
	for _, bd := range ssfs.bases {
		if bd.mountPoint == path {
			return fmt.Errorf("moount point %q is already mounted", mntPoint)
		}
	}
	abspath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	bd := &BaseDir{abspath: abspath, mountPoint: mntPoint, readOnly: readOnly}
	return ssfs.mountBaseDir(bd)
}

func (ssfs *SSFS) mountBaseDir(bd *BaseDir) error {
	if bd.mountPoint == "/" {
		ssfs.bases = append(ssfs.bases, bd)
		return nil
	}
	ssfs.bases = append(ssfs.bases, bd)
	ssfs.restrutVirtualDirs()
	return nil
}

func (ssfs *SSFS) restrutVirtualDirs() {
	ssfs.virtualDirs = map[string][]string{}
	for _, bd := range ssfs.bases {
		path := bd.mountPoint
		if path == "/" {
			continue
		}
		components := strings.Split(path, "/")
		vpath := "/"
		for i := 1; i < len(components); i++ {
			if children, ok := ssfs.virtualDirs[vpath]; ok {
				if !slices.Contains(children, components[i]) {
					ssfs.virtualDirs[vpath] = append(children, components[i])
					slices.Sort(ssfs.virtualDirs[vpath])
				}
			} else {
				ssfs.virtualDirs[vpath] = []string{components[i]}
			}
			if vpath == "/" {
				vpath = ""
			}
			vpath = strings.Join([]string{vpath, components[i]}, "/")
		}
	}
}

func (ssfs *SSFS) Unmount(mntPoint string) error {
	ssfs.mountLock.Lock()
	defer ssfs.mountLock.Unlock()

	mntPoint = normalizeMountPoint(mntPoint)
	if mntPoint == "/" {
		return fmt.Errorf("unable to unmount root directory")
	}
	for i, bd := range ssfs.bases {
		if bd.mountPoint == mntPoint {
			ssfs.bases = slices.Delete(ssfs.bases, i, i+1)
		}
	}
	ssfs.restrutVirtualDirs()
	return nil
}

func (ssfs *SSFS) FindBaseDir(path string) *BaseDir {
	ssfs.mountLock.RLock()
	defer ssfs.mountLock.RUnlock()

	mntList := []string{}
	for _, bd := range ssfs.bases {
		mntList = append(mntList, bd.mountPoint)
	}
	slices.SortFunc(mntList, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(b, a)
	})
	for _, mnt := range mntList {
		if strings.HasPrefix(path, mnt) {
			for _, bd := range ssfs.bases {
				if bd.mountPoint == mnt {
					return bd
				}
			}
		}
	}
	return nil
}

// returns BaseDir and real path of the given path
func (ssfs *SSFS) FindRealPath(path string) (*RealPath, error) {
	bd := ssfs.FindBaseDir(path)
	if bd == nil {
		return nil, os.ErrNotExist
	}
	relPath := strings.TrimSuffix(strings.TrimPrefix(path, bd.mountPoint), "/")
	path = filepath.FromSlash(filepath.Join(bd.abspath, relPath))
	readOnly := bd.readOnly && relPath == ""

	return &RealPath{
		BaseDir:      bd,
		RelativePath: relPath,
		AbsPath:      path,
		ReadOnly:     readOnly,
	}, nil
}

type RealPath struct {
	BaseDir      *BaseDir
	RelativePath string
	AbsPath      string
	ReadOnly     bool
}

type Entry struct {
	IsDir    bool        `json:"isDir"`
	Name     string      `json:"name"`
	ReadOnly bool        `json:"readOnly,omitempty"`
	Content  []byte      `json:"content,omitempty"`  // file content, if the entry is FILE
	Children []*SubEntry `json:"children,omitempty"` // entry of sub files and dirs, if the entry is DIR
	abspath  string      `json:"-"`
	GitUrl   string      `json:"gitUrl,omitempty"`
	GitClone bool        `json:"gitClone"`
}

type SubEntry struct {
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

type SubEntryFilter func(*SubEntry) bool

// returns os.ErrNotExists if not found the path
func (ssfs *SSFS) Get(path string) (*Entry, error) {
	return ssfs.GetFilter(path, nil)
}

func (ssfs *SSFS) GetGlob(path string, pattern string) (*Entry, error) {
	path = filepath.Clean(path)
	return ssfs.GetFilter(path, func(ent *SubEntry) bool {
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

func (ssfs *SSFS) GetFilter(path string, filter SubEntryFilter) (*Entry, error) {
	return ssfs.getEntry(path, filter, true)
}

func (ssfs *SSFS) RealPath(path string) (string, error) {
	ent, err := ssfs.getEntry(path, nil, false)
	if err != nil {
		return "", err
	}
	return ent.abspath, nil
}

func (ssfs *SSFS) isGitClone(path string) (string, bool) {
	path = filepath.Clean(path)
	if stat, err := os.Stat(filepath.Join(path, ".git")); err == nil && stat.IsDir() {
		if repo, err := git.PlainOpen(path); err == nil {
			if remotes, err := repo.Remotes(); err == nil && len(remotes) > 0 {
				for _, url := range remotes[0].Config().URLs {
					if strings.HasPrefix(url, "http") {
						return url, true
					}
				}
			}
		}
	}
	return "", false
}

func (ssfs *SSFS) getEntry(path string, filter SubEntryFilter, loadContent bool) (*Entry, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return nil, err
	}
	filename := filepath.Base(path)
	if path == "/" || path == "" {
		filename = "/"
	}

	_, isVirtualDir := ssfs.virtualDirs[path]
	if isVirtualDir && path != "/" {
		filename = filepath.Base(path)
	}

	isDir := true
	stat, err := os.Stat(rp.AbsPath)
	if err != nil {
		if !isVirtualDir {
			return nil, err
		}
	} else {
		isVirtualDir = false
		isDir = stat.IsDir()
	}

	if isDir {
		ret := &Entry{
			IsDir:    true,
			Name:     filename,
			ReadOnly: isVirtualDir,
			abspath:  rp.AbsPath,
		}
		if url, isGitClone := ssfs.isGitClone(rp.AbsPath); isGitClone {
			ret.GitClone = true
			ret.GitUrl = url
		}
		if children, ok := ssfs.virtualDirs[path]; ok {
			for _, sub := range children {
				ret.Children = append(ret.Children,
					&SubEntry{
						IsDir:    true,
						Name:     sub,
						ReadOnly: isVirtualDir,
						Type:     "dir",
						Virtual:  false,
					})
			}
		}
		if isVirtualDir {
			return ret, nil
		}
		entries, err := os.ReadDir(rp.AbsPath)
		if err != nil {
			return nil, err
		}
		for _, ent := range entries {
			nfo, err := ent.Info()
			if err != nil {
				continue
			}
			entType := "dir"
			if !nfo.IsDir() {
				entType = filepath.Ext(ent.Name())
			}
			subEnt := &SubEntry{
				IsDir:              nfo.IsDir(),
				Name:               ent.Name(),
				ReadOnly:           false,
				Type:               entType,
				Size:               nfo.Size(),
				LastModifiedMillis: nfo.ModTime().UnixMilli(),
			}
			if ssfs.ignores[subEnt.Name] {
				continue
			}
			if filter != nil {
				if !filter(subEnt) {
					continue
				}
			}
			if nfo.IsDir() {
				if url, isGitClone := ssfs.isGitClone(filepath.Join(rp.AbsPath, subEnt.Name)); isGitClone {
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
		ret = appendVirtual(path, ret)
		return ret, nil
	} else {
		ret := &Entry{
			IsDir:    false,
			Name:     filename,
			ReadOnly: false,
			abspath:  rp.AbsPath,
		}
		if loadContent {
			if content, err := os.ReadFile(rp.AbsPath); err == nil {
				ret.Content = content
				return ret, nil
			} else {
				return nil, err
			}
		} else {
			return ret, nil
		}
	}
}

func (ssfs *SSFS) MkDir(path string) (*Entry, error) {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return nil, err
	}
	if rp.ReadOnly {
		return nil, fmt.Errorf("unable to create directory, %s is read-only", path)
	}
	if err := os.Mkdir(rp.AbsPath, 0755); err != nil {
		return nil, err
	}
	return ssfs.Get(path)
}

func (ssfs *SSFS) Remove(path string) error {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return err
	}
	if rp.ReadOnly {
		return fmt.Errorf("unable to remove, %s is read-only", path)
	}
	return os.Remove(rp.AbsPath)
}

func (ssfs *SSFS) RemoveRecursive(path string) error {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return err
	}
	if rp.ReadOnly {
		return fmt.Errorf("unable to remove, %s is read-only", path)
	}
	return os.RemoveAll(rp.AbsPath)
}

func (ssfs *SSFS) Rename(path string, dest string) error {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return err
	}
	if rp.ReadOnly {
		return fmt.Errorf("unable to rename, %s is read-only", path)
	}
	rp2, err := ssfs.FindRealPath(dest)
	if err != nil {
		return err
	}
	if rp2.ReadOnly {
		return fmt.Errorf("unable to rename, %s is read-only", dest)
	}
	_, err = os.Stat(rp.AbsPath)
	if err != nil {
		return fmt.Errorf("unable to access %s, %s", path, err.Error())
	}
	_, err = os.Stat(rp2.AbsPath)
	if err == nil {
		return fmt.Errorf("%q already exists", dest)
	}

	return os.Rename(rp.AbsPath, rp2.AbsPath)
}

func (ssfs *SSFS) Set(path string, content []byte) error {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return err
	}
	stat, err := os.Stat(rp.AbsPath)
	if err == nil && stat.IsDir() {
		return fmt.Errorf("unable to write, %s is directory", path)
	}
	return os.WriteFile(rp.AbsPath, content, 0644)
}

const defaultGitRemoteName = "origin"

// git clone int the given path, it discards all local changes.
func (ssfs *SSFS) GitClone(path string, gitUrl string, progress io.Writer) (*Entry, error) {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return nil, err
	}
	if rp.ReadOnly {
		return nil, fmt.Errorf("unable to clone, %s is read-only", path)
	}
	if progress == nil {
		progress = os.Stdout
	}
	progress.Write([]byte(fmt.Sprintf("Cloning into '%s'...", path)))
	repo, err := git.PlainClone(rp.AbsPath, false, &git.CloneOptions{
		URL:          gitUrl,
		SingleBranch: true,
		RemoteName:   defaultGitRemoteName,
		Progress:     progress,
		Depth:        0,
	})
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			repo, err = git.PlainOpen(rp.AbsPath)
		}
		if err != nil {
			progress.Write([]byte(err.Error()))
			return nil, err
		}
	}
	ref, err := repo.Head()
	if err != nil {
		progress.Write([]byte(err.Error()))
		return nil, err
	}
	w, err := repo.Worktree()
	if err != nil {
		progress.Write([]byte(err.Error()))
		return nil, err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash:  ref.Hash(),
		Force: true,
	})
	if err != nil {
		progress.Write([]byte(err.Error()))
		return nil, err
	} else {
		progress.Write([]byte("Done."))
	}
	return ssfs.Get(path)
}

// git clone int the given path, it discards all local changes.
func (ssfs *SSFS) GitPull(path string, gitUrl string, progress io.Writer) (*Entry, error) {
	rp, err := ssfs.FindRealPath(path)
	if err != nil {
		return nil, err
	}
	if rp.ReadOnly {
		return nil, fmt.Errorf("unable to pull, %s is read-only", path)
	}

	if progress == nil {
		progress = os.Stdout
	}
	progress.Write([]byte(fmt.Sprintf("Updating '%s'...", path)))
	repo, err := git.PlainOpen(rp.AbsPath)
	if err != nil {
		progress.Write([]byte(err.Error()))
		return nil, err
	}
	conf, err := repo.Config()
	if err != nil {
		progress.Write([]byte(err.Error()))
		return nil, err
	}
	remote := conf.Remotes[defaultGitRemoteName]
	if gitUrl != "" && gitUrl != remote.URLs[0] {
		err = fmt.Errorf("git remote url is not matched, %s", remote.URLs[0])
		progress.Write([]byte(err.Error()))
		return nil, err
	}
	w, err := repo.Worktree()
	if err != nil {
		progress.Write([]byte(err.Error()))
		return nil, err
	}

	err = w.Pull(&git.PullOptions{Force: true})
	if err != nil {
		progress.Write([]byte(err.Error()))
		if err != git.NoErrAlreadyUpToDate {
			return nil, err
		}
	} else {
		progress.Write([]byte("Done."))
	}
	return ssfs.Get(path)
}
