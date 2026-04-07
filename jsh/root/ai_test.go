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
