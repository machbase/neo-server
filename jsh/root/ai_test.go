package root_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIHelpIncludesSaveCommand(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "ai", "--help")
	if err != nil {
		t.Fatalf("ai --help failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, `/save <file_path>`) {
		t.Fatalf("help output missing save command:\n%s", output)
	}
}

func TestAITranscriptModuleWritesMarkdownTranscript(t *testing.T) {
	workDir := t.TempDir()

	script := `
const fs = require('fs');
const { saveTranscript } = require('ai/transcript');

const result = saveTranscript('logs/session.md', {
    cwd: '/work',
    savedAt: '2026-04-06T12:34:56+09:00',
    provider: 'claude',
    model: 'claude-opus-4-5',
    promptSegments: ['jsh-runtime', 'agent-api'],
    history: [
        { role: 'user', content: 'Inspect the latest table status.' },
        { role: 'assistant', content: 'The latest table status is healthy.' }
    ]
});

console.println(result.path);
console.println(String(result.turns));
console.print(fs.readFileSync('/work/logs/session.md', 'utf8'));
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("transcript module script failed: %v\n%s", err, output)
	}

	targetPath := filepath.Join(workDir, "logs", "session.md")
	contentBytes, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read saved transcript: %v", readErr)
	}
	content := string(contentBytes)

	if !strings.Contains(output, "/work/logs/session.md") {
		t.Fatalf("script output missing saved path:\n%s", output)
	}
	if !strings.Contains(output, "\n1\n") && !strings.HasSuffix(output, "\n1") {
		t.Fatalf("script output missing turn count:\n%s", output)
	}
	for _, want := range []string{
		"# AI Session",
		"- Saved at: 2026-04-06T12:34:56+09:00",
		"- Provider: claude",
		"- Model: claude-opus-4-5",
		"- Prompt segments: jsh-runtime, agent-api",
		"- Turns: 1",
		"## User",
		"Inspect the latest table status.",
		"## Assistant",
		"The latest table status is healthy.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("saved transcript missing %q:\n%s", want, content)
		}
	}
}

func TestAITranscriptModuleRequiresPath(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { saveTranscript } = require('ai/transcript');

try {
    saveTranscript('', { cwd: '/work' });
    console.println('unexpected success');
} catch (err) {
    console.println(err.message);
}
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("transcript module usage script failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, `Usage: \save <file_path>`) {
		t.Fatalf("usage output missing:\n%s", output)
	}
}

func TestAIExecutorDetectsRenderEnvelope(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { isRenderEnvelope } = require('ai/executor');

const good = {
  __agentRender: true,
  schema: 'agent-render/v1',
	renderer: 'viz.tui',
  mode: 'blocks',
  blocks: []
};
const bad = {
  __agentRender: true,
  schema: 'agent-render/v1',
  renderer: 'advn.svg',
  mode: 'blocks',
  blocks: []
};

console.println(String(isRenderEnvelope(good)));
console.println(String(isRenderEnvelope(bad)));
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("ai/executor envelope detection script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "true" {
		t.Fatalf("isRenderEnvelope(good) = %q, want true", lines[0])
	}
	if lines[1] != "false" {
		t.Fatalf("isRenderEnvelope(bad) = %q, want false", lines[1])
	}
}

func TestAIExecutorFormatResultsSummarizesRenderEnvelope(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { formatResults } = require('ai/executor');

const summary = formatResults([
  {
    ok: true,
    type: 'object',
    value: {
      __agentRender: true,
      schema: 'agent-render/v1',
	renderer: 'viz.tui',
      mode: 'lines',
      lines: ['line-a', 'line-b', 'line-c']
    }
  },
  { ok: true, type: 'print', value: 'done' }
]);

console.println(summary);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("ai/executor formatResults script failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "[rendered viz.tui lines: 3]") {
		t.Fatalf("format summary missing ADVN render hint:\n%s", output)
	}
	if !strings.Contains(output, "done") {
		t.Fatalf("format summary missing print output:\n%s", output)
	}
}

func TestAIExecutorCollectRenderEnvelopes(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { collectRenderEnvelopes } = require('ai/executor');

const results = [
  {
    ok: true,
    value: {
      __agentRender: true,
      schema: 'agent-render/v1',
	renderer: 'viz.tui',
      mode: 'blocks',
      blocks: [{ type: 'summary', title: 'ADVN' }]
    }
  },
  { ok: true, type: 'print', value: 'plain text' },
  {
    ok: true,
    value: {
      __agentRender: true,
      schema: 'agent-render/v1',
	renderer: 'viz.tui',
      mode: 'lines',
      lines: ['a', 'b']
    }
  }
];

const envelopes = collectRenderEnvelopes(results);
console.println(String(envelopes.length));
console.println(envelopes[0].mode);
console.println(envelopes[1].mode);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("ai/executor collectRenderEnvelopes script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "2" {
		t.Fatalf("envelope count = %q, want 2", lines[0])
	}
	if lines[1] != "blocks" {
		t.Fatalf("first envelope mode = %q, want blocks", lines[1])
	}
	if lines[2] != "lines" {
		t.Fatalf("second envelope mode = %q, want lines", lines[2])
	}
}

func TestAgentVizRenderSwitchesModeAndReportsErrors(t *testing.T) {
	workDir := t.TempDir()

	script := `
const advn = require('vizspec');
const agent = require('repl/profiles/agent');

const spec = advn.createSpec({
  series: [{
    id: 'cpu',
    representation: { kind: 'raw-point', fields: ['x', 'y'] },
    data: [[0, 1], [1, 3], [2, 2], [3, 4]]
  }]
});

const linesEnv = agent.viz.render(spec, { mode: 'lines', width: 16, height: 3 });
console.println(linesEnv.mode);

const blocksEnv = agent.viz.render(spec, { mode: 'blocks', width: 16, rows: 4 });
console.println(blocksEnv.mode);

try {
  agent.viz.render(spec, { mode: 'invalid-mode' });
  console.println('unexpected-success');
} catch (err) {
  console.println(String(err.message).indexOf('unsupported mode') >= 0 ? 'unsupported-mode-ok' : String(err.message));
}
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent.viz.render mode script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "lines" {
		t.Fatalf("first mode = %q, want lines", lines[0])
	}
	if lines[1] != "blocks" {
		t.Fatalf("second mode = %q, want blocks", lines[1])
	}
	if lines[2] != "unsupported-mode-ok" {
		t.Fatalf("invalid mode handling = %q, want unsupported-mode-ok", lines[2])
	}
}

