package jsh

import (
	"fmt"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

func TestServices(t *testing.T) {
	serverFs, _ := ssfs.NewServerSideFileSystem([]string{"/=./test", "/etc/services=./test/etc_services"})
	ssfs.SetDefault(serverFs)

	list, err := ReadServices()
	require.NoError(t, err)

	fmt.Println("==>", list.Errors[0].ReadError)
	require.Equal(t, "wrong1", list.Errors[0].Name)
	require.Equal(t, "json: cannot unmarshal string into Go struct field ServiceConfig.stop_args of type []string", list.Errors[0].ReadError.Error())

	require.Equal(t, 1, len(list.Added))
	require.NoError(t, list.Added[0].ReadError)
	require.Equal(t, "svc1", list.Added[0].Name)
	require.Equal(t, "/sbin/svc.js", list.Added[0].StartCmd)
	require.Equal(t, []string{"start", "arg1"}, list.Added[0].StartArgs)

	require.Equal(t, "/sbin/svc.js", list.Added[0].StopCmd)
	require.Equal(t, []string{"stop"}, list.Added[0].StopArgs)
}
