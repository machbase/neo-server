package root_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMkdirCreatesDirectory(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "mkdir", "demo")
	if err != nil {
		t.Fatalf("mkdir failed: %v\n%s", err, output)
	}

	info, err := os.Stat(filepath.Join(workDir, "demo"))
	if err != nil {
		t.Fatalf("stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected created path to be a directory")
	}
}

func TestMkdirParentsCreatesNestedDirectories(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "mkdir", "-p", "one/two/three")
	if err != nil {
		t.Fatalf("mkdir -p failed: %v\n%s", err, output)
	}

	info, err := os.Stat(filepath.Join(workDir, "one", "two", "three"))
	if err != nil {
		t.Fatalf("stat nested directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected nested path to be a directory")
	}
}

func TestMkdirRejectsExistingDirectoryWithoutParents(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "demo"), 0o755); err != nil {
		t.Fatalf("prepare existing directory: %v", err)
	}

	output, err := runCommand(workDir, nil, "mkdir", "demo")
	if err == nil {
		t.Fatalf("expected mkdir on existing directory to fail, output=%q", output)
	}
	if !strings.Contains(output, "File exists") {
		t.Fatalf("unexpected output: %q", output)
	}
}
