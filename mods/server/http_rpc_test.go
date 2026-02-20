package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHttpRpc(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	tests := []JsonRpcTestCase{
		{
			name:   "method-not-found",
			method: "nonExistentMethod",
			params: []interface{}{},
			expectFunc: func(t *testing.T, jsonRsp gjson.Result) {
				require.True(t, jsonRsp.Get("error").Exists())
				require.Equal(t, int64(-32601), jsonRsp.Get("error.code").Int())
				require.Equal(t, "Method not found", jsonRsp.Get("error.message").String())
			},
		},
		{
			name:   "getServerInfo",
			method: "getServerInfo",
			params: []interface{}{},
			expectFunc: func(t *testing.T, rsp gjson.Result) {
				require.Equal(t, runtime.GOOS, rsp.Get("result.runtime.OS").String(), rsp.String())
			},
		},
		{
			name:       "getServicePorts",
			method:     "getServicePorts",
			params:     []interface{}{"mach"},
			expectJSON: fmt.Sprintf(`[{"Service":"mach", "Address":"%s"}]`, machServerAddress),
		},
	}
	for _, tc := range tests {
		RunJsonRpcTest(t, at, tc)
	}

	JsonRpcTestCase{
		name:   "addShell_not_exists_cmd",
		method: "addShell",
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
		method: "addShell",
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
		method: "listShells",
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
		method: "deleteShell",
		params: []interface{}{addShellResult()},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listShells",
		method: "listShells",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, 0, len(rsp.Get("result").Array()), rsp.String())
		},
	}.run(t, at)

	JsonRpcTestCase{
		name:   "addBridge",
		method: "addBridge",
		params: []interface{}{"br-test", "sqlite", "file::memory:?cache=shared"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listBridges",
		method: "listBridges",
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
		name:   "deleteBridge",
		method: "deleteBridge",
		params: []interface{}{"br-test"},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("id").Exists(), rsp.String())
			require.Equal(t, "2.0", rsp.Get("jsonrpc").String(), rsp.String())
			require.Empty(t, rsp.Get("result").String(), rsp.String())
		},
	}.run(t, at)

	tests = []JsonRpcTestCase{
		{
			name:   "markdownRender-light",
			method: "markdownRender",
			params: []interface{}{"# Hello World\n\nThis is a **test**.", false},
			expectFunc: func(t *testing.T, result gjson.Result) {
				html := result.Get("result").String()
				require.Contains(t, html, "<h1")
				require.Contains(t, html, "Hello World")
				require.Contains(t, html, "<strong>test</strong>")
			},
		},
		{
			name:   "markdownRender-dark",
			method: "markdownRender",
			params: []interface{}{"## Dark Mode Test\n\n- Item 1\n- Item 2", true},
			expectFunc: func(t *testing.T, result gjson.Result) {
				html := result.Get("result").String()
				require.Contains(t, html, "<h2")
				require.Contains(t, html, "Dark Mode Test")
				require.Contains(t, html, "<li>Item 1</li>")
				require.Contains(t, html, "<li>Item 2</li>")
			},
		},
	}
	for _, tc := range tests {
		RunJsonRpcTest(t, at, tc)
	}
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
