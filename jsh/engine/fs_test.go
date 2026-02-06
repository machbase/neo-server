package engine

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
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

func TestFS_Module(t *testing.T) {
	script := `
		// Example usage of the fs module

		const fs = require('/lib/fs');
		const process = require('/lib/process');

		console.println('=== FS Module Example ===\n');

		// 1. Read a file
		try {
			console.println('1. Reading /lib/fs/index.js (first 100 chars):');
			const content = fs.readFileSync('/lib/fs/index.js', 'utf8');
			console.println(content.substring(0, 100) + '...\n');
		} catch (e) {
			console.println('Error reading file:', e);
			process.exit(1);
		}


		// 2. Create tmp directory
		try {
			console.println('2. Creating directory /work/tmp:');
			fs.mkdirSync('/work/tmp');
			console.println('Directory created');
			
			console.println('Checking if directory exists:', fs.existsSync('/work/tmp'));
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}
			

		// 3. Write and read a file
		try {
			console.println('3. Writing to /work/tmp/test.txt:');
			fs.writeFileSync('/work/tmp/test.txt', 'Hello from fs module!\n', 'utf8');
			console.println('File written successfully');
			
			console.println('Reading back /work/tmp/test.txt:');
			const content = fs.readFileSync('/work/tmp/test.txt', 'utf8');
			console.println(content);
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 4. Append to a file
		try {
			console.println('4. Appending to /work/tmp/test.txt:');
			fs.appendFileSync('/work/tmp/test.txt', 'Appended line!\n', 'utf8');
			const content = fs.readFileSync('/work/tmp/test.txt', 'utf8');
			console.println('File content after append:');
			console.println(content);
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 5. Check if file exists
		try {
			console.println('5. Checking if files exist:');
			console.println('/work/tmp/test.txt exists:', fs.existsSync('/work/tmp/test.txt'));
			console.println('/work/tmp/nonexistent.txt exists:', fs.existsSync('/work/tmp/nonexistent.txt'));
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 6. Get file stats
		try {
			console.println('6. Getting stats for /work/tmp/test.txt:');
			const stats = fs.statSync('/work/tmp/test.txt');
			console.println('Is file:', stats.isFile());
			console.println('Is directory:', stats.isDirectory());
			console.println('Size:', stats.size, 'bytes');
			console.println('Modified:', stats.mtime);
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 7. List directory contents
		try {
			console.println('7. Listing /lib directory:');
			const files = fs.readdirSync('/lib');
			files.forEach(file => console.println('  -', file));
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 8. List directory with file types
		try {
			console.println('8. Listing /lib with file types:');
			const entries = fs.readdirSync('/lib', { withFileTypes: true });
			entries.forEach(entry => {
				const type = entry.isDirectory() ? '[DIR]' : '[FILE]';
				console.println('  ' + type + ' ' + entry.name);
			});
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 9. Create nested directories
		try {
			console.println('9. Creating nested directories /work/tmp/a/b/c:');
			fs.mkdirSync('/work/tmp/a/b/c', { recursive: true });
			console.println('Nested directories created');
			
			console.println('Removing nested directories:');
			fs.rmSync('/work/tmp/a', { recursive: true });
			console.println('Nested directories removed');
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 10. Copy file
		try {
			console.println('10. Copying /work/tmp/test.txt to /work/tmp/test-copy.txt:');
			fs.copyFileSync('/work/tmp/test.txt', '/work/tmp/test-copy.txt');
			console.println('File copied');
			
			const original = fs.readFileSync('/work/tmp/test.txt', 'utf8');
			const copy = fs.readFileSync('/work/tmp/test-copy.txt', 'utf8');
			console.println('Original and copy match:', original === copy);
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 11. Rename file
		try {
			console.println('11. Renaming /work/tmp/test-copy.txt to /work/tmp/test-renamed.txt:');
			fs.renameSync('/work/tmp/test-copy.txt', '/work/tmp/test-renamed.txt');
			console.println('File renamed');
			console.println('Old file exists:', fs.existsSync('/work/tmp/test-copy.txt'));
			console.println('New file exists:', fs.existsSync('/work/tmp/test-renamed.txt'));
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 12. Clean up
		try {
			console.println('12. Cleaning up test files:');
			fs.unlinkSync('/work/tmp/test.txt');
			fs.unlinkSync('/work/tmp/test-renamed.txt');
			console.println('Test files removed');
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 13. Remove tmp directory
		try {
			console.println('13. Removing directory:');
			fs.rmdirSync('/work/tmp');
			console.println('Directory removed');
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		console.println('\n=== Example Complete ===');
	`

	// Run the test script
	tc := TestCase{
		name:   "module_fs_complete",
		script: script,
		output: []string{}, // Output will be checked during execution, not a simple string match
	}

	t.Run(tc.name, func(t *testing.T) {
		conf := Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []FSTab{{MountPoint: "/", Source: "../root/embed/"}, {MountPoint: "/work", Source: "../test/"}},
			Env: map[string]any{
				"PATH": "/lib:/work:/sbin",
				"PWD":  "/work",
			},
			Reader:      &bytes.Buffer{},
			Writer:      &bytes.Buffer{},
			ExecBuilder: testExecBuilder,
		}
		jr, err := New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/fs", jr.Filesystem)

		if err := jr.Run(); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()

		// Check that key operations completed successfully
		expectedStrings := []string{
			"=== FS Module Example ===",
			"1. Reading /lib/fs/index.js (first 100 chars):",
			"2. Creating directory /work/tmp:",
			"Directory created",
			"Checking if directory exists: true",
			"3. Writing to /work/tmp/test.txt:",
			"File written successfully",
			"Reading back /work/tmp/test.txt:",
			"Hello from fs module!",
			"4. Appending to /work/tmp/test.txt:",
			"File content after append:",
			"Appended line!",
			"5. Checking if files exist:",
			"/work/tmp/test.txt exists: true",
			"/work/tmp/nonexistent.txt exists: false",
			"6. Getting stats for /work/tmp/test.txt:",
			"Is file: true",
			"Is directory: false",
			"7. Listing /lib directory:",
			"8. Listing /lib with file types:",
			"9. Creating nested directories /work/tmp/a/b/c:",
			"Nested directories created",
			"Removing nested directories:",
			"Nested directories removed",
			"10. Copying /work/tmp/test.txt to /work/tmp/test-copy.txt:",
			"File copied",
			"Original and copy match: true",
			"11. Renaming /work/tmp/test-copy.txt to /work/tmp/test-renamed.txt:",
			"File renamed",
			"Old file exists: false",
			"New file exists: true",
			"12. Cleaning up test files:",
			"Test files removed",
			"13. Removing directory:",
			"Directory removed",
			"=== Example Complete ===",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(gotOutput, expected) {
				t.Errorf("Expected output to contain %q, but it didn't.\nFull output:\n%s", expected, gotOutput)
			}
		}

		t.Logf("Full output:\n%s", gotOutput)
	})
}

