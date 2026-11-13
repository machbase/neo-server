package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGetVersionReachableTags(t *testing.T) {
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	signature := &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(1000, 0)}
	commit := func(message string, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		signature.When = signature.When.Add(time.Hour)
		hash, err := worktree.Commit(message, &git.CommitOptions{
			Author: signature, Parents: parents, AllowEmptyCommits: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	tag := func(name string, hash plumbing.Hash, annotated bool) {
		t.Helper()
		var options *git.CreateTagOptions
		if annotated {
			options = &git.CreateTagOptions{Tagger: signature, Message: name}
		}
		if _, err := repo.CreateTag(name, hash, options); err != nil {
			t.Fatal(err)
		}
	}
	base := commit("base")
	tag("v8.5.10", base, true)
	mainRelease := commit("main release", base)
	tag("v8.7.3", mainRelease, true)
	mainHead := commit("main development", mainRelease)
	maintenance := commit("maintenance release", base)
	tag("v8.5.13", maintenance, true)
	merge := commit("merge", mainHead, maintenance)
	equalDistanceMerge := commit("equal distance merge", mainRelease, maintenance)
	rc := commit("release candidate", mainHead)
	tag("v8.9.0-rc1", rc, false)
	rcHead := commit("after release candidate", rc)
	tag("v-not-semver", rcHead, false)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	check := func(hash plumbing.Hash, wantTag, wantVersion string, nightly bool) {
		t.Helper()
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, hash)); err != nil {
			t.Fatal(err)
		}
		if err := GetVersion(); err != nil {
			t.Fatal(err)
		}
		if vLastVersion != wantTag || vBuildVersion != wantVersion || vIsNightly != nightly || vLastCommit != hash.String() {
			t.Fatalf("got tag=%s version=%s nightly=%t commit=%s; want tag=%s version=%s nightly=%t commit=%s",
				vLastVersion, vBuildVersion, vIsNightly, vLastCommit, wantTag, wantVersion, nightly, hash)
		}
	}
	check(mainHead, "v8.7.3", "v8.7.4-snapshot", true)
	check(maintenance, "v8.5.13", "v8.5.13", false)
	check(merge, "v8.5.13", "v8.5.14-snapshot", true)
	check(equalDistanceMerge, "v8.7.3", "v8.7.4-snapshot", true)
	check(rc, "v8.9.0-rc1", "v8.9.0-rc1", false)
	check(rcHead, "v8.9.0-rc1", "v8.9.0-rc2-snapshot", true)
	tag("v8.9.0", rc, true)
	check(rc, "v8.9.0", "v8.9.0", false)
}

func TestGetVersionWithoutReachableTag(t *testing.T) {
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	signature := &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(1000, 0)}
	first, err := worktree.Commit("first", &git.CommitOptions{Author: signature, AllowEmptyCommits: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag("v1.0.0", first, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("orphan"))); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("orphan", &git.CommitOptions{Author: signature, AllowEmptyCommits: true}); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := GetVersion(); err == nil || !strings.Contains(err.Error(), "no version tag reachable") {
		t.Fatalf("GetVersion() error = %v, want no reachable version tag", err)
	}
}

func TestGetVersionWithoutGit(t *testing.T) {
	root := t.TempDir()
	lastVersion, lastCommit, nightly, buildVersion := vLastVersion, vLastCommit, vIsNightly, vBuildVersion
	t.Cleanup(func() {
		vLastVersion, vLastCommit, vIsNightly, vBuildVersion = lastVersion, lastCommit, nightly, buildVersion
	})
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	vLastVersion = "v8.7.3"
	vLastCommit = "1234567890"
	vIsNightly = false
	vBuildVersion = "v8.7.3"
	if err := GetVersion(); err != nil {
		t.Fatal(err)
	}
	if vLastVersion != "" || vLastCommit != "unknown" || !vIsNightly || vBuildVersion != "v0.0.0-snapshot.nogit" {
		t.Fatalf("unexpected no-Git version state: tag=%q commit=%q nightly=%t version=%q",
			vLastVersion, vLastCommit, vIsNightly, vBuildVersion)
	}
	if got := shortGitSHA(); got != "unknown" {
		t.Fatalf("shortGitSHA() = %q, want unknown", got)
	}
	vLastCommit = "1234567890"
	if got := shortGitSHA(); got != "12345678" {
		t.Fatalf("shortGitSHA() = %q, want 12345678", got)
	}

	requireWriteFile(t, ".git", []byte("invalid git file"), 0644)
	if err := GetVersion(); err == nil {
		t.Fatal("GetVersion() accepted an invalid .git file")
	}
	requireWriteFile(t, ".git", []byte("gitdir: missing\n"), 0644)
	if err := GetVersion(); err == nil {
		t.Fatal("GetVersion() accepted a missing Git directory")
	}
	if err := os.Remove(".git"); err != nil {
		t.Fatal(err)
	}
	requireMkdirAll(t, ".git")
	if err := GetVersion(); err == nil {
		t.Fatal("GetVersion() accepted an incomplete .git directory")
	}
}

func TestMageTestInstallsEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script as the go command")
	}
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	t.Setenv("GOOS", runtime.GOOS)
	t.Setenv("GOARCH", runtime.GOARCH)

	a := engineArtifact(neo_engine_version, runtime.GOOS, runtime.GOARCH)
	cache := filepath.Join("tmp", "release-cache", strings.TrimSuffix(a.name, ".zip"), a.version, a.name)
	requireMkdirAll(t, filepath.Dir(cache))
	requireMkdirAll(t, a.destination)
	requireMkdirAll(t, "bin")
	requireWriteFile(t, cache, testZip(t, map[string]string{a.required: "expected engine"}), 0644)
	script := fmt.Sprintf("#!/bin/sh\ngrep -qx 'expected engine' '%s' || exit 1\nprintf '%%s\\n' \"$*\" >> calls\n", filepath.Join(a.destination, a.required))
	requireWriteFile(t, "bin/go", []byte(script), 0755)
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, test := range []struct {
		name string
		run  func() error
	}{
		{"test", Test},
		{"testN", func() error { return TestN(2) }},
		{"testAll", TestAll},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireWriteFile(t, filepath.Join(a.destination, a.required), []byte("wrong engine"), 0644)
			if err := test.run(); err != nil {
				t.Fatal(err)
			}
			assertFileContent(t, filepath.Join(a.destination, a.required), "expected engine")
		})
	}
	t.Run("installation error stops tests", func(t *testing.T) {
		calls, err := os.ReadFile("calls")
		if err != nil {
			t.Fatal(err)
		}
		previousVersion := neo_engine_version
		neo_engine_version = "../invalid"
		t.Cleanup(func() { neo_engine_version = previousVersion })
		if err := Test(); err == nil {
			t.Fatal("Test succeeded with an invalid engine version")
		}
		assertFileContent(t, "calls", string(calls))
	})
}

func TestReleaseArtifactCache(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	requireMkdirAll(t, "ui")

	version := "one"
	requests := 0
	payload := map[string][]byte{
		"one": testZip(t, map[string]string{"index.html": "one", "assets/app.js": "first"}),
		"two": testZip(t, map[string]string{"index.html": "two", "assets/app.js": "second"}),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(payload[version])
	}))
	defer server.Close()
	artifact := releaseArtifact{version: version, name: "web-ui.zip", url: server.URL, destination: "ui", required: "index.html", web: true}

	install := func(force bool, wantRequests int, want string) {
		t.Helper()
		if err := installArtifact(artifact, force); err != nil {
			t.Fatal(err)
		}
		if requests != wantRequests {
			t.Fatalf("requests = %d, want %d", requests, wantRequests)
		}
		assertFileContent(t, "ui/index.html", want)
	}
	install(false, 1, "one")
	install(false, 1, "one")
	version, artifact.version = "two", "two"
	install(false, 2, "two")
	version, artifact.version = "one", "one"
	install(false, 2, "one")
	requireWriteFile(t, "ui/index.html", []byte("modified"), 0644)
	install(false, 2, "one")
	requireWriteFile(t, "ui/extra", []byte("extra"), 0644)
	install(false, 2, "one")
	if _, err := os.Stat("ui/extra"); !os.IsNotExist(err) {
		t.Fatalf("extra file still exists: %v", err)
	}
	payload["one"] = testZip(t, map[string]string{"index.html": "refreshed"})
	install(true, 3, "refreshed")
	install(false, 3, "refreshed")

	cache := filepath.Join("tmp", "release-cache", "web-ui", "one", "web-ui.zip")
	requireWriteFile(t, cache, []byte("corrupt"), 0644)
	install(false, 4, "refreshed")
	payload["one"] = []byte("invalid")
	if err := installArtifact(artifact, true); err == nil {
		t.Fatal("invalid download succeeded")
	}
	assertFileContent(t, "ui/index.html", "refreshed")
	install(false, 5, "refreshed")
}

