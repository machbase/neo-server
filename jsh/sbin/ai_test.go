package sbin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIHelpMentionsSlashAlias(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "ai", "--help")
	if err != nil {
		t.Fatalf("ai --help failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, `prefix with "\" or "/"`) {
		t.Fatalf("help output missing slash alias note:\n%s", output)
	}
	if !strings.Contains(output, `/help`) {
		t.Fatalf("help output missing /help alias:\n%s", output)
	}
}

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

func TestAIHelpIncludesMetricsCommand(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "ai", "--help")
	if err != nil {
		t.Fatalf("ai --help failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, `/metrics [reset]`) {
		t.Fatalf("help output missing metrics command:\n%s", output)
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

func TestAIExecutorCollectErrorDiagnosticsWithContext(t *testing.T) {
	workDir := t.TempDir()

	script := `
const fs = require('fs');
const { collectErrorDiagnostics } = require('ai/executor');

fs.writeFileSync('/work/sample.go', [
  'package main',
  '',
  'func main() {',
  '    x := missingSymbol',
  '    println(x)',
  '}',
  ''
].join('\n'), 'utf8');

const diagnostics = collectErrorDiagnostics([
  { ok: false, error: 'sample.go:4:10: undefined: missingSymbol' }
], { contextLines: 1 });

console.println(String(diagnostics.length));
console.println(String(diagnostics[0].path));
console.println(String(diagnostics[0].context && diagnostics[0].context.path || ''));
console.println(String(diagnostics[0].line));
console.println(diagnostics[0].context && diagnostics[0].context.snippet.indexOf('missingSymbol') >= 0 ? 'context-ok' : 'context-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("collectErrorDiagnostics script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 5 {
		t.Fatalf("unexpected output lines: %q", output)
	}
	if lines[0] != "1" {
		t.Fatalf("diagnostics length = %q, want 1", lines[0])
	}
	if lines[1] != "sample.go" {
		t.Fatalf("diagnostic source path = %q, want sample.go", lines[1])
	}
	if lines[2] != "/work/sample.go" {
		t.Fatalf("diagnostic context path = %q, want /work/sample.go", lines[2])
	}
	if lines[3] != "4" {
		t.Fatalf("diagnostic line = %q, want 4", lines[3])
	}
	if lines[4] != "context-ok" {
		t.Fatalf("diagnostic context missing:\n%s", output)
	}
}

func TestAIExecutorFormatDiagnosticsPrompt(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { formatDiagnosticsPrompt } = require('ai/executor');

const text = formatDiagnosticsPrompt([
  {
    message: 'sample.go:4:10: undefined: missingSymbol',
    path: 'sample.go',
    line: 4,
    col: 10,
    context: { snippet: 'func main() {\\n    x := missingSymbol\\n}' }
  }
], 3);

console.println(text.indexOf('Execution diagnostics:') >= 0 ? 'header-ok' : 'header-missing');
console.println(text.indexOf('agent.fs.patch') >= 0 ? 'patch-first-ok' : 'patch-first-missing');
console.println(text.indexOf('sample.go:4:10') >= 0 ? 'location-ok' : 'location-missing');
console.println(text.indexOf('missingSymbol') >= 0 ? 'context-ok' : 'context-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("formatDiagnosticsPrompt script failed: %v\n%s", err, output)
	}

	for _, want := range []string{"header-ok", "patch-first-ok", "location-ok", "context-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostics prompt check missing %q:\n%s", want, output)
		}
	}
}

func TestAIExecutorDetectPatchFirstViolation(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { detectPatchFirstViolation } = require('ai/executor');

const diagnostics = [{ path: 'main.go', line: 20, col: 5, message: 'undefined symbol' }];
const tick = String.fromCharCode(96);
const fence = tick + tick + tick;
const shortResp = fence + 'jsh-run\nconsole.log("short");\n' + fence;
const longCode = Array(120).fill('let x = 1;').join('\n');
const longResp = fence + 'jsh-run\n' + longCode + '\n' + fence;

console.println(detectPatchFirstViolation(shortResp, diagnostics, { lineThreshold: 80 }) ? 'short-bad' : 'short-ok');
console.println(detectPatchFirstViolation(longResp, diagnostics, { lineThreshold: 80 }) ? 'long-ok' : 'long-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("detectPatchFirstViolation script failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "short-ok") || !strings.Contains(output, "long-ok") {
		t.Fatalf("patch-first violation detection failed:\n%s", output)
	}
}

func TestAIExecutorBuildPatchGuardrailPrompt(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { buildPatchGuardrailPrompt } = require('ai/executor');

const text = buildPatchGuardrailPrompt([
  { path: 'main.go', line: 20, col: 5, message: 'undefined: foo' }
], { maxCount: 2 });

console.println(text.indexOf('Patch-first guardrail:') >= 0 ? 'header-ok' : 'header-missing');
console.println(text.indexOf('agent.fs.patch') >= 0 ? 'patch-ok' : 'patch-missing');
console.println(text.indexOf('main.go:20:5') >= 0 ? 'target-ok' : 'target-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("buildPatchGuardrailPrompt script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"header-ok", "patch-ok", "target-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("guardrail prompt check missing %q:\n%s", want, output)
		}
	}
}

func TestAIExecutorBuildAutoPatchSuggestionPrompt(t *testing.T) {
	workDir := t.TempDir()

	script := `
const { buildAutoPatchSuggestionPrompt } = require('ai/executor');

const text = buildAutoPatchSuggestionPrompt([
  { path: 'main.go', line: 20, col: 5, message: 'undefined: foo' }
], { maxCount: 2 });

console.println(text.indexOf('Auto patch suggestions:') >= 0 ? 'header-ok' : 'header-missing');
console.println(text.indexOf('lineRangePatch') >= 0 ? 'kind-ok' : 'kind-missing');
console.println(text.indexOf('main.go') >= 0 ? 'path-ok' : 'path-missing');
console.println(text.indexOf('"startLine": 20') >= 0 ? 'line-ok' : 'line-missing');
console.println(text.indexOf('minimal patch') >= 0 ? 'minimal-ok' : 'minimal-missing');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("buildAutoPatchSuggestionPrompt script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"header-ok", "kind-ok", "path-ok", "line-ok", "minimal-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("auto patch prompt check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentFsAndExecMVP(t *testing.T) {
	workDir := t.TempDir()

	script := `
const fs = require('fs');
const agent = require('repl/profiles/agent');

const writeResult = agent.fs.write('src/main.txt', 'line-1\nline-2\nline-3\n');
const readResult = agent.fs.read('src/main.txt', { startLine: 2, endLine: 3 });
const patchResult = agent.fs.patch('src/main.txt', {
  kind: 'lineRangePatch',
  startLine: 2,
  endLine: 2,
  replacement: 'line-2-fixed'
});
const anchorResult = agent.fs.patch('src/main.txt', {
  kind: 'anchorPatch',
  before: 'line-3',
  replacement: '\nline-4'
});
const finalContent = fs.readFileSync('/work/src/main.txt', 'utf8');
const runResult = agent.exec.run('pwd');

console.println(writeResult.opType);
console.println(String(writeResult.retryCount));
console.println(String(writeResult.editStats && writeResult.editStats.totalOps || 0));
console.println(readResult.content);
console.println(patchResult.kind);
console.println(String(patchResult.retryCount));
console.println(String(patchResult.editStats && patchResult.editStats.patchOps || 0));
console.println(anchorResult.kind);
console.println(finalContent.indexOf('line-2-fixed') >= 0 ? 'patch-ok' : 'patch-fail');
console.println(finalContent.indexOf('line-4') >= 0 ? 'anchor-ok' : 'anchor-fail');
console.println(String(runResult.opType));
console.println(String(runResult.retryCount));
console.println(String(runResult.editStats && runResult.editStats.runOps || 0));
console.println(String(runResult.exitCode));
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent fs/exec script failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "create") {
		t.Fatalf("missing fs.write opType in output:\n%s", output)
	}
	if !strings.Contains(output, "line-2\nline-3") {
		t.Fatalf("missing fs.read range content in output:\n%s", output)
	}
	if !strings.Contains(output, "lineRangePatch") {
		t.Fatalf("missing lineRangePatch result in output:\n%s", output)
	}
	if !strings.Contains(output, "anchorPatch") {
		t.Fatalf("missing anchorPatch result in output:\n%s", output)
	}
	if !strings.Contains(output, "patch-ok") || !strings.Contains(output, "anchor-ok") {
		t.Fatalf("patch checks failed:\n%s", output)
	}
	if !strings.Contains(output, "run") {
		t.Fatalf("missing exec opType in output:\n%s", output)
	}
	if !strings.Contains(output, "\n1\n") && !strings.HasSuffix(strings.TrimSpace(output), "\n1") {
		t.Fatalf("missing editStats counter output:\n%s", output)
	}
}

func TestAgentFsAndExecReadOnlyAndWorkspacePolicy(t *testing.T) {
	workDir := t.TempDir()

	script := `
globalThis.__agentConfig = { readOnly: true };
const agent = require('repl/profiles/agent');

try {
  agent.fs.write('blocked.txt', 'x');
  console.println('write-unexpected-success');
} catch (err) {
  console.println(String(err.message).indexOf('fs.write') >= 0 ? 'write-denied-ok' : String(err.message));
}

try {
  agent.fs.patch('missing.txt', { kind: 'lineRangePatch', startLine: 1, endLine: 1, replacement: 'x' });
  console.println('patch-unexpected-success');
} catch (err) {
  console.println(String(err.message).indexOf('fs.patch') >= 0 ? 'patch-denied-ok' : String(err.message));
}

try {
  agent.exec.run('mkdir denied-dir');
  console.println('exec-unexpected-success');
} catch (err) {
  console.println(String(err.message).indexOf('not allowed in read-only mode') >= 0 ? 'exec-denied-ok' : String(err.message));
}
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("readOnly policy script failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "write-denied-ok") {
		t.Fatalf("fs.write readOnly policy failed:\n%s", output)
	}
	if !strings.Contains(output, "patch-denied-ok") {
		t.Fatalf("fs.patch readOnly policy failed:\n%s", output)
	}
	if !strings.Contains(output, "exec-denied-ok") {
		t.Fatalf("exec.run readOnly policy failed:\n%s", output)
	}
}

func TestAgentFsWorkspaceBoundaryPolicy(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');

try {
  agent.fs.write('/tmp/out-of-work.txt', 'x');
  console.println('boundary-unexpected-success');
} catch (err) {
  console.println(String(err.message).indexOf('workspace boundary violation') >= 0 ? 'boundary-denied-ok' : String(err.message));
}
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("workspace boundary policy script failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "boundary-denied-ok") {
		t.Fatalf("workspace boundary policy failed:\n%s", output)
	}
}

func TestAgentCreateFailPatchPassFlow(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');

agent.fs.write('flow/main.txt', 'hello\nworld\n');
const failRun = agent.exec.run('cat flow/missing.txt');
agent.fs.patch('flow/main.txt', {
  kind: 'lineRangePatch',
  startLine: 2,
  endLine: 2,
  replacement: 'world-fixed'
});
const passRun = agent.exec.run('cat flow/main.txt');

console.println(String(failRun.exitCode));
console.println(String(passRun.exitCode));
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("create-fail-patch-pass flow script failed: %v\n%s", err, output)
	}

	rawLines := strings.Split(strings.TrimSpace(output), "\n")
	numeric := make([]string, 0, 4)
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isNumeric := true
		for _, ch := range trimmed {
			if ch < '0' || ch > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			numeric = append(numeric, trimmed)
		}
	}
	if len(numeric) < 2 {
		t.Fatalf("expected at least two numeric exit codes in output:\n%s", output)
	}
	if numeric[len(numeric)-2] == "0" {
		t.Fatalf("first run should fail (non-zero exit code), got: %q\nfull output:\n%s", numeric[len(numeric)-2], output)
	}
	if numeric[len(numeric)-1] != "0" {
		t.Fatalf("second run should pass (zero exit code), got: %q\nfull output:\n%s", numeric[len(numeric)-1], output)
	}
}

func TestAgentExecRunOptionHooks(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');

const result = agent.exec.run('pwd', {
  timeoutMs: 2500,
	maxOutputBytes: 999999,
	retryCount: 3
});

console.println(String(result.exitCode));
console.println(String(result.limits.timeoutMs));
console.println(String(result.limits.maxOutputBytes > 0 ? 'max-output-ok' : 'max-output-bad'));
console.println(String(result.retryCount));
console.println(String(result.editStats && result.editStats.runOps || 0));
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("exec option hook script failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "\n0\n") && !strings.HasPrefix(output, "0\n") {
		t.Fatalf("exec exit code missing or not zero:\n%s", output)
	}
	if !strings.Contains(output, "2500") {
		t.Fatalf("timeout hook not reflected in output:\n%s", output)
	}
	if !strings.Contains(output, "max-output-ok") {
		t.Fatalf("maxOutput hook not reflected in output:\n%s", output)
	}
	if !strings.Contains(output, "\n3\n") && !strings.HasSuffix(strings.TrimSpace(output), "\n3") {
		t.Fatalf("retryCount not reflected in output:\n%s", output)
	}
}

func TestAgentFsPatchDryRunSuccess(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
agent.fs.write('dry/main.txt', 'alpha\nbeta\ngamma\n');

const lineResult = agent.fs.patch('dry/main.txt', {
  kind: 'lineRangePatch',
  startLine: 2,
  endLine: 2,
  replacement: 'BETA'
}, { dryRun: true });

const anchorResult = agent.fs.patch('dry/main.txt', {
  kind: 'anchorPatch',
  before: 'beta',
  replacement: '-patched'
}, { dryRun: true });

const fs = require('fs');
const after = fs.readFileSync('/work/dry/main.txt', 'utf8');

console.println(lineResult.dryRun === true ? 'dryRun-flag-ok' : 'dryRun-flag-bad');
console.println(lineResult.ok === true ? 'line-ok' : 'line-bad');
console.println(anchorResult.ok === true ? 'anchor-ok' : 'anchor-bad');
console.println(after.indexOf('BETA') < 0 ? 'file-unchanged-ok' : 'file-was-modified');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("dryRun success script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"dryRun-flag-ok", "line-ok", "anchor-ok", "file-unchanged-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dryRun success check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentFsPatchDryRunAnchorNotFound(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
agent.fs.write('dry/main.txt', 'alpha\nbeta\ngamma\n');

const notFound = agent.fs.patch('dry/main.txt', {
  kind: 'anchorPatch',
  before: 'DOES_NOT_EXIST',
  replacement: 'x'
}, { dryRun: true });

const ambiguous = agent.fs.patch('dry/main.txt', {
  kind: 'anchorPatch',
  before: 'a',
  replacement: 'x'
}, { dryRun: true });

console.println(notFound.ok === false ? 'not-found-ok' : 'not-found-bad');
console.println(notFound.reason === 'anchor-not-found' ? 'reason-ok' : 'reason-bad:' + notFound.reason);
console.println(ambiguous.ok === false ? 'ambiguous-ok' : 'ambiguous-bad');
console.println(ambiguous.reason === 'ambiguous' ? 'ambiguous-reason-ok' : 'ambiguous-reason-bad:' + ambiguous.reason);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("dryRun anchor-not-found script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"not-found-ok", "reason-ok", "ambiguous-ok", "ambiguous-reason-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dryRun anchor check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentFsPatchDryRunOutOfRange(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
agent.fs.write('dry/main.txt', 'alpha\nbeta\ngamma\n');

const result = agent.fs.patch('dry/main.txt', {
  kind: 'lineRangePatch',
  startLine: 1,
  endLine: 999,
  replacement: 'x'
}, { dryRun: true });

console.println(result.ok === false ? 'out-range-ok' : 'out-range-bad');
console.println(result.reason === 'out-of-range' ? 'reason-ok' : 'reason-bad:' + result.reason);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("dryRun out-of-range script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"out-range-ok", "reason-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dryRun out-of-range check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentFsPatchAnchorFallbackOnOutOfRange(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
const fs = require('fs');
agent.fs.write('fb/main.txt', 'alpha\nbeta\ngamma\n');

// lineRangePatch with out-of-range endLine, but anchorFallback provided
const result = agent.fs.patch('fb/main.txt', {
  kind: 'lineRangePatch',
  startLine: 100,
  endLine: 200,
  replacement: 'beta-fixed',
  anchorFallback: {
    before: 'beta',
    replacement: '-fixed'
  }
});

const content = fs.readFileSync('/work/fb/main.txt', 'utf8');

console.println(result.usedFallback === true ? 'fallback-ok' : 'fallback-bad');
console.println(result.kind === 'anchorPatch' ? 'kind-ok' : 'kind-bad:' + result.kind);
console.println(content.indexOf('beta-fixed') >= 0 ? 'content-ok' : 'content-bad:' + content);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("anchorFallback script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"fallback-ok", "kind-ok", "content-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("anchorFallback check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentFsPatchAnchorFallbackNotTriggeredOnSuccess(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
const fs = require('fs');
agent.fs.write('fb/main.txt', 'alpha\nbeta\ngamma\n');

// lineRangePatch succeeds — anchorFallback should NOT be used
const result = agent.fs.patch('fb/main.txt', {
  kind: 'lineRangePatch',
  startLine: 2,
  endLine: 2,
  replacement: 'beta-fixed',
  anchorFallback: {
    before: 'beta',
    replacement: 'should-not-appear'
  }
});

const content = fs.readFileSync('/work/fb/main.txt', 'utf8');

console.println(result.usedFallback ? 'fallback-triggered' : 'fallback-skipped-ok');
console.println(result.kind === 'lineRangePatch' ? 'kind-ok' : 'kind-bad:' + result.kind);
console.println(content.indexOf('beta-fixed') >= 0 ? 'content-ok' : 'content-bad');
console.println(content.indexOf('should-not-appear') < 0 ? 'no-fb-content-ok' : 'fb-content-leaked');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("anchorFallback not-triggered script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"fallback-skipped-ok", "kind-ok", "content-ok", "no-fb-content-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("anchorFallback not-triggered check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentFsPatchDryRunWithAnchorFallback(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
agent.fs.write('fb/main.txt', 'alpha\nbeta\ngamma\n');

// dryRun: lineRangePatch out-of-range → fallback anchor valid → ok+usedFallback
const okResult = agent.fs.patch('fb/main.txt', {
  kind: 'lineRangePatch',
  startLine: 100,
  endLine: 200,
  replacement: 'x',
  anchorFallback: { before: 'beta', replacement: 'x' }
}, { dryRun: true });

// dryRun: lineRangePatch out-of-range → fallback anchor also not found → ok:false
const failResult = agent.fs.patch('fb/main.txt', {
  kind: 'lineRangePatch',
  startLine: 100,
  endLine: 200,
  replacement: 'x',
  anchorFallback: { before: 'NO_SUCH_ANCHOR', replacement: 'x' }
}, { dryRun: true });

const fs = require('fs');
const afterContent = fs.readFileSync('/work/fb/main.txt', 'utf8');

console.println(okResult.dryRun === true ? 'dryRun-flag-ok' : 'dryRun-flag-bad');
console.println(okResult.ok === true ? 'fb-ok' : 'fb-bad');
console.println(okResult.usedFallback === true ? 'fb-used-ok' : 'fb-used-bad');
console.println(failResult.ok === false ? 'fb-fail-ok' : 'fb-fail-bad');
console.println(afterContent.indexOf('beta') >= 0 ? 'file-unchanged-ok' : 'file-was-modified');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("dryRun+anchorFallback script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"dryRun-flag-ok", "fb-ok", "fb-used-ok", "fb-fail-ok", "file-unchanged-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dryRun+anchorFallback check missing %q:\n%s", want, output)
		}
	}
}
func TestAgentDiagnosticsFromOutput(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
const fs = require('fs');

// Write a file so context reading works
agent.fs.write('src/main.go', 'package main\n\nfunc main() {\n\tprintln("hello")\n}\n');

const goErrText = '/work/src/main.go:3:5: undefined: println';
const diags = agent.diagnostics.fromOutput(goErrText, { contextLines: 1 });

console.println(Array.isArray(diags) ? 'array-ok' : 'array-bad');
console.println(diags.length > 0 ? 'length-ok' : 'length-bad');
const d = diags[0];
console.println(d.path && d.path.indexOf('main.go') >= 0 ? 'path-ok' : 'path-bad:' + d.path);
console.println(d.line === 3 ? 'line-ok' : 'line-bad:' + d.line);
console.println(d.col === 5 ? 'col-ok' : 'col-bad:' + d.col);
console.println(d.context && d.context.snippet ? 'context-ok' : 'context-bad');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("diagnostics.fromOutput script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"array-ok", "length-ok", "path-ok", "line-ok", "col-ok", "context-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostics.fromOutput check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentDiagnosticsFromOutputNoLocation(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');

const plainErr = 'something went wrong: unexpected token';
const diags = agent.diagnostics.fromOutput(plainErr);

console.println(Array.isArray(diags) ? 'array-ok' : 'array-bad');
console.println(diags.length === 1 ? 'length-ok' : 'length-bad:' + diags.length);
const d = diags[0];
console.println(d.path === null ? 'path-null-ok' : 'path-not-null:' + d.path);
console.println(d.line === null ? 'line-null-ok' : 'line-not-null:' + d.line);
console.println(d.message.indexOf('unexpected token') >= 0 ? 'msg-ok' : 'msg-bad');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("diagnostics.fromOutput no-location script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"array-ok", "length-ok", "path-null-ok", "line-null-ok", "msg-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostics.fromOutput no-location check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentDiagnosticsSuggest(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');

const diags = [
  { message: 'undefined: println', path: 'src/main.go', line: 3, col: 5, context: null }
];
const prompt = agent.diagnostics.suggest(diags, { maxCount: 1 });

console.println(typeof prompt === 'string' && prompt.length > 0 ? 'string-ok' : 'string-bad');
console.println(prompt.indexOf('Patch suggestions') >= 0 ? 'header-ok' : 'header-bad');
console.println(prompt.indexOf('src/main.go') >= 0 ? 'path-ok' : 'path-bad');
console.println(prompt.indexOf('lineRangePatch') >= 0 ? 'kind-ok' : 'kind-bad');
console.println(prompt.indexOf('"startLine": 3') >= 0 ? 'line-ok' : 'line-bad');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("diagnostics.suggest script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"string-ok", "header-ok", "path-ok", "kind-ok", "line-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostics.suggest check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentDiagnosticsSuggestEmpty(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
const r1 = agent.diagnostics.suggest([]);
const r2 = agent.diagnostics.suggest(null);
console.println(r1 === '' ? 'empty-ok' : 'empty-bad');
console.println(r2 === '' ? 'null-ok' : 'null-bad');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("diagnostics.suggest empty script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"empty-ok", "null-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostics.suggest empty check missing %q:\n%s", want, output)
		}
	}
}

func TestCollectEditStatsAggregatesAgentOps(t *testing.T) {
	workDir := t.TempDir()

	script := `
const executor = require('ai/executor');
const agent = require('repl/profiles/agent');

// Perform write (create), patch, exec.run and capture their return values
const writeResult = agent.fs.write('stats/main.txt', 'alpha\nbeta\ngamma\n');
const patchResult = agent.fs.patch('stats/main.txt', { kind: 'lineRangePatch', startLine: 2, endLine: 2, replacement: 'beta-fixed' });
const execResult  = agent.exec.run('pwd');

// Simulate the results array that executeBlock produces
const fakeResults = [
  { ok: true, type: 'value', value: writeResult },
  { ok: true, type: 'value', value: patchResult },
  { ok: true, type: 'value', value: execResult },
];

const stats = executor.collectEditStats(fakeResults);

console.println(stats.totalOps === 3 ? 'total-ok' : 'total-bad:' + stats.totalOps);
console.println(stats.createOps === 1 ? 'create-ok' : 'create-bad:' + stats.createOps);
console.println(stats.patchOps === 1 ? 'patch-ok' : 'patch-bad:' + stats.patchOps);
console.println(stats.runOps === 1 ? 'run-ok' : 'run-bad:' + stats.runOps);
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("collectEditStats aggregation script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"total-ok", "create-ok", "patch-ok", "run-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("collectEditStats aggregation check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentRuntimeCapabilitiesIncludesDiagnostics(t *testing.T) {
	workDir := t.TempDir()

	script := `
const agent = require('repl/profiles/agent');
const caps = agent.runtime.capabilities();
const joined = caps.join(',');
console.println(joined.indexOf('diagnostics') >= 0 ? 'diag-ok' : 'diag-bad');
console.println(joined.indexOf('fs.patch') >= 0 ? 'patch-ok' : 'patch-bad');
console.println(joined.indexOf('fs.read') >= 0 ? 'read-ok' : 'read-bad');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("capabilities script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"diag-ok", "patch-ok", "read-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("capabilities check missing %q:\n%s", want, output)
		}
	}
}

func TestAgentRuntimeCapabilitiesReadOnlyHasDryRun(t *testing.T) {
	workDir := t.TempDir()

	script := `
globalThis.__agentConfig = { readOnly: true };
const agent = require('repl/profiles/agent');
const caps = agent.runtime.capabilities();
const joined = caps.join(',');
console.println(joined.indexOf('fs.patch.dryRun') >= 0 ? 'dryrun-ok' : 'dryrun-bad');
console.println(joined.indexOf('fs.write') < 0 ? 'no-write-ok' : 'no-write-bad');
console.println(joined.indexOf('diagnostics') >= 0 ? 'diag-ok' : 'diag-bad');
`

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("capabilities readOnly script failed: %v\n%s", err, output)
	}
	for _, want := range []string{"dryrun-ok", "no-write-ok", "diag-ok"} {
		if !strings.Contains(output, want) {
			t.Fatalf("capabilities readOnly check missing %q:\n%s", want, output)
		}
	}
}
