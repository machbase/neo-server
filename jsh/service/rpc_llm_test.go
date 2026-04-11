package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	shelllib "github.com/machbase/neo-server/v8/jsh/lib/shell"
	"github.com/stretchr/testify/require"
)

type testRpcNotifier struct {
	mu       sync.Mutex
	events   []map[string]any
	notified chan struct{}
	err      error
	failAt   int
	count    int
}

func newTestRpcNotifier() *testRpcNotifier {
	return &testRpcNotifier{notified: make(chan struct{}, 32)}
}

func (n *testRpcNotifier) NotifyJsonRpc(_ string, payload map[string]any) error {
	n.count++
	if n.failAt > 0 && n.count == n.failAt {
		return errors.New("forced notify failure")
	}
	if n.err != nil {
		return n.err
	}
	n.mu.Lock()
	n.events = append(n.events, payload)
	n.mu.Unlock()
	select {
	case n.notified <- struct{}{}:
	default:
	}
	return nil
}

func (n *testRpcNotifier) eventNames() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	ret := make([]string, 0, len(n.events))
	for _, evt := range n.events {
		params, _ := evt["params"].(map[string]any)
		name, _ := params["event"].(string)
		ret = append(ret, name)
	}
	return ret
}

func newLLMTestController(t *testing.T) *Controller {
	t.Helper()
	useMockLLMStream(t)
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	require.NoError(t, os.MkdirAll(servicesDir, 0o755))

	ctl, err := NewController(&ControllerConfig{
		ConfigDir: "/work/services",
		Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
	})
	require.NoError(t, err)
	return ctl
}

func useMockLLMStream(t *testing.T) {
	t.Helper()
	prev := llmStreamFunc
	llmStreamFunc = func(ctx context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input := ""
		if len(req.Messages) > 0 {
			input = strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
		}
		output := "Echo: " + input
		if input == "" {
			output = "Echo: (empty)"
		}
		parts := strings.Fields(output)
		if len(parts) == 0 {
			parts = []string{output}
		}
		for i, p := range parts {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			token := p
			if i < len(parts)-1 {
				token += " "
			}
			if onToken != nil {
				onToken(token)
			}
		}
		inTokens := len(strings.Fields(input))
		outTokens := len(strings.Fields(output))
		return &shelllib.LLMStreamResponse{
			Content:      output,
			InputTokens:  inTokens,
			OutputTokens: outTokens,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})
}

func onlySessionID(t *testing.T, ctl *Controller) string {
	t.Helper()
	ctl.llmMu.RLock()
	defer ctl.llmMu.RUnlock()
	require.Len(t, ctl.llmSessions, 1)
	for id := range ctl.llmSessions {
		return id
	}
	return ""
}

func callWithContext(ctl *Controller, method string, params []any, ctx context.Context) (any, *JsonRpcError) {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	return ctl.CallJsonRpc(method, params, func(paramType reflect.Type) (reflect.Value, bool) {
		if paramType == ctxType {
			return reflect.ValueOf(ctx), true
		}
		return reflect.Value{}, false
	})
}

func TestLLMRPCSessionLifecycle(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)

	sessionID := onlySessionID(t, ctl)
	require.NotEmpty(t, sessionID)

	result, rpcErr := ctl.CallJsonRpc("llm.session.get", []any{map[string]any{"sessionId": sessionID}}, nil)
	require.Nil(t, rpcErr)
	got, ok := result.(llmSessionGetResponse)
	require.True(t, ok)
	require.Equal(t, "active", got.SessionState)
	require.Equal(t, llmDefaultProvider, got.Provider)
	require.Equal(t, llmDefaultModelForClaude, got.Model)

	_, rpcErr = ctl.CallJsonRpc("llm.provider.set", []any{map[string]any{"sessionId": sessionID, "payload": map[string]any{"provider": "openai"}}}, nil)
	require.Nil(t, rpcErr)

	result, rpcErr = ctl.CallJsonRpc("llm.model.set", []any{map[string]any{"sessionId": sessionID, "payload": map[string]any{"model": "gpt-4.1-mini"}}}, nil)
	require.Nil(t, rpcErr)
	pm, ok := result.(llmProviderModelResponse)
	require.True(t, ok)
	require.Equal(t, "openai", pm.Provider)
	require.Equal(t, "gpt-4.1-mini", pm.Model)

	result, rpcErr = ctl.CallJsonRpc("llm.session.reset", []any{map[string]any{"sessionId": sessionID, "payload": map[string]any{"clearHistory": true}}}, nil)
	require.Nil(t, rpcErr)
	resetRsp, ok := result.(llmSessionResetResponse)
	require.True(t, ok)
	require.True(t, resetRsp.Reset)
	require.NotEqual(t, sessionID, resetRsp.SessionID)
}

