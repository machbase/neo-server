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
	require.Equal(t, "select.sql", ret.Children[1].Name)

	ssfs, err = NewServerSideFileSystem([]string{"./test/root", "./test/data1"})
	require.Nil(t, err)
	require.NotNil(t, ssfs)

	ret, err = ssfs.Get("/")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/", ret.Name)
	require.Equal(t, 4, len(ret.Children))
	require.Equal(t, "/data1/", ret.Children[0].Name)
	require.Equal(t, true, ret.Children[0].IsDir)
	require.Equal(t, "example.tql", ret.Children[1].Name)
	require.Equal(t, false, ret.Children[1].IsDir)
	require.Equal(t, "hello.sql", ret.Children[2].Name)
	require.Equal(t, false, ret.Children[2].IsDir)
	require.Equal(t, "select.sql", ret.Children[3].Name)
	require.Equal(t, false, ret.Children[3].IsDir)

	ret, err = ssfs.Get("/data1/")
	require.Nil(t, err)
	require.NotNil(t, ssfs)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/data1/", ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	ret, err = ssfs.MkDir("/data1/newdir")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "newdir", ret.Name)

	err = ssfs.Set("/data1/newdir/test.txt", []byte("Hello World"))
	require.Nil(t, err)

	ret, err = ssfs.Get("/data1/newdir/test.txt")
	require.Nil(t, err)
	require.Equal(t, "test.txt", ret.Name)
	require.Equal(t, "Hello World", string(ret.Content))

	err = ssfs.Remove("/data1/newdir/test.txt")
	require.Nil(t, err)

	err = ssfs.Remove("/data1/newdir")
	require.Nil(t, err)
}
