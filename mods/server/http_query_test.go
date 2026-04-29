package server

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

func TestHandleTqlQueryExec(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	t.Run("token query param authorizes request and delegates to tql handler", func(t *testing.T) {
		query := url.Values{}
		query.Set(TQL_SCRIPT_PARAM, "FAKE(linspace(0,1,2))\nCSV()")
		query.Set(TQL_TOKEN_PARAM, at)

		req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tql-exec?"+query.Encode(), nil)
		require.NoError(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.Equal(t, "text/csv; charset=utf-8", rsp.Header.Get("Content-Type"))
		require.Equal(t, strings.Join([]string{"0", "1", "\n"}, "\n"), string(body))
	})

	t.Run("invalid token aborts before tql execution", func(t *testing.T) {
		query := url.Values{}
		query.Set(TQL_SCRIPT_PARAM, "FAKE(linspace(0,1,2))\nCSV()")
		query.Set(TQL_TOKEN_PARAM, "not-a-valid-token")

		req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tql-exec?"+query.Encode(), nil)
		require.NoError(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, rsp.StatusCode)
		require.Contains(t, rsp.Header.Get("Content-Type"), "application/json")
		require.Contains(t, string(body), `"success":false`)
		require.Contains(t, string(body), "reason")
		require.NotContains(t, string(body), "columns")
	})
}

func TestHandleTqlQuery(t *testing.T) {
	svr := newTestHTTPServer(t)

	t.Run("get request without script returns bad request", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/tql", nil)

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":false`)
		require.Contains(t, writer.Body.String(), "script not found")
	})

	t.Run("post body script executes successfully", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/tql", []byte("FAKE(linspace(0,1,2))\nCSV()"))

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "text/csv; charset=utf-8", writer.Header().Get("Content-Type"))
		require.Equal(t, strings.Join([]string{"0", "1", "\n"}, "\n"), writer.Body.String())
	})

	t.Run("post query script accepts payload", func(t *testing.T) {
		script := "CSV(payload())\nCSV()"
		target := "/web/api/tql?$=" + url.QueryEscape(script)
		ctx, writer := newTestHTTPContext(http.MethodPost, target, []byte("a,1\nb,2\n"))
		ctx.Request.Header.Set("Content-Type", "text/csv")

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "text/csv; charset=utf-8", writer.Header().Get("Content-Type"))
		require.Equal(t, strings.Join([]string{"a,1", "b,2", "\n"}, "\n"), writer.Body.String())
	})

	t.Run("unsupported method returns method not allowed", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPut, "/web/api/tql?$="+url.QueryEscape("FAKE(linspace(0,1,2))\nCSV()"), nil)

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusMethodNotAllowed, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":false`)
		require.Contains(t, writer.Body.String(), "unsupported method")
	})

	t.Run("compile error returns bad request", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/tql?$="+url.QueryEscape("FAKE("), nil)

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":false`)
		require.Contains(t, writer.Body.String(), "reason")
	})
}

func TestHandleTqlFile(t *testing.T) {
	oldDefault := ssfs.Default()
	ssfs.SetDefault(httpServer.serverFs)
	t.Cleanup(func() {
		ssfs.SetDefault(oldDefault)
	})

	writeServerFile := func(t *testing.T, path string, content []byte) {
		t.Helper()
		require.NoError(t, httpServer.serverFs.Set(path, content))
		t.Cleanup(func() {
			_ = httpServer.serverFs.Remove(path)
		})
	}

	doRequest := func(t *testing.T, method, target string, body []byte, headers map[string]string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, httpServerAddress+target, bytes.NewReader(body))
		require.NoError(t, err)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return rsp
	}

	t.Run("non tql public path redirects", func(t *testing.T) {
		noRedirectClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/db/tql/public/redirect-policy.txt", nil)
		require.NoError(t, err)

		rsp, err := noRedirectClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusFound, rsp.StatusCode)
		require.Equal(t, "/public/redirect-policy.txt", rsp.Header.Get("Location"))
	})

	t.Run("non tql static file returns content", func(t *testing.T) {
		const staticPath = "/query_test_static.txt"
		writeServerFile(t, staticPath, []byte("hello from static file"))

		rsp := doRequest(t, http.MethodGet, "/db/tql"+staticPath, nil, nil)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.Equal(t, "text/plain", rsp.Header.Get("Content-Type"))
		require.Equal(t, "hello from static file", string(body))
	})

	t.Run("missing tql file returns not found", func(t *testing.T) {
		rsp := doRequest(t, http.MethodGet, "/db/tql/query_test_missing.tql", nil, nil)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rsp.StatusCode)
		require.Contains(t, string(body), `"success":false`)
		require.Contains(t, string(body), "not found")
	})

	t.Run("compile failure returns internal server error", func(t *testing.T) {
		const brokenPath = "/query_test_broken.tql"
		writeServerFile(t, brokenPath, []byte("FAKE("))

		rsp := doRequest(t, http.MethodGet, "/db/tql"+brokenPath, nil, nil)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusInternalServerError, rsp.StatusCode)
		require.Contains(t, string(body), `"success":false`)
		require.Contains(t, string(body), "reason")
	})

	t.Run("json output header changes response format", func(t *testing.T) {
		const scriptPath = "/query_test_output.tql"
		writeServerFile(t, scriptPath, []byte("FAKE(linspace(0,360,5))\nMAPVALUE(1, sin((value(0)/180)*PI))\nCHART()"))

		rsp := doRequest(t, http.MethodGet, "/db/tql"+scriptPath, nil, map[string]string{TqlHeaderChartOutput: "json"})
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.NotEmpty(t, rsp.Header.Get("Content-Type"))
		require.Equal(t, "echarts", rsp.Header.Get(TqlHeaderChartType))
		require.Contains(t, string(body), `"chartID"`)
		require.Contains(t, string(body), `"jsAssets"`)
		require.Contains(t, string(body), `"jsCodeAssets"`)
	})
}
