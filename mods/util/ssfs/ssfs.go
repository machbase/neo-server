package ssfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/machbase/neo-server/mods/util/glob"
)

// Server Side File System
type SSFS struct {
	bases []BaseDir

	ignores map[string]bool
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
	ret := &SSFS{}
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
	ret.ignores = map[string]bool{
		".git":          true,
		"machbase_home": true,
		"node_modules":  true,
		".pnp":          true,
		".DS_Store":     true,
	}
	return ret, nil
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
	GitUrl   string      `json:"gitUrl,omitempty"`
	GitClone bool        `json:"gitClone"`
}

type SubEntry struct {
	IsDir              bool   `json:"isDir"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	Size               int64  `json:"size,omitempty"`
	LastModifiedMillis int64  `json:"lastModifiedUnixMillis"`
	GitUrl             string `json:"gitUrl,omitempty"`
	GitClone           bool   `json:"gitClone"`
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

func (ssfs *SSFS) isGitClone(path string) (string, bool) {
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
		if url, isGitClone := ssfs.isGitClone(abspath); isGitClone {
			ret.GitClone = true
			ret.GitUrl = url
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
			if ssfs.ignores[subEnt.Name] {
				continue
			}
			if filter != nil {
				if !filter(subEnt) {
					continue
				}
			}
			if nfo.IsDir() {
				if url, isGitClone := ssfs.isGitClone(filepath.Join(abspath, subEnt.Name)); isGitClone {
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
		ret := &Entry{
			IsDir:   false,
			Name:    name,
			abspath: abspath,
		}
		if loadContent {
			if content, err := os.ReadFile(abspath); err == nil {
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
func (ssfs *SSFS) GitClone(path string, gitUrl string, progress io.Writer) (*Entry, error) {
	idx, _, abspath := ssfs.findDir(path)
	if idx == -1 {
		return nil, os.ErrNotExist
	}

	if progress == nil {
		progress = os.Stdout
	}
	repo, err := git.PlainClone(abspath, false, &git.CloneOptions{
		URL:          gitUrl,
		SingleBranch: true,
		RemoteName:   "origin",
		Progress:     progress,
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
