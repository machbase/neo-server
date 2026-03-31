package root_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

var pkgTestJshBinPath string

func TestMain(m *testing.M) {
	tmpDir := os.TempDir()
	pkgTestJshBinPath = filepath.Join(tmpDir, "jsh-root-pkg-test")
	args := []string{"build", "-o"}
	if runtime.GOOS == "windows" {
		pkgTestJshBinPath += ".exe"
	}
	args = append(args, pkgTestJshBinPath, "..")
	cmd := exec.Command("go", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("Failed to build jsh binary for pkg tests:", err)
		fmt.Print(string(output))
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func TestPkgInitCommand(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "init", "demo-app")
	if err != nil {
		t.Fatalf("pkg init failed: %v\n%s", err, output)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		t.Fatalf("failed to read package.json: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("failed to parse package.json: %v", err)
	}

	if got := manifest["name"]; got != "demo-app" {
		t.Fatalf("package.json name = %v, want demo-app", got)
	}
	if got := manifest["version"]; got != "1.0.0" {
		t.Fatalf("package.json version = %v, want 1.0.0", got)
	}
	if _, ok := manifest["dependencies"].(map[string]any); !ok {
		t.Fatalf("package.json dependencies missing or invalid: %#v", manifest["dependencies"])
	}
}

func TestPkgInitCreatesScriptsObject(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "init", "demo-app")
	if err != nil {
		t.Fatalf("pkg init failed: %v\n%s", err, output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if _, ok := manifest["scripts"].(map[string]any); !ok {
		t.Fatalf("package.json scripts missing or invalid: %#v", manifest["scripts"])
	}
	if got := len(manifest["scripts"].(map[string]any)); got != 0 {
		t.Fatalf("package.json scripts length = %d, want 0", got)
	}
}

func TestPkgInitUsesTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "init", "--dir", "workspace/app", "demo-app")
	if err != nil {
		t.Fatalf("pkg init with --dir failed: %v\n%s", err, output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "workspace", "app", "package.json"))
	if got := manifest["name"]; got != "demo-app" {
		t.Fatalf("target package.json name = %v, want demo-app", got)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); !os.IsNotExist(err) {
		t.Fatalf("caller cwd should not receive package.json, err=%v", err)
	}
}

func TestPkgInitSupportsShortTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "init", "-C", "pkg-root", "demo-app")
	if err != nil {
		t.Fatalf("pkg init with -C failed: %v\n%s", err, output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "pkg-root", "package.json"))
	if got := manifest["name"]; got != "demo-app" {
		t.Fatalf("short-option package.json name = %v, want demo-app", got)
	}
}

func TestPkgInitRejectsFileTargetPath(t *testing.T) {
	workDir := t.TempDir()
	targetFile := filepath.Join(workDir, "target-file")
	if err := os.WriteFile(targetFile, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	output, err := runCommand(workDir, nil, "pkg", "init", "--dir", "target-file", "demo-app")
	if err == nil {
		t.Fatalf("expected file target path to fail, output=%q", output)
	}
	if !strings.Contains(err.Error(), "Install target is not a directory") {
		t.Fatalf("unexpected error: %v\n%s", err, output)
	}
}

func TestPkgInitHelpIncludesTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "init", "--help")
	if err != nil && !strings.Contains(output, "pkg init [options] <name>") {
		t.Fatalf("pkg init --help failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "-C, --dir") {
		t.Fatalf("help output missing short/long dir option: %q", output)
	}
	if !strings.Contains(output, "Use this project directory") {
		t.Fatalf("help output missing dir description: %q", output)
	}
}

func TestPkgInstallGitHubLatestTag(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/demo/tags":
			writeJSON(w, []map[string]any{
				{"name": "v1.1.0"},
				{"name": "v1.0.0"},
			})
		case "/api/repos/acme/demo/contents":
			if got := r.URL.Query().Get("ref"); got != "v1.1.0" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/demo/v1.1.0/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/demo/v1.1.0/index.js",
				},
			})
		case "/download/demo/v1.1.0/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"demo\",\n  \"version\": \"0.9.0\"\n}\n"))
		case "/download/demo/v1.1.0/index.js":
			_, _ = w.Write([]byte("module.exports = { message: 'github-latest' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/demo")
	if err != nil {
		t.Fatalf("pkg install failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Installed github.com/acme/demo#tag=v1.1.0") {
		t.Fatalf("install output = %q, want canonical tag ref", output)
	}

	target := filepath.Join(workDir, "node_modules", "github.com", "acme", "demo")
	if _, err := os.Stat(filepath.Join(target, "package.json")); err != nil {
		t.Fatalf("expected installed GitHub package.json: %v", err)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/demo'); console.println(pkg.message);")
	if err != nil {
		t.Fatalf("require installed GitHub package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "github-latest" {
		t.Fatalf("require output = %q, want github-latest", strings.TrimSpace(message))
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/demo"]; got != "#tag=v1.1.0" {
		t.Fatalf("saved dependency = %v, want #tag=v1.1.0", got)
	}
	lockJSON := readJSONFile(t, filepath.Join(workDir, "package-lock.json"))
	packages := lockJSON["packages"].(map[string]any)
	if got := packages["node_modules/github.com/acme/demo"].(map[string]any)["resolved"]; got != "github.com/acme/demo#tag=v1.1.0" {
		t.Fatalf("locked resolved source = %v, want github.com/acme/demo#tag=v1.1.0", got)
	}
}

func TestPkgInstallGitHubFallsBackToDefaultBranchWhenNoTags(t *testing.T) {
	workDir := t.TempDir()
	var repoHits atomic.Int64
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/notags/tags":
			writeJSON(w, []map[string]any{})
		case "/api/repos/acme/notags":
			repoHits.Add(1)
			writeJSON(w, map[string]any{
				"default_branch": "main",
			})
		case "/api/repos/acme/notags/contents":
			if got := r.URL.Query().Get("ref"); got != "main" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/notags/main/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/notags/main/index.js",
				},
			})
		case "/download/notags/main/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"notags\",\n  \"version\": \"0.0.1\"\n}\n"))
		case "/download/notags/main/index.js":
			_, _ = w.Write([]byte("module.exports = { message: 'default-branch' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/notags")
	if err != nil {
		t.Fatalf("pkg install without tags failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Installed github.com/acme/notags#branch=main") {
		t.Fatalf("install output = %q, want canonical branch ref", output)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/notags'); console.println(pkg.message);")
	if err != nil {
		t.Fatalf("require installed no-tag GitHub package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "default-branch" {
		t.Fatalf("require output = %q, want default-branch", strings.TrimSpace(message))
	}
	if repoHits.Load() != 1 {
		t.Fatalf("default branch metadata lookup count = %d, want 1", repoHits.Load())
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/notags"]; got != "#branch=main" {
		t.Fatalf("saved dependency = %v, want #branch=main", got)
	}
	lockJSON := readJSONFile(t, filepath.Join(workDir, "package-lock.json"))
	packages := lockJSON["packages"].(map[string]any)
	if got := packages["node_modules/github.com/acme/notags"].(map[string]any)["resolved"]; got != "github.com/acme/notags#branch=main" {
		t.Fatalf("locked resolved source = %v, want github.com/acme/notags#branch=main", got)
	}
}

func TestPkgRunExecutesPackageScript(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "pkg-run-fixture.js"), filepath.Join(workDir, "pkg-run-fixture.js"))
	writeJSONFile(t, filepath.Join(workDir, "package.json"), map[string]any{
		"name":    "pkg-run-app",
		"version": "1.0.0",
		"scripts": map[string]any{
			"fixture": "./pkg-run-fixture.js alpha 'beta gamma'",
		},
	})

	output, err := runCommand(workDir, nil, "pkg", "run", "fixture", "delta")
	if err != nil {
		t.Fatalf("pkg run failed: %v\n%s", err, output)
	}

	if got := strings.TrimSpace(output); got != "alpha|beta gamma|delta" {
		t.Fatalf("pkg run output = %q, want alpha|beta gamma|delta", got)
	}
}

func TestPkgRunUsesTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()
	targetDir := filepath.Join(workDir, "workspace", "app")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	copyTestFile(t, filepath.Join("..", "test", "pkg-run-fixture.js"), filepath.Join(targetDir, "pkg-run-fixture.js"))
	writeJSONFile(t, filepath.Join(targetDir, "package.json"), map[string]any{
		"name":    "pkg-run-target-app",
		"version": "1.0.0",
		"scripts": map[string]any{
			"fixture": "./pkg-run-fixture.js nested",
		},
	})

	output, err := runCommand(workDir, nil, "pkg", "run", "--dir", "workspace/app", "fixture", "value")
	if err != nil {
		t.Fatalf("pkg run --dir failed: %v\n%s", err, output)
	}

	if got := strings.TrimSpace(output); got != "nested|value" {
		t.Fatalf("pkg run --dir output = %q, want nested|value", got)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); !os.IsNotExist(err) {
		t.Fatalf("caller cwd should not receive package.json, err=%v", err)
	}
}