func TestLLMRPCTurnAskValidationAndNotifications(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	_, rpcErr = ctl.CallJsonRpc("llm.turn.ask", []any{map[string]any{"sessionId": sessionID, "turnId": "", "payload": map[string]any{"text": "hello"}}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonRPCInvalidParam, rpcErr.Code)

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-1")

	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-1",
		"traceId":   "trace-1",
		"payload":   map[string]any{"text": "hello neo"},
	}}, ctx)
	require.Nil(t, rpcErr)
	require.Eventually(t, func() bool {
		names := notifier.eventNames()
		for _, n := range names {
			if n == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	names := notifier.eventNames()
	require.Contains(t, names, "turn.started")
	require.Contains(t, names, "turn.block.started")
	require.Contains(t, names, "turn.delta")
	require.Contains(t, names, "turn.block.completed")
	require.Contains(t, names, "turn.completed")
}

func TestLLMRPCTurnIdempotentResponse(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	startCh := make(chan struct{}, 1)
	releaseCh := make(chan struct{})
	callCount := 0
	prev := llmStreamFunc
	llmStreamFunc = func(ctx context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		callCount++
		if callCount == 1 {
			startCh <- struct{}{}
			select {
			case <-releaseCh:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if onToken != nil {
			onToken("ok")
		}
		return &shelllib.LLMStreamResponse{Content: "ok", InputTokens: 1, OutputTokens: 1, Provider: req.Provider, Model: req.Model}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), newTestRpcNotifier()), "ws-session-idem")
	result, rpcErr := callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-idem",
		"payload":   map[string]any{"text": "first"},
	}}, ctx)
	require.Nil(t, rpcErr)
	first, ok := result.(llmTurnAskResponse)
	require.True(t, ok)
	require.Equal(t, "streaming", first.Status)

	select {
	case <-startCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first stream did not start")
	}

	result, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-idem",
		"payload":   map[string]any{"text": "second"},
	}}, ctx)
	require.Nil(t, rpcErr)
	dup, ok := result.(llmTurnAskResponse)
	require.True(t, ok)
	require.Equal(t, "streaming", dup.Status)
	require.Equal(t, 1, callCount)

	close(releaseCh)
	require.Eventually(t, func() bool {
		ctl.llmMu.RLock()
		defer ctl.llmMu.RUnlock()
		return ctl.llmSessions[sessionID].TurnStatus["turn-idem"] == "completed"
	}, 1*time.Second, 20*time.Millisecond)

	result, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-idem",
		"payload":   map[string]any{"text": "third"},
	}}, ctx)
	require.Nil(t, rpcErr)
	after, ok := result.(llmTurnAskResponse)
	require.True(t, ok)
	require.Equal(t, "completed", after.Status)
	require.Equal(t, 1, callCount)
}

