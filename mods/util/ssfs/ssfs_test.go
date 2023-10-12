package ssfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFsGET(t *testing.T) {
	ssfs, err := NewServerSideFileSystem([]string{"./test/root"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ret, err := ssfs.GetGlob("/", "*.sql")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, string(os.PathSeparator), ret.Name)
	require.Equal(t, 3, len(ret.Children))
	require.Equal(t, "hello.sql", ret.Children[0].Name)
	require.Equal(t, ".sql", ret.Children[0].Type)
	require.Equal(t, "select.sql", ret.Children[1].Name)
	require.Equal(t, ".sql", ret.Children[1].Type)
	require.Equal(t, "Tutorials", ret.Children[2].Name)
	require.Equal(t, urlGitSample, ret.Children[2].GitUrl)
	require.True(t, ret.Children[2].Virtual)

	ssfs, err = NewServerSideFileSystem([]string{"./test/root", "./test/data1"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ret, err = ssfs.Get("/")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, string(os.PathSeparator), ret.Name)
	require.Equal(t, 5, len(ret.Children))
	require.Equal(t, fmt.Sprintf("%sdata1", string(os.PathSeparator)), ret.Children[0].Name)
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
	require.Equal(t, "Tutorials", ret.Children[4].Name)
	require.Equal(t, urlGitSample, ret.Children[4].GitUrl)

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
	require.Equal(t, fmt.Sprintf("%sdata1", string(os.PathSeparator)), ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	// directory with trailing slash
	ret, err = ssfs.Get("/data1/")
	require.Nil(t, err)
	require.NotNil(t, ssfs)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, fmt.Sprintf("%sdata1", string(os.PathSeparator)), ret.Name)
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
