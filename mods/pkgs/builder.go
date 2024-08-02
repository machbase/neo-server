package pkgs

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Builder struct {
	ds *PackageMeta

	workDir    string
	distDir    string
	httpClient *http.Client
}

func NewBuilder(meta *PackageMeta, version string, opts ...BuilderOption) (*Builder, error) {
	ret := &Builder{ds: meta}
	for _, opt := range opts {
		opt(ret)
	}
	return ret, nil
}

type BuilderOption func(*Builder)

func WithWorkDir(workDir string) BuilderOption {
	return func(b *Builder) {
		b.workDir = workDir
	}
}

func WithDistDir(distDir string) BuilderOption {
	return func(b *Builder) {
		b.distDir = distDir
	}
}

// Build builds the package with the given version.
// if version is empty or "latest", it will use the latest version.
func (b *Builder) Build(ver string) error {
	if b.httpClient == nil {
		b.httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
			Timeout: time.Duration(10) * time.Second,
		}
	}
	var targetRelease *GhReleaseInfo
	if lr, err := b.getReleaseInfo(ver); err != nil {
		return err
	} else {
		targetRelease = lr
	}

	// mkdir workdir
	if err := os.MkdirAll(b.workDir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	// download the tarball
	srcTarBall := "src.tar.gz"
	cmd := exec.Command("sh", "-c", fmt.Sprintln("wget", targetRelease.TarballUrl, "-O", srcTarBall))
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	// extract the tarball // tar xvf ./src.tar.gz --strip-components=1
	cmd = exec.Command("sh", "-c", fmt.Sprintln("tar", "xf", srcTarBall, "--strip-components", fmt.Sprintf("%d", b.ds.Distributable.StripComponents)))
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	// make build script
	buildScript := "__build__.sh"
	f, err := os.OpenFile(filepath.Join(b.workDir, buildScript), os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, line := range b.ds.BuildRecipe.Script {
		fmt.Fprintln(f, line)
	}
	if err := f.Sync(); err != nil {
		return err
	}

	// run build script
	cmd = exec.Command("sh", "-c", "./"+buildScript)
	cmd.Env = append(os.Environ(), b.ds.BuildRecipe.Env...)
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// mkdir workdir
	if err := os.MkdirAll(b.distDir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	// copy the built files to dist dir
	for _, pv := range b.ds.Provides {
		cmd = exec.Command("rsync", "-r", filepath.Join(b.workDir, pv), b.distDir)
		cmd.Env = append(os.Environ(), b.ds.BuildRecipe.Env...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// tar xvf ./src.tar.gz --strip-components=1
	fmt.Printf("%+v\n", targetRelease)
	return nil
}

func (b *Builder) getReleaseInfo(ver string) (*GhReleaseInfo, error) {
	if b.ds.Distributable.Github == "" {
		return nil, fmt.Errorf("distributable.github is not set")
	}
	org, repo, err := GithubSplitPath(b.ds.Distributable.Github)
	if err != nil {
		return nil, err
	}
	// github's default distribution, strip_components is 1
	if b.ds.Distributable.StripComponents == 0 {
		b.ds.Distributable.StripComponents = 1
	}

	if ver != "" || strings.ToLower(ver) == "latest" {
		return GithubLatestReleaseInfo(b.httpClient, org, repo)
	} else {
		return GithubReleaseInfo(b.httpClient, org, repo, ver)
	}
}