func TestPkgInstallGitHubExplicitTag(t *testing.T) {
	workDir := t.TempDir()
	var tagHits atomic.Int64
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/filestat/tags":
			tagHits.Add(1)
			writeJSON(w, []map[string]any{{"name": "v9.9.9"}})
		case "/api/repos/acme/filestat/contents":
			if got := r.URL.Query().Get("ref"); got != "v0.1.0" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/filestat/v0.1.0/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/filestat/v0.1.0/index.js",
				},
			})
		case "/download/filestat/v0.1.0/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"filestat\",\n  \"version\": \"0.1.0\"\n}\n"))
		case "/download/filestat/v0.1.0/index.js":
			_, _ = w.Write([]byte("module.exports = { value: 'explicit-tag' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/filestat#tag=v0.1.0")
	if err != nil {
		t.Fatalf("pkg install explicit tag failed: %v\n%s", err, output)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/filestat'); console.println(pkg.value);")
	if err != nil {
		t.Fatalf("require installed explicit tag package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "explicit-tag" {
		t.Fatalf("require output = %q, want explicit-tag", strings.TrimSpace(message))
	}
	if tagHits.Load() != 0 {
		t.Fatalf("tags API should not be called for explicit tag installs, got %d hits", tagHits.Load())
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/filestat"]; got != "#tag=v0.1.0" {
		t.Fatalf("saved dependency = %v, want #tag=v0.1.0", got)
	}
}

func TestPkgInstallGitHubExplicitBranchSkipsTagDiscovery(t *testing.T) {
	workDir := t.TempDir()
	var tagHits atomic.Int64
	var repoHits atomic.Int64
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/filestat/tags":
			tagHits.Add(1)
			writeJSON(w, []map[string]any{{"name": "v9.9.9"}})
		case "/api/repos/acme/filestat":
			repoHits.Add(1)
			writeJSON(w, map[string]any{
				"default_branch": "main",
			})
		case "/api/repos/acme/filestat/contents":
			if got := r.URL.Query().Get("ref"); got != "develop" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/filestat/develop/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/filestat/develop/index.js",
				},
			})
		case "/download/filestat/develop/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"filestat\",\n  \"version\": \"0.1.0\"\n}\n"))
		case "/download/filestat/develop/index.js":
			_, _ = w.Write([]byte("module.exports = { value: 'explicit-branch' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/filestat#branch=develop")
	if err != nil {
		t.Fatalf("pkg install explicit branch failed: %v\n%s", err, output)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/filestat'); console.println(pkg.value);")
	if err != nil {
		t.Fatalf("require installed explicit branch package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "explicit-branch" {
		t.Fatalf("require output = %q, want explicit-branch", strings.TrimSpace(message))
	}
	if tagHits.Load() != 0 {
		t.Fatalf("tags API should not be called for explicit branch installs, got %d hits", tagHits.Load())
	}
	if repoHits.Load() != 0 {
		t.Fatalf("repo metadata API should not be called for explicit branch installs, got %d hits", repoHits.Load())
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/filestat"]; got != "#branch=develop" {
		t.Fatalf("saved dependency = %v, want #branch=develop", got)
	}
}

func TestPkgInstallGitHubAtRefAliasesToExplicitTag(t *testing.T) {
	workDir := t.TempDir()
	var tagHits atomic.Int64
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/filestat/tags":
			tagHits.Add(1)
			writeJSON(w, []map[string]any{{"name": "v9.9.9"}})
		case "/api/repos/acme/filestat/contents":
			if got := r.URL.Query().Get("ref"); got != "v0.1.0" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/filestat/v0.1.0/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/filestat/v0.1.0/index.js",
				},
			})
		case "/download/filestat/v0.1.0/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"filestat\",\n  \"version\": \"0.1.0\"\n}\n"))
		case "/download/filestat/v0.1.0/index.js":
			_, _ = w.Write([]byte("module.exports = { value: 'explicit-tag-alias' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/filestat@v0.1.0")
	if err != nil {
		t.Fatalf("pkg install github @ref alias failed: %v\n%s", err, output)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/filestat'); console.println(pkg.value);")
	if err != nil {
		t.Fatalf("require installed @ref alias package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "explicit-tag-alias" {
		t.Fatalf("require output = %q, want explicit-tag-alias", strings.TrimSpace(message))
	}
	if tagHits.Load() != 0 {
		t.Fatalf("tags API should not be called for github @ref alias installs, got %d hits", tagHits.Load())
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/filestat"]; got != "#tag=v0.1.0" {
		t.Fatalf("saved dependency = %v, want #tag=v0.1.0", got)
	}
}

func TestPkgInstallGitHubReusesLockedTag(t *testing.T) {
	workDir := t.TempDir()
	var tagHits atomic.Int64
	var directoryAPIHits atomic.Int64
	var lockMode atomic.Bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/demo/tags":
			tagHits.Add(1)
			if lockMode.Load() {
				http.Error(w, "tags should not be requested while using lock file", http.StatusGone)
				return
			}
			writeJSON(w, []map[string]any{{"name": "v1.0.1"}})
		case "/api/repos/acme/demo/contents":
			directoryAPIHits.Add(1)
			if got := r.URL.Query().Get("ref"); got != "v1.0.1" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/demo/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/demo/index.js",
				},
				{
					"type": "dir",
					"path": "lib",
				},
			})
		case "/api/repos/acme/demo/contents/lib":
			directoryAPIHits.Add(1)
			if got := r.URL.Query().Get("ref"); got != "v1.0.1" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "lib/helper.js",
					"download_url": server.URL + "/download/demo/lib/helper.js",
				},
			})
		case "/download/demo/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"demo\",\n  \"version\": \"0.0.1\"\n}\n"))
		case "/download/demo/index.js":
			_, _ = w.Write([]byte("const helper = require('./lib/helper'); module.exports = { message: helper.message };\n"))
		case "/download/demo/lib/helper.js":
			_, _ = w.Write([]byte("module.exports = { message: 'directory-fallback' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/demo")
	if err != nil {
		t.Fatalf("pkg install github latest failed: %v\n%s", err, output)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/demo'); console.println(pkg.message);")
	if err != nil {
		t.Fatalf("require installed github package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "directory-fallback" {
		t.Fatalf("require output = %q, want directory-fallback", strings.TrimSpace(message))
	}

	lockJSON := readJSONFile(t, filepath.Join(workDir, "package-lock.json"))
	packages := lockJSON["packages"].(map[string]any)
	resolved := packages["node_modules/github.com/acme/demo"].(map[string]any)["resolved"]
	wantResolved := "github.com/acme/demo#tag=v1.0.1"
	if resolved != wantResolved {
		t.Fatalf("locked resolved source = %v, want %s", resolved, wantResolved)
	}

	if err := os.RemoveAll(filepath.Join(workDir, "node_modules")); err != nil {
		t.Fatalf("remove node_modules: %v", err)
	}
	lockMode.Store(true)

	secondOutput, err := runCommand(workDir, env, "pkg", "install")
	if err != nil {
		t.Fatalf("pkg reinstall from lock failed: %v\n%s", err, secondOutput)
	}
	if tagHits.Load() != 1 {
		t.Fatalf("expected exactly one tags lookup before lock reuse, got %d", tagHits.Load())
	}
	if directoryAPIHits.Load() < 4 {
		t.Fatalf("expected directory API to be used for both installs, got %d hits", directoryAPIHits.Load())
	}
}

func TestPkgInstallGitHubExplicitRequestRefreshesLatestTagDespiteLock(t *testing.T) {
	workDir := t.TempDir()
	var latestTag atomic.Value
	latestTag.Store("v1.0.0")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/demo/tags":
			writeJSON(w, []map[string]any{{"name": latestTag.Load().(string)}})
		case "/api/repos/acme/demo/contents":
			ref := r.URL.Query().Get("ref")
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/demo/" + ref + "/package.json",
				},
				{
					"type":         "file",
					"path":         "index.js",
					"download_url": server.URL + "/download/demo/" + ref + "/index.js",
				},
			})
		case "/download/demo/v1.0.0/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"demo\",\n  \"version\": \"0.0.1\"\n}\n"))
		case "/download/demo/v1.0.0/index.js":
			_, _ = w.Write([]byte("module.exports = { message: 'v1.0.0' };\n"))
		case "/download/demo/v1.1.0/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"demo\",\n  \"version\": \"0.0.1\"\n}\n"))
		case "/download/demo/v1.1.0/index.js":
			_, _ = w.Write([]byte("module.exports = { message: 'v1.1.0' };\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/demo")
	if err != nil {
		t.Fatalf("first pkg install failed: %v\n%s", err, output)
	}

	latestTag.Store("v1.1.0")
	output, err = runCommand(workDir, env, "pkg", "install", "github.com/acme/demo")
	if err != nil {
		t.Fatalf("second explicit pkg install failed: %v\n%s", err, output)
	}

	message, err := runScript(workDir, env, "const pkg = require('github.com/acme/demo'); console.println(pkg.message);")
	if err != nil {
		t.Fatalf("require refreshed github package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "v1.1.0" {
		t.Fatalf("require output = %q, want v1.1.0", strings.TrimSpace(message))
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/demo"]; got != "#tag=v1.1.0" {
		t.Fatalf("saved dependency = %v, want #tag=v1.1.0", got)
	}
	lockJSON := readJSONFile(t, filepath.Join(workDir, "package-lock.json"))
	packages := lockJSON["packages"].(map[string]any)
	if got := packages["node_modules/github.com/acme/demo"].(map[string]any)["resolved"]; got != "github.com/acme/demo#tag=v1.1.0" {
		t.Fatalf("locked resolved source = %v, want github.com/acme/demo#tag=v1.1.0", got)
	}
}

func TestPkgInstallGitHubExposesPackageCommandInTargetProject(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/helloapp/tags":
			writeJSON(w, []map[string]any{})
		case "/api/repos/acme/helloapp":
			writeJSON(w, map[string]any{
				"default_branch": "main",
			})
		case "/api/repos/acme/helloapp/contents":
			if got := r.URL.Query().Get("ref"); got != "main" {
				t.Fatalf("unexpected ref query: %q", got)
			}
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/helloapp/main/package.json",
				},
				{
					"type":         "file",
					"path":         "hello.js",
					"download_url": server.URL + "/download/helloapp/main/hello.js",
				},
			})
		case "/download/helloapp/main/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"helloapp\",\n  \"version\": \"1.0.0\",\n  \"bin\": {\n    \"helloapp\": \"./hello.js\"\n  },\n  \"dependencies\": {}\n}\n"))
		case "/download/helloapp/main/hello.js":
			_, _ = w.Write([]byte("(() => { const process = require('process'); console.println(process.argv.slice(2).join('|')); })()\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "--dir", "public/hello", "github.com/acme/helloapp")
	if err != nil {
		t.Fatalf("pkg install github package failed: %v\n%s", err, output)
	}

	targetDir := filepath.Join(workDir, "public", "hello")
	manifest := readJSONFile(t, filepath.Join(targetDir, "package.json"))
	if got := manifest["name"]; got != "hello" {
		t.Fatalf("target package.json name = %v, want hello", got)
	}
	if got := manifest["dependencies"].(map[string]any)["github.com/acme/helloapp"]; got != "#branch=main" {
		t.Fatalf("target dependency = %v, want #branch=main", got)
	}
	if _, ok := manifest["pkg"]; ok {
		t.Fatalf("legacy pkg metadata should not be written: %#v", manifest["pkg"])
	}
	if _, err := os.Stat(filepath.Join(targetDir, "node_modules", ".bin", "helloapp.js")); err != nil {
		t.Fatalf("expected generated wrapper: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "node_modules", "github.com", "acme", "helloapp", "hello.js")); err != nil {
		t.Fatalf("expected installed package hello.js: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".pkg-tmp")); !os.IsNotExist(err) {
		t.Fatalf("install should clean temporary staging root, err=%v", err)
	}

	message, err := runScript(targetDir, env, "const process = require('process'); const exitCode = process.exec('./node_modules/.bin/helloapp.js', 'delta'); if (exitCode instanceof Error) throw exitCode; if (exitCode !== 0) process.exit(exitCode);")
	if err != nil {
		t.Fatalf("executing generated wrapper failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "delta" {
		t.Fatalf("wrapper output = %q, want delta", strings.TrimSpace(message))
	}

	lockJSON := readJSONFile(t, filepath.Join(targetDir, "package-lock.json"))
	packages := lockJSON["packages"].(map[string]any)
	rootPackage := packages[""].(map[string]any)
	if got := rootPackage["name"]; got != "hello" {
		t.Fatalf("lock root name = %v, want hello", got)
	}
	if _, ok := rootPackage["pkg"]; ok {
		t.Fatalf("lock root should not contain legacy pkg metadata: %#v", rootPackage["pkg"])
	}
	if got := packages["node_modules/github.com/acme/helloapp"].(map[string]any)["resolved"]; got != "github.com/acme/helloapp#branch=main" {
		t.Fatalf("locked resolved source = %v, want github.com/acme/helloapp#branch=main", got)
	}

	if err := os.RemoveAll(filepath.Join(targetDir, "node_modules")); err != nil {
		t.Fatalf("remove node_modules: %v", err)
	}
	reinstallOutput, err := runCommand(workDir, env, "pkg", "install", "--dir", "public/hello")
	if err != nil {
		t.Fatalf("pkg reinstall failed: %v\n%s", err, reinstallOutput)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "node_modules", ".bin", "helloapp.js")); err != nil {
		t.Fatalf("expected recreated wrapper: %v", err)
	}
}

func TestPkgInstallSupportsPackageBinAlias(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/helloapp/tags":
			writeJSON(w, []map[string]any{})
		case "/api/repos/acme/helloapp":
			writeJSON(w, map[string]any{"default_branch": "main"})
		case "/api/repos/acme/helloapp/contents":
			writeJSON(w, []map[string]any{
				{"type": "file", "path": "package.json", "download_url": server.URL + "/download/helloapp/main/package.json"},
				{"type": "file", "path": "hello.js", "download_url": server.URL + "/download/helloapp/main/hello.js"},
			})
		case "/download/helloapp/main/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"helloapp\",\n  \"version\": \"1.0.0\",\n  \"bin\": {\n    \"hello\": \"./hello.js\"\n  },\n  \"dependencies\": {}\n}\n"))
		case "/download/helloapp/main/hello.js":
			_, _ = w.Write([]byte("(() => { const process = require('process'); console.println(process.argv.slice(2).join('|')); })()\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{"PKG_GITHUB_API_URL": server.URL + "/api"}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/helloapp")
	if err != nil {
		t.Fatalf("pkg install failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(workDir, "node_modules", ".bin", "hello.js")); err != nil {
		t.Fatalf("expected alias wrapper: %v", err)
	}

	runOutput, err := runScript(workDir, env, "const process = require('process'); const exitCode = process.exec('./node_modules/.bin/hello.js', 'delta'); if (exitCode instanceof Error) throw exitCode; if (exitCode !== 0) process.exit(exitCode);")
	if err != nil {
		t.Fatalf("package bin alias execution failed: %v\n%s", err, runOutput)
	}
	if strings.TrimSpace(runOutput) != "delta" {
		t.Fatalf("package bin alias output = %q, want delta", strings.TrimSpace(runOutput))
	}
}

func TestPkgInstallWarnsAndSkipsConflictingBinAlias(t *testing.T) {
	workDir := t.TempDir()
	targetDir := filepath.Join(workDir, "public", "hello")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatalf("mkdir wrapper dir: %v", err)
	}
	writeJSONFile(t, filepath.Join(targetDir, "package.json"), map[string]any{"name": "hello", "version": "1.0.0", "scripts": map[string]any{}, "dependencies": map[string]any{}})
	if err := os.WriteFile(filepath.Join(targetDir, "node_modules", ".bin", "helloapp.js"), []byte("// neo-pkg-wrapper-owner:github.com/acme/otherapp\n"), 0o644); err != nil {
		t.Fatalf("write existing wrapper: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/helloapp/tags":
			writeJSON(w, []map[string]any{})
		case "/api/repos/acme/helloapp":
			writeJSON(w, map[string]any{
				"default_branch": "main",
			})
		case "/api/repos/acme/helloapp/contents":
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/helloapp/main/package.json",
				},
				{
					"type":         "file",
					"path":         "hello.js",
					"download_url": server.URL + "/download/helloapp/main/hello.js",
				},
			})
		case "/download/helloapp/main/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"helloapp\",\n  \"version\": \"1.0.0\",\n  \"bin\": {\n    \"helloapp\": \"./hello.js\"\n  },\n  \"dependencies\": {}\n}\n"))
		case "/download/helloapp/main/hello.js":
			_, _ = w.Write([]byte("(() => { const process = require('process'); console.println(process.argv.slice(2).join('|')); })()\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "--dir", "public/hello", "github.com/acme/helloapp")
	if err != nil {
		t.Fatalf("pkg install with conflicting bin alias failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Warning: package bin alias helloapp from github.com/acme/helloapp conflicts with github.com/acme/otherapp") {
		t.Fatalf("missing bin conflict warning: %q", output)
	}
	wrapperBytes, err := os.ReadFile(filepath.Join(targetDir, "node_modules", ".bin", "helloapp.js"))
	if err != nil {
		t.Fatalf("read existing wrapper: %v", err)
	}
	if !strings.Contains(string(wrapperBytes), "github.com/acme/otherapp") {
		t.Fatalf("existing wrapper should be preserved: %q", string(wrapperBytes))
	}
}

func TestPkgInstallWarnsAndSkipsUnmanagedBinAlias(t *testing.T) {
	workDir := t.TempDir()
	targetDir := filepath.Join(workDir, "public", "hello")
	if err := os.MkdirAll(filepath.Join(targetDir, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatalf("mkdir wrapper dir: %v", err)
	}
	writeJSONFile(t, filepath.Join(targetDir, "package.json"), map[string]any{"name": "hello", "version": "1.0.0", "scripts": map[string]any{}, "dependencies": map[string]any{}})
	if err := os.WriteFile(filepath.Join(targetDir, "node_modules", ".bin", "helloapp.js"), []byte("(() => { console.println('old'); })()\n"), 0o644); err != nil {
		t.Fatalf("write existing wrapper: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/helloapp/tags":
			writeJSON(w, []map[string]any{})
		case "/api/repos/acme/helloapp":
			writeJSON(w, map[string]any{
				"default_branch": "main",
			})
		case "/api/repos/acme/helloapp/contents":
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/helloapp/main/package.json",
				},
				{
					"type":         "file",
					"path":         "hello.js",
					"download_url": server.URL + "/download/helloapp/main/hello.js",
				},
			})
		case "/download/helloapp/main/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"helloapp\",\n  \"version\": \"1.0.0\",\n  \"bin\": {\n    \"helloapp\": \"./hello.js\"\n  },\n  \"dependencies\": {}\n}\n"))
		case "/download/helloapp/main/hello.js":
			_, _ = w.Write([]byte("(() => { const process = require('process'); console.println(process.argv.slice(2).join('|')); })()\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "--dir", "public/hello", "github.com/acme/helloapp")
	if err != nil {
		t.Fatalf("pkg install with unmanaged conflicting bin alias failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Warning: package bin alias helloapp from github.com/acme/helloapp conflicts with existing wrapper") {
		t.Fatalf("missing unmanaged conflict warning: %q", output)
	}

	wrapperBytes, err := os.ReadFile(filepath.Join(targetDir, "node_modules", ".bin", "helloapp.js"))
	if err != nil {
		t.Fatalf("read existing unmanaged wrapper: %v", err)
	}
	if strings.TrimSpace(string(wrapperBytes)) != "(() => { console.println('old'); })()" {
		t.Fatalf("existing unmanaged wrapper should be preserved: %q", string(wrapperBytes))
	}
}

func TestPkgInstallUsesTargetDirectoryOptionForNewProject(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/generic-pkg":
			writeJSON(w, map[string]any{
				"name":      "generic-pkg",
				"dist-tags": map[string]any{"latest": "1.2.0"},
				"versions": map[string]any{
					"1.2.0": map[string]any{
						"name":    "generic-pkg",
						"version": "1.2.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/generic-pkg-1.2.0.tgz"},
					},
				},
			})
		case "/tarballs/generic-pkg-1.2.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "generic-pkg",
				"version": "1.2.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'target-dir' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "--dir", "app", "generic-pkg")
	if err != nil {
		t.Fatalf("pkg install with --dir failed: %v\n%s", err, output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "app", "package.json"))
	if got := manifest["name"]; got != "app" {
		t.Fatalf("target package.json name = %v, want app", got)
	}
	if got := manifest["dependencies"].(map[string]any)["generic-pkg"]; got != "^1.2.0" {
		t.Fatalf("target dependency = %v, want ^1.2.0", got)
	}

	if _, err := os.Stat(filepath.Join(workDir, "app", "node_modules", "generic-pkg", "package.json")); err != nil {
		t.Fatalf("expected package inside target dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "node_modules", "generic-pkg", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("package should not be installed in caller cwd, err=%v", err)
	}

	message, err := runScript(workDir, env, strings.Join([]string{
		"const pkg = require('/work/app/node_modules/generic-pkg');",
		"console.println(pkg.message);",
	}, "\n"))
	if err != nil {
		t.Fatalf("require installed target-dir package failed: %v\n%s", err, message)
	}
	if strings.TrimSpace(message) != "target-dir" {
		t.Fatalf("require output = %q, want target-dir", strings.TrimSpace(message))
	}
}

func TestPkgInstallUsesTargetDirectoryOptionForExistingManifest(t *testing.T) {
	workDir := t.TempDir()
	targetDir := filepath.Join(workDir, "workspace", "client")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	writeJSONFile(t, filepath.Join(targetDir, "package.json"), map[string]any{
		"name":    "client-app",
		"version": "1.0.0",
		"dependencies": map[string]any{
			"range-pkg": "1.2 - 1.4",
		},
	})

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/range-pkg":
			writeJSON(w, map[string]any{
				"name":      "range-pkg",
				"dist-tags": map[string]any{"latest": "2.0.0"},
				"versions": map[string]any{
					"1.4.2": map[string]any{
						"name":         "range-pkg",
						"version":      "1.4.2",
						"dist":         map[string]any{"tarball": server.URL + "/tarballs/range-pkg-1.4.2.tgz"},
						"dependencies": map[string]any{"transitive-pkg": "~1.1"},
					},
				},
			})
		case "/npm/transitive-pkg":
			writeJSON(w, map[string]any{
				"name":      "transitive-pkg",
				"dist-tags": map[string]any{"latest": "1.1.4"},
				"versions": map[string]any{
					"1.1.4": map[string]any{
						"name":    "transitive-pkg",
						"version": "1.1.4",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/transitive-pkg-1.1.4.tgz"},
					},
				},
			})
		case "/tarballs/range-pkg-1.4.2.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":         "range-pkg",
				"version":      "1.4.2",
				"main":         "index.js",
				"dependencies": map[string]any{"transitive-pkg": "~1.1"},
			}, map[string]string{
				"index.js": "module.exports = { message: 'range-target' };\n",
			}))
		case "/tarballs/transitive-pkg-1.1.4.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "transitive-pkg",
				"version": "1.1.4",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'transitive-target' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "--dir", "workspace/client")
	if err != nil {
		t.Fatalf("pkg install existing target manifest failed: %v\n%s", err, output)
	}

	assertPackageVersion(t, filepath.Join(targetDir, "node_modules", "range-pkg", "package.json"), "1.4.2")
	assertPackageVersion(t, filepath.Join(targetDir, "node_modules", "transitive-pkg", "package.json"), "1.1.4")

	lockJSON := readJSONFile(t, filepath.Join(targetDir, "package-lock.json"))
	packages := lockJSON["packages"].(map[string]any)
	rootPackage := packages[""].(map[string]any)
	if got := rootPackage["name"]; got != "client-app" {
		t.Fatalf("lock root name = %v, want client-app", got)
	}
	if got := rootPackage["dependencies"].(map[string]any)["range-pkg"]; got != "1.2 - 1.4" {
		t.Fatalf("lock root dependency = %v, want 1.2 - 1.4", got)
	}

	if _, err := os.Stat(filepath.Join(workDir, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("caller cwd should not receive node_modules, err=%v", err)
	}
}

func TestPkgInstallSupportsShortTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/generic-pkg":
			writeJSON(w, map[string]any{
				"name":      "generic-pkg",
				"dist-tags": map[string]any{"latest": "1.0.0"},
				"versions": map[string]any{
					"1.0.0": map[string]any{
						"name":    "generic-pkg",
						"version": "1.0.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/generic-pkg-1.0.0.tgz"},
					},
				},
			})
		case "/tarballs/generic-pkg-1.0.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "generic-pkg",
				"version": "1.0.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'short-option' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "-C", "pkg-root", "generic-pkg")
	if err != nil {
		t.Fatalf("pkg install with -C failed: %v\n%s", err, output)
	}

	assertPackageVersion(t, filepath.Join(workDir, "pkg-root", "node_modules", "generic-pkg", "package.json"), "1.0.0")
}

func TestPkgInstallGlobalIgnoresTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/generic-pkg":
			writeJSON(w, map[string]any{
				"name":      "generic-pkg",
				"dist-tags": map[string]any{"latest": "1.3.0"},
				"versions": map[string]any{
					"1.3.0": map[string]any{
						"name":    "generic-pkg",
						"version": "1.3.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/generic-pkg-1.3.0.tgz"},
					},
				},
			})
		case "/tarballs/generic-pkg-1.3.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "generic-pkg",
				"version": "1.3.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'global-install' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "-g", "--dir", "app", "generic-pkg")
	if err != nil {
		t.Fatalf("pkg install -g failed: %v\n%s", err, output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "node_modules", ".pkg", "manifest.json"))
	if got := manifest["name"]; got != "global" {
		t.Fatalf("global manifest name = %v, want global", got)
	}
	if got := manifest["dependencies"].(map[string]any)["generic-pkg"]; got != "^1.3.0" {
		t.Fatalf("global dependency = %v, want ^1.3.0", got)
	}
	if _, err := os.Stat(filepath.Join(workDir, "node_modules", ".pkg", "lock.json")); err != nil {
		t.Fatalf("global lock metadata should exist: %v", err)
	}

	assertPackageVersion(t, filepath.Join(workDir, "node_modules", "generic-pkg", "package.json"), "1.3.0")
	if _, err := os.Stat(filepath.Join(workDir, "app", "node_modules", "generic-pkg", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("package should not be installed in --dir target during global install, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); !os.IsNotExist(err) {
		t.Fatalf("global install should not create /work/package.json, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package-lock.json")); !os.IsNotExist(err) {
		t.Fatalf("global install should not create /work/package-lock.json, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "app", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("global install should ignore --dir manifest location, err=%v", err)
	}
}

func TestPkgInstallRejectsFileTargetPath(t *testing.T) {
	workDir := t.TempDir()
	targetFile := filepath.Join(workDir, "target-file")
	if err := os.WriteFile(targetFile, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	output, err := runCommand(workDir, nil, "pkg", "install", "--dir", "target-file", "generic-pkg")
	if err == nil {
		t.Fatalf("expected file target path to fail, output=%q", output)
	}
	if !strings.Contains(err.Error(), "Install target is not a directory") {
		t.Fatalf("unexpected error: %v\n%s", err, output)
	}
}

func TestPkgInstallHelpIncludesTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "install", "--help")
	if err != nil && !strings.Contains(output, "pkg install [options] [name]") {
		t.Fatalf("pkg install --help failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "-C, --dir") {
		t.Fatalf("help output missing short/long dir option: %q", output)
	}
	if !strings.Contains(output, "-g, --[no-]global") {
		t.Fatalf("help output missing short/long global option: %q", output)
	}
	if strings.Contains(output, "--as") {
		t.Fatalf("help output should not mention alias option: %q", output)
	}
	if strings.Contains(output, "--[no-]force") || strings.Contains(output, "--force") {
		t.Fatalf("help output should not mention force option: %q", output)
	}
	if !strings.Contains(output, "ignore --dir") {
		t.Fatalf("help output missing global option description: %q", output)
	}
	if !strings.Contains(output, "Use this project directory") {
		t.Fatalf("help output missing dir description: %q", output)
	}
}

func TestPkgUninstallHelpIncludesTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "pkg", "uninstall", "--help")
	if err != nil && !strings.Contains(output, "pkg uninstall [options] <name>") {
		t.Fatalf("pkg uninstall --help failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "-C, --dir") {
		t.Fatalf("help output missing short/long dir option: %q", output)
	}
	if !strings.Contains(output, "-g, --[no-]global") {
		t.Fatalf("help output missing short/long global option: %q", output)
	}
	if !strings.Contains(output, "ignore --dir") {
		t.Fatalf("help output missing global option description: %q", output)
	}
	if !strings.Contains(output, "Use this project directory") {
		t.Fatalf("help output missing dir description: %q", output)
	}
}

func TestPkgUninstallRemovesPackageCommandAndDependency(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/helloapp/tags":
			writeJSON(w, []map[string]any{})
		case "/api/repos/acme/helloapp":
			writeJSON(w, map[string]any{
				"default_branch": "main",
			})
		case "/api/repos/acme/helloapp/contents":
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/helloapp/main/package.json",
				},
				{
					"type":         "file",
					"path":         "hello.js",
					"download_url": server.URL + "/download/helloapp/main/hello.js",
				},
			})
		case "/download/helloapp/main/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"helloapp\",\n  \"version\": \"1.0.0\",\n  \"bin\": {\n    \"helloapp\": \"./hello.js\"\n  },\n  \"dependencies\": {}\n}\n"))
		case "/download/helloapp/main/hello.js":
			_, _ = w.Write([]byte("(() => { const process = require('process'); console.println(process.argv.slice(2).join('|')); })()\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	_, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/helloapp")
	if err != nil {
		t.Fatalf("pkg install before uninstall failed: %v", err)
	}

	output, err := runCommand(workDir, env, "pkg", "uninstall", "github.com/acme/helloapp")
	if err != nil {
		t.Fatalf("pkg uninstall failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Removed github.com/acme/helloapp") {
		t.Fatalf("unexpected uninstall output: %q", output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	if _, ok := manifest["dependencies"].(map[string]any)["github.com/acme/helloapp"]; ok {
		t.Fatalf("dependency should be removed: %#v", manifest["dependencies"])
	}
	if _, ok := manifest["pkg"]; ok {
		t.Fatalf("legacy pkg metadata should not remain: %#v", manifest["pkg"])
	}
	if _, err := os.Stat(filepath.Join(workDir, "node_modules", ".bin", "helloapp.js")); !os.IsNotExist(err) {
		t.Fatalf("wrapper should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "node_modules", "github.com", "acme", "helloapp")); !os.IsNotExist(err) {
		t.Fatalf("package directory should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package-lock.json")); !os.IsNotExist(err) {
		t.Fatalf("lockfile should be removed when no dependencies remain, err=%v", err)
	}
}

func TestPkgUninstallGlobalIgnoresTargetDirectoryOption(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/generic-pkg":
			writeJSON(w, map[string]any{
				"name":      "generic-pkg",
				"dist-tags": map[string]any{"latest": "1.3.0"},
				"versions": map[string]any{
					"1.3.0": map[string]any{
						"name":    "generic-pkg",
						"version": "1.3.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/generic-pkg-1.3.0.tgz"},
					},
				},
			})
		case "/tarballs/generic-pkg-1.3.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "generic-pkg",
				"version": "1.3.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'global-install' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "-g", "generic-pkg")
	if err != nil {
		t.Fatalf("pkg install -g before uninstall failed: %v\n%s", err, output)
	}

	output, err = runCommand(workDir, env, "pkg", "uninstall", "-g", "--dir", "app", "generic-pkg")
	if err != nil {
		t.Fatalf("pkg uninstall -g failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Removed generic-pkg") {
		t.Fatalf("unexpected uninstall output: %q", output)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "node_modules", ".pkg", "manifest.json"))
	if deps, ok := manifest["dependencies"].(map[string]any); ok {
		if _, exists := deps["generic-pkg"]; exists {
			t.Fatalf("global dependency should be removed: %#v", deps)
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, "node_modules", "generic-pkg", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("global package should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); !os.IsNotExist(err) {
		t.Fatalf("global uninstall should not create /work/package.json, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "package-lock.json")); !os.IsNotExist(err) {
		t.Fatalf("global uninstall should not create /work/package-lock.json, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "app", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("pkg uninstall -g should ignore --dir manifest location, err=%v", err)
	}
}

func TestPkgInstallNpmWithRecursiveDependencies(t *testing.T) {
	workDir := t.TempDir()
	writeJSONFile(t, filepath.Join(workDir, "package.json"), map[string]any{
		"name":         "root-app",
		"version":      "1.0.0",
		"dependencies": map[string]any{},
	})
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/generic-pkg":
			writeJSON(w, map[string]any{
				"name":      "generic-pkg",
				"dist-tags": map[string]any{"latest": "1.2.0"},
				"versions": map[string]any{
					"1.2.0": map[string]any{
						"name":         "generic-pkg",
						"version":      "1.2.0",
						"dist":         map[string]any{"tarball": server.URL + "/tarballs/generic-pkg-1.2.0.tgz"},
						"dependencies": map[string]any{"dep-one": "^1.0.0"},
					},
				},
			})
		case "/npm/dep-one":
			writeJSON(w, map[string]any{
				"name":      "dep-one",
				"dist-tags": map[string]any{"latest": "1.1.0"},
				"versions": map[string]any{
					"1.0.0": map[string]any{
						"name":    "dep-one",
						"version": "1.0.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/dep-one-1.0.0.tgz"},
					},
					"1.1.0": map[string]any{
						"name":         "dep-one",
						"version":      "1.1.0",
						"dist":         map[string]any{"tarball": server.URL + "/tarballs/dep-one-1.1.0.tgz"},
						"dependencies": map[string]any{"sub-dep": "1.0.0"},
					},
				},
			})
		case "/npm/sub-dep":
			writeJSON(w, map[string]any{
				"name":      "sub-dep",
				"dist-tags": map[string]any{"latest": "1.0.0"},
				"versions": map[string]any{
					"1.0.0": map[string]any{
						"name":    "sub-dep",
						"version": "1.0.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/sub-dep-1.0.0.tgz"},
					},
				},
			})
		case "/tarballs/generic-pkg-1.2.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":         "generic-pkg",
				"version":      "1.2.0",
				"main":         "index.js",
				"dependencies": map[string]any{"dep-one": "^1.0.0"},
			}, map[string]string{
				"index.js": "module.exports = { message: 'generic-pkg' };\n",
			}))
		case "/tarballs/dep-one-1.0.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "dep-one",
				"version": "1.0.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'dep-one-1.0.0' };\n",
			}))
		case "/tarballs/dep-one-1.1.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":         "dep-one",
				"version":      "1.1.0",
				"main":         "index.js",
				"dependencies": map[string]any{"sub-dep": "1.0.0"},
			}, map[string]string{
				"index.js": "module.exports = { message: 'dep-one-1.1.0' };\n",
			}))
		case "/tarballs/sub-dep-1.0.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "sub-dep",
				"version": "1.0.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'sub-dep-1.0.0' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "generic-pkg")
	if err != nil {
		t.Fatalf("pkg install generic-pkg failed: %v\n%s", err, output)
	}

	paths := []string{
		filepath.Join(workDir, "node_modules", "generic-pkg", "package.json"),
		filepath.Join(workDir, "node_modules", "dep-one", "package.json"),
		filepath.Join(workDir, "node_modules", "sub-dep", "package.json"),
	}
	for _, filePath := range paths {
		if _, err := os.Stat(filePath); err != nil {
			t.Fatalf("expected installed file missing %s: %v", filePath, err)
		}
	}

	message, err := runScript(workDir, env, strings.Join([]string{
		"const generic = require('generic-pkg');",
		"const dep = require('dep-one');",
		"const sub = require('sub-dep');",
		"console.println(generic.message);",
		"console.println(dep.message);",
		"console.println(sub.message);",
	}, "\n"))
	if err != nil {
		t.Fatalf("require installed npm packages failed: %v\n%s", err, message)
	}

	lines := strings.Split(strings.TrimSpace(message), "\n")
	want := []string{"generic-pkg", "dep-one-1.1.0", "sub-dep-1.0.0"}
	if len(lines) != len(want) {
		t.Fatalf("unexpected output lines: %q", message)
	}
	for i := range want {
		if strings.TrimSpace(lines[i]) != want[i] {
			t.Fatalf("line %d = %q, want %q", i, strings.TrimSpace(lines[i]), want[i])
		}
	}

	secondOutput, err := runCommand(workDir, env, "pkg", "install", "generic-pkg")
	if err != nil {
		t.Fatalf("second pkg install failed: %v\n%s", err, secondOutput)
	}
	if !strings.Contains(secondOutput, "Up to date: generic-pkg@1.2.0") {
		t.Fatalf("expected up-to-date message, got %q", secondOutput)
	}

	manifest := readJSONFile(t, filepath.Join(workDir, "package.json"))
	manifestDeps := manifest["dependencies"].(map[string]any)
	if got := manifestDeps["generic-pkg"]; got != "^1.2.0" {
		t.Fatalf("expected saved dependency ^1.2.0, got %v", got)
	}

	lockFile := readJSONFile(t, filepath.Join(workDir, "package-lock.json"))
	if got := lockFile["lockfileVersion"]; got != float64(2) {
		t.Fatalf("expected lockfileVersion 2, got %v", got)
	}
	deps := lockFile["dependencies"].(map[string]any)
	packages := lockFile["packages"].(map[string]any)
	if deps["generic-pkg"].(map[string]any)["version"] != "1.2.0" {
		t.Fatalf("generic-pkg lock version mismatch: %#v", deps["generic-pkg"])
	}
	if packages["node_modules/dep-one"].(map[string]any)["version"] != "1.1.0" {
		t.Fatalf("dep-one package entry mismatch: %#v", packages["node_modules/dep-one"])
	}
	if packages["node_modules/sub-dep"].(map[string]any)["version"] != "1.0.0" {
		t.Fatalf("sub-dep package entry mismatch: %#v", packages["node_modules/sub-dep"])
	}
	if got := deps["generic-pkg"].(map[string]any)["requires"].(map[string]any)["dep-one"]; got != "^1.0.0" {
		t.Fatalf("generic-pkg requires mismatch: %v", got)
	}
	if got := deps["generic-pkg"].(map[string]any)["dependencies"].(map[string]any)["dep-one"].(map[string]any)["version"]; got != "1.1.0" {
		t.Fatalf("generic-pkg nested dep-one mismatch: %v", got)
	}
	if got := deps["generic-pkg"].(map[string]any)["dependencies"].(map[string]any)["dep-one"].(map[string]any)["dependencies"].(map[string]any)["sub-dep"].(map[string]any)["version"]; got != "1.0.0" {
		t.Fatalf("generic-pkg nested sub-dep mismatch: %v", got)
	}
}