func TestEngineArtifactPreservesHeader(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	requireMkdirAll(t, "native")
	requireWriteFile(t, "native/machEngine.h", []byte("tracked"), 0644)
	data := testZip(t, map[string]string{"engine.a": "library", "machEngine.h": "release"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()
	a := releaseArtifact{version: "1", name: "engine.zip", url: server.URL, destination: "native", required: "engine.a"}
	if err := installArtifact(a, false); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, "native/engine.a", "library")
	assertFileContent(t, "native/machEngine.h", "tracked")

	data = testZip(t, map[string]string{"../outside": "bad", "engine.a": "library"})
	if err := installArtifact(a, true); err == nil {
		t.Fatal("unsafe archive succeeded")
	}
	assertFileContent(t, "native/engine.a", "library")
}

func testZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range entries {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprint(file, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestArchivePackageAndUnzip(t *testing.T) {
	root := t.TempDir()
	previousWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWorkingDirectory)
	})

	source := filepath.Join("packages", "bundle")
	requireMkdirAll(t, filepath.Join(source, "nested"))
	requireWriteFile(t, filepath.Join(source, "root.txt"), []byte("root"), 0640)
	requireWriteFile(t, filepath.Join(source, "nested", "child.txt"), []byte("child"), 0600)

	archivePath := "bundle.zip"
	if err := archivePackage(archivePath, source); err != nil {
		t.Fatalf("archivePackage() error: %v", err)
	}

	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("zip.OpenReader() error: %v", err)
	}
	var names []string
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	archive.Close()
	sort.Strings(names)
	wantNames := []string{"bundle/", "bundle/nested/", "bundle/nested/child.txt", "bundle/root.txt"}
	if len(names) != len(wantNames) {
		t.Fatalf("archive entries were %v, want %v", names, wantNames)
	}
	for index := range names {
		if names[index] != wantNames[index] {
			t.Fatalf("archive entries were %v, want %v", names, wantNames)
		}
	}

	destination := filepath.Join(root, "extracted")
	if err := unzip(archivePath, destination); err != nil {
		t.Fatalf("unzip() error: %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "bundle", "root.txt"), "root")
	assertFileContent(t, filepath.Join(destination, "bundle", "nested", "child.txt"), "child")
}

func TestArchiveAndUnzipErrors(t *testing.T) {
	root := t.TempDir()
	if err := archivePackage(filepath.Join(root, "missing", "bundle.zip"), root); err == nil {
		t.Fatal("archivePackage() returned nil for an invalid destination")
	}

	archivePath := filepath.Join(root, "bundle.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("os.Create() error: %v", err)
	}
	writer := zip.NewWriter(archiveFile)
	if err := archiveAddEntry(writer, filepath.Join(root, "missing"), root); err == nil {
		t.Fatal("archiveAddEntry() returned nil for a missing entry")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zip.Writer.Close() error: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatalf("archive file Close() error: %v", err)
	}

	invalidArchive := filepath.Join(root, "invalid.zip")
	requireWriteFile(t, invalidArchive, []byte("not a zip"), 0600)
	if err := unzip(invalidArchive, filepath.Join(root, "output")); err == nil {
		t.Fatal("unzip() returned nil for invalid zip data")
	}
}

func TestCopyFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "destination.txt")
	requireWriteFile(t, source, []byte("copied"), 0600)

	if err := copyFile(source, destination, 0644); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}
	assertFileContent(t, destination, "copied")
	if runtime.GOOS != "windows" {
		info, err := os.Stat(destination)
		if err != nil {
			t.Fatalf("os.Stat() error: %v", err)
		}
		if got := info.Mode().Perm(); got != 0644 {
			t.Fatalf("destination mode was %o, want 644", got)
		}
	}

	if err := copyFile(filepath.Join(root, "missing"), destination, 0600); err == nil {
		t.Fatal("copyFile() returned nil for a missing source")
	}
	if err := copyFile(source, root, 0600); err == nil {
		t.Fatal("copyFile() returned nil for a directory destination")
	}
}

func TestWget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/redirect":
			response.Header().Set("Location", "/download")
			response.WriteHeader(http.StatusFound)
		case "/download":
			_, _ = io.WriteString(response, "downloaded")
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	for _, path := range []string{"/download", "/redirect"} {
		destination := filepath.Join(root, filepath.Base(path)+".txt")
		if err := wget(server.URL+path, destination); err != nil {
			t.Fatalf("wget(%q) error: %v", path, err)
		}
		assertFileContent(t, destination, "downloaded")
	}

	if err := wget("://invalid", filepath.Join(root, "invalid")); err == nil {
		t.Fatal("wget() returned nil for an invalid URL")
	}
	if err := wget(server.URL+"/download", filepath.Join(root, "missing", "file")); err == nil {
		t.Fatal("wget() returned nil for an invalid destination")
	}
}

func TestCheckTmpAndCleanPackage(t *testing.T) {
	previousWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error: %v", err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWorkingDirectory)
	})

	if err := CheckTmp(); err != nil {
		t.Fatalf("CheckTmp() error: %v", err)
	}
	if info, err := os.Stat("tmp"); err != nil || !info.IsDir() {
		t.Fatalf("tmp directory was not created: info=%v err=%v", info, err)
	}

	requireMkdirAll(t, filepath.Join("packages", "one"))
	requireMkdirAll(t, filepath.Join("packages", "two"))
	if err := CleanPackage(); err != nil {
		t.Fatalf("CleanPackage() error: %v", err)
	}
	entries, err := os.ReadDir("packages")
	if err != nil {
		t.Fatalf("os.ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("packages still contains %d entries", len(entries))
	}

	if err := os.RemoveAll("packages"); err != nil {
		t.Fatalf("os.RemoveAll() error: %v", err)
	}
	if err := CleanPackage(); err != nil {
		t.Fatalf("CleanPackage() with no directory error: %v", err)
	}
}

func requireMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error: %v", path, err)
	}
}

func requireWriteFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error: %v", path, err)
	}
	if got := string(data); got != want {
		t.Fatalf("file content was %q, want %q", got, want)
	}
}