func TestLLMRPCTurnCancelAndSessionErrors(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	_, rpcErr = ctl.CallJsonRpc("llm.turn.cancel", []any{map[string]any{"sessionId": "", "turnId": "t1"}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonRPCInvalidParam, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.turn.cancel", []any{map[string]any{"sessionId": sessionID, "turnId": ""}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonRPCInvalidParam, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.turn.cancel", []any{map[string]any{"sessionId": "missing", "turnId": "t1"}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionNotFound, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.turn.cancel", []any{map[string]any{"sessionId": sessionID, "turnId": "t1"}}, nil)
	require.Nil(t, rpcErr)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].LastActivity = time.Now().Add(-llmSessionIdleTimeout - time.Minute)
	ctl.llmMu.Unlock()

	_, rpcErr = ctl.CallJsonRpc("llm.turn.cancel", []any{map[string]any{"sessionId": sessionID, "turnId": "t2"}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionExpired, rpcErr.Code)
}

func TestLLMRPCSessionOpenResumeAndProviderDefaultModel(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	result, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": true, "sessionHint": sessionID}}}, nil)
	require.Nil(t, rpcErr)
	openRsp, ok := result.(llmSessionOpenResponse)
	require.True(t, ok)
	require.False(t, openRsp.Created)
	require.Equal(t, sessionID, openRsp.SessionID)
	require.Equal(t, "active", openRsp.SessionState)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].LastActivity = time.Now().Add(-llmSessionIdleTimeout - time.Minute)
	ctl.llmMu.Unlock()

	result, rpcErr = ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": true, "sessionHint": sessionID}}}, nil)
	require.Nil(t, rpcErr)
	openRsp, ok = result.(llmSessionOpenResponse)
	require.True(t, ok)
	require.False(t, openRsp.Created)
	require.Equal(t, sessionID, openRsp.SessionID)
	require.Equal(t, "restored", openRsp.SessionState)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].LastActivity = time.Now().Add(-llmSessionIdleTimeout - llmSessionReconnectGrace - time.Minute)
	ctl.llmMu.Unlock()

	result, rpcErr = ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": true, "sessionHint": sessionID}}}, nil)
	require.Nil(t, rpcErr)
	openRsp, ok = result.(llmSessionOpenResponse)
	require.True(t, ok)
	require.True(t, openRsp.Created)
	require.NotEqual(t, sessionID, openRsp.SessionID)

	newSessionID := onlySessionID(t, ctl)
	ctl.llmMu.Lock()
	ctl.llmSessions[newSessionID].Model = ""
	ctl.llmMu.Unlock()

	result, rpcErr = ctl.CallJsonRpc("llm.provider.set", []any{map[string]any{
		"sessionId": newSessionID,
		"payload":   map[string]any{"provider": "claude"},
	}}, nil)
	require.Nil(t, rpcErr)
	pm, ok := result.(llmProviderModelResponse)
	require.True(t, ok)
	require.Equal(t, llmDefaultModelForClaude, pm.Model)
}

func TestLLMNotifyHelpers(t *testing.T) {
	require.False(t, emitJsonRpcNotification(context.TODO(), "llm.event", map[string]any{"a": 1}))
	require.False(t, emitJsonRpcNotification(context.Background(), "", map[string]any{"a": 1}))

	ctx := WithJsonRpcNotificationWriter(context.Background(), nil)
	require.False(t, emitJsonRpcNotification(ctx, "llm.event", map[string]any{"a": 1}))

	bad := newTestRpcNotifier()
	bad.err = errors.New("write failed")
	badCtx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), bad), "s1")
	require.False(t, emitJsonRpcNotification(badCtx, "llm.event", map[string]any{"a": 1}))

	good := newTestRpcNotifier()
	goodCtx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), good), "s2")
	require.True(t, emitJsonRpcNotification(goodCtx, "llm.event", map[string]any{"k": "v"}))

	good.mu.Lock()
	require.Len(t, good.events, 1)
	require.Equal(t, "2.0", good.events[0]["jsonrpc"])
	require.Equal(t, "llm.event", good.events[0]["method"])
	good.mu.Unlock()
}

