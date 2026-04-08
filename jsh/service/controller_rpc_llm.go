package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	shelllib "github.com/machbase/neo-server/v8/jsh/lib/shell"
)

func init() {
	shelllib.SetAgentExecRuntimeBootstrap(lib.Enable)
}

const (
	llmCodeSessionNotFound     = -32001
	llmCodeSessionExpired      = -32002
	llmCodeProviderUnavailable = -32004
	llmCodeBackendTimeout      = -32005
	llmCodeTurnCancelled       = -32006
	llmDefaultProvider         = "claude"
	llmDefaultModelForClaude   = "claude-haiku-4-5-20251001"
	llmSessionIdleTimeout      = 30 * time.Minute
	llmSessionReconnectGrace   = 5 * time.Minute
	llmMaxHistoryMessages      = 20
	llmExecMaxRows             = 1000
	llmExecTimeoutMs           = 30000
	llmExecMaxOutputBytes      = 65536
	llmExecFollowupMaxRounds   = 3
)

var llmJshRunBlockRe = regexp.MustCompile("(?s)```jsh-run\\n(.*?)```")

type llmSession struct {
	ID            string
	Provider      string
	Model         string
	CreatedAt     time.Time
	LastActivity  time.Time
	LastTurnID    string
	NextSeq       int64
	TurnStatus    map[string]string
	TurnCancels   map[string]context.CancelFunc
	TurnResponses map[string]llmTurnAskResponse
	History       []shelllib.LLMChatMessage
}

var llmStreamFunc = shelllib.StreamLLM

// SetLLMStreamFuncForTest replaces the internal stream implementation and
// returns a restore function. It is intended for integration tests that need
// deterministic LLM event sequencing.
func SetLLMStreamFuncForTest(fn func(context.Context, shelllib.LLMStreamRequest, func(string)) (*shelllib.LLMStreamResponse, error)) func() {
	prev := llmStreamFunc
	if fn != nil {
		llmStreamFunc = fn
	}
	return func() {
		llmStreamFunc = prev
	}
}

type llmSessionOpenPayload struct {
	Resume      bool   `json:"resume"`
	SessionHint string `json:"sessionHint,omitempty"`
}

type llmSessionOpenRequest struct {
	SessionID string                `json:"sessionId,omitempty"`
	TurnID    string                `json:"turnId,omitempty"`
	TraceID   string                `json:"traceId,omitempty"`
	Payload   llmSessionOpenPayload `json:"payload"`
}

type llmSessionOpenResponse struct {
	Created      bool   `json:"created"`
	SessionID    string `json:"sessionId"`
	SessionState string `json:"sessionState"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
}

type llmSessionGetRequest struct {
	SessionID string `json:"sessionId"`
	TurnID    string `json:"turnId,omitempty"`
	TraceID   string `json:"traceId,omitempty"`
}

type llmSessionGetResponse struct {
	SessionState string `json:"sessionState"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	LastTurnID   string `json:"lastTurnId,omitempty"`
}

type llmSessionResetPayload struct {
	ClearHistory bool `json:"clearHistory"`
}

type llmSessionResetRequest struct {
	SessionID string                 `json:"sessionId"`
	TurnID    string                 `json:"turnId,omitempty"`
	TraceID   string                 `json:"traceId,omitempty"`
	Payload   llmSessionResetPayload `json:"payload"`
}

type llmSessionResetResponse struct {
	Reset     bool   `json:"reset"`
	SessionID string `json:"sessionId"`
}

type llmTurnAskPayload struct {
	Text               string            `json:"text"`
	Provider           string            `json:"provider,omitempty"`
	Model              string            `json:"model,omitempty"`
	SystemPrompt       string            `json:"systemPrompt,omitempty"`
	ClientContext      *llmClientContext `json:"clientContext,omitempty"`
	MaxTokens          int               `json:"maxTokens,omitempty"`
	Temperature        float64           `json:"temperature,omitempty"`
	AutoExecute        *bool             `json:"autoExecute,omitempty"`
	ExecReadOnly       *bool             `json:"execReadOnly,omitempty"`
	ExecMaxRows        int               `json:"execMaxRows,omitempty"`
	ExecTimeoutMs      int64             `json:"execTimeoutMs,omitempty"`
	ExecMaxOutputBytes int               `json:"execMaxOutputBytes,omitempty"`
	ExecMaxRounds      int               `json:"execMaxRounds,omitempty"`
}

type llmClientContext struct {
	Surface       string   `json:"surface,omitempty"`
	Transport     string   `json:"transport,omitempty"`
	RenderTargets []string `json:"renderTargets,omitempty"`
	FilePolicy    string   `json:"filePolicy,omitempty"`
	BinaryInline  *bool    `json:"binaryInline,omitempty"`
}