func TestAIExecutorExecuteJshReturnsRenderEnvelope(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { executeJsh, collectRenderEnvelopes } = require('ai/executor');

const code = [
  "const spec = {",
  "  version: 1,",
  "  series: [{",
  "    id: 'cpu',",
  "    representation: { kind: 'raw-point', fields: ['x', 'y'] },",
  "    data: [[0, 1], [1, 3], [2, 2], [3, 4]]",
  "  }]",
  "};",
  "agent.viz.lines(spec, { width: 20, height: 3, timeformat: 'rfc3339' });"
].join("\n");

const results = executeJsh(code, { readOnly: true, maxRows: 1000, timeoutMs: 30000 });
const envs = collectRenderEnvelopes(results);

console.println(String(envs.length));
if (envs.length > 0) {
  console.println(String(envs[0].renderer));
  console.println(String(envs[0].mode));
  console.println(String((envs[0].lines || []).length));
}
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("executeJsh render envelope script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 4 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "1" {
		t.Fatalf("envelope count = %q, want 1", lines[0])
	}
	if lines[1] != "viz.tui" {
		t.Fatalf("renderer = %q, want viz.tui", lines[1])
	}
	if lines[2] != "lines" {
		t.Fatalf("mode = %q, want lines", lines[2])
	}
	if lines[3] == "0" {
		t.Fatalf("rendered lines should not be empty: %q", output)
	}
}

func TestAIExecutorExecuteJshRenderEnvelopeAndPrintMix(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { executeJsh, collectRenderEnvelopes, formatResults } = require('ai/executor');

const code = [
  "console.log('render-start');",
  "const spec = {",
  "  version: 1,",
  "  series: [{",
  "    id: 'cpu',",
  "    representation: { kind: 'raw-point', fields: ['x', 'y'] },",
  "    data: [[0, 1], [1, 3], [2, 2], [3, 4]]",
  "  }]",
  "};",
  "agent.viz.blocks(spec, { width: 20, rows: 4, compact: true });"
].join("\n");

const results = executeJsh(code, { readOnly: true, maxRows: 1000, timeoutMs: 30000 });
const envs = collectRenderEnvelopes(results);
const summary = formatResults(results);

console.println(String(envs.length));
console.println(summary.indexOf('render-start') >= 0 ? 'print-ok' : 'print-missing');
console.println(summary.indexOf('[rendered viz.tui blocks:') >= 0 ? 'render-ok' : 'render-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("executeJsh mixed output script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "1" {
		t.Fatalf("envelope count = %q, want 1", lines[0])
	}
	if lines[1] != "print-ok" {
		t.Fatalf("print summary check = %q, want print-ok", lines[1])
	}
	if lines[2] != "render-ok" {
		t.Fatalf("render summary check = %q, want render-ok", lines[2])
	}
}

func TestAIExecutorExecuteJshRenderEnvelopeTruncationFallback(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { executeJsh, collectRenderEnvelopes, formatResults } = require('ai/executor');

const code = [
  "const data = [];",
  "for (let i = 0; i < 120; i++) { data.push([i, i % 9]); }",
  "const spec = {",
  "  version: 1,",
  "  series: [{",
  "    id: 'cpu',",
  "    representation: { kind: 'raw-point', fields: ['x', 'y'] },",
  "    data: data",
  "  }]",
  "};",
  "agent.viz.blocks(spec, { width: 120, rows: 120, compact: false });"
].join("\n");

const results = executeJsh(code, {
  readOnly: true,
  maxRows: 1000,
  timeoutMs: 30000,
  maxOutputBytes: 64,
});

const envs = collectRenderEnvelopes(results);
const summary = formatResults(results);

console.println(String(envs.length));
console.println(summary.indexOf('[truncated]') >= 0 ? 'truncated-ok' : 'truncated-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("executeJsh truncation fallback script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "0" {
		t.Fatalf("envelope count under truncation = %q, want 0", lines[0])
	}
	if lines[1] != "truncated-ok" {
		t.Fatalf("truncation summary check = %q, want truncated-ok", lines[1])
	}
}

func TestAIExecutorFormatResultsAddsRenderTruncationHint(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { formatResults } = require('ai/executor');

const summary = formatResults([
  {
    ok: true,
    type: 'object',
    value: '[truncated: 12345 bytes]',
    truncated: true
  }
]);

console.println(summary.indexOf('render payload truncated') >= 0 ? 'hint-ok' : summary);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("ai/executor truncation hint script failed: %v\n%s", err, output)
	}

	if strings.TrimSpace(output) != "hint-ok" {
		t.Fatalf("render truncation hint missing:\n%s", output)
	}
}
