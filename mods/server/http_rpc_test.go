package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHttpRpc(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	var originalSessionLimit map[string]int
	generatedKeyID := fmt.Sprintf("rpc-test-key-%d", time.Now().UnixNano())
	sshKeyMaterial := fmt.Sprintf("AAAAC3NzaC1lZDI1NTE5AAAAIRPCTestKey%d", time.Now().UnixNano())
	sshKeyComment := fmt.Sprintf("rpc-test-comment-%d", time.Now().UnixNano())
	var addedSshKeyFingerprint string
	bridgeTableName := fmt.Sprintf("rpc_bridge_%d", time.Now().UnixNano())
	insertedBridgeMemo := fmt.Sprintf("rpc-row-%d", time.Now().UnixNano())
	insertedBridgeCreatedOn := "2023-09-09T00:00:00Z"
	var bridgeQueryHandle string

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
			method: "server.info.get",
			params: []interface{}{},
			expectFunc: func(t *testing.T, rsp gjson.Result) {
				require.Equal(t, runtime.GOOS, rsp.Get("result.runtime.OS").String(), rsp.String())
			},
		},
		{
			name:       "getServicePorts",
			method:     "service.port.list",
			params:     []interface{}{"mach"},
			expectJSON: fmt.Sprintf(`[{"Service":"mach", "Address":"%s"}]`, machServerAddress),
		},
		{
			name:   "getServerCertificate",
			method: "server.certificate.get",
			params: []interface{}{},
			expectFunc: func(t *testing.T, rsp gjson.Result) {
				certificate := rsp.Get("result").String()
				require.Contains(t, certificate, "BEGIN CERTIFICATE", rsp.String())
				require.Contains(t, certificate, "END CERTIFICATE", rsp.String())
			},
		},
		{
			name:   "getSessionLimit",
			method: "session.limit.get",
			params: []interface{}{},
			expectFunc: func(t *testing.T, rsp gjson.Result) {
				result := rsp.Get("result")
				require.True(t, result.Get("MaxPoolSize").Exists(), rsp.String())
				require.True(t, result.Get("MaxOpenConn").Exists(), rsp.String())
				require.True(t, result.Get("RemainedOpenConn").Exists(), rsp.String())
				require.True(t, result.Get("MaxOpenQuery").Exists(), rsp.String())
				require.True(t, result.Get("RemainedOpenQuery").Exists(), rsp.String())
				require.GreaterOrEqual(t, int(result.Get("MaxPoolSize").Int()), -1, rsp.String())
				require.GreaterOrEqual(t, int(result.Get("MaxOpenConn").Int()), -1, rsp.String())
				require.GreaterOrEqual(t, int(result.Get("RemainedOpenConn").Int()), -1, rsp.String())
				require.GreaterOrEqual(t, int(result.Get("MaxOpenQuery").Int()), -1, rsp.String())
				require.GreaterOrEqual(t, int(result.Get("RemainedOpenQuery").Int()), -1, rsp.String())
				originalSessionLimit = map[string]int{
					"MaxPoolSize":       int(result.Get("MaxPoolSize").Int()),
					"MaxOpenConn":       int(result.Get("MaxOpenConn").Int()),
					"RemainedOpenConn":  int(result.Get("RemainedOpenConn").Int()),
					"MaxOpenQuery":      int(result.Get("MaxOpenQuery").Int()),
					"RemainedOpenQuery": int(result.Get("RemainedOpenQuery").Int()),
				}
			},
		},
		{
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
		},
	}
	for _, tc := range tests {
		RunJsonRpcTest(t, at, tc)
	}
	require.NotNil(t, originalSessionLimit)

	JsonRpcTestCase{
		name:   "listKeys_beforeGenerate",
		method: "key.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			for _, item := range rsp.Get("result").Array() {
				require.NotEqual(t, generatedKeyID, item.Get("id").String(), rsp.String())
			}
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "generateKey",
		method: "key.generate",
		params: []interface{}{generatedKeyID},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			result := rsp.Get("result")
			require.Equal(t, generatedKeyID, result.Get("id").String(), rsp.String())
			require.Contains(t, result.Get("certificate").String(), "BEGIN CERTIFICATE", rsp.String())
			privateKey := result.Get("key").String()
			require.Contains(t, privateKey, "BEGIN ", rsp.String())
			require.Contains(t, privateKey, "PRIVATE KEY", rsp.String())
			require.NotEmpty(t, result.Get("token").String(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "listKeys_afterGenerate",
		method: "key.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			found := false
			for _, item := range rsp.Get("result").Array() {
				if item.Get("id").String() == generatedKeyID {
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
		params: []interface{}{generatedKeyID},
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
				require.NotEqual(t, generatedKeyID, item.Get("id").String(), rsp.String())
			}
		},
	}.run(t, at)

	JsonRpcTestCase{
		name:   "listSshKeys_beforeAdd",
		method: "sshkey.list",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			for _, item := range rsp.Get("result").Array() {
				require.NotEqual(t, sshKeyMaterial, item.Get("Key").String(), rsp.String())
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
				if item.Get("Key").String() == sshKeyMaterial {
					found = true
					addedSshKeyFingerprint = item.Get("Fingerprint").String()
					require.Equal(t, "ed25519", item.Get("KeyType").String(), rsp.String())
					require.Equal(t, sshKeyComment, item.Get("Comment").String(), rsp.String())
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
				require.NotEqual(t, sshKeyMaterial, item.Get("Key").String(), rsp.String())
			}
		},
	}.run(t, at)

	newMaxPoolSize := originalSessionLimit["MaxPoolSize"] + 1
	if originalSessionLimit["MaxPoolSize"] < 0 {
		newMaxPoolSize = 1
	}
	JsonRpcTestCase{
		name:   "setSessionLimit",
		method: "session.limit.set",
		params: []interface{}{map[string]any{"MaxPoolSize": newMaxPoolSize}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getSessionLimit_afterSet",
		method: "session.limit.get",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, int64(newMaxPoolSize), rsp.Get("result.MaxPoolSize").Int(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "restoreSessionLimit",
		method: "session.limit.set",
		params: []interface{}{map[string]any{"MaxPoolSize": originalSessionLimit["MaxPoolSize"]}},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.True(t, rsp.Get("result").Exists(), rsp.String())
		},
	}.run(t, at)
	JsonRpcTestCase{
		name:   "getSessionLimit_afterRestore",
		method: "session.limit.get",
		params: []interface{}{},
		expectFunc: func(t *testing.T, rsp gjson.Result) {
			require.Equal(t, int64(originalSessionLimit["MaxPoolSize"]), rsp.Get("result.MaxPoolSize").Int(), rsp.String())
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

	tests = []JsonRpcTestCase{
		{
			name:   "markdownRender-light",
			method: "markdown.render",
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
			method: "markdown.render",
			params: []interface{}{"## Dark Mode Test\n\n- Item 1\n- Item 2", true},
			expectFunc: func(t *testing.T, result gjson.Result) {
				html := result.Get("result").String()
				require.Contains(t, html, "<h2")
				require.Contains(t, html, "Dark Mode Test")
				require.Contains(t, html, "<li>Item 1</li>")
				require.Contains(t, html, "<li>Item 2</li>")
			},
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
