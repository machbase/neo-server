package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	shelllib "github.com/machbase/neo-server/v8/jsh/lib/shell"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/machbase/neo-server/v8/spi"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// BenchmarkHandleQuery benchmarks the /db/query handler directly, excluding HTTP client and transport overhead.
//
// goos: linux
// goarch: amd64
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// == v8.5.2 (macheng)
// BenchmarkHandleQuery-24    	    4317	    276325 ns/op	   25009 B/op	     147 allocs/op
// BenchmarkHandleQuery-24    	    4876	    267507 ns/op	   25218 B/op	     147 allocs/op
// BenchmarkHandleQuery-24    	    4260	    275556 ns/op	   24547 B/op	     147 allocs/op
//
// == v8.5.3 (machgo)
// BenchmarkHandleQuery-24    	     187	   6185017 ns/op	  292638 B/op	     316 allocs/op
// BenchmarkHandleQuery-24    	     194	   6217183 ns/op	  292623 B/op	     316 allocs/op
// BenchmarkHandleQuery-24    	     194	   6234090 ns/op	  292717 B/op	     317 allocs/op
//
// == v8.5.5 (machgo+pooling)
// BenchmarkHandleQuery-24    	   12969	     98218 ns/op	   13059 B/op	     124 allocs/op
// BenchmarkHandleQuery-24    	   12799	     92416 ns/op	   13058 B/op	     124 allocs/op
// BenchmarkHandleQuery-24    	   12914	     97126 ns/op	   13059 B/op	     124 allocs/op
func BenchmarkHandleQuery(b *testing.B) {
	sql := "SELECT 1"
	target := "/db/query?q=" + url.QueryEscape(sql) + "&tz=Asia/Seoul&timeformat=Default"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, writer := newTestHTTPContext(http.MethodGet, target, nil)
		httpServer.handleQuery(ctx)
		require.Equal(b, http.StatusOK, writer.Code)
	}
}

// goos: linux
// goarch: amd64
// cpu: AMD Ryzen 9 3900X 12-Core Processor
//
// v8.5.5 (machgo+pooling)
// == db.SetMaxOpenConns(200) db.SetMaxIdleConns(2)
// BenchmarkHTTPQueryParallel-24    	   11654	     97116 ns/op	   81587 B/op	     224 allocs/op
//
// == db.SetMaxOpenConns(200) db.SetMaxIdleConns(200)
// BenchmarkHTTPQueryParallel-24    	   99537	     11865 ns/op	   19512 B/op	     175 allocs/op
func BenchmarkHTTPQueryParallel(b *testing.B) {
	db, _ := spi.DefaultPool()
	db.SetMaxOpenConns(200)
	db.SetMaxIdleConns(2)
	sql := "SELECT 1"
	target := httpServerAddress + "/db/query?q=" + url.QueryEscape(sql) + "&tz=Asia/Seoul&timeformat=Default"

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     0,
		},
	}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, err := http.NewRequest(http.MethodGet, target, nil)
			if err != nil {
				b.Fatal(err)
			}
			rsp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, rsp.Body)
			rsp.Body.Close()
			if rsp.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status: %d", rsp.StatusCode)
			}
		}
	})
}

func TestStatz(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/debug/statz", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	result := map[string]any{}
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	rsp.Body.Close()

	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(result), 2)
}

func TestDebugMetrics(t *testing.T) {
	prevToken := spi.PrometheusBearerToken()
	t.Cleanup(func() {
		spi.SetPrometheusBearerToken(prevToken)
	})

	t.Run("without fixed token", func(t *testing.T) {
		spi.SetPrometheusBearerToken("")

		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/debug/metrics", nil)
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.Contains(t, rsp.Header.Get("Content-Type"), "text/plain")
	})

	t.Run("with fixed token", func(t *testing.T) {
		spi.SetPrometheusBearerToken("prom-fixed-token")

		t.Run("rejects missing token", func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/debug/metrics", nil)
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer rsp.Body.Close()

			require.Equal(t, http.StatusUnauthorized, rsp.StatusCode)
		})

		t.Run("accepts matching token", func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/debug/metrics", nil)
			req.Header.Set("Authorization", "Bearer prom-fixed-token")
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer rsp.Body.Close()

			require.Equal(t, http.StatusOK, rsp.StatusCode)
		})
	})
}

func TestAllowDebug(t *testing.T) {
	svr := &httpd{log: logging.GetLog("httpd-fake")}

	t.Run("allows loopback", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/debug/metrics", nil)
		ctx.Request.RemoteAddr = "127.0.0.1:12345"

		svr.allowDebug(ctx)

		require.False(t, ctx.IsAborted())
		require.Equal(t, http.StatusOK, writer.Code)
	})

	t.Run("rejects non allowed remote", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/debug/metrics", nil)
		ctx.Request.RemoteAddr = "192.0.2.10:12345"

		svr.allowDebug(ctx)

		require.True(t, ctx.IsAborted())
		require.Equal(t, http.StatusForbidden, writer.Code)
	})

	t.Run("allows configured remote", func(t *testing.T) {
		svr.statzAllowed = []string{"192.0.2.10"}
		ctx, writer := newTestHTTPContext(http.MethodGet, "/debug/metrics", nil)
		ctx.Request.RemoteAddr = "192.0.2.10:12345"

		svr.allowDebug(ctx)

		require.False(t, ctx.IsAborted())
		require.Equal(t, http.StatusOK, writer.Code)
	})
}

func newHttpdForOptionTest() *httpd {
	return &httpd{
		log:        logging.GetLog("http-opts-test"),
		authServer: &Server{},
		pathMap:    map[string]string{},
	}
}

