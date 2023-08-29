package util_test

import (
	"net"
	"testing"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestGetInterfaceAddr(t *testing.T) {
	lst := util.GetAllAddresses()
	for i, a := range lst {
		t.Logf("[%d] %v", i, a)
	}
}

func TestFindAllAddresses(t *testing.T) {
	lst := util.FindAllAddresses(net.IPv4(127, 0, 0, 1))
	require.Equal(t, 1, len(lst))
	require.True(t, lst[0].IP.IsLoopback())
}

func TestFindAllAddressesAllBind(t *testing.T) {
	lst := util.FindAllAddresses(net.IPv4zero)
	for i, a := range lst {
		t.Logf("[%d] %v", i, a)
	}
}
