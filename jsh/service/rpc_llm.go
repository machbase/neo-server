package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
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
	llmExecPromptMaxChars      = 12000
	llmExecPromptSummaryChars  = 3000
)

var llmFollowupTimeout = 30 * time.Second

var llmRunnableBlockRe = regexp.MustCompile("(?is)```[ \\t]*(jsh-run|jsh-shell|jsh-sql)(?:[^\\r\\n`]*)\\r?\\n(.*?)```")
var llmAnyFenceRe = regexp.MustCompile("(?is)```[ \\t]*([a-z0-9_-]+)(?:[^\\r\\n`]*)\\r?\\n(.*?)```")
var llmEvidenceNumberRe = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
var llmAnalysisIntentRe = regexp.MustCompile(`(?i)(\banaly[sz]e\b|\banalysis\b|\breport\b|\bsummarize\b|\bsummary\b|\bdiagnos(?:e|is)\b|\binsight\b|\bfindings\b|분석|리포트|보고서|요약|진단|통계|이상\s*징후)`)

type llmRunnableBlock struct {
	Lang     string
	Code     string
	ExecCode string
	Promoted bool
	FromLang string
}

type llmEditStats struct {
	TotalOps  int
	RunOps    int
	CreateOps int
	PatchOps  int
	ByLang    map[string]int
}

type llmMutationSummary struct {
	OpType    string
	Path      string
	StartLine int
	EndLine   int
}

type llmStructuredEvidence struct {
	Kind      string           `json:"kind"`
	Source    map[string]any   `json:"source,omitempty"`
	SQL       string           `json:"sql,omitempty"`
	Columns   []string         `json:"columns,omitempty"`
	Rows      []map[string]any `json:"rows,omitempty"`
	RowCount  int              `json:"rowCount,omitempty"`
	Truncated bool             `json:"truncated,omitempty"`
	Rendered  string           `json:"rendered,omitempty"`
	Renderer  string           `json:"renderer,omitempty"`
	Mode      string           `json:"mode,omitempty"`
	Meta      map[string]any   `json:"meta,omitempty"`
	Text      string           `json:"text,omitempty"`
	Value     any              `json:"value,omitempty"`
	ValueType string           `json:"valueType,omitempty"`
}

type llmSessionMetrics struct {
	AnalysisIntentTurns       int
	EvidenceGateRetryCount    int
	GroundedReportRetryCount  int
	GroundedCitationPassCount int
	AutoRepairCount           int
}

func newLLMSessionMetrics() *llmSessionMetrics {
	return &llmSessionMetrics{
		AnalysisIntentTurns:       0,
		EvidenceGateRetryCount:    0,
		GroundedReportRetryCount:  0,
		GroundedCitationPassCount: 0,
		AutoRepairCount:           0,
	}
}

func detectLLMAnalysisIntent(text string) bool {
	return llmAnalysisIntentRe.MatchString(strings.TrimSpace(text))
}

type llmSlashSaveCommand struct {
	Path string
}

func newLLMEditStats() *llmEditStats {
	return &llmEditStats{ByLang: map[string]int{}}
}

func (s *llmEditStats) recordRun(lang string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(strings.ToLower(lang))
	s.TotalOps++
	s.RunOps++
	if trimmed == "" {
		trimmed = "unknown"
	}
	s.ByLang[trimmed]++
}

func (s *llmEditStats) clone() *llmEditStats {
	if s == nil {
		return newLLMEditStats()
	}
	out := &llmEditStats{
		TotalOps:  s.TotalOps,
		RunOps:    s.RunOps,
		CreateOps: s.CreateOps,
		PatchOps:  s.PatchOps,
		ByLang:    map[string]int{},
	}
	for k, v := range s.ByLang {
		out.ByLang[k] = v
	}
	return out
}

func (s *llmEditStats) merge(other *llmEditStats) {
	if s == nil || other == nil {
		return
	}
	if s.ByLang == nil {
		s.ByLang = map[string]int{}
	}
	s.TotalOps += other.TotalOps
	s.RunOps += other.RunOps
	s.CreateOps += other.CreateOps
	s.PatchOps += other.PatchOps
	for lang, n := range other.ByLang {
		s.ByLang[lang] += n
	}
}

func (s *llmEditStats) toMap() map[string]any {
	out := map[string]any{
		"totalOps":  0,
		"runOps":    0,
		"createOps": 0,
		"patchOps":  0,
		"byLang":    map[string]any{},
	}
	if s == nil {
		return out
	}
	out["totalOps"] = s.TotalOps
	out["runOps"] = s.RunOps
	out["createOps"] = s.CreateOps
	out["patchOps"] = s.PatchOps
	langMap := map[string]any{}
	for lang, n := range s.ByLang {
		langMap[lang] = n
	}
	out["byLang"] = langMap
	return out
}

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
	Metrics       *llmSessionMetrics
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
	Surface             string   `json:"surface,omitempty"`
	Transport           string   `json:"transport,omitempty"`
	RenderTargets       []string `json:"renderTargets,omitempty"`
	PreferredVizFormats []string `json:"preferredVizFormats,omitempty"`
	FilePolicy          string   `json:"filePolicy,omitempty"`
	BinaryInline        *bool    `json:"binaryInline,omitempty"`
}

