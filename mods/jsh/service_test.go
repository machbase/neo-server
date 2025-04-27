package jsh

import (
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

func TestServices(t *testing.T) {
	serverFs, _ := ssfs.NewServerSideFileSystem([]string{"/=./test", "/etc/services=./test/etc_services"})
	ssfs.SetDefault(serverFs)

	list, err := ReadServices()
	require.NoError(t, err)
	require.Equal(t, 1, len(list.Added))
	require.Equal(t, "svc1", list.Added[0].Name)
	require.Equal(t, 1, len(list.Errors))
	require.Equal(t, "wrong1", list.Errors[0].Name)
	require.Equal(t, "json: cannot unmarshal string into Go struct field ServiceConfig.stop_args of type []string", list.Errors[0].ReadError.Error())
}
