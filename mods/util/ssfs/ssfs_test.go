package ssfs

import (
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
	require.Equal(t, "/", ret.Name)
	require.Equal(t, 2, len(ret.Children))
	require.Equal(t, "hello.sql", ret.Children[0].Name)
	require.Equal(t, ".sql", ret.Children[0].Type)
	require.Equal(t, "select.sql", ret.Children[1].Name)
	require.Equal(t, ".sql", ret.Children[1].Type)

	ssfs, err = NewServerSideFileSystem([]string{"./test/root", "./test/data1"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ret, err = ssfs.Get("/")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/", ret.Name)
	require.Equal(t, 4, len(ret.Children))
	require.Equal(t, "/data1", ret.Children[0].Name)
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
	require.Equal(t, "/data1", ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	// directory with trailing slash
	ret, err = ssfs.Get("/data1/")
	require.Nil(t, err)
	require.NotNil(t, ssfs)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/data1", ret.Name)
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

	// delete file
	err = ssfs.Remove("/data1/newdir/test.txt")
	require.Nil(t, err)

	// delete directory
	err = ssfs.Remove("/data1/newdir")
	require.Nil(t, err)
}