func TestWithHttpAuthServer(t *testing.T) {
	t.Run("enabled_with_rpc_controller", func(t *testing.T) {
		authSvc := &Server{neoShellAddress: "before", serviceController: &service.Controller{}}
		h := newHttpdForOptionTest()

		WithHttpAuthServer(authSvc, true)(h)

		require.Same(t, authSvc, h.authServer)
		require.True(t, h.enableTokenAuth.Load())
		require.Same(t, authSvc.serviceController, h.serviceController)
	})

	t.Run("disabled_nil_service", func(t *testing.T) {
		h := newHttpdForOptionTest()
		h.serviceController = &service.Controller{}

		WithHttpAuthServer(nil, false)(h)

		require.Nil(t, h.authServer)
		require.False(t, h.enableTokenAuth.Load())
		require.NotNil(t, h.serviceController)
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

func TestHandleStatzConfig(t *testing.T) {
	prevDest := spi.MetricsDestTable()
	t.Cleanup(func() {
		require.NoError(t, spi.SetMetricsDestTable(prevDest))
	})

	svr := &httpd{log: logging.GetLog("httpd-fake")}

	t.Run("get", func(t *testing.T) {
		require.NoError(t, spi.SetMetricsDestTable(""))

		ctx, writer := newTestHTTPContext(http.MethodGet, "/debug/statz/config", nil)
		svr.handleStatzConfig(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(writer.Body.Bytes(), &payload))
		require.Equal(t, true, payload["success"])
		data, ok := payload["data"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "", data["out"])
	})

	t.Run("rejects malformed body", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/debug/statz/config", []byte(`{"out":`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		svr.handleStatzConfig(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "unexpected EOF")
	})

	t.Run("rejects non string out", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/debug/statz/config", []byte(`{"out":123}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		svr.handleStatzConfig(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "invalid out value")
	})

	t.Run("accepts empty output table", func(t *testing.T) {
		require.NoError(t, spi.SetMetricsDestTable(""))

		ctx, writer := newTestHTTPContext(http.MethodPost, "/debug/statz/config", []byte(`{"out":"   "}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		svr.handleStatzConfig(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "", spi.MetricsDestTable())
	})

	t.Run("rejects unsupported method", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodDelete, "/debug/statz/config", nil)

		svr.handleStatzConfig(ctx)

		require.Equal(t, http.StatusMethodNotAllowed, writer.Code)
	})
}

func TestWebConsole(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.Nil(t, err)

	// Convert http://127.0.0.1 to ws://127.0.0.1
	u := "ws" + strings.TrimPrefix(httpServerAddress, "http") + "/web/api/console/1234/data?token=" + at
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer ws.Close()

	// PING
	ping := eventbus.NewPingTime(time.Now())
	ws.WriteJSON(ping)

	evt := eventbus.Event{}
	ws.ReadJSON(&evt)
	require.Equal(t, eventbus.EVT_PING, evt.Type)
	require.Equal(t, ping.Ping.Tick, evt.Ping.Tick)

	// LOG
	topic := "console:sys:1234"
	eventbus.PublishLog(topic, "INFO", "test message")

	evt = eventbus.Event{}
	ws.ReadJSON(&evt)
	require.Equal(t, eventbus.EVT_LOG, evt.Type)
	require.Equal(t, "test message", evt.Log.Message)

	// TQL Log
	expectLines := []string{
		"1 0",
		"2 0.25",
		"3 0.5",
		"4 0.75",
		"5 1",
	}
	expectCount := len(expectLines)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < expectCount; i++ {
			evt := eventbus.Event{}
			err := ws.ReadJSON(&evt)
			if err != nil {
				t.Log(err.Error())
			}
			require.Nil(t, err, "read websocket failed")
			require.Equal(t, expectLines[i], evt.Log.Message)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := bytes.NewBufferString(`
			FAKE(linspace(0,1,5))
			SCRIPT("js", {
				console.log($.key, $.values[0]);
				$.yieldKey($.key, $.values[0]);
			})
			PUSHKEY('test')
			CSV(precision(2))
		`)
		req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql", reader)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		req.Header.Set("X-Console-Id", "1234 console-log-level=INFO log-level=ERROR")
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		result, _ := io.ReadAll(rsp.Body)
		require.Equal(t, strings.Join([]string{"1,0.00", "2,0.25", "3,0.50", "4,0.75", "5,1.00", "\n"}, "\n"), string(result))
	}()
	wg.Wait()
}

func TestWebConsoleRpc(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	consoleID := "rpc-1234"
	u := "ws" + strings.TrimPrefix(httpServerAddress, "http") + "/web/api/console/" + consoleID + "/data?token=" + at
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer ws.Close()

	methodOK := "test.ws.rpc.ok"
	methodErr := "test.ws.rpc.err"
	httpServer.serviceController.RegisterJsonRpcHandler(methodOK, func(ctx context.Context, cons *WebConsole, name string) (map[string]any, error) {
		return map[string]any{
			"user":      cons.username,
			"console":   cons.consoleId,
			"has_ctx":   ctx != nil,
			"echo_name": name,
		}, nil
	})
	httpServer.serviceController.RegisterJsonRpcHandler(methodErr, func() error {
		return fmt.Errorf("boom")
	})
	defer httpServer.serviceController.UnregisterJsonRpcHandler(methodOK)
	defer httpServer.serviceController.UnregisterJsonRpcHandler(methodErr)

	sendRPC := func(session string, method string, params []any) gjson.Result {
		t.Helper()
		req := eventbus.Event{
			Type:    eventbus.EVT_RPC_REQ,
			Session: session,
			Rpc: &eventbus.RPC{
				Ver:    "2.0",
				ID:     7,
				Method: method,
				Params: params,
			},
		}
		require.NoError(t, ws.WriteJSON(req))

		_, body, err := ws.ReadMessage()
		require.NoError(t, err)
		result := gjson.ParseBytes(body)
		require.Equal(t, eventbus.EVT_RPC_RSP, result.Get("type").String(), result.String())
		require.Equal(t, session, result.Get("session").String(), result.String())
		require.Equal(t, int64(7), result.Get("rpc.id").Int(), result.String())
		require.Equal(t, "2.0", result.Get("rpc.jsonrpc").String(), result.String())
		return result
	}

	rsp := sendRPC("ok", methodOK, []any{"neo"})
	require.Equal(t, "sys", rsp.Get("rpc.result.user").String(), rsp.String())
	require.Equal(t, consoleID, rsp.Get("rpc.result.console").String(), rsp.String())
	require.True(t, rsp.Get("rpc.result.has_ctx").Bool(), rsp.String())
	require.Equal(t, "neo", rsp.Get("rpc.result.echo_name").String(), rsp.String())

	rsp = sendRPC("missing", "missing.method", []any{})
	require.Equal(t, int64(-32601), rsp.Get("rpc.error.code").Int(), rsp.String())
	require.Equal(t, "Method not found", rsp.Get("rpc.error.message").String(), rsp.String())

	rsp = sendRPC("internal", methodErr, []any{})
	require.Equal(t, int64(-32000), rsp.Get("rpc.error.code").Int(), rsp.String())
	require.Equal(t, "boom", rsp.Get("rpc.error.message").String(), rsp.String())
}

func TestWebConsoleRpcLLMEventSequence(t *testing.T) {
	restoreLLM := service.SetLLMStreamFuncForTest(func(_ context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		tokens := []string{"sequence", " ", "ok"}
		for _, tok := range tokens {
			onToken(tok)
		}
		return &shelllib.LLMStreamResponse{
			Content:      "sequence ok",
			InputTokens:  2,
			OutputTokens: 2,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	})
	t.Cleanup(restoreLLM)

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	consoleID := "rpc-llm-seq"
	u := "ws" + strings.TrimPrefix(httpServerAddress, "http") + "/web/api/console/" + consoleID + "/data?token=" + at
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer ws.Close()

	writeReq := func(session string, id int64, method string, params []any) {
		t.Helper()
		req := eventbus.Event{
			Type:    eventbus.EVT_RPC_REQ,
			Session: session,
			Rpc: &eventbus.RPC{
				Ver:    "2.0",
				ID:     id,
				Method: method,
				Params: params,
			},
		}
		require.NoError(t, ws.WriteJSON(req))
	}

	readUntilRsp := func(session string, id int64) gjson.Result {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			require.NoError(t, ws.SetReadDeadline(time.Now().Add(1*time.Second)))
			_, body, err := ws.ReadMessage()
			require.NoError(t, err)
			msg := gjson.ParseBytes(body)
			if msg.Get("type").String() != eventbus.EVT_RPC_RSP {
				continue
			}
			if msg.Get("session").String() != session {
				continue
			}
			if msg.Get("rpc.id").Int() == id {
				return msg
			}
		}
		t.Fatalf("response timeout for session=%s id=%d", session, id)
		return gjson.Result{}
	}

	openSession := "llm-open"
	writeReq(openSession, 1, "llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}})
	openRsp := readUntilRsp(openSession, 1)
	require.Equal(t, "2.0", openRsp.Get("rpc.jsonrpc").String(), openRsp.String())
	sessionID := openRsp.Get("rpc.result.sessionId").String()
	require.NotEmpty(t, sessionID, openRsp.String())

	askSession := "llm-ask"
	turnID := "turn-seq-1"
	traceID := "trace-seq-1"
	writeReq(askSession, 2, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    turnID,
		"traceId":   traceID,
		"payload": map[string]any{
			"text": "sequence check",
		},
	}})

	var askRsp gjson.Result
	var eventSeq []int64
	var eventNames []string
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, ws.SetReadDeadline(time.Now().Add(1*time.Second)))
		_, body, readErr := ws.ReadMessage()
		require.NoError(t, readErr)

		msg := gjson.ParseBytes(body)
		if msg.Get("type").String() != eventbus.EVT_RPC_RSP {
			continue
		}
		if msg.Get("session").String() != askSession {
			continue
		}

		if msg.Get("rpc.id").Int() == 2 {
			askRsp = msg
		}

		if msg.Get("rpc.method").String() == "llm.event" {
			if msg.Get("rpc.params.turnId").String() != turnID {
				continue
			}
			eventSeq = append(eventSeq, msg.Get("rpc.params.seq").Int())
			eventNames = append(eventNames, msg.Get("rpc.params.event").String())
			if msg.Get("rpc.params.event").String() == "turn.completed" {
				break
			}
		}
	}

	require.NotEmpty(t, askRsp.Raw)
	require.True(t, askRsp.Get("rpc.result.accepted").Bool(), askRsp.String())
	require.Equal(t, "streaming", askRsp.Get("rpc.result.status").String(), askRsp.String())

	require.GreaterOrEqual(t, len(eventNames), 5, "events=%v", eventNames)
	require.Equal(t, "turn.started", eventNames[0], "events=%v", eventNames)
	require.Equal(t, "turn.block.started", eventNames[1], "events=%v", eventNames)
	require.Equal(t, "turn.block.completed", eventNames[len(eventNames)-2], "events=%v", eventNames)
	require.Equal(t, "turn.completed", eventNames[len(eventNames)-1], "events=%v", eventNames)

	for i := 1; i < len(eventSeq); i++ {
		require.Equal(t, eventSeq[i-1]+1, eventSeq[i], "seq=%v events=%v", eventSeq, eventNames)
	}
}

func TestImageFiles(t *testing.T) {
	require.Equal(t, "image/apng", contentTypeOfFile("some/dir/file.apng"))
	require.Equal(t, "image/avif", contentTypeOfFile("some/dir/file.avif"))
	require.Equal(t, "image/gif", contentTypeOfFile("some/dir/file.gif"))
	require.Equal(t, "image/jpeg", contentTypeOfFile("some/dir/file.Jpeg"))
	require.Equal(t, "image/jpeg", contentTypeOfFile("some/dir/file.JPG"))
	require.Equal(t, "image/png", contentTypeOfFile("some/dir/file.PNG"))
	require.Equal(t, "image/svg+xml", contentTypeOfFile("some/dir/file.svg"))
	require.Equal(t, "image/webp", contentTypeOfFile("some/dir/file.webp"))
	require.Equal(t, "image/bmp", contentTypeOfFile("some/dir/file.BMP"))
	require.Equal(t, "image/x-icon", contentTypeOfFile("some/dir/file.ico"))
	require.Equal(t, "image/tiff", contentTypeOfFile("some/dir/file.tiff"))
	require.Equal(t, "text/plain", contentTypeOfFile("some/dir/file.txt"))
	require.Equal(t, "text/csv", contentTypeOfFile("some/dir/file.csv"))
	require.Equal(t, "application/json", contentTypeOfFile("some/dir/file.json"))
	require.Equal(t, "text/markdown", contentTypeOfFile("some/dir/file.md"))
	require.Equal(t, "text/markdown", contentTypeOfFile("some/dir/file.markdown"))

	// additional file type coverage
	require.Equal(t, "text/plain", contentTypeOfFile("query.sql"))
	require.Equal(t, "text/plain", contentTypeOfFile("flow.tql"))
	require.Equal(t, "application/json", contentTypeOfFile("analysis.taz"))
	require.Equal(t, "application/json", contentTypeOfFile("work.wrk"))
	require.Equal(t, "application/json", contentTypeOfFile("board.dsh"))
	require.Equal(t, "text/css", contentTypeOfFile("style.css"))
	require.Equal(t, "text/javascript", contentTypeOfFile("app.js"))
	require.Equal(t, "text/javascript", contentTypeOfFile("mod.mjs"))
	require.Equal(t, "text/html", contentTypeOfFile("page.htm"))
	require.Equal(t, "text/html", contentTypeOfFile("page.html"))
	require.Equal(t, "text/x-python", contentTypeOfFile("script.py"))
	require.Equal(t, "text/x-shellscript", contentTypeOfFile("run.sh"))
	require.Equal(t, "application/x-ipynb+json", contentTypeOfFile("notebook.ipynb"))
	require.Equal(t, "", contentTypeOfFile("file.unknown"))
}

func TestIsFsFile(t *testing.T) {
	require.True(t, isFsFile("test.sql"))
	require.True(t, isFsFile("test.tql"))
	require.True(t, isFsFile("test.json"))
	require.True(t, isFsFile("test.png"))
	require.False(t, isFsFile("test.xyz"))
	require.False(t, isFsFile("noext"))
}

func TestIsErrTokenExpired(t *testing.T) {
	// expired token
	claim := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		Subject:   "sys",
	}
	token, err := SignTokenWithClaim(claim)
	require.NoError(t, err)

	_, err = VerifyTokenWithClaim(token, NewClaimEmpty())
	require.Error(t, err)
	require.True(t, IsErrTokenExpired(err))

	// not expired
	require.False(t, IsErrTokenExpired(fmt.Errorf("other error")))
}

func TestMaskLogQueryToken(t *testing.T) {
	require.Equal(t, "", maskLogQueryToken(""))
	require.Equal(t, "token=****", maskLogQueryToken("token=nt_abc123"))
	require.Equal(t, "a=1&token=****&b=2", maskLogQueryToken("a=1&token=nt_abc123&b=2"))
	require.Equal(t, "a=1&b=2", maskLogQueryToken("a=1&b=2"))
	require.Equal(t, "nottoken=nt_abc123", maskLogQueryToken("nottoken=nt_abc123"))
}

func TestHandleAuthToken(t *testing.T) {
	t.Run("rejects when auth server is missing", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake")}
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/files?token=ignored", nil)

		svr.handleAuthToken(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "no auth server")
		require.True(t, ctx.IsAborted())
	})

	t.Run("rejects when token is missing", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake"), authServer: &Server{authorizedKeysDir: t.TempDir()}}
		svr.enableTokenAuth.Store(true)
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/files", nil)

		svr.handleAuthToken(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "missing authorization token")
		require.True(t, ctx.IsAborted())
	})

	t.Run("rejects invalid bearer token", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake"), authServer: &Server{authorizedKeysDir: t.TempDir()}}
		svr.enableTokenAuth.Store(true)
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/files", nil)
		ctx.Request.Header.Set("Authorization", "Bearer invalid-token")

		svr.handleAuthToken(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "missing valid token")
		require.True(t, ctx.IsAborted())
	})

	t.Run("rejects legacy signed query token", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake"), authServer: &Server{}}
		svr.enableTokenAuth.Store(true)
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/files?token="+url.QueryEscape("client1:b:deadbeef"), nil)

		svr.handleAuthToken(ctx)

		require.True(t, ctx.IsAborted())
		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "missing valid token")
	})

	t.Run("optional mode passes through without a token", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake"), authServer: &Server{authorizedKeysDir: t.TempDir()}}
		ctx, writer := newTestHTTPContext(http.MethodGet, "/db/query", nil)

		svr.handleAuthToken(ctx)

		require.False(t, ctx.IsAborted(), writer.Body.String())
		_, ok := ctx.Get("api-token-authenticated")
		require.False(t, ok)
	})

	t.Run("optional mode rejects an invalid token", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake"), authServer: &Server{authorizedKeysDir: t.TempDir()}}
		ctx, writer := newTestHTTPContext(http.MethodGet, "/db/query", nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+FormatApiToken(999999999, strings.Repeat("a", 43)))

		svr.handleAuthToken(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "missing valid token")
		require.True(t, ctx.IsAborted())
	})

	t.Run("optional mode ignores a non-api-token bearer (e.g. JWT)", func(t *testing.T) {
		svr := &httpd{log: logging.GetLog("httpd-fake"), authServer: &Server{authorizedKeysDir: t.TempDir()}}
		ctx, writer := newTestHTTPContext(http.MethodGet, "/db/query", nil)
		ctx.Request.Header.Set("Authorization", "Bearer not-an-api-token")

		svr.handleAuthToken(ctx)

		require.False(t, ctx.IsAborted(), writer.Body.String())
		_, ok := ctx.Get("api-token-authenticated")
		require.False(t, ok)
	})
}

func TestHandleChangePassword(t *testing.T) {
	newServer := func() *httpd {
		return &httpd{
			log:        logging.GetLog("httpd-fake"),
			authServer: &Server{},
			jwtCache:   NewJwtCache(),
		}
	}

	t.Run("bind error", func(t *testing.T) {
		svr := newServer()
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/password", []byte("{"))
		ctx.Request.Header.Set("Content-Type", "application/json")

		svr.handleChangePassword(ctx)
		require.Equal(t, http.StatusBadRequest, writer.Code)
	})

	t.Run("rejects invalid password", func(t *testing.T) {
		svr := newServer()
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/password", []byte(`{"newPassword":"bad'pw"}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		svr.handleChangePassword(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "invalid new password")
	})

	t.Run("rejects unauthorized request", func(t *testing.T) {
		svr := newServer()
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/password", []byte(`{"newPassword":"updated-password"}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		svr.handleChangePassword(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "unauthorized request")
	})

	t.Run("successfully changes password for new test user", func(t *testing.T) {
		testUsername := "test_pwd_change_user"
		testPassword := "initial_password"
		newPassword := "updated_password"

		sysConn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err)
		defer sysConn.Close()

		_, err = sysConn.ExecContext(t.Context(),
			fmt.Sprintf("CREATE USER %s IDENTIFIED BY '%s'", testUsername, testPassword))
		require.NoError(t, err)

		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sysConn, err := spi.Connect(ctx, "sys")
			require.NoError(t, err)
			defer sysConn.Close()
			if err == nil {
				defer sysConn.Close()
				_, err := sysConn.ExecContext(ctx, fmt.Sprintf("DROP USER %s", testUsername))
				if err != nil {
					t.Logf("warning: failed to drop test user: %v", err.Error())
				}
			}
		})

		svr := newServer()
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/password", []byte(fmt.Sprintf(`{"newPassword":"%s"}`, newPassword)))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("jwt-claim", NewClaim(testUsername))

		svr.handleChangePassword(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":true`)

		ctx, writer = newTestHTTPContext(http.MethodPost, "/web/api/login", []byte(`{"loginName":"`+testUsername+`","password":"`+newPassword+`"}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		svr.handleLogin(ctx)
		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":true`, writer.Body.String())
	})
}

func TestDebugMode(t *testing.T) {
	debug, latency := httpServer.DebugMode()
	require.False(t, debug)
	require.Equal(t, time.Duration(0), latency)

	httpServer.SetDebugMode(true, 100*time.Millisecond)
	debug, latency = httpServer.DebugMode()
	require.True(t, debug)
	require.Equal(t, 100*time.Millisecond, latency)

	// restore
	httpServer.SetDebugMode(false, 0)
}

func TestAdvertiseAddress(t *testing.T) {
	addr := httpServer.AdvertiseAddress()
	require.NotEmpty(t, addr)
	require.True(t, strings.HasPrefix(addr, "http://"))
}

func TestStatzConfig(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	// GET statz config
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/statz/config", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	body, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()

	result := map[string]any{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	require.Equal(t, true, result["success"])
}

func TestRefsFiles(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/refs/", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))

	result, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	rsp.Body.Close()

	var obj RefsResponse
	err = json.Unmarshal(result, &obj)
	require.Nil(t, err)

	require.Equal(t, 3, len(obj.Data.Refs))
	require.Equal(t, obj.Data.Refs[0].Label, "REFERENCES")
	require.Equal(t, 5, len(obj.Data.Refs[0].Items))

	require.Equal(t, obj.Data.Refs[1].Label, "SDK")
	require.Equal(t, 5, len(obj.Data.Refs[1].Items))

	require.Equal(t, obj.Data.Refs[2].Label, "CHEAT SHEETS")
	require.Equal(t, 3, len(obj.Data.Refs[2].Items))
}

func HttpTestLogin(t *testing.T, username, password string) *LoginRsp {
	t.Helper()

	b := &bytes.Buffer{}
	loginReq := &LoginReq{
		LoginName: username,
		Password:  password,
	}
	if err := json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec := json.NewDecoder(rsp.Body)
	loginRsp := &LoginRsp{}
	err = dec.Decode(loginRsp)
	require.NoError(t, err)
	rsp.Body.Close()
	return loginRsp
}

func TestLoginRoute(t *testing.T) {
	// wrong password case - login
	b := &bytes.Buffer{}
	loginReq := &LoginReq{
		LoginName: "sys",
		Password:  "wrong",
	}
	if err := json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rsp.StatusCode)
	rsp.Body.Close()

	// success case - login
	b.Reset()
	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "manager",
	}
	if err := json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec := json.NewDecoder(rsp.Body)
	loginRsp := &LoginRsp{}
	err = dec.Decode(loginRsp)
	require.NoError(t, err)
	rsp.Body.Close()

	// Access Token default expire 5 minutes
	claim := NewClaimEmpty()
	_, err = jwt.ParseWithClaims(loginRsp.AccessToken, claim, func(t *jwt.Token) (interface{}, error) {
		return []byte("__secr3t__"), nil
	})
	require.Nil(t, err, "parse access token")
	require.True(t, claim.VerifyExpiresAt(time.Now().Add(4*time.Minute), true))
	require.False(t, claim.VerifyExpiresAt(time.Now().Add(6*time.Minute), true))

	// Access Token default expire 60 minutes
	claim = NewClaimEmpty()
	_, err = jwt.ParseWithClaims(loginRsp.RefreshToken, claim, func(t *jwt.Token) (interface{}, error) {
		return []byte("__secr3t__"), nil
	})
	require.Nil(t, err, "parse refresh token")
	require.True(t, claim.VerifyExpiresAt(time.Now().Add(59*time.Minute), true))
	require.False(t, claim.VerifyExpiresAt(time.Now().Add(61*time.Minute), true))

	// success case - re-login
	b.Reset()
	reloginReq := &ReLoginReq{
		RefreshToken: loginRsp.RefreshToken,
	}
	if err := json.NewEncoder(b).Encode(reloginReq); err != nil {
		t.Fatal(err)
	}

	req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/relogin", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec = json.NewDecoder(rsp.Body)
	reRsp := &ReLoginRsp{}
	err = dec.Decode(reRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	// success case - logout
	b.Reset()
	logoutReq := &LogoutReq{
		RefreshToken: reRsp.RefreshToken,
	}
	if err := json.NewEncoder(b).Encode(logoutReq); err != nil {
		t.Fatal(err)
	}

	req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/logout", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec = json.NewDecoder(rsp.Body)
	logoutRsp := &LogoutRsp{}
	err = dec.Decode(logoutRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.True(t, logoutRsp.Success)
	rsp.Body.Close()

	// session check
	b.Reset()
	req, _ = http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/check", b)
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	dec = json.NewDecoder(rsp.Body)
	checkRsp := &LoginCheckRsp{}
	err = dec.Decode(checkRsp)
	require.NoError(t, err)
	rsp.Body.Close()
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.True(t, checkRsp.Success)
}

func TestLogin(t *testing.T) {
	at, rt, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)
	require.NotEmpty(t, rt)

	at, rt, err = jwtLogin("sys", "wrong")
	require.Equal(t, "404 Not Found", err.Error())
	require.Empty(t, at)
	require.Empty(t, rt)
}

func jwtLogin(username, password string) (string, string, error) {
	req, _ := http.NewRequest(
		http.MethodPost,
		httpServerAddress+"/web/api/login",
		bytes.NewBufferString(fmt.Sprintf(`{"loginName":"%s","password":"%s"}`, username, password)))
	req.Header.Set("Content-Type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	if rsp.StatusCode != http.StatusOK {
		return "", "", errors.New(rsp.Status)
	}
	loginRsp := &LoginRsp{}
	err = json.NewDecoder(rsp.Body).Decode(loginRsp)
	if err != nil {
		return "", "", err
	}
	rsp.Body.Close()
	return loginRsp.AccessToken, loginRsp.RefreshToken, nil
}

func TestHandleServiceProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer target.Close()

	pm := NewProxyManager()
	_, err := pm.Register(ProxyRegisterRequest{Service: "github.com/acme/chart", Prefix: "/api/", Target: target.URL})
	require.NoError(t, err)
	svr := &httpd{authServer: &Server{proxyMgr: pm}}

	router := gin.New()
	router.Any("/web/services/*path", svr.handleServiceProxy)
	frontend := httptest.NewServer(router)
	defer frontend.Close()

	rsp, err := http.Get(frontend.URL + "/web/services/github.com/acme/chart/api/v1")
	require.NoError(t, err)
	defer rsp.Body.Close()
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "/v1", string(body))
}

