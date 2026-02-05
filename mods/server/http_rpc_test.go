package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHttpRpc(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	tests := []struct {
		name     string
		method   string
		params   []interface{}
		validate func(t *testing.T, result gjson.Result)
	}{
		{
			name:   "markdownRender-light",
			method: "markdownRender",
			params: []interface{}{"# Hello World\n\nThis is a **test**.", false},
			validate: func(t *testing.T, result gjson.Result) {
				require.True(t, result.Exists())
				html := result.String()
				require.Contains(t, html, "<h1")
				require.Contains(t, html, "Hello World")
				require.Contains(t, html, "<strong>test</strong>")
			},
		},
		{
			name:   "markdownRender-dark",
			method: "markdownRender",
			params: []interface{}{"## Dark Mode Test\n\n- Item 1\n- Item 2", true},
			validate: func(t *testing.T, result gjson.Result) {
				require.True(t, result.Exists())
				html := result.String()
				require.Contains(t, html, "<h2")
				require.Contains(t, html, "Dark Mode Test")
				require.Contains(t, html, "<li>Item 1</li>")
				require.Contains(t, html, "<li>Item 2</li>")
			},
		},
		{
			name:   "method-not-found",
			method: "nonExistentMethod",
			params: []interface{}{},
			validate: func(t *testing.T, result gjson.Result) {
				require.False(t, result.Exists())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
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

			// Check for error or result
			if tc.method == "nonExistentMethod" {
				// Expect error
				require.True(t, jsonRsp.Get("error").Exists())
				require.Equal(t, int64(-32601), jsonRsp.Get("error.code").Int())
				require.Equal(t, "Method not found", jsonRsp.Get("error.message").String())
			} else {
				// Expect result
				require.True(t, jsonRsp.Get("result").Exists())
				require.False(t, jsonRsp.Get("error").Exists())
				tc.validate(t, jsonRsp.Get("result"))
			}
		})
	}
}
