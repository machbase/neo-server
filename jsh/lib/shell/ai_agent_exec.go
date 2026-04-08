package shell

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	jshlog "github.com/machbase/neo-server/v8/jsh/log"
)

// AgentExecOptions defines capability and resource limits for agent-profile
// code execution.
//
// New non-shell integrations should prefer package jsh/agentexec. This helper
// remains the implementation used by ai.js and shell-internal flows.
type AgentExecOptions struct {
	ReadOnly       bool
	MaxRows        int
	TimeoutMs      int64
	MaxOutputBytes int
}

func normalizeAgentExecOptions(opts AgentExecOptions) AgentExecOptions {
	if opts.MaxRows <= 0 {
		opts.MaxRows = 1000
	}
	if opts.TimeoutMs <= 0 {
		opts.TimeoutMs = 30000
	}
	if opts.MaxOutputBytes <= 0 {
		opts.MaxOutputBytes = 65536
	}
	return opts
}

// ExecAgentCode runs JavaScript code under the agent REPL profile and returns
// the structured result objects emitted by AgentRenderer.
func ExecAgentCode(rt *goja.Runtime, code string, opts AgentExecOptions) ([]map[string]any, error) {
	if rt == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	opts = normalizeAgentExecOptions(opts)

	agentCfg := agentProfileConfig{
		ReadOnly:       opts.ReadOnly,
		MaxRows:        opts.MaxRows,
		MaxOutputBytes: opts.MaxOutputBytes,
	}
	cfg := defaultReplConfig()
	cfg.Profile = agentReplProfileWith(agentCfg)
	cfg.Renderer = &AgentRenderer{MaxOutputBytes: opts.MaxOutputBytes}
	cfg.Eval = code
	cfg.PrintEval = true
	cfg.ReadOnly = opts.ReadOnly
	cfg.MaxRows = opts.MaxRows
	cfg.MaxOutputBytes = opts.MaxOutputBytes
	cfg.TimeoutMs = opts.TimeoutMs
	cfg.History.Enabled = false

	var buf bytes.Buffer
	var consoleBuf bytes.Buffer
	oldWriter := jshlog.SetDefaultWriter(&consoleBuf)

	r := &Repl{rt: rt, cfg: cfg}
	r.registerBuiltinCommands()
	if err := cfg.Profile.RunStartup(r.rt); err != nil {
		jshlog.SetDefaultWriter(oldWriter)
		return nil, err
	}
	r.runEval(code, true, &buf, cfg.Renderer, opts.TimeoutMs)
	jshlog.SetDefaultWriter(oldWriter)

	var combined bytes.Buffer
	if consoleBuf.Len() > 0 {
		text := strings.TrimRight(consoleBuf.String(), "\n")
		text = stripLogLevelPrefixes(text)
		printLine, _ := json.Marshal(map[string]any{
			"ok":        true,
			"type":      "print",
			"value":     text,
			"elapsedMs": 0,
		})
		combined.Write(printLine)
		combined.WriteByte('\n')
	}
	combined.Write(buf.Bytes())

	results := make([]map[string]any, 0)
	scanner := bufio.NewScanner(&combined)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err == nil {
			results = append(results, obj)
		}
	}

	return results, nil
}
