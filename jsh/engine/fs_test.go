package engine

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dop251/goja"
)

func TestFS_NewFS(t *testing.T) {
	mfs := NewFS()
	if mfs == nil {
		t.Fatal("NewMountFS returned nil")
	}
	if mfs.mounts == nil {
		t.Fatal("mounts map is nil")
	}
	if len(mfs.mounts) != 0 {
		t.Errorf("Expected empty mounts, got %d", len(mfs.mounts))
	}
}

func TestFS_Mount(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		wantErr    bool
	}{
		{"simple path", "foo", false},
		{"nested path", "foo/bar", false},
		{"with leading slash", "/foo", false},
		{"with trailing slash", "foo/", false},
		{"root", "/", false},
		{"dot", ".", false},
		{"nil filesystem", "test", true}, // Special case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewFS()
			testFS := fstest.MapFS{
				"file.txt": &fstest.MapFile{Data: []byte("content")},
			}

			var err error
			if tt.name == "nil filesystem" {
				err = mfs.Mount(tt.mountPoint, nil)
			} else {
				err = mfs.Mount(tt.mountPoint, testFS)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Mount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFS_Mount_Conflicts(t *testing.T) {
	testFS1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
	}
	testFS2 := fstest.MapFS{
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	tests := []struct {
		name        string
		firstMount  string
		secondMount string
		wantErr     bool
	}{
		{"exact duplicate", "/foo", "/foo", true},
		{"parent-child", "/foo", "/foo/bar", true},
		{"child-parent", "/foo/bar", "/foo", true},
		{"siblings", "/foo", "/bar", false},
		{"different nested", "/foo/bar", "/foo/baz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewFS()

			if err := mfs.Mount(tt.firstMount, testFS1); err != nil {
				t.Fatalf("First mount failed: %v", err)
			}

			err := mfs.Mount(tt.secondMount, testFS2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Second mount error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFS_Unmount(t *testing.T) {
	mfs := NewFS()
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	// Mount a filesystem
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Verify it's mounted
	if len(mfs.Mounts()) != 1 {
		t.Errorf("Expected 1 mount, got %d", len(mfs.Mounts()))
	}

	// Unmount it
	if err := mfs.Unmount("/test"); err != nil {
		t.Errorf("Unmount failed: %v", err)
	}

	// Verify it's unmounted
	if len(mfs.Mounts()) != 0 {
		t.Errorf("Expected 0 mounts after unmount, got %d", len(mfs.Mounts()))
	}

	// Try to unmount non-existent mount
	err := mfs.Unmount("/nonexistent")
	if err != fs.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}
}

func TestFS_Mounts(t *testing.T) {
	mfs := NewFS()
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	// Empty mounts
	mounts := mfs.Mounts()
	if len(mounts) != 0 {
		t.Errorf("Expected empty mounts, got %v", mounts)
	}

	// Add some mounts
	mountPoints := []string{"/foo", "/bar", "/baz/qux"}
	for _, mp := range mountPoints {
		if err := mfs.Mount(mp, testFS); err != nil {
			t.Fatalf("Mount %s failed: %v", mp, err)
		}
	}

	// Verify all are returned and sorted
	mounts = mfs.Mounts()
	if len(mounts) != len(mountPoints) {
		t.Errorf("Expected %d mounts, got %d", len(mountPoints), len(mounts))
	}

	// Check they are sorted
	expected := []string{"/bar", "/baz/qux", "/foo"}
	for i, exp := range expected {
		if mounts[i] != exp {
			t.Errorf("Mount %d: expected %s, got %s", i, exp, mounts[i])
		}
	}
}

func TestFS_Open(t *testing.T) {
	fs0 := fstest.MapFS{
		"rootfile.txt": &fstest.MapFile{Data: []byte("rootcontent")},
	}
	fs1 := fstest.MapFS{
		"file1.txt":     &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}
	fs2 := fstest.MapFS{
		"file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", fs0); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("/mount1", fs1); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("/mount2", fs2); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantErr     bool
	}{
		{"root file", "/rootfile.txt", "rootcontent", false},
		{"file in mount1", "/mount1/file1.txt", "content1", false},
		{"nested file in mount1", "/mount1/dir/file2.txt", "content2", false},
		{"file in mount2", "/mount2/file3.txt", "content3", false},
		{"non-existent mount", "/mount3/file.txt", "", true},
		{"non-existent file", "/mount1/nonexistent.txt", "", true},
		{"invalid path", "../outside.txt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := mfs.Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			defer f.Close()

			data, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}

			if string(data) != tt.wantContent {
				t.Errorf("Expected content %q, got %q", tt.wantContent, string(data))
			}
		})
	}
}

func TestFS_Open_LongestMatch(t *testing.T) {
	// Test that the longest matching mount point is used
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("short")},
	}
	fs2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("long")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/foo", fs1); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("/foo/bar", fs2); err != nil {
		// This should fail due to conflict prevention
		if err != fs.ErrExist {
			t.Fatalf("Expected ErrExist, got %v", err)
		}
		return
	}

	// If we reach here, the mount succeeded (shouldn't happen with new logic)
	f, err := mfs.Open("/foo/bar/file.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	// Should get content from the longer mount
	if string(data) != "long" {
		t.Errorf("Expected 'long', got %q", string(data))
	}
}