func (ctx *llmClientContext) isEmpty() bool {
	if ctx == nil {
		return true
	}
	return strings.TrimSpace(ctx.Surface) == "" &&
		strings.TrimSpace(ctx.Transport) == "" &&
		len(ctx.RenderTargets) == 0 &&
		len(ctx.PreferredVizFormats) == 0 &&
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
	if len(ctx.PreferredVizFormats) > 0 {
		formats := make([]string, 0, len(ctx.PreferredVizFormats))
		for _, one := range ctx.PreferredVizFormats {
			trimmed := strings.TrimSpace(one)
			if trimmed == "" {
				continue
			}
			formats = append(formats, trimmed)
		}
		if len(formats) > 0 {
			out["preferredVizFormats"] = formats
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
	if len(ctx.PreferredVizFormats) > 0 {
		formats := make([]string, 0, len(ctx.PreferredVizFormats))
		for _, one := range ctx.PreferredVizFormats {
			trimmed := strings.TrimSpace(one)
			if trimmed == "" {
				continue
			}
			formats = append(formats, trimmed)
		}
		if len(formats) > 0 {
			lines = append(lines, "- client.preferredVizFormats: "+strings.Join(formats, ", "))
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
		Metrics:       newLLMSessionMetrics(),
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
		Metrics:       newLLMSessionMetrics(),
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
	if sess.Metrics == nil {
		sess.Metrics = newLLMSessionMetrics()
	}
	sess.LastActivity = now
	sess.LastTurnID = req.TurnID
	sess.TurnStatus[req.TurnID] = "in-flight"
	sess.TurnResponses[req.TurnID] = llmTurnAskResponse{Accepted: true, Status: "streaming"}
	if detectLLMAnalysisIntent(req.Payload.Text) {
		sess.Metrics.AnalysisIntentTurns++
	}

	provider := sess.Provider
	model := sess.Model
	text := req.Payload.Text
	sessionID := req.SessionID
	turnID := req.TurnID
	traceID := req.TraceID
	historySnapshot := append([]shelllib.LLMChatMessage(nil), sess.History...)
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
	streamCtx, cancel := context.WithCancel(DetachJsonRpcContext(ctx))
	sess.TurnCancels[turnID] = cancel

	policy := llmExecPolicyFromPayload(req.Payload)
	if slashCmd, ok := parseLLMSlashSaveCommand(text); ok {
		go ctl.runLLMSlashSaveTurn(streamCtx, sessionID, turnID, traceID, provider, model, text, historySnapshot, slashCmd)
		return llmTurnAskResponse{Accepted: true, Status: "streaming"}, nil
	}

	go ctl.streamLLMSkeleton(streamCtx, sessionID, turnID, traceID, shelllib.LLMStreamRequest{
		Provider:     provider,
		Model:        model,
		SystemPrompt: systemPrompt,
		MaxTokens:    maxTokens,
		Messages:     messages,
	}, req.Payload.ClientContext.toMap(), policy)

	return llmTurnAskResponse{Accepted: true, Status: "streaming"}, nil
}

func parseLLMSlashSaveCommand(input string) (llmSlashSaveCommand, bool) {
	line := strings.TrimSpace(input)
	if !strings.HasPrefix(line, "/") {
		return llmSlashSaveCommand{}, false
	}
	parts := strings.Fields(line)
	if len(parts) < 1 || strings.ToLower(parts[0]) != "/save" {
		return llmSlashSaveCommand{}, false
	}
	if len(parts) < 2 {
		return llmSlashSaveCommand{Path: ""}, true
	}
	return llmSlashSaveCommand{Path: strings.TrimSpace(strings.Join(parts[1:], " "))}, true
}

func normalizeLLMSaveTarget(input string) (string, error) {
	target := strings.TrimSpace(input)
	if target == "" {
		return "", fmt.Errorf("usage: /save <file_path>")
	}
	if strings.HasPrefix(target, "/") {
		target = path.Clean(target)
	} else {
		target = path.Clean(path.Join("/work", target))
	}
	if target == "/" {
		return "", fmt.Errorf("invalid save target")
	}
	if target != "/work" && !strings.HasPrefix(target, "/work/") {
		return "", fmt.Errorf("save target must be under /work")
	}
	return target, nil
}

func llmCountUserTurns(entries []shelllib.LLMChatMessage) int {
	turns := 0
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry.Role), "user") {
			turns++
		}
	}
	return turns
}

func llmFormatTranscriptRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return "User"
	case "assistant":
		return "Assistant"
	case "":
		return "Message"
	default:
		r := strings.TrimSpace(role)
		if r == "" {
			return "Message"
		}
		return strings.ToUpper(r[:1]) + r[1:]
	}
}

func buildLLMTranscriptMarkdown(entries []shelllib.LLMChatMessage, provider string, model string, savedAt time.Time) string {
	lines := []string{
		"# AI Session",
		"",
		"- Saved at: " + savedAt.Format(time.RFC3339),
		"- Provider: " + provider,
		"- Model: " + model,
		fmt.Sprintf("- Turns: %d", llmCountUserTurns(entries)),
		"",
		"---",
		"",
	}
	if len(entries) == 0 {
		lines = append(lines, "_No conversation history saved._", "")
		return strings.Join(lines, "\n")
	}
	for _, entry := range entries {
		lines = append(lines, "## "+llmFormatTranscriptRole(entry.Role), "")
		lines = append(lines, strings.TrimSpace(entry.Content), "")
	}
	return strings.Join(lines, "\n")
}

