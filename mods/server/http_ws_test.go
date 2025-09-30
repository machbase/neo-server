package server

import (
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/stretchr/testify/require"
)

func TestWsLLMGetProviders(t *testing.T) {
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

func TestWsLLMMessages(t *testing.T) {
	tests := []struct {
		request eventbus.Question
		expects func(*eventbus.Message) bool
	}{
		{
			request: eventbus.Question{
				Provider: "claude",
				Model:    "claude-sonnet-4-20250514",
				Text:     "Hello, Claude!",
			},
			expects: func(msg *eventbus.Message) bool {
				return true
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
		reqMsg := &eventbus.Message{
			Ver:  "1.0",
			ID:   int64(id + 1),
			Type: eventbus.BodyTypeQuestion,
			Body: &eventbus.BodyUnion{
				OfQuestion: &eventbus.Question{
					Provider: tc.request.Provider,
					Model:    tc.request.Model,
					Text:     tc.request.Text,
				},
			},
		}
		req := &eventbus.Event{Type: eventbus.EVT_MSG, Message: reqMsg}
		err = ws.WriteJSON(req)
		require.NoError(t, err)

		rsp := &eventbus.Event{}
		err = ws.ReadJSON(rsp)
		require.NoError(t, err)
		require.Equal(t, int64(id+1), rsp.Message.ID)
		require.NotNil(t, rsp.Message)
		require.Equal(t, eventbus.BodyTypeStreamMessageStart, rsp.Message.Type)

		err = ws.ReadJSON(rsp)
		require.NoError(t, err)
		require.Equal(t, int64(id+1), rsp.Message.ID)
		require.NotNil(t, rsp.Message)
		require.Equal(t, eventbus.BodyTypeStreamBlockStart, rsp.Message.Type)

		for {
			err = ws.ReadJSON(rsp)
			require.NoError(t, err)
			require.Equal(t, int64(id+1), rsp.Message.ID)
			require.NotNil(t, rsp.Message)
			if rsp.Message.Type == eventbus.BodyTypeStreamBlockStop {
				break
			}
			require.Equal(t, eventbus.BodyTypeStreamBlockDelta, rsp.Message.Type)
			// require.Equal(t, fmt.Sprintf("This is a simulated response from %s model %s to your message: %s\n",
			// 	tc.request.Provider, tc.request.Model, tc.request.Text),
			// 	rsp.Message.Body.OfStreamBlockDelta.Text)
		}

		err = ws.ReadJSON(rsp)
		require.NoError(t, err)
		require.Equal(t, int64(id+1), rsp.Message.ID)
		require.NotNil(t, rsp.Message)
		require.Equal(t, eventbus.BodyTypeStreamMessageStop, rsp.Message.Type)
	}
}
