package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleLspDiagnostics(t *testing.T) {
	body := `{"language":"tql","uri":"memory://test.tql","text":"MAPVALUE(0, 1)\nCSV()"}`
	rsp := postLspTestRequest(t, "/lsp/diagnostics", body, func(router *gin.Engine, svr *httpd) {
		router.POST("/lsp/diagnostics", svr.handleLspDiagnostics)
	})

	if rsp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	if !result.Success {
		t.Fatalf("expected success response: %+v", result)
	}
	data := result.Data.(map[string]any)
	diagnostics := data["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	diagnostic := diagnostics[0].(map[string]any)
	if diagnostic["code"] != "invalid_source" {
		t.Fatalf("expected invalid_source, got %+v", diagnostic)
	}
}

func TestHandleLspCompletion(t *testing.T) {
	body := `{"language":"tql","uri":"memory://test.tql","text":"","position":{"line":1,"column":1}}`
	rsp := postLspTestRequest(t, "/lsp/completion", body, func(router *gin.Engine, svr *httpd) {
		router.POST("/lsp/completion", svr.handleLspCompletion)
	})

	if rsp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	data := result.Data.(map[string]any)
	items := data["items"].([]any)
	if !containsLspLabel(items, "FAKE") {
		t.Fatalf("expected FAKE completion")
	}
}

func TestHandleLspSignatureHelp(t *testing.T) {
	body := `{"language":"jsh","uri":"memory://test.js","text":"const fs = require('fs');\nfs.readFileSync('/tmp/a', ","position":{"line":2,"column":27}}`
	rsp := postLspTestRequest(t, "/lsp/signature", body, func(router *gin.Engine, svr *httpd) {
		router.POST("/lsp/signature", svr.handleLspSignatureHelp)
	})

	if rsp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	if !result.Success {
		t.Fatalf("expected success response: %+v", result)
	}
	data := result.Data.(map[string]any)
	help := data["signatureHelp"].(map[string]any)
	signatures := help["signatures"].([]any)
	if !containsLspLabelPrefix(signatures, "readFileSync(path: string, options: object)") {
		t.Fatalf("expected readFileSync signature help, got %+v", signatures)
	}
	if help["activeParameter"] != float64(1) {
		t.Fatalf("expected active parameter 1, got %+v", help)
	}
}

func TestHandleLspMetadata(t *testing.T) {
	rsp := getLspTestRequest(t, "/lsp/metadata?language=tql", func(router *gin.Engine, svr *httpd) {
		router.GET("/lsp/metadata", svr.handleLspMetadata)
	})

	if rsp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	if !result.Success {
		t.Fatalf("expected success response: %+v", result)
	}
	data := result.Data.(map[string]any)
	metadata := data["metadata"].(map[string]any)
	if metadata["language"] != "tql" {
		t.Fatalf("expected tql metadata, got %+v", metadata)
	}
	keywords := metadata["keywords"].([]any)
	if !containsLspLabel(keywords, "FAKE") || !containsLspLabel(keywords, "MAPVALUE") || !containsLspLabel(keywords, "CSV") {
		t.Fatalf("expected TQL function keywords, got %+v", keywords)
	}
	symbols := metadata["symbols"].([]any)
	if !containsLspStatementKind(symbols, "FAKE", "source") {
		t.Fatalf("expected FAKE source symbol, got %+v", symbols)
	}
}

func TestHandleLspJshMetadata(t *testing.T) {
	rsp := getLspTestRequest(t, "/lsp/metadata?language=jsh", func(router *gin.Engine, svr *httpd) {
		router.GET("/lsp/metadata", svr.handleLspMetadata)
	})

	if rsp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	if !result.Success {
		t.Fatalf("expected success response: %+v", result)
	}
	data := result.Data.(map[string]any)
	metadata := data["metadata"].(map[string]any)
	if metadata["language"] != "jsh" {
		t.Fatalf("expected jsh metadata, got %+v", metadata)
	}
	modules := metadata["modules"].([]any)
	if containsLspModuleID(modules, "@jsh/fs") {
		t.Fatalf("did not expect @jsh/fs module metadata")
	}
	fsModule := findLspModule(modules, "fs")
	if fsModule == nil {
		t.Fatalf("expected fs module metadata, got %+v", modules)
	}
	exports := fsModule["exports"].([]any)
	if !containsLspLabel(exports, "readFileSync") {
		t.Fatalf("expected readFileSync export metadata, got %+v", exports)
	}
}

func TestHandleLspUnsupportedLanguage(t *testing.T) {
	body := `{"language":"sql","uri":"memory://test.sql","text":"select 1"}`
	rsp := postLspTestRequest(t, "/lsp/diagnostics", body, func(router *gin.Engine, svr *httpd) {
		router.POST("/lsp/diagnostics", svr.handleLspDiagnostics)
	})

	if rsp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	if result.Success {
		t.Fatalf("expected failure response")
	}
	if !strings.Contains(result.Reason, "not implemented yet") {
		t.Fatalf("unexpected reason: %s", result.Reason)
	}
}

func TestHandleLspJshDiagnostics(t *testing.T) {
	body := `{"language":"jsh","uri":"memory://test.js","text":"await run()"}`
	rsp := postLspTestRequest(t, "/lsp/diagnostics", body, func(router *gin.Engine, svr *httpd) {
		router.POST("/lsp/diagnostics", svr.handleLspDiagnostics)
	})

	if rsp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rsp.Code, rsp.Body.String())
	}
	result := decodeLspTestResponse(t, rsp.Body.Bytes())
	if !result.Success {
		t.Fatalf("expected success response: %+v", result)
	}
	data := result.Data.(map[string]any)
	diagnostics := data["diagnostics"].([]any)
	if len(diagnostics) == 0 {
		t.Fatal("expected JSH diagnostics")
	}
	diagnostic := diagnostics[0].(map[string]any)
	if diagnostic["source"] != "jsh" {
		t.Fatalf("expected jsh diagnostic, got %+v", diagnostic)
	}
}

type lspTestResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Data    any    `json:"data"`
}

func postLspTestRequest(t *testing.T, path string, body string, register func(*gin.Engine, *httpd)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	svr := &httpd{}
	register(router, svr)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rsp := httptest.NewRecorder()
	router.ServeHTTP(rsp, req)
	return rsp
}

func getLspTestRequest(t *testing.T, path string, register func(*gin.Engine, *httpd)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	svr := &httpd{}
	register(router, svr)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rsp := httptest.NewRecorder()
	router.ServeHTTP(rsp, req)
	return rsp
}

func decodeLspTestResponse(t *testing.T, body []byte) lspTestResponse {
	t.Helper()
	var result lspTestResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return result
}

func containsLspLabel(items []any, label string) bool {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok && obj["label"] == label {
			return true
		}
	}
	return false
}

func containsLspLabelPrefix(items []any, prefix string) bool {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		label, labelOK := obj["label"].(string)
		if ok && labelOK && strings.HasPrefix(label, prefix) {
			return true
		}
	}
	return false
}

func containsLspStatementKind(items []any, label string, statementKind string) bool {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok && obj["label"] == label && obj["statementKind"] == statementKind {
			return true
		}
	}
	return false
}

func containsLspModuleID(items []any, id string) bool {
	return findLspModule(items, id) != nil
}

func findLspModule(items []any, id string) map[string]any {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok && obj["id"] == id {
			return obj
		}
	}
	return nil
}
