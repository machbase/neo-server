package pkgs

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/semver/v3"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type RosterName string

const ROSTER_CENTRAL RosterName = "central"

var ROSTER_REPOS = map[RosterName]string{
	ROSTER_CENTRAL: "https://github.com/machbase/neo-pkg.git",
}

type Roster struct {
	metaDir       string
	distDir       string
	buildDir      string
	cacheManagers map[RosterName]*CacheManager
}

type RosterOption func(*Roster)

func NewRoster(metaDir string, distDir string, opts ...RosterOption) (*Roster, error) {
	if abs, err := filepath.Abs(metaDir); err != nil {
		return nil, err
	} else {
		metaDir = abs
	}
	if abs, err := filepath.Abs(distDir); err != nil {
		return nil, err
	} else {
		distDir = abs
	}

	ret := &Roster{
		metaDir: metaDir,
		distDir: distDir,
	}
	for _, opt := range opts {
		opt(ret)
	}
	ret.buildDir = filepath.Join(metaDir, ".build")
	centralCacheDir := filepath.Join(metaDir, ".cache", string(ROSTER_CENTRAL))
	if err := os.MkdirAll(centralCacheDir, 0755); err != nil {
		return nil, err
	}
	ret.cacheManagers = map[RosterName]*CacheManager{
		ROSTER_CENTRAL: NewCacheManager(centralCacheDir),
	}
	return ret, nil
}

func (r *Roster) MetaDir(metaType RosterName) string {
	return filepath.Join(r.metaDir, string(metaType))
}

// WalkPackages walks all packages in the central repository.
// if callback returns false, it will stop walking.
func (r *Roster) WalkPackages(cb func(name string) bool) error {
	entries, err := os.ReadDir(filepath.Join(r.MetaDir(ROSTER_CENTRAL), "projects"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if !cb(entry.Name()) {
				return nil
			}
		}
	}
	return nil
}

func (r *Roster) Sync() error {
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		if err := r.SyncRoster(rosterName, rosterRepoUrl); err != nil {
			return err
		}
	}
	return nil
}

func (r *Roster) SyncRoster(rosterName RosterName, rosterRepoUrl string) error {
	repo, err := git.PlainClone(r.MetaDir(rosterName), false, &git.CloneOptions{
		URL:           rosterRepoUrl,
		RemoteName:    string(git.DefaultRemoteName),
		ReferenceName: plumbing.ReferenceName("refs/heads/main"),
		SingleBranch:  true,
	})
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			repo, err = git.PlainOpen(r.MetaDir(rosterName))
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	w, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = w.Reset(&git.ResetOptions{Mode: git.HardReset})
	if err != nil {
		return err
	}

	err = w.Pull(&git.PullOptions{
		RemoteName:    string(git.DefaultRemoteName),
		ReferenceName: plumbing.ReferenceName("refs/heads/main"),
		Depth:         1,
		Force:         true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	r.WalkPackages(func(name string) bool {
		meta, err := r.LoadPackageMeta(name)
		if err != nil {
			return true
		}
		_, err = r.LoadPackageCache(name, meta, true)
		if err != nil {
			return true
		}
		return true
	})
	return nil
}

func (r *Roster) LoadPackageMeta(pkgName string) (*PackageMeta, error) {
	return r.LoadPackageMetaRoster(ROSTER_CENTRAL, pkgName)
}

// LoadPackageMetaRoster loads package.yml file from the given package name.
// if the package.yml file is not found, it will return nil, and nil error.
// if the package.yml file is found, it will return the package meta info and nil error
// if the package.yml has an error, it will return the error.
func (r *Roster) LoadPackageMetaRoster(rosterName RosterName, pkgName string) (*PackageMeta, error) {
	path := filepath.Join(r.MetaDir(rosterName), "projects", pkgName, "package.yml")
	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		path = filepath.Join(r.MetaDir(rosterName), "projects", pkgName, "package.yaml")
		if stat, err := os.Stat(path); err != nil || stat.IsDir() {
			return nil, nil
		}
	}
	ret, err := parsePackageMetaFile(path)
	if ret != nil {
		ret.rosterName = rosterName
	}
	return ret, err
}

func (r *Roster) LoadPackageCache(name string, meta *PackageMeta, forceRefresh bool) (*PackageCache, error) {
	// if this is the first time to load the package cache,
	// it will receive the error of "file not found".
	cache, _ := r.cacheManagers[meta.rosterName].ReadCache(name)
	if !forceRefresh {
		return cache, nil
	}

	if cache == nil {
		cache = &PackageCache{
			Name: name,
		}
	}
	org, repo, err := GithubSplitPath(meta.Distributable.Github)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(10) * time.Second,
	}

	var ghRepo *GhRepoInfo
	if lr, err := GithubRepoInfo(httpClient, org, repo); err != nil {
		return cache, err
	} else {
		ghRepo = lr
	}

	var ghRelease *GhReleaseInfo
	if lr, err := GithubLatestReleaseInfo(httpClient, org, repo); err != nil {
		return cache, err
	} else {
		ghRelease = lr
	}

	tmpl, err := template.New("url").Parse(meta.Distributable.Url)
	if err != nil {
		return cache, err
	}
	buff := &strings.Builder{}
	tmpl.Execute(buff, map[string]string{
		"tag":     ghRelease.TagName,
		"version": strings.TrimPrefix(ghRelease.TagName, "v"),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	})
	ver, err := semver.NewVersion(ghRelease.Name)
	if err != nil {
		return nil, err
	}

	cache.Name = name
	cache.Github = ghRepo
	cache.LatestRelease = ver.String()
	cache.LatestReleaseTag = ghRelease.TagName
	cache.StripComponents = meta.Distributable.StripComponents
	cache.PublishedAt = ghRelease.PublishedAt
	cache.Url = buff.String()
	cache.CachedAt = time.Now()

	thisPkgDir := filepath.Join(r.distDir, name)
	currentVerDir := filepath.Join(thisPkgDir, "current")
	if _, err := os.Stat(currentVerDir); err == nil {
		cache.InstalledPath = currentVerDir
	}
	current, err := os.Readlink(currentVerDir)
	if err == nil {
		linkName := filepath.Base(current)
		ver, err := semver.NewVersion(linkName)
		if err == nil {
			cache.InstalledVersion = ver.String()
		}
	}
	return cache, r.cacheManagers[ROSTER_CENTRAL].WriteCache(cache)
}

