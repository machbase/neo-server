package root_test

import (
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
