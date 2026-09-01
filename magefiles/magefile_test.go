package main

import (
	"archive/zip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

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