// Install installs the package to the distDir
// returns the installed symlink path '~/dist/<name>/current'
func (r *Roster) Install(name string, output io.Writer) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	cache, err := r.LoadPackageCache(name, meta, true)
	if err != nil {
		return err
	}

	force := true
	fileBase := filepath.Base(cache.Url)
	fileExt := filepath.Ext(fileBase)
	thisPkgDir := filepath.Join(r.distDir, cache.Name)
	archiveFile := filepath.Join(thisPkgDir, fmt.Sprintf("%s%s", cache.LatestRelease, fileExt))
	unarchiveDir := filepath.Join(thisPkgDir, cache.LatestRelease)
	currentVerDir := filepath.Join(thisPkgDir, "current")

	if err := os.MkdirAll(unarchiveDir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	if _, err := os.Stat(archiveFile); err == nil && !force {
		return fmt.Errorf("file %q already exists", archiveFile)
	}

	srcUrl, err := url.Parse(cache.Url)
	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(10) * time.Second,
	}

	rsp, err := httpClient.Do(&http.Request{
		Method: "GET",
		URL:    srcUrl,
	})
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	download, err := os.OpenFile(archiveFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	_ /*written*/, err = io.Copy(download, rsp.Body)
	if err != nil {
		return err
	}

	switch fileExt {
	case ".zip":
		cmd := exec.Command("unzip", "-o", "-d", unarchiveDir, archiveFile)
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	case ".tar.gz":
		cmd := exec.Command("tar", "xf", archiveFile, "--strip-components", fmt.Sprintf("%d", cache.StripComponents))
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(currentVerDir); err == nil {
		if err := os.Remove(currentVerDir); err != nil {
			return err
		}
	}
	err = os.Symlink(cache.LatestRelease, currentVerDir)
	if err == nil {
		cache.InstalledVersion = cache.LatestRelease
		cache.InstalledPath = currentVerDir
		cache.CachedAt = time.Now()
		err = r.cacheManagers[meta.rosterName].WriteCache(cache)
	}
	if err != nil {
		return err
	}

	if meta.InstallRecipe != nil {
		cmd := exec.Command("sh", "-c", meta.InstallRecipe.Script)
		cmd.Dir = currentVerDir
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Roster) Uninstall(name string, output io.Writer) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	cache, err := r.LoadPackageCache(name, meta, true)
	if err != nil {
		return err
	}

	if !filepath.IsAbs(cache.InstalledPath) || !strings.HasPrefix(cache.InstalledPath, r.distDir) {
		return fmt.Errorf("invalid installed path: %q", cache.InstalledPath)
	}
	if err := os.RemoveAll(cache.InstalledPath); err != nil {
		return err
	}
	cache.InstalledPath = ""
	cache.InstalledVersion = ""
	err = r.cacheManagers[meta.rosterName].WriteCache(cache)
	return err
}