func TestFSModule(t *testing.T) {
	lib_fs_index_js, err := os.ReadFile("../root/embed/lib/fs/index.js")
	if err != nil {
		t.Fatalf("Failed to read fs module source: %v", err)
	}

	tests := []TestCase{
		{
			name: "fs_resolve_path",
			script: `
				const fs = require('@jsh/fs');
				paths = [
					'$HOME/../lib/baz',
					'local.text',
					'/absolute/path',
					'./relative/path',
					'../up/one/level'
				];
				for (const p of paths) {
					const resolved = fs.resolvePath(p);
					console.println(p, '=>', resolved);
				}
			`,
			output: []string{
				"$HOME/../lib/baz => /lib/baz",
				"local.text => local.text",
				"/absolute/path => /absolute/path",
				"./relative/path => relative/path",
				"../up/one/level => ../up/one/level",
			},
		},
		{
			name: "fs_resolve_abs_path",
			script: `
				const fs = require('@jsh/fs');
				paths = [
					'$HOME/../lib/baz',
					'local.text',
					'/absolute/path',
					'./relative/path',
					'../up/one/level'
				];
				for (const p of paths) {
					const resolved = fs.resolveAbsPath(p);
					console.println(p, '=>', resolved);
				}
			`,
			output: []string{
				"$HOME/../lib/baz => /lib/baz",
				"local.text => /work/local.text",
				"/absolute/path => /absolute/path",
				"./relative/path => /work/relative/path",
				"../up/one/level => /up/one/level",
			},
		},
		{
			name: "fs_read_file",
			script: `
				const fs = require('/lib/fs');
				const content = fs.readFile('/lib/fs/index.js', 'utf8');
				console.println('Read /lib/fs/index.js, length =', content.length);
			`,
			output: []string{
				fmt.Sprintf("Read /lib/fs/index.js, length = %d", len(lib_fs_index_js)),
			},
		},
		{
			name: "fs_read_file_nonexistent",
			script: `
				const fs = require('/lib/fs');
				try {
					const content = fs.readFile('/lib/fs/nonexistent.js', 'utf8');
				} catch (e) {
					console.println('Error reading nonexistent file:', e.message);
				}
			`,
			output: []string{
				"Error reading nonexistent file: ENOENT: no such file or directory, open '/lib/fs/nonexistent.js'",
			},
		},
		{
			name: "fs_write_and_read_file",
			script: `
				const fs = require('/lib/fs');
				fs.writeFile('/tmp/testfile.txt', 'Hello, World!', 'utf8');
				const content = fs.readFile('/tmp/testfile.txt', 'utf8');
				console.println('Content of /tmp/testfile.txt:', content);
			`,
			output: []string{
				"Content of /tmp/testfile.txt: Hello, World!",
			},
		},
		{
			name: "fs_append_file",
			script: `
				const fs = require('/lib/fs');
				fs.writeFile('/tmp/append.txt', 'Line 1\n', 'utf8');
				fs.exists('/tmp/append.txt') && console.println('/tmp/append.txt exists after write');
				fs.appendFile('/tmp/append.txt', 'Line 2\n', 'utf8');
				const content = fs.readFile('/tmp/append.txt', 'utf8');
				console.println('Content of /tmp/append.txt:\n' + content);
			`,
			output: []string{
				"/tmp/append.txt exists after write",
				"Content of /tmp/append.txt:",
				"Line 1",
				"Line 2",
				"",
			},
		},
		{
			name: "fs_stat_file",
			script: `
				const fs = require('/lib/fs');
				fs.writeFile('/tmp/stat_file.txt', 'Stat me!', 'utf8');
				const stats = fs.stat('/tmp/stat_file.txt');
				console.println('/tmp/stat_file.txt name:', stats.name);
				console.println('/tmp/stat_file.txt isSymbolicLink():', stats.isSymbolicLink());
				console.println('/tmp/stat_file.txt isFile():', stats.isFile());
				console.println('/tmp/stat_file.txt size:', stats.size);
			`,
			output: []string{
				"/tmp/stat_file.txt name: stat_file.txt",
				"/tmp/stat_file.txt isSymbolicLink(): false",
				"/tmp/stat_file.txt isFile(): true",
				"/tmp/stat_file.txt size: 8",
			},
		},
		{
			name: "fs_mkdir_and_readdir_rename_unlink_rmdir",
			script: `
				const fs = require('/lib/fs');
				fs.mkdir('/tmp/my_dir');
				fs.writeFile('/tmp/my_dir/file1.txt', 'File 1', 'utf8');
				fs.writeFile('/tmp/my_dir/file2.txt', 'File 2', 'utf8');
				const entries = fs.readdir('/tmp/my_dir');
				console.println('Entries in /tmp/my_dir:');
				entries.forEach(entry => console.println(' -', entry));

				fs.rename('/tmp/my_dir/file1.txt', '/tmp/my_dir/file1_renamed.txt');
				fs.unlink('/tmp/my_dir/file2.txt');
				const entriesAfter = fs.readdir('/tmp/my_dir');
				console.println('Entries in /tmp/my_dir after rename and unlink:');
				entriesAfter.forEach(entry => console.println(' -', entry));

				fs.rmdir('/tmp/my_dir', {recursive: true});

				const exists = fs.exists('/tmp/my_dir');
				console.println('/tmp/my_dir exists after rmdir:', exists);
			`,
			output: []string{
				"Entries in /tmp/my_dir:",
				" - .",
				" - ..",
				" - file1.txt",
				" - file2.txt",
				"Entries in /tmp/my_dir after rename and unlink:",
				" - .",
				" - ..",
				" - file1_renamed.txt",
				"/tmp/my_dir exists after rmdir: false",
			},
		},
		{
			name: "fs_symlink_and_readlink",
			script: `
				const fs = require('fs');
				
				// Create a test file in /work (which is writable)
				fs.writeFileSync('/work/original.txt', 'Original content');
				console.println('Original file created');
				
				// Create a symbolic link
				try {
					fs.symlinkSync('/work/original.txt', '/work/link.txt');
					console.println('Symlink created');
					
					// Read the symbolic link target
					const target = fs.readlinkSync('/work/link.txt');
					console.println('Link target:', target);
				} catch (e) {
					console.println('Symlink error:', e.message);
				} finally {
					// Cleanup
					try {
						fs.unlinkSync('/work/link.txt');
					} catch (e) {
						console.println('Cleanup link error:', e.message);
					}
					try {
						fs.unlinkSync('/work/original.txt');
					} catch (e) {
						console.println('Cleanup original error:', e.message);
					}
				}
			`,
			output: []string{
				"Original file created",
				"Symlink created",
				"Link target: /work/original.txt",
			},
		},
		{
			name: "fs_chown",
			script: `
				const fs = require('fs');
				
				// Create a test file in /work
				fs.writeFileSync('/work/chown_test.txt', 'Test file');
				
				try {
					// Try to chown with uid=0, gid=0 (should work as the file owner)
					// On Unix systems, this typically preserves current ownership
					fs.chownSync('/work/chown_test.txt', -1, -1);
					console.println('Chown succeeded');
				} catch (e) {
					console.println('Chown error:', e.message);
				} finally {
					try {
						fs.unlinkSync('/work/chown_test.txt');
					} catch (e) {}
				}
			`,
			output: []string{
				"Chown succeeded",
			},
		},
		{
			name: "fs_readdir_recursive",
			script: `
				const fs = require('fs');
				
				// Create nested directory structure
				fs.mkdirSync('/tmp/recursive_test', { recursive: true });
				fs.mkdirSync('/tmp/recursive_test/dir1', { recursive: true });
				fs.mkdirSync('/tmp/recursive_test/dir2', { recursive: true });
				fs.mkdirSync('/tmp/recursive_test/dir1/subdir', { recursive: true });
				
				// Create some files
				fs.writeFileSync('/tmp/recursive_test/file1.txt', 'File 1');
				fs.writeFileSync('/tmp/recursive_test/dir1/file2.txt', 'File 2');
				fs.writeFileSync('/tmp/recursive_test/dir1/subdir/file3.txt', 'File 3');
				fs.writeFileSync('/tmp/recursive_test/dir2/file4.txt', 'File 4');
				
				// Read directory recursively
				const entries = fs.readdirSync('/tmp/recursive_test', { recursive: true });
				console.println('Recursive entries:');
				entries.filter(e => e !== '.' && e !== '..').sort().forEach(e => console.println(' -', e));
				
				// Cleanup
				fs.unlinkSync('/tmp/recursive_test/dir1/subdir/file3.txt');
				fs.rmdirSync('/tmp/recursive_test/dir1/subdir');
				fs.unlinkSync('/tmp/recursive_test/dir1/file2.txt');
				fs.rmdirSync('/tmp/recursive_test/dir1');
				fs.unlinkSync('/tmp/recursive_test/dir2/file4.txt');
				fs.rmdirSync('/tmp/recursive_test/dir2');
				fs.unlinkSync('/tmp/recursive_test/file1.txt');
				fs.rmdirSync('/tmp/recursive_test');
			`,
			output: []string{
				"Recursive entries:",
				" - dir1",
				" - dir1/file2.txt",
				" - dir1/subdir",
				" - dir1/subdir/file3.txt",
				" - dir2",
				" - dir2/file4.txt",
				" - file1.txt",
			},
		},
		{
			name: "fs_cp_sync",
			script: `
				const fs = require('fs');
				
				// Create source directory structure
				fs.mkdirSync('/tmp/cp_source', { recursive: true });
				fs.mkdirSync('/tmp/cp_source/subdir', { recursive: true });
				fs.writeFileSync('/tmp/cp_source/file1.txt', 'File 1');
				fs.writeFileSync('/tmp/cp_source/subdir/file2.txt', 'File 2');
				
				// Copy directory recursively
				fs.cpSync('/tmp/cp_source', '/tmp/cp_dest', { recursive: true });
				console.println('Directory copied');
				
				// Verify copied files
				const content1 = fs.readFileSync('/tmp/cp_dest/file1.txt', 'utf8');
				const content2 = fs.readFileSync('/tmp/cp_dest/subdir/file2.txt', 'utf8');
				console.println('Copied file1:', content1);
				console.println('Copied file2:', content2);
				
				// Cleanup
				fs.unlinkSync('/tmp/cp_source/subdir/file2.txt');
				fs.rmdirSync('/tmp/cp_source/subdir');
				fs.unlinkSync('/tmp/cp_source/file1.txt');
				fs.rmdirSync('/tmp/cp_source');
				fs.unlinkSync('/tmp/cp_dest/subdir/file2.txt');
				fs.rmdirSync('/tmp/cp_dest/subdir');
				fs.unlinkSync('/tmp/cp_dest/file1.txt');
				fs.rmdirSync('/tmp/cp_dest');
			`,
			output: []string{
				"Directory copied",
				"Copied file1: File 1",
				"Copied file2: File 2",
			},
		},
		{
			name: "fs_file_descriptor_operations",
			script: `
				const fs = require('fs');
				
				// Test openSync, writeSync, readSync, closeSync
				console.println('=== File Descriptor Operations ===');
				
				// Open file for writing
				const fd1 = fs.openSync('/work/fd_test.txt', 'w', 0o666);
				console.println('Opened file for writing, fd:', fd1);
				
				// Write to file descriptor
				const writeData = 'Hello from file descriptor!';
				const bytesWritten = fs.writeSync(fd1, writeData);
				console.println('Bytes written:', bytesWritten);
				
				// Close write descriptor
				fs.closeSync(fd1);
				console.println('Closed write descriptor');
				
				// Open file for reading
				const fd2 = fs.openSync('/work/fd_test.txt', 'r');
				console.println('Opened file for reading, fd:', fd2);
				
				// Read from file descriptor
				const buffer = Buffer.alloc(100);
				const bytesRead = fs.readSync(fd2, buffer, 0, 100);
				console.println('Bytes read:', bytesRead);
				
				// Convert buffer to string
				let content = '';
				for (let i = 0; i < bytesRead; i++) {
					content += String.fromCharCode(buffer[i]);
				}
				console.println('Content:', content);
				
				// Test fstat
				const stats = fs.fstatSync(fd2);
				console.println('File size from fstat:', stats.size);
				console.println('Is file:', stats.isFile());
				
				// Close read descriptor
				fs.closeSync(fd2);
				console.println('Closed read descriptor');
				
				// Test fsync
				const fd3 = fs.openSync('/work/fd_test.txt', 'a');
				fs.writeSync(fd3, '\nAppended line');
				fs.fsyncSync(fd3);
				console.println('Synced file');
				fs.closeSync(fd3);
				
				// Cleanup
				fs.unlinkSync('/work/fd_test.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== File Descriptor Operations ===",
				"Opened file for writing, fd: 3",
				"Bytes written: 27",
				"Closed write descriptor",
				"Opened file for reading, fd: 4",
				"Bytes read: 27",
				"Content: Hello from file descriptor!",
				"File size from fstat: 27",
				"Is file: true",
				"Closed read descriptor",
				"Synced file",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_fchmod_fchown",
			script: `
				const fs = require('fs');
				
				// Create test file
				fs.writeFileSync('/work/fchmod_test.txt', 'Test content');
				
				// Open file
				const fd = fs.openSync('/work/fchmod_test.txt', 'r+');
				console.println('File opened, fd:', fd);
				
				// Test fchmod
				try {
					fs.fchmodSync(fd, 0o644);
					console.println('fchmod succeeded');
				} catch (e) {
					console.println('fchmod error:', e.message);
				}
				
				// Test fchown (with -1 to preserve ownership)
				try {
					fs.fchownSync(fd, -1, -1);
					console.println('fchown succeeded');
				} catch (e) {
					console.println('fchown error:', e.message);
				}
				
				// Close and cleanup
				fs.closeSync(fd);
				fs.unlinkSync('/work/fchmod_test.txt');
				console.println('Test complete');
			`,
			output: []string{
				"File opened, fd: 3",
				"fchmod succeeded",
				"fchown succeeded",
				"Test complete",
			},
		},
		{
			name: "fs_error_handling_bad_fd",
			script: `
				const fs = require('fs');
				
				// Test with invalid file descriptor
				console.println('=== Testing Bad File Descriptor ===');
				
				try {
					fs.closeSync(999);
					console.println('closeSync with bad fd: unexpected success');
				} catch (e) {
					console.println('closeSync error:', e.code);
				}
				
				try {
					const buffer = new Array(100);
					fs.readSync(999, buffer, 0, 100);
					console.println('readSync with bad fd: unexpected success');
				} catch (e) {
					console.println('readSync error:', e.code);
				}
				
				try {
					fs.writeSync(999, 'test');
					console.println('writeSync with bad fd: unexpected success');
				} catch (e) {
					console.println('writeSync error:', e.code);
				}
				
				try {
					fs.fstatSync(999);
					console.println('fstatSync with bad fd: unexpected success');
				} catch (e) {
					console.println('fstatSync error:', e.code);
				}
				
				try {
					fs.fchmodSync(999, 0o644);
					console.println('fchmodSync with bad fd: unexpected success');
				} catch (e) {
					console.println('fchmodSync error:', e.code);
				}
				
				try {
					fs.fchownSync(999, -1, -1);
					console.println('fchownSync with bad fd: unexpected success');
				} catch (e) {
					console.println('fchownSync error:', e.code);
				}
				
				try {
					fs.fsyncSync(999);
					console.println('fsyncSync with bad fd: unexpected success');
				} catch (e) {
					console.println('fsyncSync error:', e.code);
				}
				
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Bad File Descriptor ===",
				"closeSync error: EBADF",
				"readSync error: EBADF",
				"writeSync error: EBADF",
				"fstatSync error: EBADF",
				"fchmodSync error: EBADF",
				"fchownSync error: EBADF",
				"fsyncSync error: EBADF",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_open_flags_variations",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Open Flags ===');
				
				// Create a test file
				fs.writeFileSync('/work/flags_test.txt', 'Initial content');
				
				// Test 'r' - read only
				const fd1 = fs.openSync('/work/flags_test.txt', 'r');
				console.println('Opened with r flag, fd:', fd1);
				fs.closeSync(fd1);
				
				// Test 'r+' - read/write
				const fd2 = fs.openSync('/work/flags_test.txt', 'r+');
				console.println('Opened with r+ flag, fd:', fd2);
				fs.closeSync(fd2);
				
				// Test 'w' - write (truncate)
				const fd3 = fs.openSync('/work/flags_test.txt', 'w');
				console.println('Opened with w flag, fd:', fd3);
				fs.writeSync(fd3, 'New content');
				fs.closeSync(fd3);
				
				// Test 'a' - append
				const fd4 = fs.openSync('/work/flags_test.txt', 'a');
				console.println('Opened with a flag, fd:', fd4);
				fs.writeSync(fd4, ' appended');
				fs.closeSync(fd4);
				
				// Verify content
				const content = fs.readFileSync('/work/flags_test.txt', 'utf8');
				console.println('Final content:', content);
				
				// Test 'wx' - write exclusive (should fail if exists)
				try {
					const fd5 = fs.openSync('/work/flags_test.txt', 'wx');
					console.println('wx flag: unexpected success');
					fs.closeSync(fd5);
				} catch (e) {
					console.println('wx flag on existing file: error as expected');
				}
				
				// Test 'wx' on new file
				const fd6 = fs.openSync('/work/new_file.txt', 'wx');
				console.println('Opened new file with wx flag, fd:', fd6);
				fs.closeSync(fd6);
				
				// Cleanup
				fs.unlinkSync('/work/flags_test.txt');
				fs.unlinkSync('/work/new_file.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Open Flags ===",
				"Opened with r flag, fd: 3",
				"Opened with r+ flag, fd: 4",
				"Opened with w flag, fd: 5",
				"Opened with a flag, fd: 6",
				"Final content: New content appended",
				"wx flag on existing file: error as expected",
				"Opened new file with wx flag, fd: 7",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_rename_cross_mount_error",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Cross-Mount Rename ===');
				
				// Create test file in /work
				fs.writeFileSync('/work/test_rename.txt', 'test');
				console.println('Created file in /work');
				
				// Try to rename across mount points (should fail)
				// /work and /tmp are different mount points
				try {
					fs.renameSync('/work/test_rename.txt', '/tmp/test_renamed.txt');
					console.println('Cross-mount rename: unexpected success');
				} catch (e) {
					console.println('Cross-mount rename failed as expected');
				}
				
				// Normal rename within same mount
				fs.renameSync('/work/test_rename.txt', '/work/test_renamed.txt');
				console.println('Same-mount rename succeeded');
				
				// Verify
				const exists1 = fs.existsSync('/work/test_rename.txt');
				const exists2 = fs.existsSync('/work/test_renamed.txt');
				console.println('Original exists:', exists1);
				console.println('Renamed exists:', exists2);
				
				// Cleanup
				fs.unlinkSync('/work/test_renamed.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Cross-Mount Rename ===",
				"Created file in /work",
				"Cross-mount rename failed as expected",
				"Same-mount rename succeeded",
				"Original exists: false",
				"Renamed exists: true",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_access_check",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Access Check ===');
				
				// Create test file
				fs.writeFileSync('/work/access_test.txt', 'test content');
				
				// Test F_OK (file exists)
				try {
					fs.accessSync('/work/access_test.txt', fs.constants.F_OK);
					console.println('F_OK: file exists');
				} catch (e) {
					console.println('F_OK: file does not exist');
				}
				
				// Test on non-existent file
				try {
					fs.accessSync('/work/nonexistent.txt', fs.constants.F_OK);
					console.println('Nonexistent file: unexpected success');
				} catch (e) {
					console.println('Nonexistent file: error as expected, code:', e.code);
				}
				
				// Test R_OK (read permission)
				try {
					fs.accessSync('/work/access_test.txt', fs.constants.R_OK);
					console.println('R_OK: read permission granted');
				} catch (e) {
					console.println('R_OK: no read permission');
				}
				
				// Cleanup
				fs.unlinkSync('/work/access_test.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Access Check ===",
				"F_OK: file exists",
				"Nonexistent file: error as expected, code: ENOENT",
				"R_OK: read permission granted",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_truncate",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Truncate ===');
				
				// Create test file with content
				fs.writeFileSync('/work/truncate_test.txt', 'Hello, World! This is a long content.');
				let stats = fs.statSync('/work/truncate_test.txt');
				console.println('Original size:', stats.size);
				
				// Truncate to 0 (default)
				fs.truncateSync('/work/truncate_test.txt');
				stats = fs.statSync('/work/truncate_test.txt');
				console.println('After truncate to 0, size:', stats.size);
				
				// Write new content
				fs.writeFileSync('/work/truncate_test.txt', 'New content here!');
				stats = fs.statSync('/work/truncate_test.txt');
				console.println('After write, size:', stats.size);
				
				// Truncate to specific length
				fs.truncateSync('/work/truncate_test.txt', 11);
				stats = fs.statSync('/work/truncate_test.txt');
				console.println('After truncate to 11, size:', stats.size);
				
				const content = fs.readFileSync('/work/truncate_test.txt', 'utf8');
				console.println('Content after truncate:', content);
				
				// Cleanup
				fs.unlinkSync('/work/truncate_test.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Truncate ===",
				"Original size: 37",
				"After truncate to 0, size: 0",
				"After write, size: 17",
				"After truncate to 11, size: 11",
				"Content after truncate: New content",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_realpath",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Realpath ===');
				
				// Create test file and symlink
				fs.writeFileSync('/work/realpath_test.txt', 'test');
				fs.symlinkSync('/work/realpath_test.txt', '/work/realpath_link.txt');
				
				// Get realpath of symlink
				const realpath = fs.realpathSync('/work/realpath_link.txt');
				console.println('Realpath of symlink:', realpath);
				
				// Get realpath of regular file
				const realpath2 = fs.realpathSync('/work/realpath_test.txt');
				console.println('Realpath of regular file:', realpath2);
				
				// Cleanup
				fs.unlinkSync('/work/realpath_link.txt');
				fs.unlinkSync('/work/realpath_test.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Realpath ===",
				"Realpath of symlink: /work/realpath_link.txt",
				"Realpath of regular file: /work/realpath_test.txt",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_rmdir_with_recursive",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing rmdir with recursive ===');
				
				// Create nested structure
				fs.mkdirSync('/work/rm_test', { recursive: true });
				fs.mkdirSync('/work/rm_test/subdir', { recursive: true });
				fs.writeFileSync('/work/rm_test/file.txt', 'content');
				fs.writeFileSync('/work/rm_test/subdir/file2.txt', 'content2');
				console.println('Created nested structure');
				
				// Try rmdir without recursive on non-empty dir
				try {
					fs.rmdirSync('/work/rm_test');
					console.println('rmdir without recursive: unexpected success');
				} catch (e) {
					console.println('rmdir without recursive: failed as expected');
				}
				
				// Use rmSync with recursive
				fs.rmSync('/work/rm_test', { recursive: true });
				console.println('rmSync with recursive: succeeded');
				
				// Verify deletion
				const exists = fs.existsSync('/work/rm_test');
				console.println('Directory exists after rm:', exists);
				
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing rmdir with recursive ===",
				"Created nested structure",
				"rmdir without recursive: failed as expected",
				"rmSync with recursive: succeeded",
				"Directory exists after rm: false",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_readdir_with_file_types_details",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing readdir with withFileTypes ===');
				
				// Create test structure
				fs.mkdirSync('/work/readdir_test', { recursive: true });
				fs.mkdirSync('/work/readdir_test/subdir');
				fs.writeFileSync('/work/readdir_test/file1.txt', 'content1');
				fs.writeFileSync('/work/readdir_test/file2.txt', 'content2');
				
				// Read with withFileTypes: false
				const entries1 = fs.readdirSync('/work/readdir_test');
				console.println('Without withFileTypes:');
				entries1.filter(e => e !== '.' && e !== '..').sort().forEach(e => {
					console.println('  -', e);
				});
				
				// Read with withFileTypes: true
				const entries2 = fs.readdirSync('/work/readdir_test', { withFileTypes: true });
				console.println('With withFileTypes:');
				entries2.filter(e => e.name !== '.' && e.name !== '..').sort((a, b) => a.name.localeCompare(b.name)).forEach(e => {
					const type = e.isDirectory() ? 'DIR' : 'FILE';
					console.println('  -', e.name, type);
				});
				
				// Cleanup
				fs.unlinkSync('/work/readdir_test/file1.txt');
				fs.unlinkSync('/work/readdir_test/file2.txt');
				fs.rmdirSync('/work/readdir_test/subdir');
				fs.rmdirSync('/work/readdir_test');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing readdir with withFileTypes ===",
				"Without withFileTypes:",
				"  - file1.txt",
				"  - file2.txt",
				"  - subdir",
				"With withFileTypes:",
				"  - file1.txt FILE",
				"  - file2.txt FILE",
				"  - subdir DIR",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_buffer_operations",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Buffer Operations ===');
				
				// Write string data
				const testData = "Hello World";
				fs.writeFileSync('/work/buffer_test.txt', testData);
				console.println('Written text data');
				
				// Read as string
				const readData = fs.readFileSync('/work/buffer_test.txt', 'utf8');
				console.println('Read text:', readData);
				console.println('Data matches:', readData === testData);
				
				// Test with file descriptor
				const fd = fs.openSync('/work/fd_test.txt', 'w');
				fs.writeSync(fd, 'FD test content');
				fs.closeSync(fd);
				console.println('Written via fd');
				
				// Read via fd
				const fd2 = fs.openSync('/work/fd_test.txt', 'r');
				const buffer = Buffer.alloc(100);
				const bytesRead = fs.readSync(fd2, buffer, 0, 100);
				fs.closeSync(fd2);
				console.println('Read bytes via fd:', bytesRead);
				
				// Cleanup
				fs.unlinkSync('/work/buffer_test.txt');
				fs.unlinkSync('/work/fd_test.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Buffer Operations ===",
				"Written text data",
				"Read text: Hello World",
				"Data matches: true",
				"Written via fd",
				"Read bytes via fd: 15",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_chmod",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing chmod ===');
				
				// Create test file
				fs.writeFileSync('/work/chmod_test.txt', 'test');
				
				// Change permission to 0644
				fs.chmodSync('/work/chmod_test.txt', 0o644);
				console.println('Changed permission to 0644');
				
				// Verify permission
				const stats = fs.statSync('/work/chmod_test.txt');
				const mode = stats.mode & 0o777;
				console.println('Current mode:', mode.toString(8));
				
				// Change permission to 0600
				fs.chmodSync('/work/chmod_test.txt', 0o600);
				console.println('Changed permission to 0600');
				
				const stats2 = fs.statSync('/work/chmod_test.txt');
				const mode2 = stats2.mode & 0o777;
				console.println('Current mode:', mode2.toString(8));
				
				// Cleanup
				fs.unlinkSync('/work/chmod_test.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing chmod ===",
				"Changed permission to 0644",
				"Current mode: 644",
				"Changed permission to 0600",
				"Current mode: 600",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_error_invalid_paths",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Invalid Paths ===');
				
				// Test reading non-existent file
				try {
					fs.readFileSync('/nonexistent/path/file.txt');
					console.println('Read nonexistent: unexpected success');
				} catch (e) {
					console.println('Read nonexistent: error as expected');
				}
				
				// Test writing to non-existent directory
				try {
					fs.writeFileSync('/nonexistent/path/file.txt', 'data');
					console.println('Write to nonexistent dir: unexpected success');
				} catch (e) {
					console.println('Write to nonexistent dir: error as expected');
				}
				
				// Test rmdir on non-empty directory
				fs.mkdirSync('/work/nonempty_test');
				fs.writeFileSync('/work/nonempty_test/file.txt', 'data');
				try {
					fs.rmdirSync('/work/nonempty_test');
					console.println('Rmdir non-empty: unexpected success');
				} catch (e) {
					console.println('Rmdir non-empty: error as expected');
				}
				
				// Cleanup
				fs.unlinkSync('/work/nonempty_test/file.txt');
				fs.rmdirSync('/work/nonempty_test');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Invalid Paths ===",
				"Read nonexistent: error as expected",
				"Write to nonexistent dir: error as expected",
				"Rmdir non-empty: error as expected",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_stat_properties",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Stat Properties ===');
				
				// Create test file
				fs.writeFileSync('/work/stat_test.txt', 'test content');
				const stats = fs.statSync('/work/stat_test.txt');
				
				// Check properties
				console.println('isFile:', stats.isFile());
				console.println('isDirectory:', stats.isDirectory());
				console.println('isSymbolicLink:', stats.isSymbolicLink());
				console.println('size:', stats.size);
				console.println('Has mtime:', stats.mtime !== undefined);
				console.println('Has mode:', stats.mode !== undefined);
				
				// Create directory and check
				fs.mkdirSync('/work/stat_dir_test');
				const dirStats = fs.statSync('/work/stat_dir_test');
				console.println('Dir isDirectory:', dirStats.isDirectory());
				console.println('Dir isFile:', dirStats.isFile());
				
				// Cleanup
				fs.unlinkSync('/work/stat_test.txt');
				fs.rmdirSync('/work/stat_dir_test');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Stat Properties ===",
				"isFile: true",
				"isDirectory: false",
				"isSymbolicLink: false",
				"size: 12",
				"Has mtime: true",
				"Has mode: true",
				"Dir isDirectory: true",
				"Dir isFile: false",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_dirent_properties",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Dirent Properties ===');
				
				// Create test structure
				fs.mkdirSync('/work/dirent_test');
				fs.writeFileSync('/work/dirent_test/file.txt', 'test');
				fs.mkdirSync('/work/dirent_test/subdir');
				fs.symlinkSync('/work/dirent_test/file.txt', '/work/dirent_test/link.txt');
				
				// Read with withFileTypes
				const entries = fs.readdirSync('/work/dirent_test', { withFileTypes: true });
				
				// Filter and sort entries by name for consistent output
				const filtered = entries.filter(e => e.name !== '.' && e.name !== '..');
				filtered.sort((a, b) => a.name.localeCompare(b.name));
				
				filtered.forEach(entry => {
					console.println('Name:', entry.name);
					console.println('  isFile:', entry.isFile());
					console.println('  isDirectory:', entry.isDirectory());
				});
				
				// Cleanup
				fs.unlinkSync('/work/dirent_test/link.txt');
				fs.unlinkSync('/work/dirent_test/file.txt');
				fs.rmdirSync('/work/dirent_test/subdir');
				fs.rmdirSync('/work/dirent_test');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Dirent Properties ===",
				"Name: file.txt",
				"  isFile: true",
				"  isDirectory: false",
				"Name: link.txt",
				"  isFile: true",
				"  isDirectory: false",
				"Name: subdir",
				"  isFile: false",
				"  isDirectory: true",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_append_multiple",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Multiple Appends ===');
				
				// Create initial file
				fs.writeFileSync('/work/append_multi.txt', 'Line 1\n');
				
				// Append multiple times
				fs.appendFileSync('/work/append_multi.txt', 'Line 2\n');
				fs.appendFileSync('/work/append_multi.txt', 'Line 3\n');
				fs.appendFileSync('/work/append_multi.txt', 'Line 4\n');
				
				// Read and verify
				const content = fs.readFileSync('/work/append_multi.txt', 'utf8');
				const lines = content.split('\n').filter(l => l.length > 0);
				console.println('Number of lines:', lines.length);
				console.println('Line 1:', lines[0]);
				console.println('Line 2:', lines[1]);
				console.println('Line 3:', lines[2]);
				console.println('Line 4:', lines[3]);
				
				// Cleanup
				fs.unlinkSync('/work/append_multi.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Multiple Appends ===",
				"Number of lines: 4",
				"Line 1: Line 1",
				"Line 2: Line 2",
				"Line 3: Line 3",
				"Line 4: Line 4",
				"=== Test Complete ===",
			},
		},
		{
			name: "fs_open_modes",
			script: `
				const fs = require('fs');
				
				console.println('=== Testing Open Modes ===');
				
				// Test creating file with 'w+'
				const fd1 = fs.openSync('/work/open_modes.txt', 'w+');
				fs.writeSync(fd1, 'Hello');
				fs.closeSync(fd1);
				console.println('Created with w+');
				
				// Test appending with 'a+'
				const fd2 = fs.openSync('/work/open_modes.txt', 'a+');
				fs.writeSync(fd2, ' World');
				fs.closeSync(fd2);
				console.println('Appended with a+');
				
				// Read result
				const content = fs.readFileSync('/work/open_modes.txt', 'utf8');
				console.println('Final content:', content);
				
				// Cleanup
				fs.unlinkSync('/work/open_modes.txt');
				console.println('=== Test Complete ===');
			`,
			output: []string{
				"=== Testing Open Modes ===",
				"Created with w+",
				"Appended with a+",
				"Final content: Hello World",
				"=== Test Complete ===",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestFSStream(t *testing.T) {
	tests := []TestCase{
		{
			name: "fs_create_read_stream",
			script: `
				const fs = require('fs');
				
				// Create a test file
				const testData = 'Line 1\nLine 2\nLine 3\n';
				fs.writeFileSync('/work/stream_test.txt', testData, 'utf8');
				
				// Create read stream
				const readStream = fs.createReadStream('/work/stream_test.txt', { encoding: 'utf8', bufferSize: 7 });
				
				let data = '';
				readStream.on('data', chunk => {
					data += chunk;
				});
				
				readStream.on('end', () => {
					console.println('Read stream data:');
					console.println(data);
					
					// Cleanup
					fs.unlinkSync('/work/stream_test.txt');
				});
				
				readStream.on('error', err => {
					console.println('Read stream error:', err.message);
				});
			`,
			output: []string{
				"Read stream data:",
				"Line 1",
				"Line 2",
				"Line 3",
				"",
			},
		},
		{
			name: "fs_create_write_stream",
			script: `
				const fs = require('fs');

				// Create write stream
				const writeStream = fs.createWriteStream('/work/stream_write_test.txt', { encoding: 'utf8' });

				writeStream.on('finish', () => {
					console.println('Write stream finished');

					// Read back the file to verify
					const content = fs.readFileSync('/work/stream_write_test.txt', 'utf8');
					console.println('Written content:');
					console.println(content.trim());

					// Cleanup
					fs.unlinkSync('/work/stream_write_test.txt');
				});

				writeStream.on('error', err => {
					console.println('Write stream error:', err.message);
				});

				writeStream.write('First line\n');
				writeStream.write('Second line\n');
				writeStream.end('Third line\n');
			`,
			output: []string{
				"Write stream finished",
				"Written content:",
				"First line",
				"Second line",
				"Third line",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}
