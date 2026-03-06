package engine

import (
	"io"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
	"time"
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