func TestPkgInstallSupportsSemverRangesAndLockFile(t *testing.T) {
	workDir := t.TempDir()
	writeJSONFile(t, filepath.Join(workDir, "package.json"), map[string]any{
		"name":    "lock-app",
		"version": "1.0.0",
		"dependencies": map[string]any{
			"range-pkg": "1.2 - 1.4",
		},
	})

	var metadataHits atomic.Int64
	var lockMode atomic.Bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/npm/range-pkg":
			metadataHits.Add(1)
			if lockMode.Load() {
				http.Error(w, "metadata should not be requested while using lock file", http.StatusGone)
				return
			}
			writeJSON(w, map[string]any{
				"name":      "range-pkg",
				"dist-tags": map[string]any{"latest": "2.0.0"},
				"versions": map[string]any{
					"1.2.0": map[string]any{
						"name":    "range-pkg",
						"version": "1.2.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/range-pkg-1.2.0.tgz"},
					},
					"1.4.2": map[string]any{
						"name":         "range-pkg",
						"version":      "1.4.2",
						"dist":         map[string]any{"tarball": server.URL + "/tarballs/range-pkg-1.4.2.tgz"},
						"dependencies": map[string]any{"transitive-pkg": "~1.1"},
					},
					"2.0.0": map[string]any{
						"name":    "range-pkg",
						"version": "2.0.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/range-pkg-2.0.0.tgz"},
					},
				},
			})
		case "/npm/transitive-pkg":
			metadataHits.Add(1)
			if lockMode.Load() {
				http.Error(w, "metadata should not be requested while using lock file", http.StatusGone)
				return
			}
			writeJSON(w, map[string]any{
				"name":      "transitive-pkg",
				"dist-tags": map[string]any{"latest": "1.2.0"},
				"versions": map[string]any{
					"1.0.0": map[string]any{
						"name":    "transitive-pkg",
						"version": "1.0.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/transitive-pkg-1.0.0.tgz"},
					},
					"1.1.4": map[string]any{
						"name":    "transitive-pkg",
						"version": "1.1.4",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/transitive-pkg-1.1.4.tgz"},
					},
					"1.2.0": map[string]any{
						"name":    "transitive-pkg",
						"version": "1.2.0",
						"dist":    map[string]any{"tarball": server.URL + "/tarballs/transitive-pkg-1.2.0.tgz"},
					},
				},
			})
		case "/tarballs/range-pkg-1.2.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "range-pkg",
				"version": "1.2.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'range-pkg-1.2.0' };\n",
			}))
		case "/tarballs/range-pkg-1.4.2.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":         "range-pkg",
				"version":      "1.4.2",
				"main":         "index.js",
				"dependencies": map[string]any{"transitive-pkg": "~1.1"},
			}, map[string]string{
				"index.js": "module.exports = { message: 'range-pkg-1.4.2' };\n",
			}))
		case "/tarballs/range-pkg-2.0.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "range-pkg",
				"version": "2.0.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'range-pkg-2.0.0' };\n",
			}))
		case "/tarballs/transitive-pkg-1.0.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "transitive-pkg",
				"version": "1.0.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'transitive-pkg-1.0.0' };\n",
			}))
		case "/tarballs/transitive-pkg-1.1.4.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "transitive-pkg",
				"version": "1.1.4",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'transitive-pkg-1.1.4' };\n",
			}))
		case "/tarballs/transitive-pkg-1.2.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(makeTgzPackage(t, map[string]any{
				"name":    "transitive-pkg",
				"version": "1.2.0",
				"main":    "index.js",
			}, map[string]string{
				"index.js": "module.exports = { message: 'transitive-pkg-1.2.0' };\n",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_NPM_REGISTRY_URL": server.URL + "/npm",
	}

	output, err := runCommand(workDir, env, "pkg", "install")
	if err != nil {
		t.Fatalf("pkg install from package.json failed: %v\n%s", err, output)
	}

	assertPackageVersion(t, filepath.Join(workDir, "node_modules", "range-pkg", "package.json"), "1.4.2")
	assertPackageVersion(t, filepath.Join(workDir, "node_modules", "transitive-pkg", "package.json"), "1.1.4")

	lockJSON := readJSONFile(t, filepath.Join(workDir, "package-lock.json"))
	if got := lockJSON["lockfileVersion"]; got != float64(2) {
		t.Fatalf("expected lockfileVersion 2, got %v", got)
	}
	lockPackages := lockJSON["packages"].(map[string]any)
	rootPackage := lockPackages[""].(map[string]any)
	if got := rootPackage["dependencies"].(map[string]any)["range-pkg"]; got != "1.2 - 1.4" {
		t.Fatalf("root dependency range mismatch: %v", got)
	}
	lockDeps := lockJSON["dependencies"].(map[string]any)
	if got := lockDeps["range-pkg"].(map[string]any)["version"]; got != "1.4.2" {
		t.Fatalf("locked range-pkg version mismatch: %v", got)
	}
	if got := lockPackages["node_modules/transitive-pkg"].(map[string]any)["version"]; got != "1.1.4" {
		t.Fatalf("transitive package entry mismatch: %v", got)
	}
	if got := lockDeps["range-pkg"].(map[string]any)["requires"].(map[string]any)["transitive-pkg"]; got != "~1.1" {
		t.Fatalf("range-pkg requires mismatch: %v", got)
	}
	if got := lockDeps["range-pkg"].(map[string]any)["dependencies"].(map[string]any)["transitive-pkg"].(map[string]any)["version"]; got != "1.1.4" {
		t.Fatalf("range-pkg nested transitive version mismatch: %v", got)
	}

	hitsAfterFirstInstall := metadataHits.Load()
	if hitsAfterFirstInstall < 2 {
		t.Fatalf("expected metadata lookups during initial install, got %d", hitsAfterFirstInstall)
	}

	if err := os.RemoveAll(filepath.Join(workDir, "node_modules")); err != nil {
		t.Fatalf("remove node_modules: %v", err)
	}
	lockMode.Store(true)

	secondOutput, err := runCommand(workDir, env, "pkg", "install")
	if err != nil {
		t.Fatalf("pkg install using lock file failed: %v\n%s", err, secondOutput)
	}
	if metadataHits.Load() != hitsAfterFirstInstall {
		t.Fatalf("lock-file install should not fetch metadata again: before=%d after=%d", hitsAfterFirstInstall, metadataHits.Load())
	}

	assertPackageVersion(t, filepath.Join(workDir, "node_modules", "range-pkg", "package.json"), "1.4.2")
	assertPackageVersion(t, filepath.Join(workDir, "node_modules", "transitive-pkg", "package.json"), "1.1.4")
}