func TestLicense(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	tests := []struct {
		name   string
		method string
		path   string
		body   func(t *testing.T) (io.Reader, string)
		setup  func(t *testing.T, req *http.Request)
		expect func(t *testing.T, rsp *http.Response)
	}{
		{
			name:   "get-check-eula-required",
			method: http.MethodGet,
			path:   "/web/api/check",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
				require.Equal(t, true, gjson.GetBytes(body, "eulaRequired").Bool())
				require.Equal(t, "Valid", gjson.GetBytes(body, "licenseStatus").String())
			},
		},
		{
			name:   "get-eula",
			method: http.MethodGet,
			path:   "/web/api/license/eula",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				require.Equal(t, "text/plain; charset=utf-8", rsp.Header.Get("Content-Type"))
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, eulaTxt, string(body))
			},
		},
		{
			name:   "post-eula",
			method: http.MethodPost,
			path:   "/web/api/license/eula",
			expect: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
			},
		},
		{
			name:   "get-license",
			method: http.MethodGet,
			path:   "/web/api/license",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool(), string(body))
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String(), string(body))
				require.True(t, gjson.GetBytes(body, "data.licenseStatus").Exists(), string(body))
			},
		},
		{
			name:   "post-license-missing-file",
			method: http.MethodPost,
			path:   "/web/api/license",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusBadRequest, rsp.StatusCode)
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, false, gjson.GetBytes(body, "success").Bool(), string(body))
				require.NotEmpty(t, gjson.GetBytes(body, "reason").String(), string(body))
			},
		},
		{
			name:   "post-license-too-large",
			method: http.MethodPost,
			path:   "/web/api/license",
			body: func(t *testing.T) (io.Reader, string) {
				t.Helper()
				buf := &bytes.Buffer{}
				writer := multipart.NewWriter(buf)
				part, err := writer.CreateFormFile("license.dat", "license.dat")
				require.NoError(t, err)
				_, err = part.Write(bytes.Repeat([]byte{'x'}, 4097))
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				return buf, writer.FormDataContentType()
			},
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusBadRequest, rsp.StatusCode)
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, false, gjson.GetBytes(body, "success").Bool(), string(body))
				require.Equal(t, "Too large file as a license file.", gjson.GetBytes(body, "reason").String(), string(body))
			},
		},
		{
			name:   "get-check-eula",
			method: http.MethodGet,
			path:   "/web/api/check",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
				require.Equal(t, false, gjson.GetBytes(body, "eulaRequired").Bool(), string(body))
				require.Equal(t, "Valid", gjson.GetBytes(body, "licenseStatus").String())
			},
		},
		{
			name:   "delete-eula",
			method: http.MethodDelete,
			path:   "/web/api/license/eula",
			expect: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			var contentType string
			if tc.body != nil {
				body, contentType = tc.body(t)
			}
			req, _ := http.NewRequest(tc.method, httpServerAddress+tc.path, body)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}
			if tc.setup != nil {
				tc.setup(t, req)
			}
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			tc.expect(t, rsp)
			rsp.Body.Close()
		})
	}
}

func TestHttpWrite(t *testing.T) {
	tests := []struct {
		name             string
		queryParams      string
		payloadType      string
		payloadReq       any
		selectSql        string
		selectQueryParam string
		selectExpect     []string
	}{
		{
			name:        "json",
			queryParams: "?timeformat=s",
			payloadType: "application/json",
			payloadReq: map[string]any{
				"data": map[string]any{
					"columns": []string{"name", "time", "value", "jsondata", "ival", "sval", "bval"},
					"rows": [][]any{
						{"test_1", testTimeTick.Unix(), 1.12, nil, 101, 102, []byte{0x1, 0x2}},
						{"test_1", testTimeTick.Unix() + 1, 2.23, nil, 201, 202, []byte{0x3, 0x4}},
					},
				},
			},
			selectSql:        `select * from test_w where name = 'test_1'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`test_1,1705291859,1.12,NULL,101,102,0x0102`,
				`test_1,1705291860,2.23,NULL,201,202,0x0304`,
				"\n"},
		},
		{
			name:        "ndjson",
			queryParams: "?timeformat=s&method=insert",
			payloadType: "application/x-ndjson",
			payloadReq: []any{
				map[string]any{"name": "test_2", "time": testTimeTick.Unix(), "value": 1.12, "jsondata": nil, "ival": 101, "sval": 102, "bval": []byte{0x1, 0x2}},
				map[string]any{"name": "test_2", "time": testTimeTick.Unix() + 1, "value": 2.23, "jsondata": nil, "ival": 201, "sval": 202, "bval": []byte{0x3, 0x4}},
			},
			selectSql:        `select * from test_w where name = 'test_2'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`test_2,1705291859,1.12,NULL,101,102,0x0102`,
				`test_2,1705291860,2.23,NULL,201,202,0x0304`,
				"\n"},
		},
		{
			name:        "ndjson-append",
			queryParams: "?timeformat=s&method=append",
			payloadType: "application/x-ndjson",
			payloadReq: []any{
				map[string]any{"name": "test_3", "time": testTimeTick.Unix(), "value": 1.12, "jsondata": nil, "ival": 101, "sval": 102, "bval": []byte{0x1, 0x2}},
				map[string]any{"name": "test_3", "time": testTimeTick.Unix() + 1, "value": 2.23, "jsondata": nil, "ival": 201, "sval": 202, "bval": []byte{0x3, 0x4}},
			},
			selectSql:        `select * from test_w where name = 'test_3'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`test_3,1705291859,1.12,NULL,101,102,0x0102`,
				`test_3,1705291860,2.23,NULL,201,202,0x0304`,
				"\n"},
		},
		{
			name:        "csv",
			queryParams: "?timeformat=s&method=insert&header=columns",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value,JSONDATA,ival,SVAL,BVAL`, // case insensitive
				`csv_1,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12,,101,102,` + base64.StdEncoding.EncodeToString([]byte{1, 2}),
				`csv_1,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23,,201,202,` + base64.StdEncoding.EncodeToString([]byte{3, 4}),
			},
			selectSql:        `select * from test_w where name = 'csv_1'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`csv_1,1705291859,1.12,NULL,101,102,0x0102`,
				`csv_1,1705291860,2.23,NULL,201,202,0x0304`,
				"\n"},
		},
		{
			name:        "csv-append-partial",
			queryParams: "?timeformat=s&method=append&header=columns",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value`, // case insensitive
				`csv_partial_1,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12`,
				`csv_partial_1,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23`,
			},
			selectSql:        `select * from test_w where name = 'csv_partial_1'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`csv_partial_1,1705291859,1.12,NULL,NULL,NULL,NULL`,
				`csv_partial_1,1705291860,2.23,NULL,NULL,NULL,NULL`,
				"\n"},
		},
		{
			name:        "csv-append-partial2",
			queryParams: "?timeformat=s&method=append&header=columns",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value,sval`, // case insensitive
				`csv_partial_2,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12,102`,
				`csv_partial_2,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23,202`,
			},
			selectSql:        `select * from test_w where name = 'csv_partial_2'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`csv_partial_2,1705291859,1.12,NULL,NULL,102,NULL`,
				`csv_partial_2,1705291860,2.23,NULL,NULL,202,NULL`,
				"\n"},
		},
		{
			name:        "csv_gzip",
			queryParams: "?timeformat=s&method=insert&header=columns&compress=gzip",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value,JSONDATA,ival,SVAL,bval`, // case insensitive
				`csv_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12,,101,102,` + base64.StdEncoding.EncodeToString([]byte{1, 2}),
				`csv_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23,,201,202,` + base64.StdEncoding.EncodeToString([]byte{3, 4}),
			},
			selectSql:        `select * from test_w where name = 'csv_gzip'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`csv_gzip,1705291859,1.12,NULL,101,102,0x0102`,
				`csv_gzip,1705291860,2.23,NULL,201,202,0x0304`,
				"\n"},
		},
		{
			name:        "csv-append-partial-gzip",
			queryParams: "?timeformat=s&method=append&header=columns&compress=gzip",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value`, // case insensitive
				`csv_partial_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12`,
				`csv_partial_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23`,
			},
			selectSql:        `select * from test_w where name = 'csv_partial_gzip'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL,BVAL`,
				`csv_partial_gzip,1705291859,1.12,NULL,NULL,NULL,NULL`,
				`csv_partial_gzip,1705291860,2.23,NULL,NULL,NULL,NULL`,
				"\n"},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test_w (
		name varchar(200) primary key,
		time datetime basetime,
		value double summarized,
		jsondata json,
		ival int,
		sval short,
		bval binary)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := `drop table test_w`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		rsp.Body.Close()
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var payload io.Reader
			var compressed bool
			switch tc.payloadType {
			case "application/json":
				b, _ := json.Marshal(tc.payloadReq)
				payload = bytes.NewBuffer(b)
			case "application/x-ndjson":
				b := &bytes.Buffer{}
				enc := json.NewEncoder(b)
				for _, row := range tc.payloadReq.([]any) {
					enc.Encode(row)
				}
				payload = b
			case "text/csv":
				var w io.Writer
				b := &bytes.Buffer{}
				if strings.Contains(tc.queryParams, "compress=gzip") {
					compressed = true
					w = gzip.NewWriter(b)
				} else {
					w = b
				}
				for _, row := range tc.payloadReq.([]any) {
					w.Write([]byte(row.(string) + "\n"))
				}
				if g, ok := w.(*gzip.Writer); ok {
					g.Close()
				}
				payload = b
			}
			req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/write/test_w"+tc.queryParams, payload)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			req.Header.Set("Content-Type", tc.payloadType)
			if compressed {
				req.Header.Set("Content-Encoding", "gzip")
			}
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			rspBody, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))

			spi.FlushAppendWorkers("", "")
			conn, err := spi.Connect(t.Context(), "sys")
			require.NoError(t, err)
			_, err = conn.ExecContext(t.Context(), `EXEC table_flush(test_w)`)
			require.NoError(t, err)
			conn.Close()

			if tc.selectSql != "" {
				req, _ = http.NewRequest(http.MethodGet,
					httpServerAddress+"/db/query?q="+url.QueryEscape(tc.selectSql)+tc.selectQueryParam, nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
				rsp, err = http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))
				rspBody, _ = io.ReadAll(rsp.Body)
				rsp.Body.Close()
				require.Equal(t, strings.Join(tc.selectExpect, "\n"), string(rspBody))
			}
		})
	}
}

