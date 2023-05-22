package main

import (
	"testing"

	"github.com/d5/tengo/v2/require"
)

func TestBestGuessBindAddress(t *testing.T) {
	g := guessBindAddress([]string{"--data", "some/data/path/db", "--host", "0.0.0.0"})
	require.Equal(t, "127.0.0.1:5654", g.httpAddr)
	require.Equal(t, "127.0.0.1:5655", g.grpcAddr)

	g = guessBindAddress([]string{"--data", "some/data/path/db", "--host=0.0.0.0"})
	require.Equal(t, "127.0.0.1:5654", g.httpAddr)
	require.Equal(t, "127.0.0.1:5655", g.grpcAddr)

	g = guessBindAddress([]string{"--data", "some/data/path/db", "--host=0.0.0.0", "--http-port", "8080"})
	require.Equal(t, "127.0.0.1:8080", g.httpAddr)
	require.Equal(t, "127.0.0.1:5655", g.grpcAddr)

	g = guessBindAddress([]string{"--data=some/data/path/db", "--host=0.0.0.0", "--grpc-port=1234"})
	require.Equal(t, "127.0.0.1:5654", g.httpAddr)
	require.Equal(t, "127.0.0.1:1234", g.grpcAddr)
}
