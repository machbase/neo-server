package sbin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRmRemovesFile(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "demo.txt")
	if err := os.WriteFile(target, []byte("demo\n"), 0o644); err != nil {
		t.Fatalf("prepare file: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "demo.txt")
	if err != nil {
		t.Fatalf("rm file failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, err=%v", err)
	}
}

func TestRmRemovesDirectoryRecursively(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "demo", "nested")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("prepare directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("demo\n"), 0o644); err != nil {
		t.Fatalf("prepare nested file: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "-r", "demo")
	if err != nil {
		t.Fatalf("rm -r failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(workDir, "demo")); !os.IsNotExist(err) {
		t.Fatalf("expected directory to be removed, err=%v", err)
	}
}

func TestRmRejectsDirectoryWithoutRecursiveFlag(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "demo"), 0o755); err != nil {
		t.Fatalf("prepare directory: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "demo")
	if err == nil {
		t.Fatalf("expected rm on directory to fail, output=%q", output)
	}
	if !strings.Contains(output, "Is a directory") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestRmRemovesEmptyDirectoryWithDirOption(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "empty"), 0o755); err != nil {
		t.Fatalf("prepare empty directory: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "-d", "empty")
	if err != nil {
		t.Fatalf("rm -d failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(workDir, "empty")); !os.IsNotExist(err) {
		t.Fatalf("expected empty directory to be removed, err=%v", err)
	}
}

func TestRmForceIgnoresMissingPath(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "rm", "-f", "missing.txt")
	if err != nil {
		t.Fatalf("rm -f missing path failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(output) != "" {
		t.Fatalf("expected no output for rm -f missing path, got %q", output)
	}
}

func TestRmSupportsUppercaseRecursiveAlias(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "demo", "nested")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("prepare directory: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "-R", "demo")
	if err != nil {
		t.Fatalf("rm -R failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(workDir, "demo")); !os.IsNotExist(err) {
		t.Fatalf("expected directory to be removed, err=%v", err)
	}
}

func TestRmSupportsDirectoryLongOption(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "empty"), 0o755); err != nil {
		t.Fatalf("prepare empty directory: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "--directory", "empty")
	if err != nil {
		t.Fatalf("rm --directory failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(workDir, "empty")); !os.IsNotExist(err) {
		t.Fatalf("expected empty directory to be removed, err=%v", err)
	}
}

func TestRmReportsDirectoryNotEmptyForDirOption(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "demo")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("prepare directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("demo\n"), 0o644); err != nil {
		t.Fatalf("prepare file: %v", err)
	}

	output, err := runCommand(workDir, nil, "rm", "-d", "demo")
	if err == nil {
		t.Fatalf("expected rm -d on non-empty directory to fail, output=%q", output)
	}
	if !strings.Contains(output, "Directory not empty") {
		t.Fatalf("unexpected output: %q", output)
	}
}