func runCommand(workDir string, extraEnv map[string]any, args ...string) (string, error) {
	conf := commandConfig(workDir, extraEnv)
	conf.Args = args
	jr, err := engine.New(conf)
	if err != nil {
		return "", err
	}
	lib.Enable(jr)
	err = jr.Run()
	return conf.Writer.(*bytes.Buffer).String(), err
}

func runScript(workDir string, extraEnv map[string]any, code string) (string, error) {
	conf := commandConfig(workDir, extraEnv)
	conf.Name = "pkg-script"
	conf.Code = code
	conf.Args = nil
	jr, err := engine.New(conf)
	if err != nil {
		return "", err
	}
	lib.Enable(jr)
	err = jr.Run()
	return conf.Writer.(*bytes.Buffer).String(), err
}

func commandConfig(workDir string, extraEnv map[string]any) engine.Config {
	env := map[string]any{
		"PATH":         "/sbin:/work",
		"PWD":          "/work",
		"HOME":         "/work",
		"LIBRARY_PATH": "./node_modules:/lib",
	}
	for k, v := range extraEnv {
		env[k] = v
	}

	conf := engine.Config{
		Name: "pkg-test",
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			{MountPoint: "/work", Source: workDir},
			{MountPoint: "/tmp", Source: workDir},
			{MountPoint: "/lib", FS: lib.LibFS()},
		},
		Env:    env,
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	}
	conf.ExecBuilder = func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
		execConf := engine.Config{
			Code:   code,
			Args:   args,
			FSTabs: conf.FSTabs,
			Env:    env,
		}
		secretBox, err := engine.NewSecretBox(execConf)
		if err != nil {
			return nil, err
		}
		execArgs := []string{"-S", secretBox.FilePath(), args[0]}
		return exec.Command(pkgTestJshBinPath, execArgs...), nil
	}

	return conf
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONFile(t *testing.T, filePath string, value any) {
	t.Helper()
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json file %s: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, append(bytes, '\n'), 0o644); err != nil {
		t.Fatalf("write json file %s: %v", filePath, err)
	}
}

