package tql

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetGrpcAddresses(t *testing.T) {
	_grpcServer = ""
	SetGrpcAddresses([]string{"tcp://10.0.0.1:9999", "tcp://127.0.0.1:5656"})
	require.Equal(t, "tcp://127.0.0.1:5656", _grpcServer)

	_grpcServer = ""
	SetGrpcAddresses([]string{"tcp://10.0.0.1:9999", "unix:///tmp/neo.sock", "tcp://127.0.0.1:5656"})
	if runtime.GOOS == "windows" {
		require.Equal(t, "tcp://127.0.0.1:5656", _grpcServer)
	} else {
		require.Equal(t, "unix:///tmp/neo.sock", _grpcServer)
	}
}

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
