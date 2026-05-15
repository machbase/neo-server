package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/engine"
	wsmod "github.com/machbase/neo-server/v8/jsh/lib/ws"
	"github.com/stretchr/testify/require"
)

func TestRouterCallHelpersWithoutEventLoop(t *testing.T) {
	router := &Router{ir: gin.New()}

	called := false
	require.NoError(t, router.callHandler(func(*RouterContext) { called = true }, nil))
	require.True(t, called)

	err := router.callHandler(func(*RouterContext) { panic("handler failed") }, nil)
	require.EqualError(t, err, "handler failed")

	accepted, err := router.callWebSocketVerify(func(*WebSocketRequest) bool { return true }, nil)
	require.NoError(t, err)
	require.True(t, accepted)

	accepted, err = router.callWebSocketVerify(func(*WebSocketRequest) bool { panic("verify failed") }, nil)
	require.EqualError(t, err, "verify failed")
	require.False(t, accepted)

	selected, err := router.callWebSocketProtocols(func(protocols []string, _ *WebSocketRequest) string {
		require.Equal(t, []string{"a", "b"}, protocols)
		return "b"
	}, []string{"a", "b"}, nil)
	require.NoError(t, err)
	require.Equal(t, "b", selected)

	selected, err = router.callWebSocketProtocols(func([]string, *WebSocketRequest) string { panic("protocol failed") }, nil, nil)
	require.EqualError(t, err, "protocol failed")
	require.Empty(t, selected)

	require.NoError(t, router.callWebSocketHandler(func(_ *wsmod.WebSocket, _ *WebSocketRequest) {}, nil, nil))
	err = router.callWebSocketHandler(func(_ *wsmod.WebSocket, _ *WebSocketRequest) { panic("websocket failed") }, nil, nil)
	require.EqualError(t, err, "websocket failed")
}

func TestRouterCallHelpersWithEventLoop(t *testing.T) {
	loop := engine.NewEventLoop()
	loop.Start()
	t.Cleanup(func() { loop.Stop() })
	router := &Router{ir: gin.New(), loop: loop}

	called := false
	require.NoError(t, router.callHandler(func(*RouterContext) { called = true }, nil))
	require.True(t, called)
	require.EqualError(t, router.callHandler(func(*RouterContext) { panic("handler failed") }, nil), "handler failed")

	accepted, err := router.callWebSocketVerify(func(*WebSocketRequest) bool { return true }, nil)
	require.NoError(t, err)
	require.True(t, accepted)

	accepted, err = router.callWebSocketVerify(func(*WebSocketRequest) bool { panic("verify failed") }, nil)
	require.EqualError(t, err, "verify failed")
	require.False(t, accepted)

	selected, err := router.callWebSocketProtocols(func(protocols []string, _ *WebSocketRequest) string {
		return protocols[0]
	}, []string{"proto"}, nil)
	require.NoError(t, err)
	require.Equal(t, "proto", selected)

	selected, err = router.callWebSocketProtocols(func([]string, *WebSocketRequest) string { panic("protocol failed") }, nil, nil)
	require.EqualError(t, err, "protocol failed")
	require.Empty(t, selected)

	require.NoError(t, router.callWebSocketHandler(func(_ *wsmod.WebSocket, _ *WebSocketRequest) {}, nil, nil))
	require.EqualError(t, router.callWebSocketHandler(func(_ *wsmod.WebSocket, _ *WebSocketRequest) { panic("websocket failed") }, nil, nil), "websocket failed")
}