func (ctl *Controller) llmMkdirAll(targetDir string) error {
	dir := path.Clean(strings.TrimSpace(targetDir))
	if dir == "" || dir == "." || dir == "/" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	cur := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		cur = cur + "/" + part
		if err := ctl.fs.Mkdir(cur); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			if info, statErr := ctl.fs.Stat(cur); statErr == nil && info.IsDir() {
				continue
			}
			return err
		}
	}
	return nil
}

func (ctl *Controller) runLLMSlashSaveTurn(ctx context.Context, sessionID string, turnID string, traceID string, provider string, model string, userText string, history []shelllib.LLMChatMessage, cmd llmSlashSaveCommand) {
	startedAt := time.Now().UnixMilli()
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

	target, err := normalizeLLMSaveTarget(cmd.Path)
	if err != nil {
		payload, status := llmTurnFailedPayload(err)
		emitJsonRpcNotification(ctx, "llm.event", map[string]any{
			"sessionId": sessionID,
			"turnId":    turnID,
			"traceId":   traceID,
			"seq":       ctl.nextLLMSeq(sessionID),
			"event":     "turn.failed",
			"ts":        time.Now().UnixMilli(),
			"payload":   payload,
		})
		ctl.finishLLMTurn(sessionID, turnID, status)
		return
	}

	historyForSave := append([]shelllib.LLMChatMessage(nil), history...)
	historyForSave = append(historyForSave, shelllib.LLMChatMessage{Role: "user", Content: userText})
	body := buildLLMTranscriptMarkdown(historyForSave, provider, model, time.Now())
	if err := ctl.llmMkdirAll(path.Dir(target)); err != nil {
		payload, status := llmTurnFailedPayload(err)
		emitJsonRpcNotification(ctx, "llm.event", map[string]any{
			"sessionId": sessionID,
			"turnId":    turnID,
			"traceId":   traceID,
			"seq":       ctl.nextLLMSeq(sessionID),
			"event":     "turn.failed",
			"ts":        time.Now().UnixMilli(),
			"payload":   payload,
		})
		ctl.finishLLMTurn(sessionID, turnID, status)
		return
	}
	if err := ctl.fs.WriteFile(target, []byte(body)); err != nil {
		payload, status := llmTurnFailedPayload(err)
		emitJsonRpcNotification(ctx, "llm.event", map[string]any{
			"sessionId": sessionID,
			"turnId":    turnID,
			"traceId":   traceID,
			"seq":       ctl.nextLLMSeq(sessionID),
			"event":     "turn.failed",
			"ts":        time.Now().UnixMilli(),
			"payload":   payload,
		})
		ctl.finishLLMTurn(sessionID, turnID, status)
		return
	}

	assistantText := "Saved AI session to " + target
	mutationText := "File mutations:\n- create " + target
	blocks := []map[string]any{
		{"type": "text", "text": assistantText},
		{"type": "text", "text": mutationText},
	}
	editStats := (&llmEditStats{TotalOps: 1, CreateOps: 1, ByLang: map[string]int{"slash": 1}}).toMap()

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
				"inputTokens":  0,
				"outputTokens": 0,
				"totalTokens":  0,
			},
			"latencyMs":  time.Now().UnixMilli() - startedAt,
			"blocks":     blocks,
			"editStats":  editStats,
			"retryCount": 0,
		},
	}) {
		ctl.finishLLMTurn(sessionID, turnID, "failed")
		return
	}

	ctl.appendLLMHistoryMessages(sessionID, []shelllib.LLMChatMessage{
		{Role: "user", Content: userText},
		{Role: "assistant", Content: assistantText},
	})
	ctl.finishLLMTurn(sessionID, turnID, "completed")
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
	completedBlocks, historyTail, editStats, retryCount, groundedRetryCount, groundedPassCount, autoRepairCount := ctl.buildTurnCompletedBlocks(ctx, sessionID, turnID, traceID, streamReq, fullText, clientContext, policy)

	// Update session metrics with grounded report results
	if groundedRetryCount > 0 || groundedPassCount > 0 || autoRepairCount > 0 {
		ctl.llmMu.Lock()
		if sess, ok := ctl.llmSessions[sessionID]; ok && sess.Metrics != nil {
			sess.Metrics.GroundedReportRetryCount += groundedRetryCount
			sess.Metrics.GroundedCitationPassCount += groundedPassCount
			sess.Metrics.AutoRepairCount += autoRepairCount
		}
		ctl.llmMu.Unlock()
	}

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

	// Get current session metrics for turn.completed event
	var sessionMetrics *llmSessionMetrics
	ctl.llmMu.Lock()
	if sess, ok := ctl.llmSessions[sessionID]; ok && sess.Metrics != nil {
		// Create a copy to avoid data race
		m := *sess.Metrics
		sessionMetrics = &m
	}
	ctl.llmMu.Unlock()

	metricsPayload := map[string]any{
		"analysisIntentTurns":       0,
		"evidenceGateRetryCount":    0,
		"groundedReportRetryCount":  0,
		"groundedCitationPassCount": 0,
		"autoRepairCount":           0,
	}
	if sessionMetrics != nil {
		metricsPayload["analysisIntentTurns"] = sessionMetrics.AnalysisIntentTurns
		metricsPayload["evidenceGateRetryCount"] = sessionMetrics.EvidenceGateRetryCount
		metricsPayload["groundedReportRetryCount"] = sessionMetrics.GroundedReportRetryCount
		metricsPayload["groundedCitationPassCount"] = sessionMetrics.GroundedCitationPassCount
		metricsPayload["autoRepairCount"] = sessionMetrics.AutoRepairCount
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
			"latencyMs":  time.Now().UnixMilli() - startedAt,
			"blocks":     completedBlocks,
			"editStats":  editStats,
			"retryCount": retryCount,
			"metrics":    metricsPayload,
		},
	}) {
		ctl.finishLLMTurn(sessionID, turnID, "failed")
		return
	}

	ctl.finishLLMTurn(sessionID, turnID, "completed")
}

