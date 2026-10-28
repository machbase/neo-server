package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/stretchr/testify/require"
)

func TestParseFileDirs(t *testing.T) {
	abs := func(p string) string {
		ret, err := filepath.Abs(p)
		require.NoError(t, err)
		return ret
	}
	tests := []struct {
		name   string
		values []string
		expect []MountSpec
		err    bool
	}{
		{name: "default root", values: []string{"/=/dir"}, expect: []MountSpec{{"/", abs("/dir")}}},
		{name: "wrapped multi mounts", values: []string{"/=/=/dir,/a=/other"},
			expect: []MountSpec{{"/", abs("/dir")}, {"/a", abs("/other")}}},
		{name: "wrapped root and bare mount", values: []string{"/=/dir,/a=/other,/b/c=/third"},
			expect: []MountSpec{{"/", abs("/dir")}, {"/a", abs("/other")}, {"/b/c", abs("/third")}}},
		{name: "wrapped sub mount only", values: []string{"/=/a=/other"}, expect: []MountSpec{{"/a", abs("/other")}}},
		{name: "semicolon is not a separator", values: []string{"/=/dir;x"},
			expect: []MountSpec{{"/", abs("/dir;x")}}},
		{name: "bare path uses base name", values: []string{"/=/dir,/x/data"},
			expect: []MountSpec{{"/", abs("/dir")}, {"/data", abs("/x/data")}}},
		{name: "normalize mount point", values: []string{"/=/dir,a/b/=/other"},
			expect: []MountSpec{{"/", abs("/dir")}, {"/a/b", abs("/other")}}},
		{name: "relative dir", values: []string{"/=./rel"}, expect: []MountSpec{{"/", abs("./rel")}}},
		{name: "empty entries ignored", values: []string{"/=/dir,,", ""}, expect: []MountSpec{{"/", abs("/dir")}}},
		{name: "escape work", values: []string{"/=/dir,/../etc=/other"}, err: true},
		{name: "duplicate", values: []string{"/=/dir,/=/other"}, err: true},
		{name: "duplicate normalized", values: []string{"/=/dir,/a=/x,a/=/y"}, err: true},
		{name: "empty dir", values: []string{"/=/dir,/a="}, err: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			specs, err := ParseFileDirs(tc.values)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, specs)
		})
	}
}

func TestNormalizeFileDirsIdempotent(t *testing.T) {
	first, err := NormalizeFileDirs([]string{"/=/=/dir,/a=/other"})
	require.NoError(t, err)
	second, err := NormalizeFileDirs(first)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Len(t, first, 2)
}

func TestWorkPath(t *testing.T) {
	require.Equal(t, "/work", WorkPath("/"))
	require.Equal(t, "/work", WorkPath(""))
	require.Equal(t, "/work/a", WorkPath("/a"))
	require.Equal(t, "/work/a/b", WorkPath("a/b/"))
}

func TestWorkFSTabs(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	third := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")

	dirs, err := NormalizeFileDirs([]string{"/=" + root + ",/b/c=" + third + ",/a=" + other + ",/m=" + missing})
	require.NoError(t, err)
	fs, err := NewWorkspaceFileSystem(dirs)
	require.NoError(t, err)

	rootDir, ok := fs.RootDir()
	require.True(t, ok)
	require.Equal(t, root, rootDir)

	tabs := fs.WorkFSTabs()
	require.Len(t, tabs, 3)
	require.Equal(t, "/work", tabs[0].MountPoint)
	require.Equal(t, root, tabs[0].Source)
	require.Equal(t, "/work/a", tabs[1].MountPoint)
	require.Equal(t, other, tabs[1].Source)
	require.Equal(t, "/work/b/c", tabs[2].MountPoint)
	require.Equal(t, third, tabs[2].Source)

	require.Equal(t, []string{
		"-v", "/work=" + root,
		"-v", "/work/a=" + other,
		"-v", "/work/b/c=" + third,
	}, fs.WorkMountArgs())

	warns := fs.CheckWorkMounts()
	require.Len(t, warns, 1)
	require.Contains(t, warns[0], `"/work/m"`)
	require.NoError(t, fs.ValidateWorkMounts())
}

