package engine

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFS_RenameAcrossMountsReportsEXDEV(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	mfs := NewFS()
	if err := mfs.Mount("/src", os.DirFS(srcDir)); err != nil {
		t.Fatal(err)
	}
	if err := mfs.Mount("/dst", os.DirFS(dstDir)); err != nil {
		t.Fatal(err)
	}

	err := mfs.Rename("/src/file.txt", "/dst/file.txt")
	if !errors.Is(err, syscall.EXDEV) {
		t.Fatalf("Rename across mounts error=%v, want EXDEV", err)
	}
	if code := FilesystemErrorCode(err); code != "EXDEV" {
		t.Fatalf("FilesystemErrorCode=%q, want EXDEV", code)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "file.txt")); err != nil {
		t.Fatalf("source must remain: %v", err)
	}
}