func (ctl *Controller) buildTurnCompletedBlocks(ctx context.Context, sessionID string, turnID string, traceID string, streamReq shelllib.LLMStreamRequest, initialText string, clientContext map[string]any, policy llmExecPolicy) ([]map[string]any, []shelllib.LLMChatMessage, map[string]any, int, int, int, int) {
	blocks := []map[string]any{{"type": "text", "text": initialText}}
	historyTail := []shelllib.LLMChatMessage{}
	currentText := initialText
	editStats := newLLMEditStats()
	retryCount := 0
	groundedRetryCount := 0
	groundedPassCount := 0
	autoRepairCount := 0

	for round := 0; round < policy.MaxRounds; round++ {
		historyTail = append(historyTail, shelllib.LLMChatMessage{Role: "assistant", Content: currentText})

		runnable := extractRunnableBlocks(currentText)
		if len(runnable) == 0 {
			break
		}
		if round > 0 {
			retryCount++
		}
		if !policy.AutoExecute {
			for _, block := range runnable {
				blocks = append(blocks, map[string]any{"type": "jsh", "lang": block.Lang, "code": block.Code})
			}
			break
		}

		tabs := engine.FSTabs{{MountPoint: "/", FS: ctl.fs}}
		summaries := make([]string, 0, len(runnable))
		evidenceItems := make([]llmStructuredEvidence, 0, len(runnable))
		for idx, block := range runnable {
			startedStats := editStats.clone()
			startedStats.recordRun(block.Lang)
			emitJsonRpcNotification(ctx, "llm.event", map[string]any{
				"sessionId": sessionID,
				"turnId":    turnID,
				"traceId":   traceID,
				"seq":       ctl.nextLLMSeq(sessionID),
				"event":     "turn.exec.started",
				"ts":        time.Now().UnixMilli(),
				"payload": map[string]any{
					"index":      idx,
					"lang":       block.Lang,
					"readOnly":   policy.ReadOnly,
					"opType":     "run",
					"retryCount": retryCount,
					"editStats":  startedStats.toMap(),
				},
			})
			blocks = append(blocks, map[string]any{"type": "jsh", "lang": block.Lang, "code": block.Code})
			if block.Promoted {
				autoRepairCount++
				blocks = append(blocks, map[string]any{"type": "text", "text": fmt.Sprintf("[Auto-repair] promoted plain %s fence to %s.", block.FromLang, block.Lang)})
			}
			rows, err := shelllib.ExecuteWithFSTabs(ctx, tabs, block.ExecCode, shelllib.Options{
				ReadOnly:       policy.ReadOnly,
				MaxRows:        policy.MaxRows,
				TimeoutMs:      policy.TimeoutMs,
				MaxOutputBytes: policy.MaxOutputBytes,
				ClientContext:  clientContext,
			})
			if err != nil {
				failedStats := newLLMEditStats()
				failedStats.recordRun(block.Lang)
				editStats.merge(failedStats)
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
						"index":      idx,
						"lang":       block.Lang,
						"ok":         false,
						"error":      err.Error(),
						"opType":     "run",
						"retryCount": retryCount,
						"editStats":  editStats.toMap(),
					},
				})
				continue
			}
			blockStats := collectLLMEditStatsFromRows(rows)
			if blockStats.TotalOps == 0 {
				blockStats.recordRun(block.Lang)
			}
			editStats.merge(blockStats)
			primaryOp := detectLLMPrimaryOpType(blockStats)
			mutations := collectLLMMutationSummaries(rows)
			summary := formatAgentExecSummary(rows)
			if len(mutations) > 0 {
				summary += "\n" + formatLLMMutationSummary(mutations)
			}
			renderBlocks := collectRenderEnvelopeBlocks(rows)
			if len(renderBlocks) > 0 {
				blocks = append(blocks, renderBlocks...)
			}
			blockEvidence := collectLLMStructuredEvidence(rows, block)
			if len(blockEvidence) == 0 {
				switch block.Lang {
				case "jsh-sql":
					blockEvidence = append(blockEvidence, llmStructuredEvidence{
						Kind:     "sql",
						Source:   map[string]any{"lang": block.Lang, "promoted": block.Promoted, "promotedFrom": block.FromLang},
						SQL:      block.Code,
						Rendered: summary,
					})
				case "jsh-run", "jsh-shell":
					blockEvidence = append(blockEvidence, llmStructuredEvidence{
						Kind:      "value",
						Source:    map[string]any{"lang": block.Lang, "promoted": block.Promoted, "promotedFrom": block.FromLang},
						Value:     summary,
						ValueType: "summary",
					})
				}
			}
			evidenceItems = append(evidenceItems, blockEvidence...)
			blocks = append(blocks, map[string]any{"type": "text", "text": "Code execution results:\n" + summary})
			if evidenceText := buildStructuredEvidencePrompt(blockEvidence); strings.TrimSpace(evidenceText) != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": evidenceText})
			}
			if len(mutations) > 0 {
				// Emit a dedicated mutation summary text block so generic clients can
				// render file/line patch details without parsing mixed execution output.
				blocks = append(blocks, map[string]any{"type": "text", "text": formatLLMMutationSummary(mutations)})
			}
			summaries = append(summaries, summary)
			execPayload := map[string]any{
				"index":      idx,
				"lang":       block.Lang,
				"ok":         true,
				"opType":     primaryOp,
				"retryCount": retryCount,
				"editStats":  editStats.toMap(),
			}
			if len(mutations) > 0 {
				execPayload["mutations"] = llmMutationsToPayload(mutations)
				execPayload["mutationSummary"] = formatLLMMutationSummary(mutations)
			}
			if block.Promoted {
				execPayload["autoRepair"] = map[string]any{"applied": true, "fromLang": block.FromLang, "lang": block.Lang}
			}
			if len(renderBlocks) > 0 {
				execPayload["renders"] = renderBlocks
			}
			if len(blockEvidence) > 0 {
				execPayload["evidence"] = llmEvidenceToPayload(blockEvidence)
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
		if evidencePrompt := buildStructuredEvidencePrompt(evidenceItems); strings.TrimSpace(evidencePrompt) != "" {
			historyTail = append(historyTail, shelllib.LLMChatMessage{Role: "user", Content: evidencePrompt})
		}

		followReq := streamReq
		followReq.Messages = make([]shelllib.LLMChatMessage, 0, len(streamReq.Messages)+len(historyTail))
		followReq.Messages = append(followReq.Messages, streamReq.Messages...)
		followReq.Messages = append(followReq.Messages, historyTail...)

		analysisText, err := streamLLMToStringWithTimeout(ctx, followReq, llmFollowupTimeout)
		if err != nil {
			blocks = append(blocks, map[string]any{"type": "text", "text": "Follow-up analysis error: " + err.Error()})
			break
		}
		if strings.TrimSpace(analysisText) == "" {
			analysisText = "(empty)"
		}
		if detectLLMUngroundedReport(analysisText, evidenceItems) {
			groundedRetryCount += 1
			groundedPrompt := buildLLMGroundedReportPrompt(evidenceItems)
			historyTail = append(historyTail, shelllib.LLMChatMessage{Role: "user", Content: groundedPrompt})
			blocks = append(blocks, map[string]any{"type": "text", "text": "[Grounded Report] requesting evidence-grounded rewrite..."})

			rewriteReq := streamReq
			rewriteReq.Messages = make([]shelllib.LLMChatMessage, 0, len(streamReq.Messages)+len(historyTail))
			rewriteReq.Messages = append(rewriteReq.Messages, streamReq.Messages...)
			rewriteReq.Messages = append(rewriteReq.Messages, historyTail...)

			rewriteText, rewriteErr := streamLLMToStringWithTimeout(ctx, rewriteReq, llmFollowupTimeout)
			if rewriteErr != nil {
				blocks = append(blocks, map[string]any{"type": "text", "text": "Grounded report retry error: " + rewriteErr.Error()})
				break
			}
			if strings.TrimSpace(rewriteText) != "" {
				analysisText = rewriteText
			}
		} else {
			// Validation passed
			groundedPassCount += 1
		}
		blocks = append(blocks, map[string]any{"type": "text", "text": analysisText})
		currentText = analysisText
	}

	return blocks, historyTail, editStats.toMap(), retryCount, groundedRetryCount, groundedPassCount, autoRepairCount
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