func (ctx *llmClientContext) isEmpty() bool {
	if ctx == nil {
		return true
	}
	return strings.TrimSpace(ctx.Surface) == "" &&
		strings.TrimSpace(ctx.Transport) == "" &&
		len(ctx.RenderTargets) == 0 &&
		strings.TrimSpace(ctx.FilePolicy) == "" &&
		ctx.BinaryInline == nil
}

func (ctx *llmClientContext) toMap() map[string]any {
	if ctx == nil || ctx.isEmpty() {
		return nil
	}
	out := map[string]any{}
	if s := strings.TrimSpace(ctx.Surface); s != "" {
		out["surface"] = s
	}
	if s := strings.TrimSpace(ctx.Transport); s != "" {
		out["transport"] = s
	}
	if len(ctx.RenderTargets) > 0 {
		targets := make([]string, 0, len(ctx.RenderTargets))
		for _, one := range ctx.RenderTargets {
			trimmed := strings.TrimSpace(one)
			if trimmed == "" {
				continue
			}
			targets = append(targets, trimmed)
		}
		if len(targets) > 0 {
			out["renderTargets"] = targets
		}
	}
	if s := strings.TrimSpace(ctx.FilePolicy); s != "" {
		out["filePolicy"] = s
	}
	if ctx.BinaryInline != nil {
		out["binaryInline"] = *ctx.BinaryInline
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildLLMClientContextPromptExtra(ctx *llmClientContext) string {
	if ctx == nil || ctx.isEmpty() {
		return ""
	}
	lines := []string{"Current client context for generated answers and jsh code:"}
	if s := strings.TrimSpace(ctx.Surface); s != "" {
		lines = append(lines, "- client.surface: "+s)
	}
	if s := strings.TrimSpace(ctx.Transport); s != "" {
		lines = append(lines, "- client.transport: "+s)
	}
	if len(ctx.RenderTargets) > 0 {
		targets := make([]string, 0, len(ctx.RenderTargets))
		for _, one := range ctx.RenderTargets {
			trimmed := strings.TrimSpace(one)
			if trimmed == "" {
				continue
			}
			targets = append(targets, trimmed)
		}
		if len(targets) > 0 {
			lines = append(lines, "- client.renderTargets: "+strings.Join(targets, ", "))
		}
	}
	if s := strings.TrimSpace(ctx.FilePolicy); s != "" {
		lines = append(lines, "- client.filePolicy: "+s)
	}
	if ctx.BinaryInline != nil {
		lines = append(lines, fmt.Sprintf("- client.binaryInline: %t", *ctx.BinaryInline))
	}
	lines = append(lines,
		"",
		"Choose outputs that match the client context.",
		"When the client supports renderable outputs such as agent-render/v1 or vizspec/v1, prefer returning renderable objects over writing files.",
		"Only save files when the user explicitly asks to save or export a file.",
	)
	return strings.Join(lines, "\n")
}

type llmExecPolicy struct {
	AutoExecute    bool
	ReadOnly       bool
	MaxRows        int
	TimeoutMs      int64
	MaxOutputBytes int
	MaxRounds      int
}

func defaultLLMExecPolicy() llmExecPolicy {
	return llmExecPolicy{
		AutoExecute:    true,
		ReadOnly:       true,
		MaxRows:        llmExecMaxRows,
		TimeoutMs:      llmExecTimeoutMs,
		MaxOutputBytes: llmExecMaxOutputBytes,
		MaxRounds:      llmExecFollowupMaxRounds,
	}
}

func llmExecPolicyFromPayload(p llmTurnAskPayload) llmExecPolicy {
	policy := defaultLLMExecPolicy()
	if p.AutoExecute != nil {
		policy.AutoExecute = *p.AutoExecute
	}
	if p.ExecReadOnly != nil {
		policy.ReadOnly = *p.ExecReadOnly
	}
	if p.ExecMaxRows > 0 {
		policy.MaxRows = p.ExecMaxRows
	}
	if p.ExecTimeoutMs > 0 {
		policy.TimeoutMs = p.ExecTimeoutMs
	}
	if p.ExecMaxOutputBytes > 0 {
		policy.MaxOutputBytes = p.ExecMaxOutputBytes
	}
	if p.ExecMaxRounds > 0 {
		policy.MaxRounds = p.ExecMaxRounds
	}
	return policy
}

type llmTurnAskRequest struct {
	SessionID string            `json:"sessionId"`
	TurnID    string            `json:"turnId"`
	TraceID   string            `json:"traceId,omitempty"`
	Payload   llmTurnAskPayload `json:"payload"`
}

type llmTurnAskResponse struct {
	Accepted bool   `json:"accepted"`
	Status   string `json:"status"`
}

type llmTurnCancelPayload struct {
	Reason string `json:"reason,omitempty"`
}

type llmTurnCancelRequest struct {
	SessionID string               `json:"sessionId"`
	TurnID    string               `json:"turnId"`
	TraceID   string               `json:"traceId,omitempty"`
	Payload   llmTurnCancelPayload `json:"payload"`
}

type llmTurnCancelResponse struct {
	Cancelled bool `json:"cancelled"`
}

type llmProviderSetPayload struct {
	Provider string `json:"provider"`
}

type llmProviderSetRequest struct {
	SessionID string                `json:"sessionId"`
	TurnID    string                `json:"turnId,omitempty"`
	TraceID   string                `json:"traceId,omitempty"`
	Payload   llmProviderSetPayload `json:"payload"`
}

type llmModelSetPayload struct {
	Model string `json:"model"`
}

type llmModelSetRequest struct {
	SessionID string             `json:"sessionId"`
	TurnID    string             `json:"turnId,omitempty"`
	TraceID   string             `json:"traceId,omitempty"`
	Payload   llmModelSetPayload `json:"payload"`
}

type llmProviderModelResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (ctl *Controller) rpcLLMSessionOpen(req llmSessionOpenRequest) (llmSessionOpenResponse, error) {
	now := time.Now()
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	ctl.ensureLLMSessionMapLocked()

	hint := req.Payload.SessionHint
	if hint == "" {
		hint = req.SessionID
	}
	if req.Payload.Resume && hint != "" {
		if sess, ok := ctl.llmSessions[hint]; ok {
			if !llmSessionExpired(sess, now) {
				sess.LastActivity = now
				return llmSessionOpenResponse{
					Created:      false,
					SessionID:    sess.ID,
					SessionState: "active",
					Provider:     sess.Provider,
					Model:        sess.Model,
				}, nil
			}
			if llmSessionWithinReconnectGrace(sess, now) {
				sess.LastActivity = now
				return llmSessionOpenResponse{
					Created:      false,
					SessionID:    sess.ID,
					SessionState: "restored",
					Provider:     sess.Provider,
					Model:        sess.Model,
				}, nil
			}
			delete(ctl.llmSessions, hint)
		}
	}

	sessionID := newLLMSessionID()
	ctl.llmSessions[sessionID] = &llmSession{
		ID:            sessionID,
		Provider:      llmDefaultProvider,
		Model:         llmDefaultModelForClaude,
		CreatedAt:     now,
		LastActivity:  now,
		TurnStatus:    map[string]string{},
		TurnCancels:   map[string]context.CancelFunc{},
		TurnResponses: map[string]llmTurnAskResponse{},
		History:       []shelllib.LLMChatMessage{},
	}
	return llmSessionOpenResponse{
		Created:      true,
		SessionID:    sessionID,
		SessionState: "active",
		Provider:     llmDefaultProvider,
		Model:        llmDefaultModelForClaude,
	}, nil
}

func (ctl *Controller) rpcLLMSessionGet(req llmSessionGetRequest) (llmSessionGetResponse, error) {
	now := time.Now()
	if req.SessionID == "" {
		return llmSessionGetResponse{}, invalidParamsError(fmt.Errorf("sessionId is required"))
	}

	ctl.llmMu.RLock()
	sess, ok := ctl.llmSessions[req.SessionID]
	ctl.llmMu.RUnlock()
	if !ok {
		return llmSessionGetResponse{}, &controllerRPCError{Code: llmCodeSessionNotFound, Message: "session not found"}
	}
	if llmSessionExpired(sess, now) {
		ctl.llmMu.Lock()
		delete(ctl.llmSessions, req.SessionID)
		ctl.llmMu.Unlock()
		return llmSessionGetResponse{}, &controllerRPCError{Code: llmCodeSessionExpired, Message: "session expired"}
	}

	return llmSessionGetResponse{
		SessionState: "active",
		Provider:     sess.Provider,
		Model:        sess.Model,
		LastTurnID:   sess.LastTurnID,
	}, nil
}

func (ctl *Controller) rpcLLMSessionReset(req llmSessionResetRequest) (llmSessionResetResponse, error) {
	if req.SessionID == "" {
		return llmSessionResetResponse{}, invalidParamsError(fmt.Errorf("sessionId is required"))
	}

	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[req.SessionID]
	if !ok {
		return llmSessionResetResponse{}, &controllerRPCError{Code: llmCodeSessionNotFound, Message: "session not found"}
	}
	if llmSessionExpired(sess, time.Now()) {
		delete(ctl.llmSessions, req.SessionID)
		return llmSessionResetResponse{}, &controllerRPCError{Code: llmCodeSessionExpired, Message: "session expired"}
	}

	delete(ctl.llmSessions, req.SessionID)
	newID := newLLMSessionID()
	now := time.Now()
	ctl.llmSessions[newID] = &llmSession{
		ID:            newID,
		Provider:      llmDefaultProvider,
		Model:         llmDefaultModelForClaude,
		CreatedAt:     now,
		LastActivity:  now,
		TurnStatus:    map[string]string{},
		TurnCancels:   map[string]context.CancelFunc{},
		TurnResponses: map[string]llmTurnAskResponse{},
		History:       []shelllib.LLMChatMessage{},
	}
	return llmSessionResetResponse{Reset: true, SessionID: newID}, nil
}

func (ctl *Controller) rpcLLMTurnAsk(ctx context.Context, req llmTurnAskRequest) (llmTurnAskResponse, error) {
	if req.SessionID == "" {
		return llmTurnAskResponse{}, invalidParamsError(fmt.Errorf("sessionId is required"))
	}
	if req.TurnID == "" {
		return llmTurnAskResponse{}, invalidParamsError(fmt.Errorf("turnId is required"))
	}
	if req.Payload.Text == "" {
		return llmTurnAskResponse{}, invalidParamsError(fmt.Errorf("payload.text is required"))
	}

	now := time.Now()
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[req.SessionID]
	if !ok {
		return llmTurnAskResponse{}, &controllerRPCError{Code: llmCodeSessionNotFound, Message: "session not found"}
	}
	if llmSessionExpired(sess, now) {
		delete(ctl.llmSessions, req.SessionID)
		return llmTurnAskResponse{}, &controllerRPCError{Code: llmCodeSessionExpired, Message: "session expired"}
	}
	if sess.TurnResponses != nil {
		if prev, exists := sess.TurnResponses[req.TurnID]; exists {
			sess.LastActivity = now
			return prev, nil
		}
	}

	if req.Payload.Provider != "" {
		sess.Provider = req.Payload.Provider
	}
	if req.Payload.Model != "" {
		sess.Model = req.Payload.Model
	}
	if sess.TurnStatus == nil {
		sess.TurnStatus = map[string]string{}
	}
	if sess.TurnCancels == nil {
		sess.TurnCancels = map[string]context.CancelFunc{}
	}
	if sess.TurnResponses == nil {
		sess.TurnResponses = map[string]llmTurnAskResponse{}
	}
	if sess.History == nil {
		sess.History = []shelllib.LLMChatMessage{}
	}
	sess.LastActivity = now
	sess.LastTurnID = req.TurnID
	sess.TurnStatus[req.TurnID] = "in-flight"
	sess.TurnResponses[req.TurnID] = llmTurnAskResponse{Accepted: true, Status: "streaming"}

	provider := sess.Provider
	model := sess.Model
	text := req.Payload.Text
	sessionID := req.SessionID
	turnID := req.TurnID
	traceID := req.TraceID
	maxTokens := req.Payload.MaxTokens
	extraContext := buildLLMClientContextPromptExtra(req.Payload.ClientContext)
	systemPrompt := shelllib.ResolveSystemPrompt(shelllib.PromptOptions{
		SystemPrompt: req.Payload.SystemPrompt,
		ExtraContext: extraContext,
	})
	if strings.TrimSpace(req.Payload.SystemPrompt) != "" && strings.TrimSpace(extraContext) != "" {
		systemPrompt = strings.TrimSpace(req.Payload.SystemPrompt) + "\n\n## context\n\n" + strings.TrimSpace(extraContext)
	}
	messages := make([]shelllib.LLMChatMessage, 0, len(sess.History)+1)
	messages = append(messages, sess.History...)
	messages = append(messages, shelllib.LLMChatMessage{Role: "user", Content: text})
	streamCtx, cancel := context.WithCancel(ctx)
	sess.TurnCancels[turnID] = cancel

	policy := llmExecPolicyFromPayload(req.Payload)
	go ctl.streamLLMSkeleton(streamCtx, sessionID, turnID, traceID, shelllib.LLMStreamRequest{
		Provider:     provider,
		Model:        model,
		SystemPrompt: systemPrompt,
		MaxTokens:    maxTokens,
		Messages:     messages,
	}, req.Payload.ClientContext.toMap(), policy)

	return llmTurnAskResponse{Accepted: true, Status: "streaming"}, nil
}

func (ctl *Controller) rpcLLMTurnCancel(req llmTurnCancelRequest) (llmTurnCancelResponse, error) {
	if req.SessionID == "" {
		return llmTurnCancelResponse{}, invalidParamsError(fmt.Errorf("sessionId is required"))
	}
	if req.TurnID == "" {
		return llmTurnCancelResponse{}, invalidParamsError(fmt.Errorf("turnId is required"))
	}

	now := time.Now()
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[req.SessionID]
	if !ok {
		return llmTurnCancelResponse{}, &controllerRPCError{Code: llmCodeSessionNotFound, Message: "session not found"}
	}
	if llmSessionExpired(sess, now) {
		delete(ctl.llmSessions, req.SessionID)
		return llmTurnCancelResponse{}, &controllerRPCError{Code: llmCodeSessionExpired, Message: "session expired"}
	}
	if sess.TurnCancels != nil {
		if cancel, ok := sess.TurnCancels[req.TurnID]; ok && cancel != nil {
			cancel()
			delete(sess.TurnCancels, req.TurnID)
		}
	}
	if sess.TurnStatus == nil {
		sess.TurnStatus = map[string]string{}
	}
	if sess.TurnResponses == nil {
		sess.TurnResponses = map[string]llmTurnAskResponse{}
	}
	sess.LastActivity = now
	sess.TurnStatus[req.TurnID] = "cancelled"
	sess.TurnResponses[req.TurnID] = llmTurnAskResponse{Accepted: true, Status: "cancelled"}
	return llmTurnCancelResponse{Cancelled: true}, nil
}

func (ctl *Controller) rpcLLMProviderSet(req llmProviderSetRequest) (llmProviderModelResponse, error) {
	if req.SessionID == "" {
		return llmProviderModelResponse{}, invalidParamsError(fmt.Errorf("sessionId is required"))
	}
	if req.Payload.Provider == "" {
		return llmProviderModelResponse{}, invalidParamsError(fmt.Errorf("payload.provider is required"))
	}

	now := time.Now()
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[req.SessionID]
	if !ok {
		return llmProviderModelResponse{}, &controllerRPCError{Code: llmCodeSessionNotFound, Message: "session not found"}
	}
	if llmSessionExpired(sess, now) {
		delete(ctl.llmSessions, req.SessionID)
		return llmProviderModelResponse{}, &controllerRPCError{Code: llmCodeSessionExpired, Message: "session expired"}
	}

	sess.Provider = req.Payload.Provider
	if sess.Provider == "claude" && sess.Model == "" {
		sess.Model = llmDefaultModelForClaude
	}
	sess.LastActivity = now
	return llmProviderModelResponse{Provider: sess.Provider, Model: sess.Model}, nil
}

func (ctl *Controller) rpcLLMModelSet(req llmModelSetRequest) (llmProviderModelResponse, error) {
	if req.SessionID == "" {
		return llmProviderModelResponse{}, invalidParamsError(fmt.Errorf("sessionId is required"))
	}
	if req.Payload.Model == "" {
		return llmProviderModelResponse{}, invalidParamsError(fmt.Errorf("payload.model is required"))
	}

	now := time.Now()
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[req.SessionID]
	if !ok {
		return llmProviderModelResponse{}, &controllerRPCError{Code: llmCodeSessionNotFound, Message: "session not found"}
	}
	if llmSessionExpired(sess, now) {
		delete(ctl.llmSessions, req.SessionID)
		return llmProviderModelResponse{}, &controllerRPCError{Code: llmCodeSessionExpired, Message: "session expired"}
	}

	sess.Model = req.Payload.Model
	sess.LastActivity = now
	return llmProviderModelResponse{Provider: sess.Provider, Model: sess.Model}, nil
}

func newLLMSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return "sess-" + hex.EncodeToString(buf)
}

func llmSessionExpired(sess *llmSession, now time.Time) bool {
	if sess == nil {
		return true
	}
	return now.Sub(sess.LastActivity) > llmSessionIdleTimeout
}

func llmSessionWithinReconnectGrace(sess *llmSession, now time.Time) bool {
	if sess == nil {
		return false
	}
	elapsed := now.Sub(sess.LastActivity)
	return elapsed > llmSessionIdleTimeout && elapsed <= llmSessionIdleTimeout+llmSessionReconnectGrace
}

func (ctl *Controller) streamLLMSkeleton(ctx context.Context, sessionID string, turnID string, traceID string, streamReq shelllib.LLMStreamRequest, clientContext map[string]any, policyOpt ...llmExecPolicy) {
	startedAt := time.Now().UnixMilli()
	policy := defaultLLMExecPolicy()
	if len(policyOpt) > 0 {
		policy = policyOpt[0]
	}
	provider := streamReq.Provider
	model := streamReq.Model
	if strings.TrimSpace(provider) == "" {
		provider = llmDefaultProvider
	}
	if strings.TrimSpace(model) == "" {
		model = llmDefaultModelForClaude
	}
	if len(streamReq.Messages) == 0 {
		streamReq.Messages = []shelllib.LLMChatMessage{{Role: "user", Content: " "}}
	}
	lastIdx := len(streamReq.Messages) - 1
	userText := streamReq.Messages[lastIdx].Content
	if strings.TrimSpace(userText) == "" {
		streamReq.Messages[lastIdx].Content = " "
		userText = " "
	}

	if !emitJsonRpcNotification(ctx, "llm.event", map[string]any{
		"sessionId": sessionID,
		"turnId":    turnID,
		"traceId":   traceID,
		"seq":       ctl.nextLLMSeq(sessionID),
		"event":     "turn.started",
		"ts":        time.Now().UnixMilli(),
		"payload": map[string]any{
			"provider": provider,
			"model":    model,
		},
	}) {
		ctl.finishLLMTurn(sessionID, turnID, "failed")
		return
	}

	if !emitJsonRpcNotification(ctx, "llm.event", map[string]any{
		"sessionId": sessionID,
		"turnId":    turnID,
		"traceId":   traceID,
		"seq":       ctl.nextLLMSeq(sessionID),
		"event":     "turn.block.started",
		"ts":        time.Now().UnixMilli(),
		"payload": map[string]any{
			"blockId":   "b1",
			"blockType": "text",
			"index":     0,
		},
	}) {
		ctl.finishLLMTurn(sessionID, turnID, "failed")
		return
	}

	var contentBuilder strings.Builder
	notifyFailed := false
	streamReq.Provider = provider
	streamReq.Model = model
	resp, streamErr := llmStreamFunc(ctx, streamReq, func(token string) {
		contentBuilder.WriteString(token)
		ok := emitJsonRpcNotification(ctx, "llm.event", map[string]any{
			"sessionId": sessionID,
			"turnId":    turnID,
			"traceId":   traceID,
			"seq":       ctl.nextLLMSeq(sessionID),
			"event":     "turn.delta",
			"ts":        time.Now().UnixMilli(),
			"payload": map[string]any{
				"blockId": "b1",
				"delta":   token,
			},
		})
		if !ok {
			notifyFailed = true
		}
	})
	if notifyFailed && streamErr == nil {
		streamErr = fmt.Errorf("failed to emit turn.delta notification")
	}

	if streamErr != nil {
		payload, eventStatus := llmTurnFailedPayload(streamErr)
		emitJsonRpcNotification(ctx, "llm.event", map[string]any{
			"sessionId": sessionID,
			"turnId":    turnID,
			"traceId":   traceID,
			"seq":       ctl.nextLLMSeq(sessionID),
			"event":     "turn.failed",
			"ts":        time.Now().UnixMilli(),
			"payload":   payload,
		})
		ctl.finishLLMTurn(sessionID, turnID, eventStatus)
		return
	}

	fullText := contentBuilder.String()
	if resp != nil && strings.TrimSpace(resp.Content) != "" {
		fullText = resp.Content
	}
	if strings.TrimSpace(fullText) == "" {
		fullText = "(empty)"
	}
	completedBlocks, historyTail := ctl.buildTurnCompletedBlocks(ctx, sessionID, turnID, traceID, streamReq, fullText, clientContext, policy)
	if len(historyTail) == 0 {
		historyTail = []shelllib.LLMChatMessage{{Role: "assistant", Content: fullText}}
	}
	historyBatch := make([]shelllib.LLMChatMessage, 0, 1+len(historyTail))
	historyBatch = append(historyBatch, shelllib.LLMChatMessage{Role: "user", Content: userText})
	historyBatch = append(historyBatch, historyTail...)
	ctl.appendLLMHistoryMessages(sessionID, historyBatch)

	if !emitJsonRpcNotification(ctx, "llm.event", map[string]any{
		"sessionId": sessionID,
		"turnId":    turnID,
		"traceId":   traceID,
		"seq":       ctl.nextLLMSeq(sessionID),
		"event":     "turn.block.completed",
		"ts":        time.Now().UnixMilli(),
		"payload": map[string]any{
			"blockId":   "b1",
			"blockType": "text",
		},
	}) {
		ctl.finishLLMTurn(sessionID, turnID, "failed")
		return
	}

	if !emitJsonRpcNotification(ctx, "llm.event", map[string]any{
		"sessionId": sessionID,
		"turnId":    turnID,
		"traceId":   traceID,
		"seq":       ctl.nextLLMSeq(sessionID),
		"event":     "turn.completed",
		"ts":        time.Now().UnixMilli(),
		"payload": map[string]any{
			"status": "completed",
			"usage": map[string]any{
				"inputTokens":  resp.InputTokens,
				"outputTokens": resp.OutputTokens,
				"totalTokens":  resp.InputTokens + resp.OutputTokens,
			},
			"latencyMs": time.Now().UnixMilli() - startedAt,
			"blocks":    completedBlocks,
		},
	}) {
		ctl.finishLLMTurn(sessionID, turnID, "failed")
		return
	}

	ctl.finishLLMTurn(sessionID, turnID, "completed")
}

func (ctl *Controller) buildTurnCompletedBlocks(ctx context.Context, sessionID string, turnID string, traceID string, streamReq shelllib.LLMStreamRequest, initialText string, clientContext map[string]any, policy llmExecPolicy) ([]map[string]any, []shelllib.LLMChatMessage) {
	blocks := []map[string]any{{"type": "text", "text": initialText}}
	historyTail := []shelllib.LLMChatMessage{}
	currentText := initialText

	for round := 0; round < policy.MaxRounds; round++ {
		historyTail = append(historyTail, shelllib.LLMChatMessage{Role: "assistant", Content: currentText})

		runnable := extractJshRunBlocks(currentText)
		if len(runnable) == 0 {
			break
		}
		if !policy.AutoExecute {
			for _, code := range runnable {
				blocks = append(blocks, map[string]any{"type": "jsh", "code": code})
			}
			break
		}

		tabs := engine.FSTabs{{MountPoint: "/", FS: ctl.fs}}
		summaries := make([]string, 0, len(runnable))
		for idx, code := range runnable {
			emitJsonRpcNotification(ctx, "llm.event", map[string]any{
				"sessionId": sessionID,
				"turnId":    turnID,
				"traceId":   traceID,
				"seq":       ctl.nextLLMSeq(sessionID),
				"event":     "turn.exec.started",
				"ts":        time.Now().UnixMilli(),
				"payload": map[string]any{
					"index":    idx,
					"readOnly": policy.ReadOnly,
				},
			})
			blocks = append(blocks, map[string]any{"type": "jsh", "code": code})
			rows, err := shelllib.ExecuteWithFSTabs(ctx, tabs, code, shelllib.Options{
				ReadOnly:       policy.ReadOnly,
				MaxRows:        policy.MaxRows,
				TimeoutMs:      policy.TimeoutMs,
				MaxOutputBytes: policy.MaxOutputBytes,
				ClientContext:  clientContext,
			})
			if err != nil {
				errText := "Code execution error: " + err.Error()
				blocks = append(blocks, map[string]any{"type": "text", "text": errText})
				summaries = append(summaries, "Error: "+err.Error())
				emitJsonRpcNotification(ctx, "llm.event", map[string]any{
					"sessionId": sessionID,
					"turnId":    turnID,
					"traceId":   traceID,
					"seq":       ctl.nextLLMSeq(sessionID),
					"event":     "turn.exec.completed",
					"ts":        time.Now().UnixMilli(),
					"payload": map[string]any{
						"index": idx,
						"ok":    false,
						"error": err.Error(),
					},
				})
				continue
			}
			summary := formatAgentExecSummary(rows)
			renderBlocks := collectRenderEnvelopeBlocks(rows)
			if len(renderBlocks) > 0 {
				blocks = append(blocks, renderBlocks...)
			}
			blocks = append(blocks, map[string]any{"type": "text", "text": "Code execution results:\n" + summary})
			summaries = append(summaries, summary)
			execPayload := map[string]any{
				"index": idx,
				"ok":    true,
			}
			if len(renderBlocks) > 0 {
				execPayload["renders"] = renderBlocks
			}
			emitJsonRpcNotification(ctx, "llm.event", map[string]any{
				"sessionId": sessionID,
				"turnId":    turnID,
				"traceId":   traceID,
				"seq":       ctl.nextLLMSeq(sessionID),
				"event":     "turn.exec.completed",
				"ts":        time.Now().UnixMilli(),
				"payload":   execPayload,
			})
		}

		execPrompt := buildExecutionResultsPrompt(summaries)
		if strings.TrimSpace(execPrompt) == "" {
			break
		}
		historyTail = append(historyTail, shelllib.LLMChatMessage{Role: "user", Content: execPrompt})

		followReq := streamReq
		followReq.Messages = make([]shelllib.LLMChatMessage, 0, len(streamReq.Messages)+len(historyTail))
		followReq.Messages = append(followReq.Messages, streamReq.Messages...)
		followReq.Messages = append(followReq.Messages, historyTail...)

		analysisText, err := streamLLMToString(ctx, followReq)
		if err != nil {
			blocks = append(blocks, map[string]any{"type": "text", "text": "Follow-up analysis error: " + err.Error()})
			break
		}
		if strings.TrimSpace(analysisText) == "" {
			analysisText = "(empty)"
		}
		blocks = append(blocks, map[string]any{"type": "text", "text": analysisText})
		currentText = analysisText
	}

	return blocks, historyTail
}

func streamLLMToString(ctx context.Context, req shelllib.LLMStreamRequest) (string, error) {
	var b strings.Builder
	resp, err := llmStreamFunc(ctx, req, func(token string) {
		b.WriteString(token)
	})
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(b.String())
	if text == "" && resp != nil {
		text = strings.TrimSpace(resp.Content)
	}
	return text, nil
}

func buildExecutionResultsPrompt(summaries []string) string {
	if len(summaries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		s := strings.TrimSpace(summary)
		if s == "" {
			continue
		}
		parts = append(parts, "```\n"+s+"\n```")
	}
	if len(parts) == 0 {
		return ""
	}
	return "Code execution results:\n\n" + strings.Join(parts, "\n\n")
}

func extractJshRunBlocks(text string) []string {
	matches := llmJshRunBlockRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		code := strings.TrimSpace(m[1])
		if code == "" {
			continue
		}
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatAgentExecSummary(rows []shelllib.Result) string {
	if len(rows) == 0 {
		return "(no output)"
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		okVal, _ := row["ok"].(bool)
		if !okVal {
			errText := fmt.Sprint(row["error"])
			if strings.TrimSpace(errText) == "" {
				errText = "unknown error"
			}
			lines = append(lines, "Error: "+errText)
			continue
		}
		typeText, _ := row["type"].(string)
		if typeText == "undefined" {
			continue
		}
		value, exists := row["value"]
		if !exists || value == nil {
			continue
		}
		if typeText == "print" {
			lines = append(lines, fmt.Sprint(value))
			continue
		}
		if s, ok := value.(string); ok {
			lines = append(lines, s)
			continue
		}
		if b, err := json.MarshalIndent(value, "", "  "); err == nil {
			lines = append(lines, string(b))
		} else {
			lines = append(lines, fmt.Sprint(value))
		}
	}
	if len(lines) == 0 {
		return "(no output)"
	}
	return strings.Join(lines, "\n")
}

func collectRenderEnvelopeBlocks(rows []shelllib.Result) []map[string]any {
	blocks := []map[string]any{}
	for _, row := range rows {
		okVal, _ := row["ok"].(bool)
		if !okVal {
			continue
		}
		value, exists := row["value"]
		if !exists || value == nil {
			continue
		}
		if !isAgentRenderEnvelope(value) {
			continue
		}
		blocks = append(blocks, map[string]any{
			"type": "vizspec",
			"spec": value,
		})
	}
	return blocks
}

func isAgentRenderEnvelope(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	renderFlag, _ := m["__agentRender"].(bool)
	schema, _ := m["schema"].(string)
	renderer, _ := m["renderer"].(string)
	mode, _ := m["mode"].(string)
	if !renderFlag {
		return false
	}
	if schema != "agent-render/v1" || (renderer != "viz.tui" && renderer != "advn.tui") {
		return false
	}
	return mode == "blocks" || mode == "lines"
}

func (ctl *Controller) nextLLMSeq(sessionID string) int64 {
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	ctl.ensureLLMSessionMapLocked()
	sess, ok := ctl.llmSessions[sessionID]
	if !ok || sess == nil {
		return 0
	}
	sess.NextSeq++
	return sess.NextSeq
}

func (ctl *Controller) ensureLLMSessionMapLocked() {
	if ctl.llmSessions == nil {
		ctl.llmSessions = map[string]*llmSession{}
	}
}

func (ctl *Controller) finishLLMTurn(sessionID string, turnID string, status string) {
	now := time.Now()
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[sessionID]
	if !ok || sess == nil {
		return
	}
	sess.LastActivity = now
	if turnID != "" {
		sess.LastTurnID = turnID
		if sess.TurnCancels != nil {
			delete(sess.TurnCancels, turnID)
		}
		if sess.TurnStatus == nil {
			sess.TurnStatus = map[string]string{}
		}
		if sess.TurnResponses == nil {
			sess.TurnResponses = map[string]llmTurnAskResponse{}
		}
		sess.TurnStatus[turnID] = status
		sess.TurnResponses[turnID] = llmTurnAskResponse{Accepted: true, Status: status}
	}
}

func (ctl *Controller) appendLLMHistory(sessionID string, userText string, assistantText string) {
	ctl.appendLLMHistoryMessages(sessionID, []shelllib.LLMChatMessage{
		shelllib.LLMChatMessage{Role: "user", Content: userText},
		shelllib.LLMChatMessage{Role: "assistant", Content: assistantText},
	})
}

func (ctl *Controller) appendLLMHistoryMessages(sessionID string, msgs []shelllib.LLMChatMessage) {
	if len(msgs) == 0 {
		return
	}
	ctl.llmMu.Lock()
	defer ctl.llmMu.Unlock()
	sess, ok := ctl.llmSessions[sessionID]
	if !ok || sess == nil {
		return
	}
	if sess.History == nil {
		sess.History = []shelllib.LLMChatMessage{}
	}
	sess.History = append(sess.History, msgs...)
	if len(sess.History) > llmMaxHistoryMessages {
		sess.History = append([]shelllib.LLMChatMessage(nil), sess.History[len(sess.History)-llmMaxHistoryMessages:]...)
	}
}

func llmTurnFailedPayload(err error) (map[string]any, string) {
	if errors.Is(err, context.Canceled) {
		return map[string]any{
			"code":      llmCodeTurnCancelled,
			"message":   "turn cancelled",
			"retryable": false,
			"reason":    "user_cancelled",
		}, "cancelled"
	}

	lower := strings.ToLower(err.Error())
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timeout") {
		return map[string]any{
			"code":      llmCodeBackendTimeout,
			"message":   "provider timeout",
			"retryable": true,
			"reason":    "timeout",
		}, "failed"
	}

	if strings.Contains(lower, "unavailable") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") {
		return map[string]any{
			"code":      llmCodeProviderUnavailable,
			"message":   err.Error(),
			"retryable": true,
			"reason":    "provider_unavailable",
		}, "failed"
	}

	return map[string]any{
		"code":      jsonRPCInternal,
		"message":   err.Error(),
		"retryable": false,
		"reason":    "internal",
	}, "failed"
}
