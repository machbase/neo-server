package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

type Options struct {
	ReadOnly       bool
	MaxRows        int
	TimeoutMs      int64
	MaxOutputBytes int
}

type Result map[string]any

const outputMarker = "__AGENTEXEC__"

var agentExecRuntimeBootstrap func(*engine.JSRuntime)

// SetAgentExecRuntimeBootstrap configures how temporary runtimes are prepared
// before running ai.exec scripts.
func SetAgentExecRuntimeBootstrap(fn func(*engine.JSRuntime)) {
	agentExecRuntimeBootstrap = fn
}

func ExecuteWithFSTabs(ctx context.Context, tabs engine.FSTabs, code string, opts Options) ([]Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	optMap := map[string]any{
		"readOnly":       opts.ReadOnly,
		"maxRows":        opts.MaxRows,
		"timeoutMs":      opts.TimeoutMs,
		"maxOutputBytes": opts.MaxOutputBytes,
	}
	codeJSON, _ := json.Marshal(code)
	optJSON, _ := json.Marshal(optMap)

	var stdout bytes.Buffer
	script := strings.Join([]string{
		"const process = require('process');",
		"const { ai } = require('@jsh/shell');",
		"const __res = ai.exec(" + string(codeJSON) + ", " + string(optJSON) + ");",
		"process.stdout.write('" + outputMarker + "' + JSON.stringify(__res));",
	}, "\n")

	jr, err := engine.New(engine.Config{
		Name:    "agentexec",
		Code:    script,
		Context: ctx,
		FSTabs:  tabs,
		Writer:  &stdout,
	})
	if err != nil {
		return nil, err
	}
	if agentExecRuntimeBootstrap == nil {
		return nil, fmt.Errorf("agent exec runtime bootstrap is not configured")
	}
	agentExecRuntimeBootstrap(jr)
	if err := jr.Run(); err != nil {
		return nil, err
	}

	out := stdout.String()
	idx := strings.LastIndex(out, outputMarker)
	if idx < 0 {
		trimmed := strings.TrimSpace(out)
		if trimmed == "" {
			return []Result{}, nil
		}
		return nil, fmt.Errorf("agentexec marker not found in output: %s", trimmed)
	}
	raw := strings.TrimSpace(out[idx+len(outputMarker):])
	if raw == "" {
		return []Result{}, nil
	}

	var rows []Result
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Result{}
	}
	return rows, nil
}