func streamLLMToStringWithTimeout(ctx context.Context, req shelllib.LLMStreamRequest, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		return streamLLMToString(ctx, req)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return streamLLMToString(callCtx, req)
}

func buildExecutionResultsPrompt(summaries []string) string {
	if len(summaries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(summaries))
	totalChars := 0
	for _, summary := range summaries {
		s := strings.TrimSpace(summary)
		if s == "" {
			continue
		}
		s = truncateLLMExecPromptText(s, llmExecPromptSummaryChars)
		part := "```\n" + s + "\n```"
		if totalChars > 0 && totalChars+len(part)+2 > llmExecPromptMaxChars {
			parts = append(parts, "[execution results truncated]")
			break
		}
		parts = append(parts, part)
		totalChars += len(part)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Code execution results:\n\n" + strings.Join(parts, "\n\n")
}

func buildStructuredEvidencePrompt(items []llmStructuredEvidence) string {
	if len(items) == 0 {
		return ""
	}
	limited := items
	if len(limited) > 3 {
		limited = limited[:3]
	}
	raw, err := json.MarshalIndent(limited, "", "  ")
	if err != nil {
		return ""
	}
	text := string(raw)
	if len(text) > llmExecPromptMaxChars {
		text = truncateLLMExecPromptText(text, llmExecPromptMaxChars)
	}
	return "Structured execution evidence:\n```json\n" + text + "\n```"
}

func collectLLMEvidenceHints(items []llmStructuredEvidence, maxHints int) []string {
	if maxHints <= 0 {
		maxHints = 8
	}
	hints := make([]string, 0, maxHints)
	seen := map[string]bool{}
	add := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" || seen[token] || len(hints) >= maxHints {
			return
		}
		seen[token] = true
		hints = append(hints, token)
	}
	for _, item := range items {
		switch item.Kind {
		case "sql":
			if item.RowCount > 0 {
				add(fmt.Sprintf("%d", item.RowCount))
			}
			for _, col := range item.Columns {
				add(col)
			}
			for _, row := range item.Rows {
				for _, v := range row {
					switch one := v.(type) {
					case string:
						add(one)
					case float64:
						add(fmt.Sprintf("%v", one))
					case int:
						add(fmt.Sprintf("%d", one))
					}
				}
			}
			for _, token := range llmEvidenceNumberRe.FindAllString(item.Rendered, -1) {
				add(token)
			}
		case "viz":
			add(item.Renderer)
			add(item.Mode)
		}
		if len(hints) >= maxHints {
			break
		}
	}
	return hints
}

