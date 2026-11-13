package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
