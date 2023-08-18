package ssfs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/machbase/neo-server/mods/util/glob"
)

// Server Side File System
type SSFS struct {
	bases []BaseDir

	recents     []string
	maxRecents  int
	recentsLock sync.Mutex
}

var defaultFs *SSFS

func SetDefault(fs *SSFS) {
	defaultFs = fs
}

func Default() *SSFS {
	return defaultFs
}

type BaseDir struct {
	name    string
	abspath string
}

func NewServerSideFileSystem(baseDirs []string) (*SSFS, error) {
	ret := &SSFS{maxRecents: 5}
	for i, dir := range baseDirs {
		abspath, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		var name string
		if runtime.GOOS == "windows" {
			name = "\\"
			if i > 0 {
				name = "\\" + filepath.Base(abspath)
			}
		} else {
			name = "/"
			if i > 0 {
				name = "/" + filepath.Base(abspath)
			}
		}
		ret.bases = append(ret.bases, BaseDir{name: name, abspath: abspath})
	}
	return ret, nil
}

func (ssfs *SSFS) GetRecentList() []string {
	ssfs.recentsLock.Lock()
	defer ssfs.recentsLock.Unlock()
	return ssfs.recents
}

func (ssfs *SSFS) AddRecentList(path string) {
	ssfs.recentsLock.Lock()
	defer ssfs.recentsLock.Unlock()

	for i, p := range ssfs.recents {
		if p == path {
			if i > 0 {
				for n := i; n > 0; n-- {
					ssfs.recents[n] = ssfs.recents[n-1]
				}
				ssfs.recents[0] = path
			}
			return
		}
	}

	ssfs.recents = append(ssfs.recents, "")
	for n := len(ssfs.recents) - 1; n > 0; n-- {
		ssfs.recents[n] = ssfs.recents[n-1]
	}
	ssfs.recents[0] = path
	if len(ssfs.recents) > ssfs.maxRecents {
		ssfs.recents = ssfs.recents[0:ssfs.maxRecents]
	}
}

// find longest path matched 'BaseDir'
//
// returns index of baseDirs, name, abstract-path
//
// returns index of baseDirs and absolute path of the give path
//
// returns -1 if it doesn't match any dir
func (ssfs *SSFS) findDir(path string) (int, string, string) {
	separatorString := "/"
	separatorChar := byte('/')
	if runtime.GOOS == "windows" {
		separatorString = "\\"
		separatorChar = '\\'
	}
	path = filepath.Join(path)
	for i := len(ssfs.bases) - 1; i >= 0; i-- {
		bd := ssfs.bases[i]
		if strings.HasPrefix(path, bd.name) && (len(path) == len(bd.name) || bd.name == separatorString || path[len(bd.name)] == separatorChar) {
			remain := strings.TrimPrefix(path, bd.name)
			if len(remain) == 0 {
				return i, bd.name, bd.abspath
			}
			abspath := filepath.Join(bd.abspath, remain)
			if strings.HasPrefix(abspath, bd.abspath) {
				return i, filepath.Base(abspath), abspath
			} else {
				return -1, "", ""
			}
		}
	}
	return -1, "", ""
}

type Entry struct {
	IsDir    bool        `json:"isDir"`
	Name     string      `json:"name"`
	Content  []byte      `json:"content,omitempty"`  // file content, if the entry is FILE
	Children []*SubEntry `json:"children,omitempty"` // entry of sub files and dirs, if the entry is DIR
	abspath  string      `json:"-"`
}

type SubEntry struct {
	IsDir              bool   `json:"isDir"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	Size               int64  `json:"size,omitempty"`
	LastModifiedMillis int64  `json:"lastModifiedUnixMillis"`
}

type SubEntryFilter func(*SubEntry) bool

// returns os.ErrNotExists if not found the path
func (ssfs *SSFS) Get(path string) (*Entry, error) {
	return ssfs.GetFilter(path, nil)
}

func (ssfs *SSFS) GetGlob(path string, pattern string) (*Entry, error) {
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

func (ssfs *SSFS) getEntry(path string, filter SubEntryFilter, loadContent bool) (*Entry, error) {
	idx, name, abspath := ssfs.findDir(path)
	if idx == -1 {
		return nil, os.ErrNotExist
	}
	stat, err := os.Stat(abspath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		ret := &Entry{
			IsDir:   true,
			Name:    name,
			abspath: abspath,
		}
		if idx == 0 && len(ssfs.bases) > 1 { // root dir and has sub dirs
			for _, sub := range ssfs.bases[1:] {
				ret.Children = append(ret.Children, &SubEntry{
					IsDir: true, Name: sub.name, Type: "dir",
				})
			}
		}
		entries, err := os.ReadDir(abspath)
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
				Type:               entType,
				Size:               nfo.Size(),
				LastModifiedMillis: nfo.ModTime().UnixMilli(),
			}
			if filter != nil {
				if !filter(subEnt) {
					continue
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
		ret := &Entry{
			IsDir:   false,
			Name:    name,
			abspath: abspath,
		}
		if loadContent {
			if content, err := os.ReadFile(abspath); err == nil {
				ret.Content = content
				ssfs.AddRecentList(path)
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
	idx, _, abspath := ssfs.findDir(path)
	if idx == -1 {
		return nil, os.ErrNotExist
	}

	if err := os.Mkdir(abspath, 0755); err != nil {
		return nil, err
	}
	return ssfs.Get(path)
}

func (ssfs *SSFS) Remove(path string) error {
	idx, _, abspath := ssfs.findDir(path)
	if idx == -1 {
		return os.ErrNotExist
	}
	return os.Remove(abspath)
}

func (ssfs *SSFS) RemoveRecursive(path string) error {
	idx, _, abspath := ssfs.findDir(path)
	if idx == -1 {
		return os.ErrNotExist
	}
	return os.RemoveAll(abspath)
}

func (ssfs *SSFS) Set(path string, content []byte) error {
	idx, _, abspath := ssfs.findDir(path)
	if idx == -1 {
		return os.ErrNotExist
	}

	stat, err := os.Stat(abspath)
	if err == nil && stat.IsDir() {
		return fmt.Errorf("unable to write, %s is directory", path)
	}

	return os.WriteFile(abspath, content, 0644)
}

// git clone int the given path, it discards all local changes.
func (ssfs *SSFS) GitClone(path string, gitUrl string) (*Entry, error) {
	idx, _, abspath := ssfs.findDir(path)
	if idx == -1 {
		return nil, os.ErrNotExist
	}

	repo, err := git.PlainClone(abspath, false, &git.CloneOptions{
		URL:          gitUrl,
		SingleBranch: true,
		RemoteName:   "origin",
		Progress:     os.Stdout,
	})
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			repo, err = git.PlainOpen(abspath)
		}
		if err != nil {
			return nil, err
		}
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}
	w, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash:  ref.Hash(),
		Force: true,
	})
	if err != nil {
		return nil, err
	}
	return ssfs.Get(path)
}