func buildLLMGroundedReportPrompt(items []llmStructuredEvidence) string {
	hints := collectLLMEvidenceHints(items, 8)
	lines := []string{
		"Grounded report retry:",
		"Write the report from the executed evidence only.",
		"Explicitly cite the observed values, counts, ranges, aggregates, or render outputs from the immediately preceding execution results.",
		"Do not ask the user to run queries manually.",
		"Do not provide unsupported generic conclusions.",
	}
	if len(hints) > 0 {
		lines = append(lines, "Observed value hints: "+strings.Join(hints, ", "))
	}
	return strings.Join(lines, "\n")
}

func detectLLMUngroundedReport(responseText string, evidence []llmStructuredEvidence) bool {
	resp := strings.TrimSpace(responseText)
	if resp == "" || len(evidence) == 0 {
		return false
	}
	if len(extractRunnableBlocks(resp)) > 0 {
		return false
	}
	lowered := strings.ToLower(resp)
	if strings.Contains(lowered, "blocked") ||
		strings.Contains(lowered, "cannot execute") ||
		strings.Contains(lowered, "execution is impossible") ||
		strings.Contains(lowered, "unable to execute") ||
		strings.Contains(lowered, "실행할 수 없") ||
		strings.Contains(lowered, "차단되었") ||
		strings.Contains(lowered, "불가능") {
		return false
	}
	if strings.Contains(lowered, "run these quer") ||
		strings.Contains(lowered, "run this query") ||
		strings.Contains(lowered, "paste the result") ||
		strings.Contains(lowered, "share the result") ||
		strings.Contains(lowered, "쿼리를 실행") ||
		strings.Contains(lowered, "결과를 붙여넣") ||
		strings.Contains(lowered, "결과를 공유") {
		return true
	}
	hints := collectLLMEvidenceHints(evidence, 8)
	for _, hint := range hints {
		if strings.Contains(resp, hint) {
			return false
		}
	}
	for _, item := range evidence {
		if item.Kind == "viz" {
			if strings.Contains(lowered, "render") ||
				strings.Contains(lowered, "chart") ||
				strings.Contains(lowered, "plot") ||
				strings.Contains(lowered, "series") ||
				strings.Contains(lowered, "blocks") ||
				strings.Contains(lowered, "lines") ||
				strings.Contains(lowered, "시각화") ||
				strings.Contains(lowered, "차트") ||
				strings.Contains(lowered, "그래프") {
				return false
			}
			return true
		}
	}
	return len(hints) > 0
}

func truncateLLMExecPromptText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	marker := "...[truncated]"
	if limit <= len(marker) {
		return trimmed[:limit]
	}
	return strings.TrimSpace(trimmed[:limit-len(marker)]) + marker
}

