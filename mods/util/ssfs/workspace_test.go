package ssfs

import (
	"os"
	"path/filepath"
	"testing"

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
	fs, err := NewServerSideFileSystem(dirs)
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

	// nested mount shadowed by a real directory in the parent mount
	require.NoError(t, os.Mkdir(filepath.Join(root, "a"), 0755))
	warns = fs.CheckWorkMounts()
	require.Len(t, warns, 2)
	require.Contains(t, warns[0], `"/work/a" conflicts`)
}

func TestWorkFSTabsNil(t *testing.T) {
	var fs *SSFS
	require.Empty(t, fs.WorkFSTabs())
	_, ok := fs.RootDir()
	require.False(t, ok)
}
