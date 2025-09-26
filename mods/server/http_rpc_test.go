package server

import (
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/stretchr/testify/require"
)

func TestWsRpc(t *testing.T) {
	tests := []struct {
		method  string
		params  []interface{}
		expects []interface{}
	}{
		{
			method: "llmGetProviders",
			params: nil,
			expects: []interface{}{
				map[string]interface{}{
					"name":     "Claude Sonnet 4",
					"provider": "claude",
					"model":    "claude-sonnet-4-20250514",
				},
				map[string]interface{}{
					"name":     "Ollama qwen3:0.6b",
					"provider": "ollama",
					"model":    "qwen3:0.6b",
				},
			},
		},
	}

	// Ensure we have some LLM providers loaded
	useTestingLLMProviders()

	at, _, err := jwtLogin("sys", "manager")
	require.Nil(t, err)

	// Convert http://127.0.0.1 to ws://127.0.0.1
	u := "ws" + strings.TrimPrefix(httpServerAddress, "http") + "/web/api/console/1234/data?token=" + at
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer ws.Close()

	for id, tc := range tests {
		rpcReq := &eventbus.RPC{
			Ver:    "2.0",
			ID:     int64(id + 1),
			Method: tc.method,
			Params: tc.params,
		}
		req := eventbus.Event{Type: eventbus.EVT_RPC_REQ, Rpc: rpcReq}
		err = ws.WriteJSON(req)
		require.NoError(t, err)

		var rsp map[string]interface{}
		err = ws.ReadJSON(&rsp)
		require.NoError(t, err)
		require.Equal(t, eventbus.EVT_RPC_RSP, rsp["type"])
		require.NotNil(t, rsp["rpc"])

		rpcRsp := rsp["rpc"].(map[string]interface{})
		require.Equal(t, "2.0", rpcRsp["jsonrpc"])
		require.Equal(t, float64(id+1), rpcRsp["id"])

		if errObj, ok := rpcRsp["error"]; ok {
			t.Logf("RPC Error: %v", errObj)
			require.Fail(t, "RPC returned error")
		} else {
			result, ok := rpcRsp["result"]
			require.True(t, ok, "RPC result not found")
			require.Equal(t, tc.expects, result)
		}
	}
}
