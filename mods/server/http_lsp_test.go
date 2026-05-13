package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHandleRpcLspDiagnostics(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "tql diagnostics",
		method: "lsp.diagnostics",
		params: []any{map[string]any{
			"language": "tql",
			"uri":      "memory://test.tql",
			"text":     "MAPVALUE(0, 1)\nCSV()",
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			diagnostics := rsp.Get("result.diagnostics").Array()
			require.Len(t, diagnostics, 1, rsp.String())
			require.Equal(t, "invalid_source", diagnostics[0].Get("code").String(), rsp.String())
		},
	})
}

func TestHandleRpcLspCompletion(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "tql completion",
		method: "lsp.completion",
		params: []any{map[string]any{
			"language": "tql",
			"uri":      "memory://test.tql",
			"text":     "",
			"position": map[string]any{"line": 1, "column": 1},
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, containsLspLabelResult(rsp.Get("result.items"), "FAKE"), rsp.String())
		},
	})
}

func TestHandleRpcLspHover(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "tql hover",
		method: "lsp.hover",
		params: []any{map[string]any{
			"language": "tql",
			"uri":      "memory://test.tql",
			"text":     "FAKE()",
			"position": map[string]any{"line": 1, "column": 2},
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Contains(t, rsp.Get("result.hover.contents").String(), "FAKE", rsp.String())
		},
	})
}

func TestHandleRpcLspSignatureHelp(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "jsh signature help",
		method: "lsp.signature",
		params: []any{map[string]any{
			"language": "jsh",
			"uri":      "memory://test.js",
			"text":     "const fs = require('fs');\nfs.readFileSync('/tmp/a', ",
			"position": map[string]any{"line": 2, "column": 27},
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, containsLspLabelPrefixResult(rsp.Get("result.signatureHelp.signatures"), "readFileSync(path: string, options: object)"), rsp.String())
			require.Equal(t, int64(1), rsp.Get("result.signatureHelp.activeParameter").Int(), rsp.String())
		},
	})
}

func TestHandleRpcLspMetadata(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "tql metadata",
		method: "lsp.metadata",
		params: []any{map[string]any{"language": "tql"}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, "tql", rsp.Get("result.metadata.language").String(), rsp.String())
			keywords := rsp.Get("result.metadata.keywords")
			require.True(t, containsLspLabelResult(keywords, "FAKE"), rsp.String())
			require.True(t, containsLspLabelResult(keywords, "MAPVALUE"), rsp.String())
			require.True(t, containsLspLabelResult(keywords, "CSV"), rsp.String())
			require.True(t, containsLspStatementKindResult(rsp.Get("result.metadata.symbols"), "FAKE", "source"), rsp.String())
		},
	})
}

func TestHandleRpcLspJshMetadata(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "jsh metadata",
		method: "lsp.metadata",
		params: []any{map[string]any{"language": "jsh"}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, "jsh", rsp.Get("result.metadata.language").String(), rsp.String())
			modules := rsp.Get("result.metadata.modules")
			require.False(t, containsLspModuleIDResult(modules, "@jsh/fs"), rsp.String())
			fsModule, ok := findLspModuleResult(modules, "fs")
			require.True(t, ok, rsp.String())
			require.True(t, containsLspLabelResult(fsModule.Get("exports"), "readFileSync"), rsp.String())
		},
	})
}

func TestHandleRpcLspUnsupportedLanguage(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "unsupported diagnostics language",
		method: "lsp.diagnostics",
		params: []any{map[string]any{
			"language": "sql",
			"uri":      "memory://test.sql",
			"text":     "select 1",
		}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, int64(-32602), rsp.Get("error.code").Int(), rsp.String())
			require.Contains(t, rsp.Get("error.message").String(), "not implemented yet", rsp.String())
		},
	})
}

func TestHandleRpcLspMetadataUnsupportedLanguage(t *testing.T) {
	runLspJsonRpcTest(t, lspJsonRpcTestCase{
		name:   "unsupported metadata language",
		method: "lsp.metadata",
		params: []any{map[string]any{"language": "sql"}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, int64(-32602), rsp.Get("error.code").Int(), rsp.String())
			require.Contains(t, rsp.Get("error.message").String(), "not implemented yet", rsp.String())
		},
	})
}

func TestLspLanguageServiceUnsupportedDefault(t *testing.T) {
	_, err := lspLanguageService("unknown")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported language")
}

func TestLspMetadataUnsupportedLanguages(t *testing.T) {
	_, err := lspMetadata("sql")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented yet")

	_, err = lspMetadata("unknown")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported language")
}

type lspJsonRpcTestCase struct {
	name       string
	method     string
	params     []any
	expectFunc func(t *testing.T, result gjson.Result)
}

func runLspJsonRpcTest(t *testing.T, tc lspJsonRpcTestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		rsp := postLspRpcTestRequest(t, tc.method, tc.params)
		require.Equal(t, http.StatusOK, rsp.Code, rsp.Body.String())

		jsonRsp := gjson.ParseBytes(rsp.Body.Bytes())
		require.Equal(t, "2.0", jsonRsp.Get("jsonrpc").String(), jsonRsp.String())
		require.Equal(t, int64(1), jsonRsp.Get("id").Int(), jsonRsp.String())
		if tc.expectFunc != nil {
			tc.expectFunc(t, jsonRsp)
		}
	})
}

func postLspRpcTestRequest(t *testing.T, method string, params []any) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	ctl := &service.Controller{}
	registerLspJsonRpcHandlers(ctl)
	svr := &httpd{rpcController: ctl}
	router.POST("/rpc", svr.handleHttpRpc)

	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	reqBody, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rsp := httptest.NewRecorder()
	router.ServeHTTP(rsp, req)
	return rsp
}

func containsLspLabelResult(items gjson.Result, label string) bool {
	for _, item := range items.Array() {
		if item.Get("label").String() == label {
			return true
		}
	}
	return false
}

func containsLspLabelPrefixResult(items gjson.Result, prefix string) bool {
	for _, item := range items.Array() {
		if strings.HasPrefix(item.Get("label").String(), prefix) {
			return true
		}
	}
	return false
}

func containsLspStatementKindResult(items gjson.Result, label string, statementKind string) bool {
	for _, item := range items.Array() {
		if item.Get("label").String() == label && item.Get("statementKind").String() == statementKind {
			return true
		}
	}
	return false
}

func containsLspModuleIDResult(items gjson.Result, id string) bool {
	_, ok := findLspModuleResult(items, id)
	return ok
}

func findLspModuleResult(items gjson.Result, id string) (gjson.Result, bool) {
	for _, item := range items.Array() {
		if item.Get("id").String() == id {
			return item, true
		}
	}
	return gjson.Result{}, false
}
