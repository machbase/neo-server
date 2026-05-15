package http

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/stretchr/testify/require"
)

func TestNewServerWithEventLoop(t *testing.T) {
	SetDefaultRouter(nil)
	t.Cleanup(func() { SetDefaultRouter(nil) })

	_, err := NewServer(map[string]any{})
	require.EqualError(t, err, "http.NewServer: address is not set")

	loop := engine.NewEventLoop()
	defaultRouter := gin.New()
	SetDefaultRouter(defaultRouter)
	listener, err := NewServerWithEventLoop(map[string]any{}, loop)
	require.NoError(t, err)
	require.IsType(t, &PListener{}, listener)
	require.Same(t, loop, listener.Router().loop)
	require.Same(t, defaultRouter, listener.Router().ir)

	listener, err = NewServerWithEventLoop(map[string]any{"network": "tcp", "address": "127.0.0.1:0"}, loop)
	require.NoError(t, err)
	require.IsType(t, &RListener{}, listener)
	require.Same(t, loop, listener.Router().loop)

	called := false
	require.NoError(t, listener.Serve(func(info map[string]any) {
		called = true
		require.Equal(t, "tcp", info["network"])
		require.NotEmpty(t, info["address"])
	}))
	require.True(t, called)
	require.EqualError(t, listener.Serve(nil), "http.Listener.Listen: already serving")
	listener.Close()
	time.Sleep(10 * time.Millisecond)
}