func TestLineProtocol(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "ilp-success",
			data: `cpu,cpu=cpu-total,host=desktop usage_irq=0,usage_softirq=0.004171359446037821,usage_guest=0,usage_user=0.3253660367906774,usage_system=0.0792558294748905,usage_idle=99.59120677410203,usage_guest_nice=0,usage_nice=0,usage_iowait=0,usage_steal=0 1670975120000000000
mem,host=desktop committed_as=8780218368i,dirty=327680i,huge_pages_free=0i,shared=67067904i,sreclaimable=414224384i,total=67377881088i,buffered=810778624i,vmalloc_total=35184372087808i,active=3356581888i,available_percent=95.04513097460023,free=56726638592i,slab=617472000i,available=64039395328i,vmalloc_used=54685696i,cached=7298387968i,inactive=6323064832i,low_total=0i,page_tables=32129024i,high_free=0i,commit_limit=35836420096i,high_total=0i,swap_total=2147479552i,write_back_tmp=0i,write_back=0i,used=2542075904i,swap_cached=0i,vmalloc_chunk=0i,mapped=652132352i,huge_page_size=2097152i,huge_pages_total=0i,low_free=0i,sunreclaim=203247616i,swap_free=2147479552i,used_percent=3.7728641253646424 1670975120000000000
disk,device=nvme0n1p3,fstype=ext4,host=desktop,mode=rw,path=/ total=1967315451904i,free=1823398948864i,used=43906785280i,used_percent=2.3513442109214915,inodes_total=122068992i,inodes_free=121125115i,inodes_used=943877i 1670975120000000000
system,host=desktop n_users=2i,load1=0.08,load5=0.1,load15=0.09,n_cpus=24i 1670975120000000000
system,host=desktop uptime=513536i 1670975120000000000
system,host=desktop uptime_format="5 days, 22:38" 1670975120000000000
processes,host=desktop zombies=0i,unknown=0i,dead=0i,paging=0i,total_threads=1084i,blocked=0i,stopped=0i,running=0i,sleeping=282i,total=426i,idle=144i 1670975120000000000`,
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test (
		NAME varchar(200) primary key,
		TIME datetime basetime,
		VALUE double summarized,
		EXT_DATA json)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := `drop table test`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		rsp.Body.Close()
	})

	for _, tc := range tests {
		for _, compress := range []bool{false, true} {
			buf := &bytes.Buffer{}
			if compress {
				w := gzip.NewWriter(buf)
				w.Write([]byte(tc.data))
				w.Close()
			} else {
				buf.WriteString(tc.data)
			}
			testName := tc.name
			if compress {
				testName += "-gzip"
			}
			t.Run(testName, func(t *testing.T) {
				// success case - line protocol
				req, _ := http.NewRequest("POST", httpServerAddress+"/metrics/write?db=test", buf)
				req.Header.Set("Content-Type", "application/octet-stream")
				if compress {
					req.Header.Set("Content-Encoding", "gzip")
				}
				rsp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				rspBody, _ := io.ReadAll(rsp.Body)
				rsp.Body.Close()
				require.Equal(t, http.StatusNoContent, rsp.StatusCode, string(rspBody))
			})
		}
	}
}

func TestHttpTables(t *testing.T) {
	tests := []struct {
		name       string
		queryParam string
		expectRows [][]any
	}{
		{
			name: "tables",
			expectRows: [][]any{
				{float64(1), "MACHBASEDB", "SYS", "EXAMPLE", "Tag Table"},
				{float64(2), "MACHBASEDB", "SYS", "LOG_DATA", "Log Table"},
				{float64(3), "MACHBASEDB", "SYS", "TAG_DATA", "Tag Table"},
			},
		},
		{
			name:       "tables_name_filter",
			queryParam: "?showall=true&name=*DATA*",
			expectRows: [][]any{
				{float64(1), "MACHBASEDB", "SYS", "LOG_DATA", "Log Table"},
				{float64(2), "MACHBASEDB", "SYS", "TAG_DATA", "Tag Table"},
				{float64(3), "MACHBASEDB", "SYS", "_EXAMPLE_DATA_0", "KeyValue Table (data)"},
				{float64(4), "MACHBASEDB", "SYS", "_TAG_DATA_DATA_0", "KeyValue Table (data)"},
				{float64(5), "MACHBASEDB", "SYS", "_TAG_DATA_META", "Lookup Table (meta)"},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tables"+tc.queryParam, nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(result))
			require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")

			require.EqualValues(t, true, resultObj["success"])
			require.EqualValues(t, "success", resultObj["reason"])

			data, ok := resultObj["data"].(map[string]any)
			require.True(t, ok)
			require.EqualValues(t, []any{"ROWNUM", "DB", "USER", "NAME", "TYPE"}, data["columns"])
			require.EqualValues(t, []any{"int32", "string", "string", "string", "string"}, data["types"])

			rows, ok := data["rows"].([]any)
			require.True(t, ok)
			assertTableRowsContain(t, rows, tc.expectRows)
		})
	}
}

func assertTableRowsContain(t *testing.T, rows []any, expected [][]any) {
	t.Helper()

	index := map[string][]any{}
	for _, row := range rows {
		cols, ok := row.([]any)
		require.True(t, ok)
		require.Len(t, cols, 5)
		name, ok := cols[3].(string)
		require.True(t, ok)
		index[name] = cols
	}

	for _, exp := range expected {
		actual, ok := index[exp[3].(string)]
		require.True(t, ok, "missing expected table row for %s", exp[3])
		require.EqualValues(t, exp[1:], actual[1:])
	}
}

func TestHttpTags(t *testing.T) {
	tests := []struct {
		name      string
		table     string
		expectObj map[string]any
	}{
		{
			name:  "tags",
			table: "example",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"ROWNUM", "NAME"},
					"types":   []any{"int32", "string"},
					"rows": []any{
						[]any{float64(1), "temp"},
						[]any{float64(2), "test.query"},
					},
				},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tables/"+tc.table+"/tags", nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

func TestHttpTagStat(t *testing.T) {
	tests := []struct {
		name      string
		table     string
		tag       string
		expectObj map[string]any
	}{
		{
			name:  "tag_stat",
			table: "example",
			tag:   "temp",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{
						"ROWNUM", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME",
						"MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME", "RECENT_ROW_TIME"},
					"types": []any{"int32", "string", "int64", "datetime", "datetime", "double", "datetime", "double", "datetime", "datetime"},
					"rows": []any{
						[]any{
							float64(1), "temp", float64(1), float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano()),
							3.14, float64(testTimeTick.UnixNano()), 3.14, float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano())},
					},
				},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tables/"+tc.table+"/tags/"+tc.tag+"/stat", nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

func TestTQL(t *testing.T) {
	tests := []struct {
		name        string
		codes       string
		contentType string
		expect      []string
		expectObj   map[string]any
	}{
		{
			name: "csv_output",
			codes: `FAKE(linspace(0,1,2))
					CSV()`,
			contentType: "text/csv; charset=utf-8",
			expect: []string{
				"0", "1", "\n",
			},
		},
		{
			name: "json_output",
			codes: `FAKE(linspace(0,1,2))
					JSON()`,
			contentType: "application/json",
			expectObj: map[string]any{"success": true, "reason": "success", "data": map[string]any{
				"columns": []any{"x"}, "types": []any{"double"},
				"rows": []any{[]any{0.0}, []any{1.0}},
			}},
		},
	}
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+"_"+tc.name, func(t *testing.T) {
				var req *http.Request
				if method == http.MethodGet {
					req, _ = http.NewRequest(method, httpServerAddress+"/web/api/tql?$="+url.QueryEscape(tc.codes), nil)
				} else {
					reader := bytes.NewBufferString(tc.codes)
					req, _ = http.NewRequest(method, httpServerAddress+"/web/api/tql", reader)
				}
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
				rsp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))
				result, _ := io.ReadAll(rsp.Body)
				rsp.Body.Close()
				if tc.expectObj != nil {
					resultObj := map[string]any{}
					err := json.Unmarshal(result, &resultObj)
					require.NoError(t, err)
					delete(resultObj, "elapse")
					require.EqualValues(t, tc.expectObj, resultObj)
				} else {
					require.Equal(t, strings.Join(tc.expect, "\n"), string(result))
				}
			})
		}
	}
}

func TestTQL_Payload(t *testing.T) {
	tests := []struct {
		name        string
		codes       string
		payload     []byte
		payloadType string
		contentType string
		expect      []string
		expectObj   map[string]any
	}{
		{
			name: "csv_from_payload",
			codes: `CSV(payload())
					CSV()`,
			payload:     []byte("a,1\nb,2\n"),
			payloadType: "text/csv",
			contentType: "text/csv; charset=utf-8",
			expect:      []string{"a,1", "b,2", "\n"},
		},
		{
			name:        "csv_map.tql",
			codes:       "@csv_map.tql",
			payload:     []byte("a,1\nb,2\n"),
			payloadType: "text/csv",
			contentType: "text/csv; charset=utf-8",
			expect:      []string{"a,10", "b,20", "\n"},
		},
	}
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			payload := bytes.NewBuffer(tc.payload)
			if strings.HasPrefix(tc.codes, "@") {
				req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql/"+tc.codes[1:], payload)
			} else {
				req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql?$="+url.QueryEscape(tc.codes), payload)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			req.Header.Set("Content-Type", tc.payloadType)

			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			if tc.expectObj != nil {
				resultObj := map[string]any{}
				err := json.Unmarshal(result, &resultObj)
				require.NoError(t, err)
				delete(resultObj, "elapse")
				require.EqualValues(t, tc.expectObj, resultObj)
			} else {
				require.Equal(t, strings.Join(tc.expect, "\n"), string(result))
			}
		})
	}
}

func TestTQL_PublicRedirect(t *testing.T) {
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/tql/public/redirect-policy.txt", nil)
	rsp, err := noRedirectClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, rsp.StatusCode)
	require.Equal(t, "/public/redirect-policy.txt", rsp.Header.Get("Location"))
	rsp.Body.Close()
}

func TestTQL_SyntaxErrors(t *testing.T) {
	tests := []struct {
		name      string
		codes     string
		expectObj map[string]any
	}{
		{
			name: "mapkey_wrong_argument",
			codes: `FAKE(linspace(0,1,2))
					MAPKEY(-1,-1) // intended syntax error
					//APPEND(table('example'))
					JSON()`,
			expectObj: map[string]any{
				"success": true,
				"reason":  "success",
				"data": map[string]any{
					"columns": []any{"x"},
					"types":   []any{"double"},
					"rows":    []any{},
				},
			},
		},
	}
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewBufferString(tc.codes)
			req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql", reader)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

type SplitHttpResult struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    struct {
		Statements []*util.HttpStatement `json:"statements"`
	} `json:"data,omitempty"`
}

func newTestHTTPServer(t *testing.T) *httpd {
	t.Helper()
	root := t.TempDir()
	serverFs, err := ssfs.NewServerSideFileSystem([]string{"/=" + root})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	return &httpd{log: logging.GetLog("httpd-fake"), serverFs: serverFs}
}

func newTestHTTPContext(method, target string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	return ctx, writer
}

func TestTerminalsRegisterFindUnregister(t *testing.T) {
	terms := &Terminals{}
	terms.list = cmap.New[*WebTerm]()

	term := &WebTerm{Rows: 25, Cols: 80}
	terms.Register("user-term", term)

	found, ok := terms.Find("user-term")
	require.True(t, ok)
	require.Same(t, term, found)

	terms.Unregister("user-term")
	_, ok = terms.Find("user-term")
	require.False(t, ok)
}

