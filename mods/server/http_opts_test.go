package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
)

func newHttpdForOptionTest() *httpd {
	return &httpd{
		log:        logging.GetLog("http-opts-test"),
		authServer: &Server{},
		pathMap:    map[string]string{},
	}
}

func TestWithHttpAuthServer(t *testing.T) {
	t.Run("enabled_with_rpc_controller", func(t *testing.T) {
		authSvc := &Server{neoShellAddress: "before", rpcController: &service.Controller{}}
		h := newHttpdForOptionTest()

		WithHttpAuthServer(authSvc, true)(h)

		require.Same(t, authSvc, h.authServer)
		require.True(t, h.enableTokenAuth)
		require.Same(t, authSvc.rpcController, h.rpcController)
	})

	t.Run("disabled_nil_service", func(t *testing.T) {
		h := newHttpdForOptionTest()
		h.rpcController = &service.Controller{}

		WithHttpAuthServer(nil, false)(h)

		require.Nil(t, h.authServer)
		require.False(t, h.enableTokenAuth)
		require.NotNil(t, h.rpcController)
	})
}

func TestWithHttpNeoShellAddress(t *testing.T) {
	t.Run("prefers_loopback", func(t *testing.T) {
		h := newHttpdForOptionTest()
		WithHttpNeoShellAddress("tcp://10.0.0.8:5655", "tcp://127.0.0.1:7777", "unix:///tmp/test.sock")(h)
		require.Equal(t, "127.0.0.1:7777", h.authServer.neoShellAddress)
	})

	t.Run("falls_back_to_first_tcp_candidate", func(t *testing.T) {
		h := newHttpdForOptionTest()
		WithHttpNeoShellAddress("http://example.com", "tcp://192.168.0.10:5655", "tcp://192.168.0.11:5656")(h)
		require.Equal(t, "192.168.0.10:5655", h.authServer.neoShellAddress)
	})

	t.Run("keeps_existing_when_no_tcp_candidate", func(t *testing.T) {
		h := newHttpdForOptionTest()
		h.authServer.neoShellAddress = "persist:1234"
		WithHttpNeoShellAddress("unix:///tmp/test.sock", "http://example.com")(h)
		require.Equal(t, "persist:1234", h.authServer.neoShellAddress)
	})
}

func TestWithHttpStatzAllowAndQueryCypher(t *testing.T) {
	t.Run("split_statz_allow", func(t *testing.T) {
		h := newHttpdForOptionTest()
		WithHttpStatzAllow("127.0.0.1,10.0.0.1", "", "::1")(h)
		require.Equal(t, []string{"127.0.0.1", "10.0.0.1", "::1"}, h.statzAllowed)
	})

	t.Run("empty_query_cypher_keeps_defaults", func(t *testing.T) {
		h := newHttpdForOptionTest()
		h.cypherAlg = "OLD"
		h.cypherKey = "OLDKEY"
		h.cypherPad = "OLDPAD"

		WithHttpQueryCypher("")(h)

		require.Equal(t, "OLD", h.cypherAlg)
		require.Equal(t, "OLDKEY", h.cypherKey)
		require.Equal(t, "OLDPAD", h.cypherPad)
	})

	t.Run("invalid_query_cypher_does_not_apply", func(t *testing.T) {
		h := newHttpdForOptionTest()
		WithHttpQueryCypher("alg=FOO key=short")(h)
		require.Empty(t, h.cypherAlg)
		require.Empty(t, h.cypherKey)
		require.Empty(t, h.cypherPad)
	})

	t.Run("valid_query_cypher_applies", func(t *testing.T) {
		require.NoError(t, util.ValidateCypherKey("AES", "1234567890abcdef"))
		h := newHttpdForOptionTest()
		WithHttpQueryCypher("algorithm=AES key=1234567890abcdef padding=pkcs5")(h)
		require.Equal(t, "AES", h.cypherAlg)
		require.Equal(t, "1234567890abcdef", h.cypherKey)
		require.Equal(t, "PKCS5", h.cypherPad)
	})
}

func TestWithHttpMiscOptions(t *testing.T) {
	h := newHttpdForOptionTest()
	called := false
	handler := func(http.ResponseWriter, *http.Request) {}

	WithHttpDebugMode(true, "150ms")(h)
	WithHttpKeepAlive(11)(h)
	WithHttpLinger(7)(h)
	WithHttpReadBufSize(1024)(h)
	WithHttpWriteBufSize(2048)(h)
	WithHttpPathMap("/data", "/tmp/data")(h)
	WithHttpExperimentModeProvider(func() bool {
		called = true
		return true
	})(h)
	WithHttpMqttWsHandlerFunc(handler)(h)

	require.True(t, h.debugMode)
	require.Equal(t, 150*time.Millisecond, h.debugLogFilterLatency)
	require.Equal(t, 11, h.keepAlive)
	require.Equal(t, 7, h.linger)
	require.Equal(t, 1024, h.readBufSize)
	require.Equal(t, 2048, h.writeBufSize)
	require.Equal(t, "/tmp/data", h.pathMap["/data"])
	require.NotNil(t, h.experimentModeProvider)
	require.True(t, h.experimentModeProvider())
	require.True(t, called)
	require.NotNil(t, h.mqttWsHandler)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NotPanics(t, func() {
		h.mqttWsHandler(ctx)
	})
}
