package jshext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Elapsed  time.Duration
	TimedOut bool
	Err      string
}

func runJSHCode(code string, opts Options) ExecResult {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var ctx context.Context = context.Background()
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
	}

	jr, err := engine.New(engine.Config{
		Name:    "jshext",
		Code:    code,
		Context: ctx,
		Env: map[string]any{
			"PATH":         "/sbin:/work",
			"HOME":         "/work",
			"PWD":          "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		},
		Aliases: map[string]string{
			"ll": "ls -l",
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
			// TODO: add /work fstab
		},
	})
	if err != nil {
		return ExecResult{Err: err.Error()}
	}
	lib.Enable(jr)

	jr.Env = engine.NewEnv(
		engine.WithWriter(&stdout),
		engine.WithErrorWriter(&stderr),
		engine.WithReader(strings.NewReader("")),
	)

	start := time.Now()
	err = jr.RunContext(ctx)
	elapsed := time.Since(start)
	res := ExecResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: jr.ExitCode(), Elapsed: elapsed}
	if err != nil {
		if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			res.TimedOut = true
			res.Err = fmt.Sprintf("timed out after %s", opts.Timeout)
			res.ExitCode = -1
		} else {
			res.Err = err.Error()
		}
	}
	return res
}

func formatExecOutput(res ExecResult) string {
	var b strings.Builder
	if res.Stdout != "" {
		b.WriteString(res.Stdout)
	}
	if res.Stderr != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(res.Stderr)
	}
	if res.Err != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(res.Err)
	}
	if b.Len() == 0 {
		return ""
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderExecResultJSON(res ExecResult) string {
	type payload struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode int    `json:"exitCode"`
		Elapsed  int64  `json:"elapsedMs"`
		TimedOut bool   `json:"timedOut"`
		Err      string `json:"error,omitempty"`
	}
	data := payload{
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		ExitCode: res.ExitCode,
		Elapsed:  res.Elapsed.Milliseconds(),
		TimedOut: res.TimedOut,
		Err:      res.Err,
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": %q}", err.Error())
	}
	return string(encoded)
}
