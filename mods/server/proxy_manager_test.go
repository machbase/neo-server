package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestProxyManagerRegisterValidationAndConflict(t *testing.T) {
	pm := NewProxyManager()

	_, err := pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api/", Target: "https://127.0.0.1:8080"})
	require.Error(t, err)

	_, err = pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api/", Target: "http://example.com:8080"})
	require.Error(t, err)

	first, err := pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api", Target: "http://127.0.0.1:8080"})
	require.NoError(t, err)
	require.Equal(t, "/api/", first.Prefix)

	second, err := pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api/", Target: "http://127.0.0.1:8080"})
	require.NoError(t, err)
	require.Equal(t, first.RegisteredAt, second.RegisteredAt)

	_, err = pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api/", Target: "http://127.0.0.1:9090"})
	require.Error(t, err)

	_, err = pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api/foo/", Target: "http://127.0.0.1:8080"})
	require.NoError(t, err)

	_, err = pm.Register(ProxyRegisterRequest{Service: "demo/api", Prefix: "/foo/", Target: "http://127.0.0.1:8080"})
	require.Error(t, err)
}

func TestProxyManagerGetAndUnregister(t *testing.T) {
	pm := NewProxyManager()
	_, err := pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/api", Target: "http://127.0.0.1:8080"})
	require.NoError(t, err)

	entry, err := pm.Get(ProxyGetRequest{Service: "demo", Prefix: "/api/"})
	require.NoError(t, err)
	require.Equal(t, "demo", entry.Service)
	require.Equal(t, "/api/", entry.Prefix)
	require.Equal(t, "http://127.0.0.1:8080", entry.Target)

	_, err = pm.Get(ProxyGetRequest{Service: "demo", Prefix: "/missing/"})
	require.ErrorIs(t, err, errProxyNotFound)

	removed, err := pm.Unregister(ProxyUnregisterRequest{Service: "demo", Prefix: "/api"})
	require.NoError(t, err)
	require.Len(t, removed, 1)

	_, err = pm.Get(ProxyGetRequest{Service: "demo", Prefix: "/api/"})
	require.ErrorIs(t, err, errProxyNotFound)
}

func TestProxyManagerHandleProxiesRegisteredServicePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := make(chan string, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- fmt.Sprintf("%s|%s|%s", r.URL.String(), r.Header.Get("X-Forwarded-Host"), r.Header.Get("X-Forwarded-Prefix"))
		_, _ = w.Write([]byte("proxied"))
	}))
	defer target.Close()

	pm := NewProxyManager()
	_, err := pm.Register(ProxyRegisterRequest{Service: "github.com/acme/chart", Prefix: "/api/", Target: target.URL})
	require.NoError(t, err)

	router := gin.New()
	router.Any("/web/services/*path", func(ctx *gin.Context) {
		pm.Handle(ctx, ctx.Param("path"))
	})
	frontend := httptest.NewServer(router)
	defer frontend.Close()

	req, err := http.NewRequest(http.MethodGet, frontend.URL+"/web/services/github.com/acme/chart/api/v1/resource?q=ok", nil)
	require.NoError(t, err)
	req.Host = "neo.local"
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer rsp.Body.Close()
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "proxied", string(body))
	require.Equal(t, "/v1/resource?q=ok|neo.local|/web/services/github.com/acme/chart/api", <-seen)
}

func TestProxyManagerHandleProxiesUnixSocketTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix domain sockets are not supported on Windows")
	}
	gin.SetMode(gin.TestMode)
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("neo-proxy-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer os.Remove(socketPath)

	seen := make(chan string, 1)
	target := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- fmt.Sprintf("%s|%s", r.URL.String(), r.Header.Get("X-Forwarded-Prefix"))
		_, _ = w.Write([]byte("unix-proxied"))
	})}
	go func() { _ = target.Serve(listener) }()
	defer target.Close()

	pm := NewProxyManager()
	_, err = pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/uds/", Target: "unix://" + socketPath})
	require.NoError(t, err)

	router := gin.New()
	router.Any("/web/services/*path", func(ctx *gin.Context) {
		pm.Handle(ctx, ctx.Param("path"))
	})
	frontend := httptest.NewServer(router)
	defer frontend.Close()

	rsp, err := http.Get(frontend.URL + "/web/services/demo/uds/v1/resource?q=ok")
	require.NoError(t, err)
	defer rsp.Body.Close()
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "unix-proxied", string(body))
	require.Equal(t, "/v1/resource?q=ok|/web/services/demo/uds", <-seen)
}

func TestProxyManagerHandleUnregisteredPathDoesNotFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pm := NewProxyManager()
	router := gin.New()
	router.Any("/web/services/*path", func(ctx *gin.Context) {
		pm.Handle(ctx, ctx.Param("path"))
	})

	req := httptest.NewRequest(http.MethodGet, "/web/services/demo/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "proxy not registered")
}

func TestProxyManagerHandleProxiesServerSentEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/events", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, _ = w.Write([]byte("data: ready\n\n"))
		flusher.Flush()
	}))
	defer target.Close()

	pm := NewProxyManager()
	_, err := pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/sse/", Target: target.URL})
	require.NoError(t, err)

	router := gin.New()
	router.Any("/web/services/*path", func(ctx *gin.Context) {
		pm.Handle(ctx, ctx.Param("path"))
	})
	frontend := httptest.NewServer(router)
	defer frontend.Close()

	rsp, err := http.Get(frontend.URL + "/web/services/demo/sse/events")
	require.NoError(t, err)
	defer rsp.Body.Close()

	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "text/event-stream", rsp.Header.Get("Content-Type"))
	reader := bufio.NewReader(rsp.Body)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "data: ready\n", line)
}

func TestProxyManagerHandleProxiesWebSocketUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upgrader := websocket.Upgrader{}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/echo", r.URL.Path)
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		messageType, message, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(messageType, append([]byte("proxied:"), message...)))
	}))
	defer target.Close()

	pm := NewProxyManager()
	_, err := pm.Register(ProxyRegisterRequest{Service: "demo", Prefix: "/ws/", Target: target.URL})
	require.NoError(t, err)

	router := gin.New()
	router.Any("/web/services/*path", func(ctx *gin.Context) {
		pm.Handle(ctx, ctx.Param("path"))
	})
	frontend := httptest.NewServer(router)
	defer frontend.Close()

	wsURL := "ws" + frontend.URL[len("http"):] + "/web/services/demo/ws/echo"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("hello")))
	messageType, message, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, messageType)
	require.Equal(t, "proxied:hello", string(message))
}