func TestFS_DelegatesMutableOperationsToMountedFS(t *testing.T) {
	mfs := NewFS()
	shared := NewVirtualFS()
	if err := mfs.Mount("/shared", shared); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	if err := mfs.Mkdir("/shared/cache"); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	if err := mfs.WriteFile("/shared/cache/data.txt", []byte("alpha")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := mfs.AppendFile("/shared/cache/data.txt", []byte("-beta")); err != nil {
		t.Fatalf("AppendFile failed: %v", err)
	}
	if err := mfs.Rename("/shared/cache", "/shared/archive"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	data, err := fs.ReadFile(mfs, "/shared/archive/data.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "alpha-beta" {
		t.Fatalf("unexpected content: %q", string(data))
	}

	if err := mfs.Remove("/shared/archive/data.txt"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := fs.Stat(mfs, "/shared/archive/data.txt"); err == nil {
		t.Fatalf("removed file should not exist")
	}
}

func TestFS_ReadDir(t *testing.T) {
	testFS := fstest.MapFS{
		"dir/file1.txt":        &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt":        &fstest.MapFile{Data: []byte("content2")},
		"dir/subdir/file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantCount int // Now includes . and ..
		wantErr   bool
	}{
		{"read dir", "/mount/dir", 5, false},           // file1.txt, file2.txt, subdir, ., ..
		{"read subdir", "/mount/dir/subdir", 3, false}, // file3.txt, ., ..
		{"non-existent dir", "/mount/nonexistent", 0, true},
		{"non-existent mount", "/nomount/dir", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := mfs.ReadDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(entries) != tt.wantCount {
				t.Errorf("Expected %d entries, got %d", tt.wantCount, len(entries))
			}
		})
	}
}

func TestFS_ReadDir_NoReadDirFS(t *testing.T) {
	// Create a filesystem that doesn't implement ReadDirFS
	type basicFS struct {
		fstest.MapFS
	}

	testFS := basicFS{
		MapFS: fstest.MapFS{
			"file.txt": &fstest.MapFile{Data: []byte("content")},
		},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// ReadDir should still work via fs.ReadDir fallback
	entries, err := mfs.ReadDir("/mount")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Expected entries, got none")
	}
}

func TestFS_Interface(t *testing.T) {
	mfs := NewFS()

	// Verify it implements fs.FS
	var _ fs.FS = mfs

	// Verify it implements fs.ReadDirFS
	var _ fs.ReadDirFS = mfs
}

func TestFS_ReadDir_DotEntries(t *testing.T) {
	testFS := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	// Should have: file1.txt, file2.txt, ., ..
	if len(entries) < 4 {
		t.Errorf("Expected at least 4 entries (including . and ..), got %d", len(entries))
	}

	// Check for . and .. entries
	hasDot := false
	hasDotDot := false
	for _, entry := range entries {
		if entry.Name() == "." {
			hasDot = true
			if !entry.IsDir() {
				t.Error(". entry should be a directory")
			}
		}
		if entry.Name() == ".." {
			hasDotDot = true
			if !entry.IsDir() {
				t.Error(".. entry should be a directory")
			}
		}
	}

	if !hasDot {
		t.Error("Missing . entry in directory listing")
	}
	if !hasDotDot {
		t.Error("Missing .. entry in directory listing")
	}
}

func TestFS_ReadDir_MountPoints(t *testing.T) {
	rootFS := fstest.MapFS{
		"rootfile.txt": &fstest.MapFile{Data: []byte("root")},
	}
	binFS := fstest.MapFS{
		"ls": &fstest.MapFile{Data: []byte("binary")},
	}
	sbinFS := fstest.MapFS{
		"init": &fstest.MapFile{Data: []byte("init")},
	}
	usrFS := fstest.MapFS{
		"readme.txt": &fstest.MapFile{Data: []byte("usr")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", rootFS); err != nil {
		t.Fatalf("Mount / failed: %v", err)
	}
	if err := mfs.Mount("/bin", binFS); err != nil {
		t.Fatalf("Mount /bin failed: %v", err)
	}
	if err := mfs.Mount("/sbin", sbinFS); err != nil {
		t.Fatalf("Mount /sbin failed: %v", err)
	}
	if err := mfs.Mount("/usr", usrFS); err != nil {
		t.Fatalf("Mount /usr failed: %v", err)
	}

	// Read root directory
	entries, err := mfs.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir(/) failed: %v", err)
	}

	// Should have: rootfile.txt, ., .., bin, sbin, usr
	expectedNames := map[string]bool{
		"rootfile.txt": false,
		".":            false,
		"..":           false,
		"bin":          false,
		"sbin":         false,
		"usr":          false,
	}

	for _, entry := range entries {
		if _, exists := expectedNames[entry.Name()]; exists {
			expectedNames[entry.Name()] = true
			// Mounted directories should appear as directories
			if entry.Name() == "bin" || entry.Name() == "sbin" || entry.Name() == "usr" {
				if !entry.IsDir() {
					t.Errorf("%s should be a directory", entry.Name())
				}
			}
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Expected to find %s in root directory listing", name)
		}
	}
}

func TestFS_ReadDir_NestedMountPoints(t *testing.T) {
	rootFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("root")},
	}
	usrFS := fstest.MapFS{
		"readme.txt": &fstest.MapFile{Data: []byte("usr")},
	}
	usrLocalFS := fstest.MapFS{
		"local.txt": &fstest.MapFile{Data: []byte("local")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", rootFS); err != nil {
		t.Fatalf("Mount / failed: %v", err)
	}
	if err := mfs.Mount("/usr", usrFS); err != nil {
		t.Fatalf("Mount /usr failed: %v", err)
	}

	// This should fail due to conflict (parent-child relationship)
	err := mfs.Mount("/usr/local", usrLocalFS)
	if err == nil {
		t.Fatal("Expected mount to fail due to parent-child conflict")
	}
}

func TestFS_ReadDir_NoDuplicateMountPoints(t *testing.T) {
	rootFS := fstest.MapFS{
		"bin/file.txt": &fstest.MapFile{Data: []byte("original")},
	}
	binFS := fstest.MapFS{
		"ls": &fstest.MapFile{Data: []byte("binary")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", rootFS); err != nil {
		t.Fatalf("Mount / failed: %v", err)
	}

	// This should fail because /bin would conflict with existing bin in rootFS
	// Actually, this will succeed but let's test that we don't get duplicate "bin" entries
	err := mfs.Mount("/bin", binFS)
	if err != nil {
		// If it fails, that's fine - conflict detection
		t.Logf("Mount /bin failed as expected: %v", err)
		return
	}

	// Read root directory
	entries, err := mfs.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir(/) failed: %v", err)
	}

	// Count "bin" entries - should only have one
	binCount := 0
	for _, entry := range entries {
		if entry.Name() == "bin" {
			binCount++
		}
	}

	if binCount != 1 {
		t.Errorf("Expected exactly 1 'bin' entry, got %d", binCount)
	}
}

func TestFS_ReadDir_DotEntriesInfo(t *testing.T) {
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." {
			// Test Info() method
			info, err := entry.Info()
			if err != nil {
				t.Errorf("Info() failed for %s: %v", entry.Name(), err)
				continue
			}

			if info.Name() != entry.Name() {
				t.Errorf("Info().Name() = %s, want %s", info.Name(), entry.Name())
			}

			if !info.IsDir() {
				t.Errorf("%s should be a directory", entry.Name())
			}

			if info.Mode()&fs.ModeDir == 0 {
				t.Errorf("%s Mode() should have ModeDir bit set", entry.Name())
			}

			// Test Type() method
			if entry.Type()&fs.ModeDir == 0 {
				t.Errorf("%s Type() should have ModeDir bit set", entry.Name())
			}
		}
	}
}

