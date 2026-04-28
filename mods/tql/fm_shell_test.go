package tql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetHttpAddresses(t *testing.T) {
	_httpServer = ""
	SetHttpAddresses([]string{"tcp://10.0.0.1:8888", "tcp://127.0.0.1:7777", "tcp://127.0.0.1:6666"})
	require.Equal(t, "tcp://127.0.0.1:7777", _httpServer)

	_httpServer = ""
	SetHttpAddresses([]string{"http://example.com", "http://other.example.com"})
	require.Equal(t, "http://other.example.com", _httpServer)
}

func TestSetServiceControllerAddress(t *testing.T) {
	_serviceControllerAddr = ""
	SetServiceControllerAddress("unix:///tmp/controller.sock")
	require.Equal(t, "unix:///tmp/controller.sock", _serviceControllerAddr)
}