func TestLLMRPCSessionGetAndModelSetErrors(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.get", []any{map[string]any{"sessionId": ""}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonRPCInvalidParam, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.session.get", []any{map[string]any{"sessionId": "missing"}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionNotFound, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].LastActivity = time.Now().Add(-llmSessionIdleTimeout - time.Minute)
	ctl.llmMu.Unlock()
	_, rpcErr = ctl.CallJsonRpc("llm.session.get", []any{map[string]any{"sessionId": sessionID}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionExpired, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.model.set", []any{map[string]any{"sessionId": "", "payload": map[string]any{"model": "m1"}}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonRPCInvalidParam, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.model.set", []any{map[string]any{"sessionId": "missing", "payload": map[string]any{"model": "m1"}}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionNotFound, rpcErr.Code)
}

func TestLLMRPCTurnAskErrorsAndStreamFailurePath(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": "missing",
		"turnId":    "turn-x",
		"payload":   map[string]any{"text": "hello"},
	}}, context.Background())
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionNotFound, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].LastActivity = time.Now().Add(-llmSessionIdleTimeout - time.Minute)
	ctl.llmMu.Unlock()
	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-expired",
		"payload":   map[string]any{"text": "hello"},
	}}, context.Background())
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionExpired, rpcErr.Code)

	now := time.Now()
	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID] = &llmSession{
		ID:           sessionID,
		Provider:     llmDefaultProvider,
		Model:        llmDefaultModelForClaude,
		CreatedAt:    now,
		LastActivity: now,
		TurnStatus:   map[string]string{},
	}
	ctl.llmMu.Unlock()

	failNotifier := newTestRpcNotifier()
	failNotifier.failAt = 2
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), failNotifier), "ws-session-fail")
	ctl.streamLLMSkeleton(ctx, sessionID, "turn-fail", "trace-fail", shelllib.LLMStreamRequest{
		Provider: "claude",
		Model:    llmDefaultModelForClaude,
		Messages: []shelllib.LLMChatMessage{{Role: "user", Content: " "}},
	}, nil)

	ctl.llmMu.RLock()
	status := ctl.llmSessions[sessionID].TurnStatus["turn-fail"]
	ctl.llmMu.RUnlock()
	require.Equal(t, "failed", status)
}

func TestLLMRPCSessionResetBoundaryErrors(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.reset", []any{map[string]any{
		"sessionId": "missing",
		"payload":   map[string]any{"clearHistory": true},
	}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionNotFound, rpcErr.Code)

	_, rpcErr = ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].LastActivity = time.Now().Add(-llmSessionIdleTimeout - time.Minute)
	ctl.llmMu.Unlock()

	_, rpcErr = ctl.CallJsonRpc("llm.session.reset", []any{map[string]any{
		"sessionId": sessionID,
		"payload":   map[string]any{"clearHistory": true},
	}}, nil)
	require.NotNil(t, rpcErr)
	require.Equal(t, llmCodeSessionExpired, rpcErr.Code)
}

func TestStreamLLMSkeletonFailBranches(t *testing.T) {
	for _, failAt := range []int{1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("failAt-%d", failAt), func(t *testing.T) {
			ctl := newLLMTestController(t)

			_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
			require.Nil(t, rpcErr)
			sessionID := onlySessionID(t, ctl)

			ctl.llmMu.Lock()
			ctl.llmSessions[sessionID].TurnStatus["turn-fail"] = "in-flight"
			ctl.llmMu.Unlock()

			notifier := newTestRpcNotifier()
			notifier.failAt = failAt
			ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-fail")

			ctl.streamLLMSkeleton(ctx, sessionID, "turn-fail", "trace-fail", shelllib.LLMStreamRequest{
				Provider: "claude",
				Model:    llmDefaultModelForClaude,
				Messages: []shelllib.LLMChatMessage{{Role: "user", Content: "hello neo"}},
			}, nil)

			ctl.llmMu.RLock()
			status := ctl.llmSessions[sessionID].TurnStatus["turn-fail"]
			ctl.llmMu.RUnlock()
			require.Equal(t, "failed", status)
		})
	}
}