func TestFS_ReadDir_DotEntriesRealInfo(t *testing.T) {
	// Create a filesystem with known ModTime
	modTime := time.Date(2025, 12, 18, 10, 30, 0, 0, time.UTC)
	testFS := fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{
			Data:    []byte("content"),
			ModTime: modTime,
		},
	}

	mfs := NewFS()
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test/dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	foundDot := false
	foundDotDot := false

	for _, entry := range entries {
		if entry.Name() == "." {
			foundDot = true
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '.': %v", err)
			}

			// Check that info is accessible
			_ = info.Size()
			_ = info.ModTime()

			// fstest.MapFS may not provide directory metadata,
			// but the code should handle this gracefully
			t.Logf("'.' entry: Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}

		if entry.Name() == ".." {
			foundDotDot = true
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '..': %v", err)
			}

			// Check that info is accessible
			_ = info.Size()
			_ = info.ModTime()

			t.Logf("'..' entry: Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}
	}

	if !foundDot {
		t.Error("'.' entry not found")
	}
	if !foundDotDot {
		t.Error("'..' entry not found")
	}
}

func TestFS_ReadDir_DotEntriesRealInfo_OS(t *testing.T) {
	// Use OS filesystem to verify real directory info
	testDir := t.TempDir()
	osFS := os.DirFS(testDir)

	mfs := NewFS()
	if err := mfs.Mount("/test", osFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "." {
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '.': %v", err)
			}

			// With real OS filesystem, ModTime should be set
			if info.ModTime().IsZero() {
				t.Error("'.' entry should have non-zero ModTime from actual OS directory")
			}

			t.Logf("'.' entry (OS): Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}

		if entry.Name() == ".." {
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '..': %v", err)
			}

			// Parent directory should also have ModTime set with real OS
			if info.ModTime().IsZero() {
				t.Error("'..' entry should have non-zero ModTime from parent OS directory")
			}

			t.Logf("'..' entry (OS): Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}
	}
}

func TestFS_OSBackedFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	mfs := NewFS()
	if err := mfs.Mount("/work", os.DirFS(tempDir)); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	if got := mfs.CleanPath("work//nested/../file.txt"); got != "/work/file.txt" {
		t.Fatalf("CleanPath() = %q, want %q", got, "/work/file.txt")
	}

	if err := mfs.WriteFile("/work/sample.txt", []byte("alpha\nbeta\n")); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	content, err := mfs.ReadFile("/work/sample.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "alpha\nbeta\n" {
		t.Fatalf("ReadFile() = %q", string(content))
	}

	lines, err := mfs.ReadLines("/work/sample.txt")
	if err != nil {
		t.Fatalf("ReadLines failed: %v", err)
	}
	if len(lines) != 2 || lines[0] != "alpha" || lines[1] != "beta" {
		t.Fatalf("ReadLines() = %#v", lines)
	}

	count, err := mfs.CountLines("/work/sample.txt")
	if err != nil {
		t.Fatalf("CountLines failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountLines() = %d, want 2", count)
	}

	info, err := mfs.Stat("/work/sample.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size() != int64(len("alpha\nbeta\n")) {
		t.Fatalf("Stat().Size() = %d", info.Size())
	}

	osPath, err := mfs.OSPath("/work/sample.txt")
	if err != nil {
		t.Fatalf("OSPath failed: %v", err)
	}
	if osPath != filepath.Join(tempDir, "sample.txt") {
		t.Fatalf("OSPath() = %q", osPath)
	}

	if err := mfs.AppendFile("/work/sample.txt", []byte("gamma\n")); err != nil {
		t.Fatalf("AppendFile failed: %v", err)
	}

	if err := mfs.Rename("/work/sample.txt", "/work/renamed.txt"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	renamed, err := mfs.ReadFile("/work/renamed.txt")
	if err != nil {
		t.Fatalf("ReadFile renamed failed: %v", err)
	}
	if string(renamed) != "alpha\nbeta\ngamma\n" {
		t.Fatalf("renamed content = %q", string(renamed))
	}

	if err := mfs.Mkdir("/work/dir/subdir"); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	dirInfo, err := mfs.Stat("/work/dir/subdir")
	if err != nil {
		t.Fatalf("Stat(dir) failed: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Fatal("expected created directory")
	}

	if err := mfs.Remove("/work/renamed.txt"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := mfs.Stat("/work/renamed.txt"); err == nil {
		t.Fatal("expected removed file to be missing")
	}

	if err := mfs.Rmdir("/work/dir/subdir"); err != nil {
		t.Fatalf("Rmdir(subdir) failed: %v", err)
	}
	if err := mfs.Rmdir("/work/dir"); err != nil {
		t.Fatalf("Rmdir(dir) failed: %v", err)
	}

	if mfs.Platform() == "" || mfs.Arch() == "" {
		t.Fatal("Platform/Arch should not be empty")
	}
}

func TestFS_OSBackedDescriptorOperations(t *testing.T) {
	tempDir := t.TempDir()
	mfs := NewFS()
	if err := mfs.Mount("/work", os.DirFS(tempDir)); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	fd, err := mfs.OpenFD("/work/fd.txt", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("OpenFD write failed: %v", err)
	}

	writer, err := mfs.HostWriterFD(fd)
	if err != nil {
		t.Fatalf("HostWriterFD failed: %v", err)
	}
	if _, err := writer.Write([]byte("hello world")); err != nil {
		t.Fatalf("writer.Write failed: %v", err)
	}
	if err := mfs.FsyncFD(fd); err != nil {
		t.Fatalf("FsyncFD failed: %v", err)
	}
	if err := mfs.FchmodFD(fd, 0o644); err != nil {
		t.Fatalf("FchmodFD failed: %v", err)
	}
	if err := mfs.FchownFD(fd, -1, -1); err != nil {
		t.Fatalf("FchownFD failed: %v", err)
	}

	statInfo, err := mfs.FstatFD(fd)
	if err != nil {
		t.Fatalf("FstatFD failed: %v", err)
	}
	if statInfo.Size() != int64(len("hello world")) {
		t.Fatalf("FstatFD size = %d", statInfo.Size())
	}

	if err := mfs.CloseFD(fd); err != nil {
		t.Fatalf("CloseFD failed: %v", err)
	}

	fd, err = mfs.OpenFD("/work/fd.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFD read failed: %v", err)
	}
	defer mfs.CloseFD(fd)

	reader, err := mfs.HostReaderFD(fd)
	if err != nil {
		t.Fatalf("HostReaderFD failed: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll failed: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("HostReaderFD content = %q", string(data))
	}

	if err := mfs.CloseFD(fd); err != nil {
		t.Fatalf("CloseFD second phase failed: %v", err)
	}
	fd = -1

	fd, err = mfs.OpenFD("/work/fd.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFD reread failed: %v", err)
	}
	defer func() {
		if fd >= 0 {
			_ = mfs.CloseFD(fd)
		}
	}()

	buf := make([]byte, 16)
	n, err := mfs.ReadFD(fd, buf, 2, 5)
	if err != nil {
		t.Fatalf("ReadFD failed: %v", err)
	}
	if n != 5 || string(buf[2:7]) != "hello" {
		t.Fatalf("ReadFD() n=%d buffer=%q", n, string(buf[2:7]))
	}

	if _, err := mfs.ReadFD(fd, buf, -1, 1); err != fs.ErrInvalid {
		t.Fatalf("ReadFD invalid bounds error = %v", err)
	}
	if _, err := mfs.ReadFD(999, buf, 0, 1); err != fs.ErrInvalid {
		t.Fatalf("ReadFD invalid fd error = %v", err)
	}
	if _, err := mfs.WriteFD(999, []byte("x")); err != fs.ErrInvalid {
		t.Fatalf("WriteFD invalid fd error = %v", err)
	}
	if _, err := mfs.HostReaderFD(999); err != fs.ErrInvalid {
		t.Fatalf("HostReaderFD invalid fd error = %v", err)
	}
	if _, err := mfs.HostWriterFD(999); err != fs.ErrInvalid {
		t.Fatalf("HostWriterFD invalid fd error = %v", err)
	}
	if err := mfs.CloseFD(999); err != fs.ErrInvalid {
		t.Fatalf("CloseFD invalid fd error = %v", err)
	}
}

func TestFS_OSOperationErrorsAndSymlink(t *testing.T) {
	tempDir := t.TempDir()
	mfs := NewFS()
	if err := mfs.Mount("/work", os.DirFS(tempDir)); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	if err := mfs.WriteFile("/work/target.txt", []byte("payload")); err != nil {
		t.Fatalf("WriteFile target failed: %v", err)
	}
	if err := mfs.Symlink("/work/target.txt", "/work/link.txt"); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}
	link, err := mfs.Readlink("/work/link.txt")
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if link != "/work/target.txt" {
		t.Fatalf("Readlink() = %q", link)
	}

	if runtime.GOOS != "windows" {
		if err := mfs.Chmod("/work/target.txt", 0o600); err != nil {
			t.Fatalf("Chmod failed: %v", err)
		}
		if err := mfs.Chown("/work/target.txt", -1, -1); err != nil {
			t.Fatalf("Chown failed: %v", err)
		}
	}

	if err := mfs.Remove("/work/link.txt"); err != nil {
		t.Fatalf("Remove link failed: %v", err)
	}

	if _, err := mfs.OSPath("/missing/file.txt"); err != fs.ErrNotExist {
		t.Fatalf("OSPath missing error = %v", err)
	}

	memFS := NewFS()
	if err := memFS.Mount("/mem", fstest.MapFS{}); err != nil {
		t.Fatalf("Mount memFS failed: %v", err)
	}
	if err := memFS.WriteFile("/mem/file.txt", []byte("x")); err != fs.ErrPermission {
		t.Fatalf("WriteFile on non-OS fs error = %v", err)
	}
	if _, err := memFS.OSPath("/mem/file.txt"); err != fs.ErrPermission {
		t.Fatalf("OSPath on non-OS fs error = %v", err)
	}
}

func TestFS_ReadDirAndDotEntryHelpers(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, "dir"), 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "dir", "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	mfs := NewFS()
	if err := mfs.Mount("/work", os.DirFS(tempDir)); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/work/dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	var dotInfo fs.FileInfo
	for _, entry := range entries {
		if entry.Name() == "." {
			dotInfo, err = entry.Info()
			if err != nil {
				t.Fatalf("dot entry Info failed: %v", err)
			}
			if dotInfo.Name() != "." || !dotInfo.IsDir() || dotInfo.Sys() != nil {
				t.Fatalf("unexpected dot info: name=%q dir=%v sys=%v", dotInfo.Name(), dotInfo.IsDir(), dotInfo.Sys())
			}
		}
	}
	if dotInfo == nil {
		t.Fatal("missing dot entry info")
	}

	if _, err := mfs.ReadDir("../invalid"); err == nil {
		t.Fatal("expected invalid ReadDir path error")
	}
}

func TestFS_FilesystemBindings(t *testing.T) {
	tempDir := t.TempDir()
	jr := &JSRuntime{filesystem: NewFS()}
	if err := jr.filesystem.Mount("/work", os.DirFS(tempDir)); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	if err := module.Set("exports", exports); err != nil {
		t.Fatalf("module.Set failed: %v", err)
	}
	if err := rt.Set("module", module); err != nil {
		t.Fatalf("rt.Set(module) failed: %v", err)
	}

	jr.Filesystem(context.Background(), rt, module)
	exportsObj := module.Get("exports").(*goja.Object)

	for _, key := range []string{"readFile", "writeFile", "appendFile", "readDir", "countLines", "hostWriter", "hostReader", "platform", "arch", "O_RDONLY", "O_APPEND"} {
		if goja.IsUndefined(exportsObj.Get(key)) {
			t.Fatalf("expected export %q", key)
		}
	}

	_, err := rt.RunString(`
		const m = module.exports;
		m.writeFile('/work/bindings.txt', new Uint8Array([65,66,67,10]));
		const fd = m.open('/work/bindings.txt', m.O_RDONLY, 0o644);
		const buffer = new Uint8Array(8);
		const count = m.read(fd, buffer, 1, 4);
		m.close(fd);
		globalThis.bindingSummary = [count, Array.from(buffer).join(','), m.countLines('/work/bindings.txt'), typeof m.platform(), typeof m.arch()].join('|');
	`)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	if got := rt.Get("bindingSummary").String(); got != "4|0,65,66,67,10,0,0,0|1|string|string" {
		t.Fatalf("bindingSummary = %q", got)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, "bindings.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile failed: %v", err)
	}
	if !bytes.Equal(content, []byte("ABC\n")) {
		t.Fatalf("bindings file content = %q", string(content))
	}

	_, err = rt.RunString(`
		const m2 = module.exports;
		try {
			m2.read(1, 'not-a-buffer', 0, 1);
		} catch (e) {
			globalThis.readTypeError = String(e).includes('buffer argument must be a Uint8Array');
		}
	`)
	if err != nil {
		t.Fatalf("RunString readTypeError failed: %v", err)
	}
	if !rt.Get("readTypeError").ToBoolean() {
		t.Fatal("expected read() type validation error")
	}

	if _, err := rt.RunString(`module.exports.read(1, new Uint8Array(1), 0);`); err == nil || !strings.Contains(err.Error(), "read requires 4 arguments") {
		t.Fatalf("expected arity error, got %v", err)
	}
}

func BenchmarkFS_Open(b *testing.B) {
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		b.Fatalf("Mount failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := mfs.Open("/mount/file.txt")
		if err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}

func BenchmarkFS_ReadDir(b *testing.B) {
	testFS := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
		"dir/file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		b.Fatalf("Mount failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mfs.ReadDir("/mount/dir")
		if err != nil {
			b.Fatal(err)
		}
	}
}
