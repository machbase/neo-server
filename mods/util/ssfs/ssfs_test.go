package ssfs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFsGET(t *testing.T) {
	ssfs, err := NewServerSideFileSystem([]string{"/=./test/root"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ret, err := ssfs.GetGlob("/", "*.sql")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/", ret.Name)
	if enableGitSample {
		require.Equal(t, 3, len(ret.Children))
	} else {
		require.Equal(t, 2, len(ret.Children))
	}
	require.Equal(t, "hello.sql", ret.Children[0].Name)
	require.Equal(t, ".sql", ret.Children[0].Type)
	require.Equal(t, "select.sql", ret.Children[1].Name)
	require.Equal(t, ".sql", ret.Children[1].Type)
	if enableGitSample {
		require.Equal(t, 3, len(ret.Children))
		require.Equal(t, "Tutorials", ret.Children[2].Name)
		require.Equal(t, urlGitSample, ret.Children[2].GitUrl)
		require.True(t, ret.Children[2].Virtual)
	}

	ssfs, err = NewServerSideFileSystem([]string{"/=./test/root", "/data1=./test/data1"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ret, err = ssfs.Get("/")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/", ret.Name)
	if enableGitSample {
		require.Equal(t, 5, len(ret.Children))
	} else {
		require.Equal(t, 4, len(ret.Children))
	}
	require.Equal(t, "data1", ret.Children[0].Name)
	require.Equal(t, "dir", ret.Children[0].Type)
	require.Equal(t, true, ret.Children[0].IsDir)
	require.Equal(t, "example.tql", ret.Children[1].Name)
	require.Equal(t, ".tql", ret.Children[1].Type)
	require.Equal(t, false, ret.Children[1].IsDir)
	require.Equal(t, "hello.sql", ret.Children[2].Name)
	require.Equal(t, ".sql", ret.Children[2].Type)
	require.Equal(t, false, ret.Children[2].IsDir)
	require.Equal(t, "select.sql", ret.Children[3].Name)
	require.Equal(t, ".sql", ret.Children[3].Type)
	require.Equal(t, false, ret.Children[3].IsDir)
	if enableGitSample {
		require.Equal(t, "Tutorials", ret.Children[4].Name)
		require.Equal(t, urlGitSample, ret.Children[4].GitUrl)
	}

	// do not allow accessing out side of the given dirs
	ret, err = ssfs.Get("/../notaccess.tql")
	require.NotNil(t, err)
	require.Nil(t, ret)

	// do not allow accessing out side of the given dirs
	ret, err = ssfs.Get("/data1/../../notaccess.tql")
	require.NotNil(t, err)
	require.Nil(t, ret)

	// access relative path
	ret, err = ssfs.Get("/data1/../example.tql")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, "example.tql", ret.Name)

	// directory without trailing slash
	ret, err = ssfs.Get("/data1")
	require.Nil(t, err)
	require.NotNil(t, ssfs)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "data1", ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	// directory with trailing slash
	ret, err = ssfs.Get("/data1/")
	require.Nil(t, err)
	require.NotNil(t, ssfs)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "data1", ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	// create subdirectory
	ret, err = ssfs.MkDir("/data1/newdir")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "newdir", ret.Name)

	// write file
	err = ssfs.Set("/data1/newdir/test.txt", []byte("Hello World"))
	require.Nil(t, err)

	// read file
	ret, err = ssfs.Get("/data1/newdir/test.txt")
	require.Nil(t, err)
	require.Equal(t, "test.txt", ret.Name)
	require.Equal(t, "Hello World", string(ret.Content))

	// subdir sort
	ret, err = ssfs.MkDir("/data1/newdir2")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "newdir2", ret.Name)

	ret, err = ssfs.Get("/data1/")
	require.Nil(t, err)
	require.Equal(t, "newdir", ret.Children[0].Name)
	require.Equal(t, "newdir2", ret.Children[1].Name)

	// rename file
	err = ssfs.Rename("/data1/newdir/test.txt", "/test.txt")
	require.Nil(t, err)

	// delete file
	err = ssfs.Remove("/test.txt")
	require.Nil(t, err)

	// delete directory
	err = ssfs.Remove("/data1/newdir")
	require.Nil(t, err)
	err = ssfs.Remove("/data1/newdir2")
	require.Nil(t, err)

	// RealPath()
	realpath, err := ssfs.RealPath("/data1/simple.tql")
	require.Nil(t, err)
	abspath, _ := filepath.Abs("test/data1/simple.tql")
	require.Equal(t, realpath, abspath)
}

func TestFsGit(t *testing.T) {
	ssfs, err := NewServerSideFileSystem([]string{"./test/root", "./test/data1"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	dest := "/data1/neo-tutorials"
	entry, err := ssfs.GitClone(dest, urlGitSample, nil)
	if err != nil {
		t.Log("ERR", err.Error())
	}
	require.Nil(t, err)
	require.NotNil(t, entry)

	require.True(t, entry.IsDir)
	require.True(t, len(entry.Children) > 0)

	entry, err = ssfs.GitPull(dest, urlGitSample, nil)
	if err != nil {
		t.Log("ERR", err.Error())
	}
	require.Nil(t, err)
	require.NotNil(t, entry)

	err = ssfs.RemoveRecursive(dest)
	require.Nil(t, err)
}

func TestMounts(t *testing.T) {
	ssfs, err := NewServerSideFileSystem([]string{"/=./test/root", "data1=./test/data1", "apps/neo-docs/=./test/apps/neo-docs"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ssfs.Unmount("/apps/neo-docs")
	mnts := ssfs.ListMounts()
	require.Equal(t, []string{"/", "/data1"}, mnts)

	err = ssfs.Unmount("/")
	require.Error(t, err)
	err = ssfs.Unmount("")
	require.Error(t, err)

	err = ssfs.Mount("data1/apps/neo-docs", "./test/apps/neo-docs", true)
	require.NoError(t, err)

	mnts = ssfs.ListMounts()
	require.Equal(t, []string{"/", "/data1", "/data1/apps/neo-docs"}, mnts)

	bd := ssfs.FindBaseDir("/data1/simple.tql")
	require.NotNil(t, bd)
	require.Equal(t, "/data1", bd.mountPoint)

	neo_docs_bd := ssfs.FindBaseDir("/data1/apps/neo-docs/simple.tql")
	require.NotNil(t, bd)
	require.Equal(t, "/data1/apps/neo-docs", neo_docs_bd.mountPoint)
	require.False(t, bd.ReadOnly())

	rp, err := ssfs.FindRealPath("/data1/apps/neo-docs")
	require.Nil(t, err)
	require.Equal(t, neo_docs_bd.abspath, rp.AbsPath)
	require.True(t, rp.ReadOnly)

	rp, err = ssfs.FindRealPath("/data1/apps/neo-docs/simple.tql")
	require.Nil(t, err)
	require.Equal(t, filepath.Join(neo_docs_bd.abspath, "simple.tql"), rp.AbsPath)
	require.False(t, bd.ReadOnly())
}

func TestMountsMultiDepth(t *testing.T) {
	ssfs, err := NewServerSideFileSystem(nil)
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ssfs.Mount("/", "./test/root", false)
	ssfs.Mount("apps/myapp1", "./test/apps/myapp1", true)
	ssfs.Mount("apps/myapp2", "./test/apps/myapp2", true)
	ssfs.Mount("apps/central/myapp4", "./test/apps/central/myapp4", true)
	ssfs.Mount("apps/central/myapp3", "./test/apps/central/myapp3", true)

	mnts := ssfs.ListMounts()
	require.Equal(t, []string{"/", "/apps/central/myapp3", "/apps/central/myapp4", "/apps/myapp1", "/apps/myapp2"}, mnts)

	entry, err := ssfs.Get("/")
	require.Nil(t, err)
	require.NotNil(t, entry)
	childrenNames := []string{}
	for _, ent := range entry.Children {
		childrenNames = append(childrenNames, ent.Name)
	}
	require.Equal(t, []string{"apps", "example.tql", "hello.sql", "select.sql"}, childrenNames)
	require.False(t, entry.ReadOnly)

	entry, err = ssfs.Get("/apps")
	require.Nil(t, err)
	require.NotNil(t, entry)
	childrenNames = []string{}
	for _, ent := range entry.Children {
		childrenNames = append(childrenNames, ent.Name)
	}
	require.Equal(t, []string{"central", "myapp1", "myapp2"}, childrenNames)
	require.True(t, entry.ReadOnly)

	entry, err = ssfs.Get("/apps/central")
	require.Nil(t, err)
	require.NotNil(t, entry)
	childrenNames = []string{}
	for _, ent := range entry.Children {
		childrenNames = append(childrenNames, ent.Name)
	}
	require.Equal(t, []string{"myapp3", "myapp4"}, childrenNames)
	require.True(t, entry.ReadOnly)
}