func TestHandleTermWindowSize(t *testing.T) {
	svr := &httpd{log: logging.GetLog("httpd-fake")}

	t.Run("requires-claim", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/term/term-1/windowsize?rows=24&cols=80", nil)
		ctx.Params = gin.Params{{Key: "term_id", Value: "term-1"}}

		svr.handleTermWindowSize(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "unauthorized access")
	})

	t.Run("rejects-zero-size", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/term/term-1/windowsize?rows=0&cols=80", nil)
		ctx.Params = gin.Params{{Key: "term_id", Value: "term-1"}}
		ctx.Set("jwt-claim", NewClaim("sys"))

		svr.handleTermWindowSize(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "rows or cols can't be zero")
	})

	t.Run("missing-terminal", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/term/term-1/windowsize?rows=24&cols=80", nil)
		ctx.Params = gin.Params{{Key: "term_id", Value: "term-1"}}
		ctx.Set("jwt-claim", NewClaim("sys"))

		svr.handleTermWindowSize(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "not found")
	})

	t.Run("accepts-jsh-terminal", func(t *testing.T) {
		termKey := "sys-term-1"
		terminals.Register(termKey, nil)
		defer terminals.Unregister(termKey)

		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/term/term-1/windowsize?rows=24&cols=80", nil)
		ctx.Params = gin.Params{{Key: "term_id", Value: "term-1"}}
		ctx.Set("jwt-claim", NewClaim("sys"))

		svr.handleTermWindowSize(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":true`)
	})
}

func TestHandleTermData(t *testing.T) {
	svr := &httpd{log: logging.GetLog("httpd-fake")}

	t.Run("rejects missing terminal id", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/term//data?token=ignored", nil)
		ctx.Params = gin.Params{{Key: "term_id", Value: ""}}

		svr.handleTermData(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "invalid termId")
	})

	t.Run("rejects invalid access token before websocket upgrade", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/term/term-1/data?token=invalid", nil)
		ctx.Params = gin.Params{{Key: "term_id", Value: "term-1"}}

		svr.handleTermData(ctx)

		require.Equal(t, http.StatusUnauthorized, writer.Code)
		require.Contains(t, writer.Body.String(), "unauthorized access")
	})
}

func TestNewWebTermInvalidAddress(t *testing.T) {
	term, err := NewWebTerm("invalid-address", "", "sys")
	require.Nil(t, term)
	require.Error(t, err)
	require.Contains(t, err.Error(), "NewTerm dial")
}

func TestWebTermCloseNilSafe(t *testing.T) {
	require.NotPanics(t, func() {
		(&WebTerm{}).Close()
	})
}

func TestHandleFiles(t *testing.T) {
	svr := newTestHTTPServer(t)

	t.Run("create-directory", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/files/docs", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":true`)
	})

	t.Run("write-and-read-file", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/files/docs/readme.md", []byte("hello world"))
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/readme.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)

		ctx, writer = newTestHTTPContext(http.MethodGet, "/web/api/files/docs/readme.md", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/readme.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "text/markdown", writer.Header().Get("Content-Type"))
		require.Equal(t, "hello world", writer.Body.String())
	})

	t.Run("list-directory", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/files/docs", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":true`)
		require.Contains(t, writer.Body.String(), "readme.md")
	})

	t.Run("rename-requires-destination", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPut, "/web/api/files/docs/readme.md", []byte(`{}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/readme.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "destination is not specified")
	})

	t.Run("rename-file", func(t *testing.T) {
		payload, err := json.Marshal(RenameReq{Dest: "/docs/guide.md"})
		require.NoError(t, err)

		ctx, writer := newTestHTTPContext(http.MethodPut, "/web/api/files/docs/readme.md", payload)
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/readme.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)

		ctx, writer = newTestHTTPContext(http.MethodGet, "/web/api/files/docs/guide.md", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/guide.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "hello world", writer.Body.String())
	})

	t.Run("delete-non-empty-directory-without-recursive", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodDelete, "/web/api/files/docs", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "directory is not empty")
	})

	t.Run("delete-file", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodDelete, "/web/api/files/docs/guide.md", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/guide.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)

		ctx, writer = newTestHTTPContext(http.MethodGet, "/web/api/files/docs/guide.md", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/docs/guide.md"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusNotFound, writer.Code)
	})

	t.Run("delete-directory-recursively", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/files/tree", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/tree"}}
		svr.handleFiles(ctx)
		require.Equal(t, http.StatusOK, writer.Code)

		ctx, writer = newTestHTTPContext(http.MethodPost, "/web/api/files/tree/child", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/tree/child"}}
		svr.handleFiles(ctx)
		require.Equal(t, http.StatusOK, writer.Code)

		require.NoError(t, svr.serverFs.Set("/tree/child/note.txt", []byte("data")))

		ctx, writer = newTestHTTPContext(http.MethodDelete, "/web/api/files/tree?recursive=true", nil)
		ctx.Params = gin.Params{{Key: "path", Value: "/tree"}}

		svr.handleFiles(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":true`)
	})
}

func newTestWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	serverConnCh := make(chan *websocket.Conn, 1)
	errCh := make(chan error, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errCh <- err
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(ts.Close)

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	require.NoError(t, err)

	var serverConn *websocket.Conn
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case serverConn = <-serverConnCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for websocket upgrade")
	}

	t.Cleanup(func() {
		clientConn.Close()
		if serverConn != nil {
			serverConn.Close()
		}
	})

	return clientConn, serverConn
}

func newTestWebConsole(conn *websocket.Conn) *WebConsole {
	return &WebConsole{
		log:           logging.GetLog("test-web-console"),
		topic:         "console:test:ws",
		conn:          conn,
		messages:      []*eventbus.Event{},
		lastFlushTime: time.Now(),
		flushPeriod:   time.Hour,
	}
}

func TestWsReadWriterRead(t *testing.T) {
	t.Run("continues across frame boundaries", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		reader := &WsReadWriter{Conn: clientConn}

		require.NoError(t, serverConn.WriteMessage(websocket.BinaryMessage, []byte("hello")))
		require.NoError(t, serverConn.WriteMessage(websocket.BinaryMessage, []byte("world")))
		require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

		buf := make([]byte, 3)
		n, err := reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "hel", string(buf[:n]))

		buf = make([]byte, 2)
		n, err = reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "lo", string(buf[:n]))

		buf = make([]byte, 5)
		n, err = reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "world", string(buf[:n]))
	})

	t.Run("propagates next reader errors after frame eof", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		reader := &WsReadWriter{Conn: clientConn}

		require.NoError(t, serverConn.WriteMessage(websocket.BinaryMessage, []byte("hello")))
		require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

		buf := make([]byte, 3)
		n, err := reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 3, n)
		require.Equal(t, "hel", string(buf[:n]))

		buf = make([]byte, 2)
		n, err = reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, "lo", string(buf[:n]))

		require.NoError(t, serverConn.Close())

		buf = make([]byte, 8)
		n, err = reader.Read(buf)
		require.Zero(t, n)
		require.Error(t, err)
	})
}

func TestWsReadWriterWrite(t *testing.T) {
	t.Run("writes binary frames", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		writer := &WsReadWriter{Conn: clientConn}

		require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(time.Second)))
		n, err := writer.Write([]byte("payload"))
		require.NoError(t, err)
		require.Equal(t, len("payload"), n)

		msgType, payload, err := serverConn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.BinaryMessage, msgType)
		require.Equal(t, []byte("payload"), payload)
	})

	t.Run("returns write errors", func(t *testing.T) {
		clientConn, _ := newTestWebsocketPair(t)
		writer := &WsReadWriter{Conn: clientConn}

		require.NoError(t, clientConn.Close())
		n, err := writer.Write([]byte("payload"))
		require.Zero(t, n)
		require.Error(t, err)
	})
}

func TestWebConsoleSend(t *testing.T) {
	t.Run("coalesces repeated log messages", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		cons := newTestWebConsole(clientConn)

		cons.Send(eventbus.NewLog("INFO", "same message"))
		cons.Send(eventbus.NewLog("INFO", "same message"))

		require.Len(t, cons.messages, 1)
		require.Equal(t, 2, cons.messages[0].Log.Repeat)

		cons.lastFlushTime = time.Now().Add(-2 * time.Hour)
		require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(time.Second)))
		cons.Send(nil)

		evt := &eventbus.Event{}
		require.NoError(t, serverConn.ReadJSON(evt))
		require.Equal(t, eventbus.EVT_LOG, evt.Type)
		require.Equal(t, "same message", evt.Log.Message)
		require.Equal(t, 2, evt.Log.Repeat)
		require.Empty(t, cons.messages)
	})

	t.Run("non log events force pending logs to flush", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		cons := newTestWebConsole(clientConn)

		cons.Send(eventbus.NewLog("INFO", "pending log"))
		require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(time.Second)))
		cons.Send(&eventbus.Event{Type: eventbus.EVT_OPEN_FILE, OpenFile: &eventbus.OpenFile{Path: "/tmp/result.txt"}})

		first := &eventbus.Event{}
		second := &eventbus.Event{}
		require.NoError(t, serverConn.ReadJSON(first))
		require.NoError(t, serverConn.ReadJSON(second))
		require.Equal(t, eventbus.EVT_LOG, first.Type)
		require.Equal(t, "pending log", first.Log.Message)
		require.Equal(t, eventbus.EVT_OPEN_FILE, second.Type)
		require.Equal(t, "/tmp/result.txt", second.OpenFile.Path)
	})

	t.Run("write failure closes the console", func(t *testing.T) {
		clientConn, _ := newTestWebsocketPair(t)
		cons := newTestWebConsole(clientConn)
		cons.flushPeriod = 0
		cons.lastFlushTime = time.Now().Add(-time.Second)

		require.NoError(t, clientConn.Close())
		cons.Send(eventbus.NewLog("INFO", "will fail"))

		require.True(t, cons.closed.Load())
	})
}

func TestWebTermCoverage_NewSetWindowSizeClose(t *testing.T) {
	hostPort := fmt.Sprintf("127.0.0.1:%d", shellPort)

	term, err := NewWebTerm(hostPort, "", "sys")
	require.NoError(t, err)
	require.NotNil(t, term)

	err = term.SetWindowSize(32, 96)
	require.NoError(t, err)
	require.Equal(t, 32, term.Rows)
	require.Equal(t, 96, term.Cols)

	term.Close()
}

func TestWebTermCoverage_NewWebTermErrorPath(t *testing.T) {
	_, err := NewWebTerm("127.0.0.1:1", "", "sys")
	require.Error(t, err)
	require.Contains(t, err.Error(), "NewTerm dial")
}

func TestStaticFSWrapOpenAppliesPrefixAndFixedModTime(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "assets", "hello.txt"), []byte("hello"), 0o644))

	fixedTime := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	fsw := &StaticFSWrap{
		TrimPrefix:      "/web/",
		PrependRealPath: "/assets/",
		Base:            http.Dir(tempDir),
		FixedModTime:    fixedTime,
	}

	file, err := fsw.Open("hello.txt")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })

	body, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	require.Equal(t, fixedTime, file.(*staticFile).ModTime())

	stat, err := file.Stat()
	require.NoError(t, err)
	require.Equal(t, fixedTime, stat.ModTime())
}

func TestWrapAssetsFallbackToIndexHTML(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("index page"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "app.js"), []byte("console.log('ok')"), 0o644))

	fs := WrapAssets(tempDir)

	assetFile, err := fs.Open("app.js")
	require.NoError(t, err)
	assetBody, err := io.ReadAll(assetFile)
	require.NoError(t, err)
	require.NoError(t, assetFile.Close())
	require.Equal(t, "console.log('ok')", string(assetBody))

	fallbackFile, err := fs.Open("missing/route")
	require.NoError(t, err)
	fallbackBody, err := io.ReadAll(fallbackFile)
	require.NoError(t, err)
	require.NoError(t, fallbackFile.Close())
	require.Equal(t, "index page", string(fallbackBody))
}

func TestGetAssetsServesKnownFileAndSpaFallback(t *testing.T) {
	fs := GetAssets("ui")

	assetFile, err := fs.Open("/vite.svg?cache=1")
	require.NoError(t, err)
	assetBody, err := io.ReadAll(assetFile)
	require.NoError(t, err)
	require.NoError(t, assetFile.Close())
	require.Contains(t, string(assetBody), "<svg")

	fallbackFile, err := fs.Open("dashboard/metrics")
	require.NoError(t, err)
	fallbackBody, err := io.ReadAll(fallbackFile)
	require.NoError(t, err)
	require.NoError(t, fallbackFile.Close())
	require.Contains(t, string(fallbackBody), "<!DOCTYPE html>")
}

func TestIsWellKnownFileType(t *testing.T) {
	require.True(t, isWellKnownFileType("app.js"))
	require.True(t, isWellKnownFileType("image.WEBP"))
	require.True(t, isWellKnownFileType("font.woff2"))
	require.False(t, isWellKnownFileType("README"))
	require.False(t, isWellKnownFileType("archive.tar.gz"))
	require.False(t, isWellKnownFileType("custom.bin"))
}