func extractRunnableBlocks(text string) []llmRunnableBlock {
	matches := llmRunnableBlockRe.FindAllStringSubmatch(text, -1)
	out := make([]llmRunnableBlock, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		lang := strings.TrimSpace(strings.ToLower(m[1]))
		code := strings.TrimSpace(m[2])
		if code == "" {
			continue
		}
		execCode := buildRunnableExecCode(lang, code)
		if strings.TrimSpace(execCode) == "" {
			continue
		}
		out = append(out, llmRunnableBlock{Lang: lang, Code: code, ExecCode: execCode})
	}
	if len(out) == 0 {
		for _, block := range tryPromoteRunnableBlocks(text) {
			out = append(out, block)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func tryPromoteRunnableBlocks(text string) []llmRunnableBlock {
	matches := llmAnyFenceRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]llmRunnableBlock, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		lang := strings.TrimSpace(strings.ToLower(m[1]))
		code := strings.TrimSpace(m[2])
		if code == "" {
			continue
		}
		if promoted, ok := tryPromotePlainSQLBlock(lang, code); ok {
			out = append(out, promoted)
			continue
		}
		if promoted, ok := tryPromotePlainJSBlock(lang, code); ok {
			out = append(out, promoted)
			continue
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func tryPromotePlainSQLBlock(lang string, code string) (llmRunnableBlock, bool) {
	if lang != "sql" {
		return llmRunnableBlock{}, false
	}
	text := strings.TrimSpace(code)
	if text == "" || strings.Contains(text, ";") {
		return llmRunnableBlock{}, false
	}
	if !regexp.MustCompile(`(?i)^(select|show|describe|desc|explain)\b`).MatchString(text) {
		return llmRunnableBlock{}, false
	}
	return llmRunnableBlock{
		Lang:     "jsh-sql",
		Code:     text,
		ExecCode: buildRunnableExecCode("jsh-sql", text),
		Promoted: true,
		FromLang: lang,
	}, true
}

func tryPromotePlainJSBlock(lang string, code string) (llmRunnableBlock, bool) {
	if lang != "js" && lang != "javascript" {
		return llmRunnableBlock{}, false
	}
	text := strings.TrimSpace(code)
	if text == "" || strings.Count(text, "\n") > 40 {
		return llmRunnableBlock{}, false
	}
	if !strings.Contains(text, "agent.") {
		return llmRunnableBlock{}, false
	}
	if strings.Contains(text, "process.exec") ||
		strings.Contains(text, "agent.exec.run") ||
		strings.Contains(text, "agent.fs.write") ||
		strings.Contains(text, "agent.fs.patch") ||
		strings.Contains(text, "agent.db.exec") {
		return llmRunnableBlock{}, false
	}
	return llmRunnableBlock{
		Lang:     "jsh-run",
		Code:     text,
		ExecCode: buildRunnableExecCode("jsh-run", text),
		Promoted: true,
		FromLang: lang,
	}, true
}

func buildRunnableExecCode(lang string, code string) string {
	switch strings.TrimSpace(strings.ToLower(lang)) {
	case "jsh-run":
		return code
	case "jsh-shell":
		commandJSON, _ := json.Marshal(code)
		return strings.Join([]string{
			"(function () {",
			"    'use strict';",
			"    const process = require('process');",
			"    const splitFields = require('util/splitFields');",
			"    const cmdline = " + string(commandJSON) + ";",
			"    const line = String(cmdline || '').trim();",
			"    if (!line) { throw new Error('jsh-shell: empty command'); }",
			"    const fields = splitFields(line);",
			"    if (!fields || fields.length === 0) { throw new Error('jsh-shell: invalid command'); }",
			"    const command = fields[0];",
			"    const args = fields.slice(1);",
			"    const readOnly = !!(agent && agent.runtime && agent.runtime.limits && agent.runtime.limits().readOnly);",
			"    const allow = { ls: true, cat: true, pwd: true, echo: true, wc: true, head: true, tail: true };",
			"    if (readOnly && !allow[command]) { throw new Error('jsh-shell: command denied in read-only mode: ' + command); }",
			"    const exitCode = process.exec(command, ...args);",
			"    return { command: line, args: args, exitCode: exitCode };",
			"}());",
		}, "\n")
	case "jsh-sql":
		sqlJSON, _ := json.Marshal(code)
		return strings.Join([]string{
			"(function () {",
			"    'use strict';",
			"    const pretty = require('pretty');",
			"    const sql = " + string(sqlJSON) + ";",
			"    const text = String(sql || '').trim();",
			"    if (!text) { throw new Error('jsh-sql: empty SQL'); }",
			"    if (text.indexOf(';') >= 0) { throw new Error('jsh-sql: only single statement is allowed'); }",
			"    const lowered = text.toLowerCase();",
			"    const readOnly = !!(agent && agent.runtime && agent.runtime.limits && agent.runtime.limits().readOnly);",
			"    const allowed = /^(select|show|describe|desc|explain)\\b/.test(lowered);",
			"    if (readOnly && !allowed) { throw new Error('jsh-sql: write statements are denied in read-only mode'); }",
			"    const result = agent.db.query(text);",
			"    const box = pretty.Table({ rownum: false, footer: false, format: 'box' });",
			"    const rows = (result && Array.isArray(result.rows)) ? result.rows : [];",
			"    if (rows.length === 0) {",
			"        return { __agentSql: true, schema: 'agent-sql/v1', sql: text, columns: [], rows: [], rowCount: 0, truncated: !!(result && result.truncated), rendered: '(no rows)' };",
			"    }",
			"    const columns = Object.keys(rows[0]);",
			"    box.appendHeader(columns);",
			"    for (let i = 0; i < rows.length; i++) {",
			"        const row = rows[i] || {};",
			"        const values = [];",
			"        for (let c = 0; c < columns.length; c++) {",
			"            values.push(row[columns[c]]);",
			"        }",
			"        box.append(values);",
			"    }",
			"    let rendered = box.render();",
			"    if (result && result.truncated) {",
			"        rendered += '\\n[truncated at ' + result.count + ' rows]';",
			"    }",
			"    return { __agentSql: true, schema: 'agent-sql/v1', sql: text, columns: columns, rows: rows, rowCount: rows.length, truncated: !!(result && result.truncated), rendered: rendered };",
			"}());",
		}, "\n")
	default:
		return ""
	}
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
		if isAgentRenderEnvelope(value) || isRawVizspec(value) {
			blocks = append(blocks, map[string]any{
				"type": "vizspec",
				"spec": value,
			})
		}
	}
	return blocks
}

func collectLLMStructuredEvidence(rows []shelllib.Result, block llmRunnableBlock) []llmStructuredEvidence {
	out := []llmStructuredEvidence{}
	source := map[string]any{"lang": block.Lang}
	if block.Promoted {
		source["promoted"] = true
		source["promotedFrom"] = block.FromLang
	}
	for _, row := range rows {
		okVal, _ := row["ok"].(bool)
		if !okVal {
			continue
		}
		value, exists := row["value"]
		if !exists || value == nil {
			continue
		}
		vm, ok := toObjectMap(value)
		if !ok {
			continue
		}
		if isAgentSQLEvidence(vm) {
			out = append(out, llmStructuredEvidence{
				Kind:      "sql",
				Source:    source,
				SQL:       strings.TrimSpace(fmt.Sprint(vm["sql"])),
				Columns:   toStringSlice(vm["columns"]),
				Rows:      toRowMaps(vm["rows"]),
				RowCount:  toInt(vm["rowCount"]),
				Truncated: toBool(vm["truncated"]),
				Rendered:  strings.TrimSpace(fmt.Sprint(vm["rendered"])),
			})
			continue
		}
		if isAgentRenderEnvelope(value) || isRawVizspec(value) {
			meta, _ := vm["meta"].(map[string]any)
			out = append(out, llmStructuredEvidence{
				Kind:     "viz",
				Source:   source,
				Renderer: strings.TrimSpace(fmt.Sprint(vm["renderer"])),
				Mode:     strings.TrimSpace(fmt.Sprint(vm["mode"])),
				Meta:     meta,
			})
			continue
		}
		typeText, _ := row["type"].(string)
		if typeText == "print" {
			out = append(out, llmStructuredEvidence{
				Kind:   "print",
				Source: source,
				Text:   fmt.Sprint(value),
			})
			continue
		}
		if typeText != "undefined" {
			out = append(out, llmStructuredEvidence{
				Kind:      "value",
				Source:    source,
				ValueType: typeText,
				Value:     value,
			})
		}
	}
	return out
}

func llmEvidenceToPayload(items []llmStructuredEvidence) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var one map[string]any
		if err := json.Unmarshal(raw, &one); err != nil {
			continue
		}
		out = append(out, one)
	}
	return out
}

func isRawVizspec(value any) bool {
	m, ok := toObjectMap(value)
	if !ok {
		return false
	}
	schema, _ := m["schema"].(string)
	return schema == "vizspec/v1" || schema == "advn/v1"
}

func isAgentSQLEvidence(value any) bool {
	m, ok := toObjectMap(value)
	if !ok {
		return false
	}
	flag, _ := m["__agentSql"].(bool)
	schema, _ := m["schema"].(string)
	return flag && schema == "agent-sql/v1"
}

func isAgentRenderEnvelope(value any) bool {
	m, ok := toObjectMap(value)
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

func collectLLMEditStatsFromRows(rows []shelllib.Result) *llmEditStats {
	stats := newLLMEditStats()
	for _, row := range rows {
		okVal, _ := row["ok"].(bool)
		if !okVal {
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
		vm, ok := value.(map[string]any)
		if !ok {
			continue
		}
		es, ok := vm["editStats"].(map[string]any)
		if !ok || es == nil {
			continue
		}
		stats.TotalOps += toInt(es["totalOps"])
		stats.RunOps += toInt(es["runOps"])
		stats.CreateOps += toInt(es["createOps"])
		stats.PatchOps += toInt(es["patchOps"])
		if byLang, ok := es["byLang"].(map[string]any); ok {
			for lang, n := range byLang {
				stats.ByLang[strings.ToLower(strings.TrimSpace(lang))] += toInt(n)
			}
		}
	}
	return stats
}

func detectLLMPrimaryOpType(stats *llmEditStats) string {
	if stats == nil {
		return "run"
	}
	if stats.PatchOps > 0 {
		return "patch"
	}
	if stats.CreateOps > 0 {
		return "create"
	}
	return "run"
}

func collectLLMMutationSummaries(rows []shelllib.Result) []llmMutationSummary {
	out := []llmMutationSummary{}
	for _, row := range rows {
		okVal, _ := row["ok"].(bool)
		if !okVal {
			continue
		}
		value, exists := row["value"]
		if !exists || value == nil {
			continue
		}
		vm, ok := value.(map[string]any)
		if !ok {
			continue
		}
		opType := strings.ToLower(strings.TrimSpace(fmt.Sprint(vm["opType"])))
		switch opType {
		case "write", "create":
			opType = "create"
		case "patch":
			// keep patch
		default:
			continue
		}
		path, _ := vm["path"].(string)
		out = append(out, llmMutationSummary{
			OpType:    opType,
			Path:      path,
			StartLine: toInt(vm["startLine"]),
			EndLine:   toInt(vm["endLine"]),
		})
	}
	return out
}

func llmMutationsToPayload(items []llmMutationSummary) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := map[string]any{"opType": it.OpType}
		if strings.TrimSpace(it.Path) != "" {
			m["path"] = it.Path
		}
		if it.StartLine > 0 {
			m["startLine"] = it.StartLine
		}
		if it.EndLine > 0 {
			m["endLine"] = it.EndLine
		}
		out = append(out, m)
	}
	return out
}

func formatLLMMutationSummary(items []llmMutationSummary) string {
	if len(items) == 0 {
		return ""
	}
	lines := []string{"File mutations:"}
	for _, it := range items {
		line := "- " + it.OpType
		if strings.TrimSpace(it.Path) != "" {
			line += " " + it.Path
		}
		if it.StartLine > 0 {
			if it.EndLine > 0 && it.EndLine != it.StartLine {
				line += fmt.Sprintf(":%d-%d", it.StartLine, it.EndLine)
			} else {
				line += fmt.Sprintf(":%d", it.StartLine)
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case int32:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}

func toBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	default:
		return false
	}
}

func toStringSlice(v any) []string {
	switch items := v.(type) {
	case []string:
		return items
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func toRowMaps(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok {
		if rows, ok := v.([]map[string]any); ok {
			return rows
		}
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func toObjectMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
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