func TestLLMRPCTurnAskHistoryAndSystemPrompt(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].History = []shelllib.LLMChatMessage{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
	}
	ctl.llmMu.Unlock()

	var capturedReq shelllib.LLMStreamRequest
	prev := llmStreamFunc
	llmStreamFunc = func(_ context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		capturedReq = req
		if onToken != nil {
			onToken("second answer")
		}
		return &shelllib.LLMStreamResponse{
			Content:      "second answer",
			InputTokens:  3,
			OutputTokens: 2,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-history")

	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-history-1",
		"traceId":   "trace-history-1",
		"payload": map[string]any{
			"text":         "second question",
			"systemPrompt": "You are a strict assistant.",
		},
	}}, ctx)
	require.Nil(t, rpcErr)

	require.Eventually(t, func() bool {
		names := notifier.eventNames()
		for _, n := range names {
			if n == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	require.Equal(t, "You are a strict assistant.", capturedReq.SystemPrompt)
	require.Len(t, capturedReq.Messages, 3)
	require.Equal(t, "first question", capturedReq.Messages[0].Content)
	require.Equal(t, "first answer", capturedReq.Messages[1].Content)
	require.Equal(t, "second question", strings.TrimSpace(capturedReq.Messages[2].Content))

	ctl.llmMu.RLock()
	history := ctl.llmSessions[sessionID].History
	ctl.llmMu.RUnlock()
	require.Len(t, history, 4)
	require.Equal(t, "second question", strings.TrimSpace(history[2].Content))
	require.Equal(t, "second answer", history[3].Content)
}

func TestLLMRPCTurnAskClientContextAppendedToSystemPrompt(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	var capturedReq shelllib.LLMStreamRequest
	prev := llmStreamFunc
	llmStreamFunc = func(_ context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		capturedReq = req
		if onToken != nil {
			onToken("ok")
		}
		return &shelllib.LLMStreamResponse{
			Content:      "ok",
			InputTokens:  1,
			OutputTokens: 1,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-client-context")

	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-client-context-1",
		"traceId":   "trace-client-context-1",
		"payload": map[string]any{
			"text": "draw the result",
			"clientContext": map[string]any{
				"surface":       "web-remote",
				"transport":     "websocket",
				"renderTargets": []any{"markdown", "agent-render/v1", "vizspec/v1"},
				"filePolicy":    "explicit-only",
			},
		},
	}}, ctx)
	require.Nil(t, rpcErr)

	require.Eventually(t, func() bool {
		for _, n := range notifier.eventNames() {
			if n == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	require.Contains(t, capturedReq.SystemPrompt, "client.surface: web-remote")
	require.Contains(t, capturedReq.SystemPrompt, "client.transport: websocket")
	require.Contains(t, capturedReq.SystemPrompt, "client.renderTargets: markdown, agent-render/v1, vizspec/v1")
	require.Contains(t, capturedReq.SystemPrompt, "Only save files when the user explicitly asks to save or export a file.")
}

func TestLLMTurnFailedPayloadNormalization(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		payload, status := llmTurnFailedPayload(errors.New("provider timeout"))
		require.Equal(t, "failed", status)
		require.Equal(t, llmCodeBackendTimeout, payload["code"])
		require.Equal(t, true, payload["retryable"])
		require.Equal(t, "timeout", payload["reason"])
	})

	t.Run("cancelled", func(t *testing.T) {
		payload, status := llmTurnFailedPayload(context.Canceled)
		require.Equal(t, "cancelled", status)
		require.Equal(t, llmCodeTurnCancelled, payload["code"])
		require.Equal(t, false, payload["retryable"])
		require.Equal(t, "user_cancelled", payload["reason"])
	})

	t.Run("provider unavailable", func(t *testing.T) {
		payload, status := llmTurnFailedPayload(errors.New("429 rate limit"))
		require.Equal(t, "failed", status)
		require.Equal(t, llmCodeProviderUnavailable, payload["code"])
		require.Equal(t, true, payload["retryable"])
		require.Equal(t, "provider_unavailable", payload["reason"])
	})

	t.Run("internal", func(t *testing.T) {
		payload, status := llmTurnFailedPayload(errors.New("unexpected panic"))
		require.Equal(t, "failed", status)
		require.Equal(t, jsonRPCInternal, payload["code"])
		require.Equal(t, false, payload["retryable"])
		require.Equal(t, "internal", payload["reason"])
	})
}

func TestLLMRPCTurnExecPayloadIncludesEditStatsAndRetryCount(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	prev := llmStreamFunc
	llmStreamFunc = func(ctx context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		last := ""
		if len(req.Messages) > 0 {
			last = strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
		}
		text := "analysis done"
		if strings.HasPrefix(last, "Code execution results:") {
			text = "final analysis"
		} else {
			text = "```jsh-run\nconsole.log('ok');\n```"
		}
		if onToken != nil {
			onToken(text)
		}
		return &shelllib.LLMStreamResponse{
			Content:      text,
			InputTokens:  3,
			OutputTokens: 2,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-exec-payload")

	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-exec-payload-1",
		"traceId":   "trace-exec-payload-1",
		"payload": map[string]any{
			"text": "please run code",
		},
	}}, ctx)
	require.Nil(t, rpcErr)

	require.Eventually(t, func() bool {
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		for _, evt := range notifier.events {
			params, _ := evt["params"].(map[string]any)
			name, _ := params["event"].(string)
			if name == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	var execStartedPayload map[string]any
	var execCompletedPayload map[string]any
	var turnCompletedPayload map[string]any

	notifier.mu.Lock()
	for _, evt := range notifier.events {
		params, _ := evt["params"].(map[string]any)
		name, _ := params["event"].(string)
		payload, _ := params["payload"].(map[string]any)
		switch name {
		case "turn.exec.started":
			execStartedPayload = payload
		case "turn.exec.completed":
			execCompletedPayload = payload
		case "turn.completed":
			turnCompletedPayload = payload
		}
	}
	notifier.mu.Unlock()

	asInt := func(v any) int {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			t.Fatalf("unexpected numeric type %T", v)
			return 0
		}
	}

	require.NotNil(t, execStartedPayload)
	require.Equal(t, "run", execStartedPayload["opType"])
	require.Equal(t, 0, asInt(execStartedPayload["retryCount"]))

	startedStats, ok := execStartedPayload["editStats"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, asInt(startedStats["totalOps"]))

	require.NotNil(t, execCompletedPayload)
	require.Equal(t, "run", execCompletedPayload["opType"])
	completedStats, ok := execCompletedPayload["editStats"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, asInt(completedStats["runOps"]))

	require.NotNil(t, turnCompletedPayload)
	require.Equal(t, 0, asInt(turnCompletedPayload["retryCount"]))
	turnStats, ok := turnCompletedPayload["editStats"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, asInt(turnStats["totalOps"]))
}

func TestLLMRPCTurnExecPayloadRetryCountIncrementsOnFollowUpExecution(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	prev := llmStreamFunc
	llmStreamFunc = func(ctx context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		last := ""
		if len(req.Messages) > 0 {
			last = strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
		}

		text := "done"
		switch {
		case strings.Contains(last, "first-pass"):
			text = "final analysis"
		case strings.HasPrefix(last, "Code execution results:"):
			text = "```jsh-run\nconsole.log('first-pass');\n```"
		default:
			text = "```jsh-run\nconsole.log('initial-pass');\n```"
		}

		if onToken != nil {
			onToken(text)
		}
		return &shelllib.LLMStreamResponse{
			Content:      text,
			InputTokens:  5,
			OutputTokens: 3,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-exec-retry")

	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-exec-retry-1",
		"traceId":   "trace-exec-retry-1",
		"payload": map[string]any{
			"text": "please run code with retry",
		},
	}}, ctx)
	require.Nil(t, rpcErr)

	require.Eventually(t, func() bool {
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		for _, evt := range notifier.events {
			params, _ := evt["params"].(map[string]any)
			name, _ := params["event"].(string)
			if name == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	asInt := func(v any) int {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			t.Fatalf("unexpected numeric type %T", v)
			return 0
		}
	}

	seenRetryOneStarted := false
	seenRetryOneCompleted := false
	var turnCompletedPayload map[string]any

	notifier.mu.Lock()
	for _, evt := range notifier.events {
		params, _ := evt["params"].(map[string]any)
		name, _ := params["event"].(string)
		payload, _ := params["payload"].(map[string]any)
		switch name {
		case "turn.exec.started":
			if asInt(payload["retryCount"]) == 1 {
				seenRetryOneStarted = true
			}
		case "turn.exec.completed":
			if asInt(payload["retryCount"]) == 1 {
				seenRetryOneCompleted = true
			}
		case "turn.completed":
			turnCompletedPayload = payload
		}
	}
	notifier.mu.Unlock()

	require.True(t, seenRetryOneStarted)
	require.True(t, seenRetryOneCompleted)
	require.NotNil(t, turnCompletedPayload)
	require.Equal(t, 1, asInt(turnCompletedPayload["retryCount"]))

	turnStats, ok := turnCompletedPayload["editStats"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 2, asInt(turnStats["totalOps"]))
	require.Equal(t, 2, asInt(turnStats["runOps"]))
}

func TestLLMRPCTurnExecPayloadIncludesMutationSummaryForCreate(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	prev := llmStreamFunc
	llmStreamFunc = func(ctx context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		last := ""
		if len(req.Messages) > 0 {
			last = strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
		}
		text := "final analysis"
		if !strings.HasPrefix(last, "Code execution results:") {
			text = "```jsh-run\n(function(){ return agent.fs.write('/work/phase6-create.txt', 'ok'); }());\n```"
		}
		if onToken != nil {
			onToken(text)
		}
		return &shelllib.LLMStreamResponse{
			Content:      text,
			InputTokens:  4,
			OutputTokens: 3,
			Provider:     req.Provider,
			Model:        req.Model,
		}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-exec-create")

	_, rpcErr = callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-exec-create-1",
		"traceId":   "trace-exec-create-1",
		"payload": map[string]any{
			"text":         "please create a file",
			"execReadOnly": false,
		},
	}}, ctx)
	require.Nil(t, rpcErr)

	require.Eventually(t, func() bool {
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		for _, evt := range notifier.events {
			params, _ := evt["params"].(map[string]any)
			name, _ := params["event"].(string)
			if name == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	asInt := func(v any) int {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			t.Fatalf("unexpected numeric type %T", v)
			return 0
		}
	}

	var execCompletedPayload map[string]any
	var turnCompletedPayload map[string]any
	notifier.mu.Lock()
	for _, evt := range notifier.events {
		params, _ := evt["params"].(map[string]any)
		name, _ := params["event"].(string)
		payload, _ := params["payload"].(map[string]any)
		if name == "turn.exec.completed" {
			execCompletedPayload = payload
		}
		if name == "turn.completed" {
			turnCompletedPayload = payload
		}
	}
	notifier.mu.Unlock()

	require.NotNil(t, execCompletedPayload)
	require.Equal(t, "create", execCompletedPayload["opType"])

	stats, ok := execCompletedPayload["editStats"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, asInt(stats["createOps"]))

	ms, ok := execCompletedPayload["mutationSummary"].(string)
	require.True(t, ok)
	require.Contains(t, ms, "File mutations:")
	require.Contains(t, ms, "/work/phase6-create.txt")

	var first map[string]any
	switch mm := execCompletedPayload["mutations"].(type) {
	case []any:
		require.NotEmpty(t, mm)
		m0, ok := mm[0].(map[string]any)
		require.True(t, ok)
		first = m0
	case []map[string]any:
		require.NotEmpty(t, mm)
		first = mm[0]
	default:
		t.Fatalf("unexpected mutations type %T", execCompletedPayload["mutations"])
	}
	require.Equal(t, "create", first["opType"])
	require.Equal(t, "/work/phase6-create.txt", first["path"])

	require.NotNil(t, turnCompletedPayload)
	var blocks []map[string]any
	switch bb := turnCompletedPayload["blocks"].(type) {
	case []any:
		for _, one := range bb {
			if m, ok := one.(map[string]any); ok {
				blocks = append(blocks, m)
			}
		}
	case []map[string]any:
		blocks = bb
	default:
		t.Fatalf("unexpected blocks type %T", turnCompletedPayload["blocks"])
	}
	foundMutationText := false
	for _, bm := range blocks {
		typeText, _ := bm["type"].(string)
		if typeText != "text" {
			continue
		}
		text, _ := bm["text"].(string)
		if strings.Contains(text, "File mutations:") && strings.Contains(text, "/work/phase6-create.txt") {
			foundMutationText = true
			break
		}
	}
	require.True(t, foundMutationText, "turn.completed blocks should include explicit mutation summary text")
}

func TestLLMRPCTurnAskSlashSaveHandledServerSide(t *testing.T) {
	ctl := newLLMTestController(t)

	_, rpcErr := ctl.CallJsonRpc("llm.session.open", []any{map[string]any{"payload": map[string]any{"resume": false}}}, nil)
	require.Nil(t, rpcErr)
	sessionID := onlySessionID(t, ctl)

	ctl.llmMu.Lock()
	ctl.llmSessions[sessionID].History = []shelllib.LLMChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	ctl.llmMu.Unlock()

	streamCalled := false
	prev := llmStreamFunc
	llmStreamFunc = func(ctx context.Context, req shelllib.LLMStreamRequest, onToken func(string)) (*shelllib.LLMStreamResponse, error) {
		streamCalled = true
		if onToken != nil {
			onToken("unexpected")
		}
		return &shelllib.LLMStreamResponse{Content: "unexpected", Provider: req.Provider, Model: req.Model}, nil
	}
	t.Cleanup(func() {
		llmStreamFunc = prev
	})

	notifier := newTestRpcNotifier()
	ctx := WithJsonRpcSession(WithJsonRpcNotificationWriter(context.Background(), notifier), "ws-session-slash-save")

	result, rpcErr := callWithContext(ctl, "llm.turn.ask", []any{map[string]any{
		"sessionId": sessionID,
		"turnId":    "turn-slash-save-1",
		"traceId":   "trace-slash-save-1",
		"payload": map[string]any{
			"text": "/save file.md",
		},
	}}, ctx)
	require.Nil(t, rpcErr)
	askRsp, ok := result.(llmTurnAskResponse)
	require.True(t, ok)
	require.True(t, askRsp.Accepted)
	require.Equal(t, "streaming", askRsp.Status)

	require.Eventually(t, func() bool {
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		for _, evt := range notifier.events {
			params, _ := evt["params"].(map[string]any)
			name, _ := params["event"].(string)
			if name == "turn.completed" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	require.False(t, streamCalled, "slash save must not call llm stream")

	content, err := ctl.fs.ReadFile("/work/file.md")
	require.NoError(t, err)
	require.Contains(t, string(content), "# AI Session")
	require.Contains(t, string(content), "## User")
	require.Contains(t, string(content), "/save file.md")

	var completedPayload map[string]any
	notifier.mu.Lock()
	for _, evt := range notifier.events {
		params, _ := evt["params"].(map[string]any)
		name, _ := params["event"].(string)
		if name == "turn.completed" {
			completedPayload, _ = params["payload"].(map[string]any)
		}
	}
	notifier.mu.Unlock()

	require.NotNil(t, completedPayload)
	var blocks []map[string]any
	switch bb := completedPayload["blocks"].(type) {
	case []any:
		for _, one := range bb {
			if m, ok := one.(map[string]any); ok {
				blocks = append(blocks, m)
			}
		}
	case []map[string]any:
		blocks = bb
	default:
		t.Fatalf("unexpected blocks type %T", completedPayload["blocks"])
	}

	foundSaveText := false
	foundMutationText := false
	for _, b := range blocks {
		typeText, _ := b["type"].(string)
		if typeText != "text" {
			continue
		}
		text, _ := b["text"].(string)
		if strings.Contains(text, "Saved AI session to /work/file.md") {
			foundSaveText = true
		}
		if strings.Contains(text, "File mutations:") && strings.Contains(text, "/work/file.md") {
			foundMutationText = true
		}
	}
	require.True(t, foundSaveText)
	require.True(t, foundMutationText)
}