func TestValidateWorkMounts(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	deep := t.TempDir()
	newFs := func(values ...string) *WorkspaceFileSystem {
		dirs, err := NormalizeFileDirs(values)
		require.NoError(t, err)
		fs, err := NewWorkspaceFileSystem(dirs)
		require.NoError(t, err)
		return fs
	}

	// no real entry at the nested path
	rootFs := newFs("/="+root, "/a="+other)
	require.NoError(t, rootFs.ValidateWorkMounts())
	// sibling prefix is not nested ("/ab" is not under "/a")
	require.NoError(t, os.Mkdir(filepath.Join(other, "b"), 0755))
	require.NoError(t, newFs("/a="+other, "/ab="+deep).ValidateWorkMounts())

	// nested mount over a real directory of the root mount
	require.NoError(t, os.Mkdir(filepath.Join(root, "a"), 0755))
	err := rootFs.ValidateWorkMounts()
	require.Error(t, err)
	require.Contains(t, err.Error(), `"/work/a"`)
	_, err = NewWorkspaceFileSystem([]string{"/=" + root, "/a=" + other})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nested file mount conflict")

	// nested mount over a real file of a non-root mount, regardless of order
	fileFs := newFs("/x/f="+deep, "/x="+other)
	require.NoError(t, os.WriteFile(filepath.Join(other, "f"), []byte("x"), 0644))
	err = fileFs.ValidateWorkMounts()
	require.Error(t, err)
	require.Contains(t, err.Error(), `"/work/x/f"`)

	// only the intermediate directory exists: not a conflict
	deepFs := newFs("/="+root, "/a/b="+deep)
	require.NoError(t, deepFs.ValidateWorkMounts())
	require.NoError(t, os.Mkdir(filepath.Join(root, "a", "b"), 0755))
	err = deepFs.ValidateWorkMounts()
	require.Error(t, err)
	require.Contains(t, err.Error(), `"/work/a/b"`)

	// an intermediate path component is a real file, regardless of order
	fileRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fileRoot, "f"), []byte("x"), 0644))
	for _, values := range [][]string{
		{"/=" + fileRoot, "/f/b=" + deep},
		{"/f/b=" + deep, "/=" + fileRoot},
	} {
		_, err = NewWorkspaceFileSystem(values)
		require.Error(t, err)
		require.Contains(t, err.Error(), "nested file mount conflict")
	}
	lateFileRoot := t.TempDir()
	lateFs := newFs("/="+lateFileRoot, "/f/b="+deep)
	require.NoError(t, os.WriteFile(filepath.Join(lateFileRoot, "f"), []byte("x"), 0644))
	err = lateFs.ValidateWorkMounts()
	require.Error(t, err)
	require.Contains(t, err.Error(), `"/work/f/b"`)
}

func TestProcessWorkFSTabsRootless(t *testing.T) {
	other := t.TempDir()
	dirs, err := NormalizeFileDirs([]string{"/a=" + other})
	require.NoError(t, err)
	fs, err := NewWorkspaceFileSystem(dirs)
	require.NoError(t, err)

	require.Equal(t, engine.FSTabs{{MountPoint: "/work/a", Source: other}}, fs.WorkFSTabs())
	require.Equal(t, engine.FSTabs{
		{MountPoint: "/work"},
		{MountPoint: "/work/a", Source: other},
	}, fs.ProcessWorkFSTabs())
	require.Equal(t, []string{"-v", "/work=", "-v", "/work/a=" + other}, fs.WorkMountArgs())
}

func TestWorkFSTabsNil(t *testing.T) {
	var fs *WorkspaceFileSystem
	require.Empty(t, fs.WorkFSTabs())
	require.Equal(t, engine.FSTabs{{MountPoint: "/work"}}, fs.ProcessWorkFSTabs())
	_, ok := fs.RootDir()
	require.False(t, ok)
}