func TestHttpRpc(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	var originalSessionLimit map[string]any
	generatedKeyID := fmt.Sprintf("rpc-test-key-%d", time.Now().UnixNano())
	sshFixture := newTestSSHKeyFixture(t)
	sshKeyMaterial := sshFixture.AuthorizedKey
	sshKeyFingerprint := sshFixture.Fingerprint
	sshKeyComment := fmt.Sprintf("rpc-test-comment-%d", time.Now().UnixNano())
	var addedSshKeyFingerprint string

	JsonRpcTestCase{
		name:   "method-not-found",
		method: "nonExistentMethod",
		params: []interface{}{},
		expectFunc: func(t *testing.T, jsonRsp gjson.Result) {
			require.True(t, jsonRsp.Get("error").Exists())
			require.Equal(t, int64(-32601), jsonRsp.Get("error.code").Int())
			require.Equal(t, "Method not found", jsonRsp.Get("error.message").String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getServerInfo",
		method: "server.info.get",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, runtime.GOOS, rsp.Get("result.runtime.OS").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getServerStatz",
		method: "server.info.statz",
		params: []interface{}{[]string{"http:count"}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
			require.Equal(t, "http:count", rsp.Get("result.statz.0.name").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:       "getServicePorts",
		method:     "service.port.list",
		params:     []interface{}{"mach"},
		expectJSON: fmt.Sprintf(`[{"Service":"mach", "Address":"%s"}]`, machServerAddress),
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getServerCertificate",
		method: "server.certificate.get",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			certificate := rsp.Get("result").String()
			require.Contains(t, certificate, "BEGIN CERTIFICATE", rsp.String())
			require.Contains(t, certificate, "END CERTIFICATE", rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getSessionLimit",
		method: "session.limit.get",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.True(t, result.Get("maxOpenConn").Exists(), rsp.String())
			require.True(t, result.Get("maxIdleConn").Exists(), rsp.String())
			require.True(t, result.Get("connMaxIdleTime").Exists(), rsp.String())
			require.True(t, result.Get("connMaxLifetime").Exists(), rsp.String())
			originalSessionLimit = map[string]any{
				"maxOpenConn":     int(result.Get("maxOpenConn").Int()),
				"maxIdleConn":     int(result.Get("maxIdleConn").Int()),
				"connMaxIdleTime": result.Get("connMaxIdleTime").String(),
				"connMaxLifetime": result.Get("connMaxLifetime").String(),
			}
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "splitSqlStatements",
		method: "sql.split",
		params: []interface{}{"select 1;\nselect 2;"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Len(t, result.Array(), 2, rsp.String())
			require.Equal(t, "select 1;", strings.TrimSpace(result.Get("0.text").String()), rsp.String())
			require.Equal(t, int64(1), result.Get("0.beginLine").Int(), rsp.String())
			require.Equal(t, int64(1), result.Get("0.endLine").Int(), rsp.String())
			require.False(t, result.Get("0.isComment").Bool(), rsp.String())
			require.Equal(t, "select 2;", strings.TrimSpace(result.Get("1.text").String()), rsp.String())
			require.Equal(t, int64(2), result.Get("1.beginLine").Int(), rsp.String())
			require.Equal(t, int64(2), result.Get("1.endLine").Int(), rsp.String())
			require.False(t, result.Get("1.isComment").Bool(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "splitSql_first",
		method: "sql.split",
		params: []interface{}{`select * from first;`},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.JSONEq(t, `[{"text":"select * from first;","beginLine":1,"endLine":1,"isComment":false,"stmtType":"select","env":{}}]`, result.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "splitSql_second",
		method: "sql.split",
		params: []interface{}{"\nselect * from second;  "},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.JSONEq(t, `[{"text":"select * from second;","beginLine":2,"endLine":2,"isComment":false,"stmtType":"select","env":{}}]`, result.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "splitSql_filters_named_environment_per_statement",
		method: "sql.split",
		params: []interface{}{`-- env: named.name=my-car named.value=1.5432
INSERT INTO example VALUES(:name, now, :value);
SELECT * FROM example WHERE name = :name;`},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Len(t, result.Array(), 3, rsp.String())
			require.Equal(t, "my-car", result.Get("1.env.named.name").String(), rsp.String())
			require.Equal(t, "1.5432", result.Get("1.env.named.value").String(), rsp.String())
			require.Equal(t, "my-car", result.Get("2.env.named.name").String(), rsp.String())
			require.False(t, result.Get("2.env.named.value").Exists(), rsp.String())
			require.False(t, result.Get("2.env.error").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "splitSql_reports_missing_named_environment_value",
		method: "sql.split",
		params: []interface{}{`-- env: named.name=my-car
SELECT * FROM example WHERE name = :name2;`},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Len(t, result.Array(), 2, rsp.String())
			require.False(t, result.Get("1.env.named").Exists(), rsp.String())
			require.Equal(t, `named parameter "name2" is not defined in the environment`, result.Get("1.env.error").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "httpSplit_simple_get",
		method: "http.split",
		params: []interface{}{`GET /web/api/tables HTTP/1.1
Host: localhost:8080`},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.JSONEq(t,
				`[{"beginLine":1, "endLine":2, "text":"GET /web/api/tables HTTP/1.1\nHost: localhost:8080\n"}]`,
				result.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "httpSplit_simple_multiple",
		method: "http.split",
		params: []interface{}{"\n###\nGET /abc\n###\nGET /def\n###\nGET /gih"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.JSONEq(t,
				`[{"beginLine":3, "endLine":3, "text":"GET /abc\n"}, {"beginLine":5, "endLine":5, "text":"GET /def\n"}, {"beginLine":7, "endLine":7, "text":"GET /gih\n"}]`,
				result.String())
		},
	}.run(t, at)
	require.NotNil(t, originalSessionLimit)

	var generatedKeyId int64
	JsonRpcTestCase{
		name:   "listKeys_beforeGenerate",
		method: "key.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			for _, item := range rsp.Get("result").Array() {
				require.NotEqual(t, generatedKeyID, item.Get("name").String(), rsp.String())
			}
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "generateKey",
		method: "key.generate",
		params: []interface{}{generatedKeyID, "ecdsa", 0, 0, true},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, generatedKeyID, result.Get("name").String(), rsp.String())
			require.NotZero(t, result.Get("id").Int(), rsp.String())
			generatedKeyId = result.Get("id").Int()
			require.Contains(t, result.Get("certificate").String(), "BEGIN CERTIFICATE", rsp.String())
			privateKey := result.Get("key").String()
			require.Contains(t, privateKey, "BEGIN ", rsp.String())
			require.Contains(t, privateKey, "PRIVATE KEY", rsp.String())
			require.Empty(t, result.Get("token").String(), rsp.String())
			require.NotEmpty(t, result.Get("zip").String(), rsp.String())
			require.NotEmpty(t, result.Get("serverKey").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listKeys_afterGenerate",
		method: "key.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			found := false
			for _, item := range rsp.Get("result").Array() {
				if item.Get("name").String() == generatedKeyID {
					found = true
					break
				}
			}
			require.True(t, found, rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteKey",
		method: "key.delete",
		params: []interface{}{generatedKeyId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listKeys_afterDelete",
		method: "key.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			for _, item := range rsp.Get("result").Array() {
				require.NotEqual(t, generatedKeyID, item.Get("name").String(), rsp.String())
			}
		},
	}.run(t, at)

	JsonRpcTestCase{
		name:   "listSshKeys_beforeAdd",
		method: "sshkey.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			for _, item := range rsp.Get("result").Array() {
				require.NotEqual(t, sshKeyFingerprint, item.Get("fingerprint").String(), rsp.String())
			}
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "addSshKey",
		method: "sshkey.add",
		params: []interface{}{"ed25519", sshKeyMaterial, sshKeyComment},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listSshKeys_afterAdd",
		method: "sshkey.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			found := false
			for _, item := range rsp.Get("result").Array() {
				if item.Get("Fingerprint").String() == sshKeyFingerprint {
					found = true
					addedSshKeyFingerprint = item.Get("Fingerprint").String()
					require.Equal(t, sshKeyFingerprint, item.Get("Fingerprint").String(), rsp.String())
				}
			}
			require.True(t, found, rsp.String())
			require.NotEmpty(t, addedSshKeyFingerprint, rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteSshKey",
		method: "sshkey.delete",
		params: []interface{}{addedSshKeyFingerprint},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.NotEmpty(t, addedSshKeyFingerprint)
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listSshKeys_afterDelete",
		method: "sshkey.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			for _, item := range rsp.Get("result").Array() {
				require.NotEqual(t, sshKeyFingerprint, item.Get("fingerprint").String(), rsp.String())
			}
		},
	}.run(t, at)

	readFile := func(path string) string {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		// replace \r\n with \n to avoid line ending issues on Windows
		content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
		return string(content)
	}

	JsonRpcTestCase{
		name:   "markdownRender-light",
		method: "markdown.render",
		params: []interface{}{"# Hello World\n\nThis is a **test**.", false},
		expectFunc: func(t *testing.T, result gjson.Result) {
			html := result.Get("result").String()
			require.Contains(t, html, "<h1")
			require.Contains(t, html, "Hello World")
			require.Contains(t, html, "<strong>test</strong>")
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "markdownRender-dark",
		method: "markdown.render",
		params: []interface{}{"## Dark Mode Test\n\n- Item 1\n- Item 2", true},
		expectFunc: func(t *testing.T, result gjson.Result) {
			html := result.Get("result").String()
			require.Contains(t, html, "<h2")
			require.Contains(t, html, "Dark Mode Test")
			require.Contains(t, html, "<li>Item 1</li>")
			require.Contains(t, html, "<li>Item 2</li>")
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "markdownRender-http-fence",
		method: "markdown.render",
		params: []interface{}{"## HTTP Test\n\n```http\nGET " +
			httpServerAddress + "/db/query?q=select * from example limit 1\n```\n", false},
		expectFunc: func(t *testing.T, result gjson.Result) {
			html := result.Get("result").String()
			require.Contains(t, html, "<h2")
			require.Contains(t, html, "HTTP Test")
			require.Contains(t, html, "<span class=\"httpext-method\">GET</span> <span class=\"httpext-path\">/db/query</span>?")
			require.Contains(t, html, "HTTP/1.1")
			require.Contains(t, html, "OK")
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "markdownRender-list",
		method: "markdown.render",
		params: []interface{}{readFile("./test/test_markdown_list.md"), false, httpServerAddress + "/web/api/tql/sample/file.wrk"},
		expectFunc: func(t *testing.T, result gjson.Result) {
			html := result.Get("result").String()
			require.Equal(t, readFile("./test/test_markdown_list.txt"), html)
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "markdownRender-utf8",
		method: "markdown.render",
		params: []interface{}{readFile("./test/test_markdown_list_utf8.md"), false, httpServerAddress + "/web/api/tql/语言/文檔.wrk"},
		expectFunc: func(t *testing.T, result gjson.Result) {
			html := result.Get("result").String()
			require.Equal(t, readFile("./test/test_markdown_list_utf8.txt"), html)
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "markdownRender-mermaid",
		method: "markdown.render",
		params: []interface{}{readFile("./test/test_markdown_mermaid.md"), false, httpServerAddress + "/web/api/tql/diagram.wrk"},
		expectFunc: func(t *testing.T, result gjson.Result) {
			html := result.Get("result").String()
			require.Equal(t, readFile("./test/test_markdown_mermaid.txt"), html)
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "vizspecRender-passthrough",
		method: "vizspec.render",
		params: []interface{}{map[string]any{
			"schema": "vizspec/v1",
			"kind":   "timeseries",
			"data": map[string]any{
				"x":      []any{"t1", "t2"},
				"series": []any{map[string]any{"name": "value", "data": []any{1, 2}}},
			},
		}},
		expectFunc: func(t *testing.T, result gjson.Result) {
			require.Equal(t, "vizspec/v1", result.Get("result.schema").String())
			require.Equal(t, "timeseries", result.Get("result.kind").String())
			require.Equal(t, "t1", result.Get("result.data.x.0").String())
			require.Equal(t, "value", result.Get("result.data.series.0.name").String())
			require.Equal(t, int64(1), result.Get("result.data.series.0.data.0").Int())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "vizspecExport-svg",
		method: "vizspec.export",
		params: []interface{}{map[string]any{
			"schema": "vizspec/v1",
			"kind":   "timeseries",
			"data": map[string]any{
				"x":      []any{"t1", "t2"},
				"series": []any{map[string]any{"name": "value", "data": []any{1, 2}}},
			},
		}, "svg"},
		expectFunc: func(t *testing.T, result gjson.Result) {
			require.Equal(t, "vizspec-export/v1", result.Get("result.schema").String())
			require.Equal(t, "svg", result.Get("result.format").String())
			require.Equal(t, "image/svg+xml", result.Get("result.mimeType").String())
			data := result.Get("result.data").String()
			require.Contains(t, data, "<svg")
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "vizspecExport-png",
		method: "vizspec.export",
		params: []interface{}{map[string]any{
			"schema": "vizspec/v1",
			"kind":   "timeseries",
			"data": map[string]any{
				"x":      []any{"t1", "t2"},
				"series": []any{map[string]any{"name": "value", "data": []any{1, 2}}},
			},
		}, "png"},
		expectFunc: func(t *testing.T, result gjson.Result) {
			require.Equal(t, "vizspec-export/v1", result.Get("result.schema").String())
			require.Equal(t, "png", result.Get("result.format").String())
			require.Equal(t, "image/png", result.Get("result.mimeType").String())
			data := result.Get("result.data").String()
			require.NotEmpty(t, data)
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "vizspecExport-echarts",
		method: "vizspec.export",
		params: []interface{}{map[string]any{
			"schema": "vizspec/v1",
			"kind":   "timeseries",
			"data": map[string]any{
				"x":      []any{"t1", "t2"},
				"series": []any{map[string]any{"name": "value", "data": []any{1, 2}}},
			},
		}, "echarts"},
		expectFunc: func(t *testing.T, result gjson.Result) {
			require.Equal(t, "vizspec-export/v1", result.Get("result.schema").String())
			require.Equal(t, "echarts", result.Get("result.format").String())
			require.Equal(t, "application/json", result.Get("result.mimeType").String())
			require.Equal(t, "line", result.Get("result.data.series.0.type").String())
		},
	}.run(t, at)

	originalDebugEnabled, originalDebugLatency := httpServer.DebugMode()
	targetDebugEnabled := !originalDebugEnabled
	targetDebugLatency := 250 * time.Millisecond
	if originalDebugLatency == targetDebugLatency {
		targetDebugLatency = 100 * time.Millisecond
	}
	JsonRpcTestCase{
		name:   "setHttpDebug",
		method: "http.debug.set",
		params: []interface{}{map[string]any{"enable": targetDebugEnabled, "logLatency": targetDebugLatency.String()}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, targetDebugEnabled, result.Get("enable").Bool(), rsp.String())
			require.Equal(t, targetDebugLatency.String(), result.Get("logLatency").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "restoreHttpDebug",
		method: "http.debug.set",
		params: []interface{}{map[string]any{"enable": originalDebugEnabled, "logLatency": originalDebugLatency.String()}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, originalDebugEnabled, result.Get("enable").Bool(), rsp.String())
			require.Equal(t, originalDebugLatency.String(), result.Get("logLatency").String(), rsp.String())
		},
	}.run(t, at)
}

func TestHttpRpc_timer(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)
	var addedTimerId int64

	JsonRpcTestCase{
		name:   "listTimers_beforeAdd",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Empty(t, rsp.Get("result").Array(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "addTimer_not_exists",
		method: "timer.add",
		params: []interface{}{map[string]any{
			"name":      "test-timer",
			"spec":      "*/1 * * * * *",
			"command":   "test-timer-not_exists.tql",
			"autoStart": true,
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, -32000, int(rsp.Get("error.code").Int()), rsp.String())
			require.Equal(t, "not found 'test-timer-not_exists.tql'", rsp.Get("error.message").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "addTimer",
		method: "timer.add",
		params: []interface{}{map[string]any{
			"name":      "test-timer",
			"spec":      "0 30 * * * *",
			"command":   "csv_map.tql",
			"autoStart": false,
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			addedTimerId = rsp.Get("result").Int()
			require.NotZero(t, addedTimerId, rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.get",
		method: "timer.get",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, addedTimerId, result.Get("id").Int(), rsp.String())
			require.Equal(t, "TEST-TIMER", result.Get("name").String(), rsp.String())
			require.Equal(t, "0 30 * * * *", result.Get("schedule").String(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("task").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.update",
		method: "timer.update",
		params: []interface{}{map[string]any{
			"id":        addedTimerId,
			"spec":      "0 30 * * * *",
			"command":   "csv_map.tql",
			"autoStart": false,
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.get_afterUpdate",
		method: "timer.get",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Equal(t, "0 30 * * * *", rsp.Get("result.schedule").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listTimers_afterAdd",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.NotEmpty(t, rsp.Get("result").Array(), rsp.String())
			require.Equal(t, "TEST-TIMER", rsp.Get("result.0.name").String(), rsp.String())
			require.Equal(t, addedTimerId, rsp.Get("result.0.id").Int(), rsp.String())
			require.Equal(t, "0 30 * * * *", rsp.Get("result.0.schedule").String(), rsp.String())
			require.Equal(t, "STOP", rsp.Get("result.0.state").String(), rsp.String())
			require.Equal(t, "csv_map.tql", rsp.Get("result.0.task").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "startTimer",
		method: "timer.start",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listTimers_afterStart",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.NotEmpty(t, rsp.Get("result").Array(), rsp.String())
			require.Equal(t, "TEST-TIMER", rsp.Get("result.0.name").String(), rsp.String())
			require.Equal(t, addedTimerId, rsp.Get("result.0.id").Int(), rsp.String())
			require.Equal(t, "0 30 * * * *", rsp.Get("result.0.schedule").String(), rsp.String())
			require.Equal(t, "RUNNING", rsp.Get("result.0.state").String(), rsp.String())
			require.Equal(t, "csv_map.tql", rsp.Get("result.0.task").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteTimer",
		method: "timer.delete",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listTimers_afterDelete",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Empty(t, rsp.Get("result").Array(), rsp.String())
		},
	}.run(t, at)

	JsonRpcTestCase{
		name:   "timer.list_beforeAdd",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Empty(t, rsp.Get("result").Array(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.add_not_exists",
		method: "timer.add",
		params: []interface{}{map[string]any{
			"name":      "test-timer2",
			"spec":      "*/1 * * * * *",
			"command":   "test-timer-not_exists.tql",
			"authStart": true,
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, -32000, int(rsp.Get("error.code").Int()), rsp.String())
			require.Equal(t, "not found 'test-timer-not_exists.tql'", rsp.Get("error.message").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.add",
		method: "timer.add",
		params: []interface{}{map[string]any{
			"name":      "test-timer2",
			"spec":      "0 30 * * * *",
			"command":   "csv_map.tql",
			"authStart": true,
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			addedTimerId = rsp.Get("result").Int()
			require.NotZero(t, addedTimerId, rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.list_afterAdd",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.NotEmpty(t, rsp.Get("result").Array(), rsp.String())
			require.Equal(t, "TEST-TIMER2", rsp.Get("result.0.name").String(), rsp.String())
			require.Equal(t, addedTimerId, rsp.Get("result.0.id").Int(), rsp.String())
			require.Equal(t, "0 30 * * * *", rsp.Get("result.0.schedule").String(), rsp.String())
			require.Equal(t, "STOP", rsp.Get("result.0.state").String(), rsp.String())
			require.Equal(t, "csv_map.tql", rsp.Get("result.0.task").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.get",
		method: "timer.get",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, addedTimerId, result.Get("id").Int(), rsp.String())
			require.Equal(t, "TEST-TIMER2", result.Get("name").String(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("task").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.update",
		method: "timer.update",
		params: []interface{}{map[string]any{
			"id":        addedTimerId,
			"spec":      "0 45 * * * *",
			"command":   "csv_map.tql",
			"autoStart": false,
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.get_afterUpdate",
		method: "timer.get",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Equal(t, "0 45 * * * *", rsp.Get("result.schedule").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.start",
		method: "timer.start",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.get_afterStart",
		method: "timer.get",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Equal(t, "RUNNING", rsp.Get("result.state").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.stop",
		method: "timer.stop",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.delete",
		method: "timer.delete",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.get_afterDelete",
		method: "timer.get",
		params: []interface{}{addedTimerId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, -32000, int(rsp.Get("error.code").Int()), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "timer.list_afterDelete",
		method: "timer.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Empty(t, rsp.Get("result").Array(), rsp.String())
		},
	}.run(t, at)
}

func TestHttpRpc_bridgeAndSubscriber(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	bridgeTableName := fmt.Sprintf("rpc_bridge_%d", time.Now().UnixNano())
	insertedBridgeMemo := fmt.Sprintf("rpc-row-%d", time.Now().UnixNano())
	insertedBridgeCreatedOn := "2023-09-09T00:00:00Z"
	var bridgeQueryHandle string
	var addedSubscriberId int64

	JsonRpcTestCase{
		name:   "addBridge",
		method: "bridge.add",
		params: []interface{}{"br-test", "sqlite", "file::memory:?cache=shared"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listBridges",
		method: "bridge.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, 1, len(result.Array()), rsp.String())
			require.Equal(t, "br-test", result.Get("0.name").String(), rsp.String())
			require.Equal(t, "sqlite", result.Get("0.type").String(), rsp.String())
			require.Equal(t, "file::memory:?cache=shared", result.Get("0.path").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getBridge",
		method: "bridge.get",
		params: []interface{}{"br-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, "br-test", result.Get("name").String(), rsp.String())
			require.Equal(t, "sqlite", result.Get("type").String(), rsp.String())
			require.Equal(t, "file::memory:?cache=shared", result.Get("path").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "testBridge",
		method: "bridge.test",
		params: []interface{}{"br-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Bool(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "statsBridge_beforeExec",
		method: "bridge.stats",
		params: []interface{}{"br-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("error").Exists(), rsp.String())
			require.Equal(t, int64(-32000), rsp.Get("error.code").Int(), rsp.String())
			require.Contains(t, rsp.Get("error.message").String(), "does not support stats", rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "execBridge_createTable",
		method: "bridge.exec",
		params: []interface{}{"br-test", fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER NOT NULL PRIMARY KEY, memo TEXT, created_on DATETIME NOT NULL)", bridgeTableName)},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, "success", result.Get("Reason").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "execBridge_insertRow",
		method: "bridge.exec",
		params: []interface{}{"br-test", fmt.Sprintf("INSERT INTO %s(id, memo, created_on) VALUES(1, '%s', '2023-09-09 00:00:00Z')", bridgeTableName, insertedBridgeMemo)},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, "success", result.Get("Reason").String(), rsp.String())
			require.Equal(t, int64(1), result.Get("RowsAffected").Int(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "queryBridge_selectRows",
		method: "bridge.query",
		params: []interface{}{"br-test", fmt.Sprintf("SELECT id, memo, created_on FROM %s ORDER BY id", bridgeTableName)},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			bridgeQueryHandle = result.Get("Handle").String()
			require.NotEmpty(t, bridgeQueryHandle, rsp.String())
			require.Equal(t, "id", result.Get("Columns.0.Name").String(), rsp.String())
			require.Equal(t, "memo", result.Get("Columns.1.Name").String(), rsp.String())
			require.Equal(t, "created_on", result.Get("Columns.2.Name").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "fetchBridgeResult_row",
		method: "bridge.result.fetch",
		params: []interface{}{func() string { return bridgeQueryHandle }()},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.False(t, result.Get("HasNoRows").Bool(), rsp.String())
			require.Equal(t, int64(1), result.Get("Values.0").Int(), rsp.String())
			require.Equal(t, insertedBridgeMemo, result.Get("Values.1").String(), rsp.String())
			require.Equal(t, insertedBridgeCreatedOn, result.Get("Values.2").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "fetchBridgeResult_noRows",
		method: "bridge.result.fetch",
		params: []interface{}{func() string { return bridgeQueryHandle }()},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.True(t, result.Get("HasNoRows").Bool(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "closeBridgeResult",
		method: "bridge.result.close",
		params: []interface{}{func() string { return bridgeQueryHandle }()},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "execBridge_dropTable",
		method: "bridge.exec",
		params: []interface{}{"br-test", fmt.Sprintf("DROP TABLE %s", bridgeTableName)},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, "success", rsp.Get("result.Reason").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "statsBridge_afterExec",
		method: "bridge.stats",
		params: []interface{}{"br-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("error").Exists(), rsp.String())
			require.Equal(t, int64(-32000), rsp.Get("error.code").Int(), rsp.String())
			require.Contains(t, rsp.Get("error.message").String(), "does not support stats", rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteBridge",
		method: "bridge.delete",
		params: []interface{}{"br-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "addMqttBridge",
		method: "bridge.add",
		params: []interface{}{"mqtt-test", "mqtt", map[string]any{
			"broker": mqttServerAddress,
			"id":     "client-id",
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "addMqttSubscriber",
		method: "subscriber.add",
		params: []interface{}{map[string]any{
			"name":      "mqtt-subscriber",
			"autoStart": false,
			"command":   "csv_map.tql",
			"bridge":    "mqtt-test",
			"mqtt": map[string]any{
				"topic": "test/topic",
				"qos":   0,
			},
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			addedSubscriberId = rsp.Get("result").Int()
			require.NotZero(t, addedSubscriberId, rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listMqttSubscribers",
		method: "subscriber.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, 1, len(result.Array()), rsp.String())
			require.Equal(t, "MQTT-SUBSCRIBER", result.Get("0.name").String(), rsp.String())
			require.Equal(t, addedSubscriberId, result.Get("0.id").Int(), rsp.String())
			require.Equal(t, "STOP", result.Get("0.state").String(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("0.task").String(), rsp.String())
			require.Equal(t, "mqtt-test", result.Get("0.bridge").String(), rsp.String())
			require.Equal(t, "test/topic", result.Get("0.topic").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getMqttSubscriber",
		method: "subscriber.get",
		params: []interface{}{addedSubscriberId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, addedSubscriberId, result.Get("id").Int(), rsp.String())
			require.Equal(t, "MQTT-SUBSCRIBER", result.Get("name").String(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("task").String(), rsp.String())
			require.Equal(t, "mqtt-test", result.Get("bridge").String(), rsp.String())
			require.Equal(t, "test/topic", result.Get("topic").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "updateMqttSubscriber",
		method: "subscriber.update",
		params: []interface{}{map[string]any{
			"id":        addedSubscriberId,
			"autoStart": false,
			"command":   "csv_map.tql",
			"bridge":    "mqtt-test",
			"mqtt": map[string]any{
				"topic": "test/topic-updated",
				"qos":   1,
			},
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getMqttSubscriber_afterUpdate",
		method: "subscriber.get",
		params: []interface{}{addedSubscriberId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, addedSubscriberId, result.Get("id").Int(), rsp.String())
			require.Equal(t, "MQTT-SUBSCRIBER", result.Get("name").String(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("task").String(), rsp.String())
			require.Equal(t, "mqtt-test", result.Get("bridge").String(), rsp.String())
			require.Equal(t, "test/topic-updated", result.Get("topic").String(), rsp.String())
			require.Equal(t, int64(1), result.Get("qos").Int(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteMqttSubscriber",
		method: "subscriber.delete",
		params: []interface{}{addedSubscriberId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteMqttBridge",
		method: "bridge.delete",
		params: []interface{}{"mqtt-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)

	JsonRpcTestCase{
		name:   "addMqttBridge2",
		method: "bridge.add",
		params: []interface{}{"mqtt-test2", "mqtt", map[string]any{
			"broker": mqttServerAddress,
			"id":     "client-id2",
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.list_beforeAdd",
		method: "subscriber.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Empty(t, rsp.Get("result").Array(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.add",
		method: "subscriber.add",
		params: []interface{}{map[string]any{
			"name":      "mqtt-subscriber2",
			"autoStart": false,
			"command":   "csv_map.tql",
			"bridge":    "mqtt-test2",
			"mqtt": map[string]any{
				"topic": "test/topic2",
				"qos":   0,
			},
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			addedSubscriberId = rsp.Get("result").Int()
			require.NotZero(t, addedSubscriberId, rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.list_afterAdd",
		method: "subscriber.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, 1, len(result.Array()), rsp.String())
			require.Equal(t, "MQTT-SUBSCRIBER2", result.Get("0.name").String(), rsp.String())
			require.Equal(t, addedSubscriberId, result.Get("0.id").Int(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("0.task").String(), rsp.String())
			require.Equal(t, "mqtt-test2", result.Get("0.bridge").String(), rsp.String())
			require.Equal(t, "test/topic2", result.Get("0.topic").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.get",
		method: "subscriber.get",
		params: []interface{}{addedSubscriberId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			result := rsp.Get("result")
			require.Equal(t, addedSubscriberId, result.Get("id").Int(), rsp.String())
			require.Equal(t, "MQTT-SUBSCRIBER2", result.Get("name").String(), rsp.String())
			require.Equal(t, "csv_map.tql", result.Get("task").String(), rsp.String())
			require.Equal(t, "mqtt-test2", result.Get("bridge").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.delete",
		method: "subscriber.delete",
		params: []interface{}{addedSubscriberId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.False(t, rsp.Get("error").Exists(), rsp.String())
			require.Nil(t, rsp.Get("result").Value(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.get_afterDelete",
		method: "subscriber.get",
		params: []interface{}{addedSubscriberId},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, -32000, int(rsp.Get("error.code").Int()), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "subscriber.list_afterDelete",
		method: "subscriber.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Empty(t, rsp.Get("result").Array(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteMqttBridge2",
		method: "bridge.delete",
		params: []interface{}{"mqtt-test2"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)
}

func TestHttpRpc_shell(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	JsonRpcTestCase{
		name:   "addShell_not_exists_cmd",
		method: "shell.add",
		params: []interface{}{"test-shell", `not_exists_cmd`},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("error").Exists())
			require.Equal(t, -32000, int(rsp.Get("error.code").Int()))
			require.Contains(t, `'not_exists_cmd' is not accessible`, rsp.Get("result.error.message").String())
		},
	}.run(t, at)

	var addShellResult func() string
	var shellCommand = "/bin/bash -il"
	if runtime.GOOS == "windows" {
		// Use cmd.exe for better compatibility in Windows environment
		shellCommand = `C:\Windows\System32\cmd.exe /c "echo off && cmd.exe /k"`
	}
	JsonRpcTestCase{
		name:   "addShell",
		method: "shell.add",
		params: []interface{}{"test-shell", shellCommand},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			id := rsp.Get("result").String()
			require.NotEmpty(t, id, rsp.String())
			addShellResult = func() string { return id }
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listShells",
		method: "shell.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, 1, len(rsp.Get("result").Array()), rsp.String())
			require.Equal(t, addShellResult(), rsp.Get("result.0.id").String(), rsp.String())
			require.Equal(t, "test-shell", rsp.Get("result.0.label").String(), rsp.String())
			require.Equal(t, shellCommand, rsp.Get("result.0.command").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "deleteShell",
		method: "shell.delete",
		params: []interface{}{addShellResult()},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listShells",
		method: "shell.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, 0, len(rsp.Get("result").Array()), rsp.String())
		},
	}.run(t, at)
}

type JsonRpcTestCase struct {
	name       string
	method     string
	params     []interface{}
	expect     []string
	expectFunc func(t *testing.T, result gjson.Result)
	expectJSON string
}

func (tc JsonRpcTestCase) run(t *testing.T, accessToken string) {
	t.Helper()
	RunJsonRpcTest(t, accessToken, tc)
}

func RunJsonRpcTest(t *testing.T, accessToken string, tc JsonRpcTestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		// Build JSON-RPC request
		rpcReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  tc.method,
			"params":  tc.params,
		}
		reqBody, err := json.Marshal(rpcReq)
		require.NoError(t, err)

		// Send HTTP POST request
		req, _ := http.NewRequest(
			http.MethodPost,
			httpServerAddress+"/web/api/rpc",
			bytes.NewBuffer(reqBody),
		)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		req.Header.Set("Content-Type", "application/json")
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rsp.StatusCode)

		// Parse JSON-RPC response
		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)
		rsp.Body.Close()

		// Validate JSON-RPC structure
		jsonRsp := gjson.ParseBytes(body)
		require.Equal(t, "2.0", jsonRsp.Get("jsonrpc").String())
		require.Equal(t, int64(1), jsonRsp.Get("id").Int())

		// If validate function is provided, use it to validate the result
		if tc.expectFunc != nil {
			tc.expectFunc(t, jsonRsp)
		}
		// If expected output is provided, validate it
		if len(tc.expect) > 0 {
			require.True(t, jsonRsp.Get("result").Exists())
			output := jsonRsp.Get("result").String()
			outputLines := strings.Split(string(output), "\n")
			for i, outputLine := range outputLines {
				if i >= len(tc.expect) {
					if outputLine != "" || i != len(outputLines)-1 {
						require.Fail(t, "Unexpected extra output", "Line: %s", outputLine)
					}
					continue
				}
				expect := tc.expect[i]
				if strings.HasPrefix(expect, "/r/") {
					// regular expression match
					pattern := expect[3:]
					matched, err := regexp.MatchString(pattern, outputLine)
					require.NoError(t, err, "Invalid regular expression: %s", pattern)
					require.True(t, matched, "Output line does not match pattern. Line: %s, Pattern: %s", outputLine, pattern)
				} else {
					require.Equal(t, expect, outputLine)
				}
			}
			for i, expectLine := range tc.expect[len(outputLines):] {
				require.Fail(t, "Expected line not found in output", "Line[%d]: %s", i+len(outputLines), expectLine)
			}
		}
		// If expected JSON is provided, validate it
		if tc.expectJSON != "" {
			require.JSONEq(t, tc.expectJSON, jsonRsp.Get("result").String())
		}
	})
}

func TestBuildRpcCallParams(t *testing.T) {
	type rpcPayload struct {
		Count int    `json:"count"`
		Name  string `json:"name"`
	}

	handler := func(ctx context.Context, count int, enabled bool, payload rpcPayload, req *rpcPayload) error {
		return nil
	}

	params, err := buildRpcCallParams(handler, []any{
		float64(7),
		true,
		map[string]any{"count": float64(3), "name": "neo"},
		map[string]any{"count": float64(9), "name": "rpc"},
	}, func(paramType reflect.Type) (reflect.Value, bool) {
		if paramType == contextType {
			return reflect.ValueOf(t.Context()), true
		}
		return reflect.Value{}, false
	})
	require.NoError(t, err)
	require.Len(t, params, 5)
	require.Equal(t, 7, params[1].Interface().(int))
	require.True(t, params[2].Interface().(bool))
	require.Equal(t, rpcPayload{Count: 3, Name: "neo"}, params[3].Interface().(rpcPayload))
	require.Equal(t, &rpcPayload{Count: 9, Name: "rpc"}, params[4].Interface().(*rpcPayload))
}

func TestBuildRpcCallParamsRejectsInvalidNumber(t *testing.T) {
	handler := func(count int) error {
		return nil
	}

	_, err := buildRpcCallParams(handler, []any{1.25}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "param 0")
	require.Contains(t, err.Error(), "int")
}

func TestHttpLoggerWithFileWritesAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logPath := filepath.Join(t.TempDir(), "access.log")

	router := gin.New()
	accessLog := logging.NewLogFile("http-util-file", logging.LogFileConf{
		Filename:             logPath,
		Level:                "DEBUG",
		MaxSize:              10,
		MaxBackups:           2,
		MaxAge:               7,
		Compress:             false,
		Append:               true,
		RotateSchedule:       "@midnight",
		Console:              false,
		PrefixWidth:          20,
		EnableSourceLocation: false,
	})
	closeLogOnCleanup(t, accessLog)
	router.Use(logger(accessLog, nil))
	router.GET("/logging/file", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/logging/file?x=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	body, err := osReadFileWithRetry(logPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "/logging/file?x=1")
}

func TestHttpLoggerWithFileConfWritesAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logPath := filepath.Join(t.TempDir(), "access-conf.log")
	fileConf := logging.LogFileConf{
		Filename:             logPath,
		Level:                "DEBUG",
		MaxSize:              1,
		MaxBackups:           1,
		MaxAge:               1,
		Append:               true,
		PrefixWidth:          12,
		EnableSourceLocation: false,
	}

	router := gin.New()
	accessLog := logging.NewLogFile("http-util-file-conf", fileConf)
	closeLogOnCleanup(t, accessLog)
	router.Use(logger(accessLog, nil))
	router.POST("/logging/file-conf", func(c *gin.Context) {
		_, _ = io.WriteString(c.Writer, "created")
	})

	req := httptest.NewRequest(http.MethodPost, "/logging/file-conf", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	body, err := osReadFileWithRetry(logPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "/logging/file-conf")
}

func TestHttpLoggerWithFilterAndFileConfFallsBackWithoutFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	filterCalled := false

	router := gin.New()
	router.Use(HttpLoggerWithFilterAndFileConf("http-util-filter", func(req *http.Request, statusCode int, latency time.Duration) bool {
		filterCalled = true
		return false
	}, logging.LogFileConf{}))
	router.GET("/logging/filter", func(c *gin.Context) {
		c.String(http.StatusNoContent, "")
	})

	req := httptest.NewRequest(http.MethodGet, "/logging/filter", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, filterCalled)
}

func TestHttpLoggerWithFileWrapperWritesAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logPath := filepath.Join(t.TempDir(), "wrapper-access.log")
	origNewLogFile := httpLoggerNewLogFile
	var createdLog logging.Log
	httpLoggerNewLogFile = func(name string, cfg logging.LogFileConf) logging.Log {
		createdLog = logging.NewLogFile(name, cfg)
		return createdLog
	}
	t.Cleanup(func() {
		httpLoggerNewLogFile = origNewLogFile
		if createdLog != nil {
			closer, ok := createdLog.(interface{ Close() error })
			require.True(t, ok)
			require.NoError(t, closer.Close())
		}
	})

	router := gin.New()
	router.Use(HttpLoggerWithFile("http-util-wrapper", logPath))
	router.GET("/logging/wrapper", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/logging/wrapper?from=wrapper", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body, err := osReadFileWithRetry(logPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "/logging/wrapper?from=wrapper")
}

func TestHttpLoggerWithFileConfWrapperWritesAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logPath := filepath.Join(t.TempDir(), "wrapper-access-conf.log")
	origNewLogFile := httpLoggerNewLogFile
	var createdLog logging.Log
	httpLoggerNewLogFile = func(name string, cfg logging.LogFileConf) logging.Log {
		createdLog = logging.NewLogFile(name, cfg)
		return createdLog
	}
	t.Cleanup(func() {
		httpLoggerNewLogFile = origNewLogFile
		if createdLog != nil {
			closer, ok := createdLog.(interface{ Close() error })
			require.True(t, ok)
			require.NoError(t, closer.Close())
		}
	})

	router := gin.New()
	router.Use(HttpLoggerWithFileConf("http-util-wrapper-conf", logging.LogFileConf{
		Filename:             logPath,
		Level:                "DEBUG",
		MaxSize:              1,
		MaxBackups:           1,
		MaxAge:               1,
		Append:               true,
		PrefixWidth:          12,
		EnableSourceLocation: false,
	}))
	router.GET("/logging/wrapper-conf", func(c *gin.Context) {
		c.String(http.StatusAccepted, "accepted")
	})

	req := httptest.NewRequest(http.MethodGet, "/logging/wrapper-conf", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	body, err := osReadFileWithRetry(logPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "/logging/wrapper-conf")
}

func TestWithHttpWebDirSetsWrappedAssets(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("index page"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "app.js"), []byte("console.log('ok')"), 0o644))

	svr := &httpd{}
	WithHttpWebDir(tempDir)(svr)

	require.NotNil(t, svr.uiContentFs)

	assetFile, err := svr.uiContentFs.Open("app.js")
	require.NoError(t, err)
	assetBody, err := io.ReadAll(assetFile)
	require.NoError(t, err)
	require.NoError(t, assetFile.Close())
	require.Equal(t, "console.log('ok')", string(assetBody))

	fallbackFile, err := svr.uiContentFs.Open("missing/route")
	require.NoError(t, err)
	fallbackBody, err := io.ReadAll(fallbackFile)
	require.NoError(t, err)
	require.NoError(t, fallbackFile.Close())
	require.Equal(t, "index page", string(fallbackBody))
}

func osReadFileWithRetry(path string) ([]byte, error) {
	var lastErr error
	for range 10 {
		body, err := os.ReadFile(path)
		if err == nil {
			return body, nil
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	return nil, lastErr
}

func closeLogOnCleanup(t *testing.T, log logging.Log) {
	t.Helper()
	closer, ok := log.(interface{ Close() error })
	if !ok {
		return
	}
	t.Cleanup(func() {
		require.NoError(t, closer.Close())
	})
}
