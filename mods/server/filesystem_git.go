package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
)

func (service *WorkspaceFileSystem) isGitClone(path string) (string, bool) {
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

const defaultGitRemoteName = "origin"

// git clone int the given path, it discards all local changes.
func (service *WorkspaceFileSystem) GitClone(path string, gitUrl string, progress io.Writer) (*WorkspaceEntry, error) {
	rp, err := service.FindRealPath(path)
	if err != nil {
		return nil, err
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
	return service.Get(path)
}

// git clone int the given path, it discards all local changes.
func (service *WorkspaceFileSystem) GitPull(path string, gitUrl string, progress io.Writer) (*WorkspaceEntry, error) {
	rp, err := service.FindRealPath(path)
	if err != nil {
		return nil, err
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
	return service.Get(path)
}