func readJSONFile(t *testing.T, filePath string) map[string]any {
	t.Helper()
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read json file %s: %v", filePath, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("parse json file %s: %v", filePath, err)
	}
	return parsed
}

func copyTestFile(t *testing.T, sourcePath string, targetPath string) {
	t.Helper()
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read test file %s: %v", sourcePath, err)
	}
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		t.Fatalf("write test file %s: %v", targetPath, err)
	}
}

func assertPackageVersion(t *testing.T, packageJSON string, expected string) {
	t.Helper()
	manifest := readJSONFile(t, packageJSON)
	if got := manifest["version"]; got != expected {
		t.Fatalf("package version mismatch for %s: got %v want %s", packageJSON, got, expected)
	}
}

func makeZipPackage(t *testing.T, rootDir string, manifest map[string]any, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal zip manifest: %v", err)
	}

	entries := map[string][]byte{
		filepath.ToSlash(filepath.Join(rootDir, "package.json")): append(manifestBytes, '\n'),
	}
	for name, content := range files {
		entries[filepath.ToSlash(filepath.Join(rootDir, name))] = []byte(content)
	}

	for name, content := range entries {
		fh := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fh.SetModTime(time.Unix(1700000000, 0))
		fileWriter, err := writer.CreateHeader(fh)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := fileWriter.Write(content); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func makeZipPackageAtRoot(t *testing.T, manifest map[string]any, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal root zip manifest: %v", err)
	}

	entries := map[string][]byte{
		"package.json": append(manifestBytes, '\n'),
	}
	for name, content := range files {
		entries[filepath.ToSlash(name)] = []byte(content)
	}

	for name, content := range entries {
		fh := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fh.SetModTime(time.Unix(1700000000, 0))
		fileWriter, err := writer.CreateHeader(fh)
		if err != nil {
			t.Fatalf("create root zip entry %s: %v", name, err)
		}
		if _, err := fileWriter.Write(content); err != nil {
			t.Fatalf("write root zip entry %s: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close root zip writer: %v", err)
	}
	return buf.Bytes()
}

func makeTgzPackage(t *testing.T, manifest map[string]any, files map[string]string) []byte {
	t.Helper()
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal tgz manifest: %v", err)
	}

	entries := map[string][]byte{
		"package/package.json": append(manifestBytes, '\n'),
	}
	for name, content := range files {
		entries[filepath.ToSlash(filepath.Join("package", name))] = []byte(content)
	}

	for name, content := range entries {
		header := &tar.Header{
			Name:    name,
			Mode:    0o644,
			Size:    int64(len(content)),
			ModTime: time.Unix(1700000000, 0),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("write tar entry %s: %v", name, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return compressed.Bytes()
}

func TestPkgInstallRejectsUnsafeGitHubEntries(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/unsafe/tags":
			writeJSON(w, []map[string]any{{"name": "v1.0.0"}})
		case "/api/repos/acme/unsafe/contents":
			writeJSON(w, []map[string]any{
				{
					"type":         "file",
					"path":         "../escape.txt",
					"download_url": server.URL + "/download/unsafe/escape.txt",
				},
				{
					"type":         "file",
					"path":         "package.json",
					"download_url": server.URL + "/download/unsafe/package.json",
				},
			})
		case "/download/unsafe/escape.txt":
			_, _ = w.Write([]byte("bad"))
		case "/download/unsafe/package.json":
			_, _ = w.Write([]byte("{\n  \"name\": \"unsafe\",\n  \"version\": \"1.0.0\"\n}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/unsafe")
	if err == nil {
		t.Fatalf("expected unsafe GitHub install to fail, output=%q", output)
	}
	if !strings.Contains(err.Error(), "Unsafe GitHub path") {
		t.Fatalf("unexpected error: %v\n%s", err, output)
	}
}

func TestPkgInstallReportsCombinedGitHubRefResolutionFailure(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/broken/tags":
			http.Error(w, "tags unavailable", http.StatusBadGateway)
		case "/api/repos/acme/broken":
			http.Error(w, "repo unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/broken")
	if err == nil {
		t.Fatalf("expected GitHub ref resolution failure, output=%q", output)
	}
	if !strings.Contains(err.Error(), "Unable to resolve GitHub ref for github.com/acme/broken") {
		t.Fatalf("unexpected error header: %v\n%s", err, output)
	}
	if !strings.Contains(err.Error(), "tags lookup failed") {
		t.Fatalf("missing tags failure detail: %v\n%s", err, output)
	}
	if !strings.Contains(err.Error(), "default branch lookup failed") {
		t.Fatalf("missing default branch failure detail: %v\n%s", err, output)
	}
}

func TestPkgInstallHintsRepoNameOrVisibilityOnGitHub404(t *testing.T) {
	workDir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/acme/private-repo/tags":
			http.NotFound(w, r)
		case "/api/repos/acme/private-repo":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env := map[string]any{
		"PKG_GITHUB_API_URL": server.URL + "/api",
	}

	output, err := runCommand(workDir, env, "pkg", "install", "github.com/acme/private-repo")
	if err == nil {
		t.Fatalf("expected GitHub 404 install failure, output=%q", output)
	}
	if !strings.Contains(err.Error(), "Check that the repository name is correct and that the repository is public.") {
		t.Fatalf("missing repository visibility hint: %v\n%s", err, output)
	}
}
