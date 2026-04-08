package shell

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDefaultAIPromptSegments_FallbackAndCopy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	seg1 := DefaultAIPromptSegments()
	if len(seg1) == 0 {
		t.Fatal("DefaultAIPromptSegments() returned empty slice")
	}
	if !slices.Contains(seg1, "jsh-runtime") {
		t.Fatalf("expected default segment jsh-runtime, got %v", seg1)
	}

	seg1[0] = "mutated"
	seg2 := DefaultAIPromptSegments()
	if len(seg2) == 0 || seg2[0] == "mutated" {
		t.Fatalf("DefaultAIPromptSegments() should return a copy, got %v", seg2)
	}
}

func TestListAndLoadAIPromptSegments_CustomOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	dir, err := LLMCustomPromptDir()
	if err != nil {
		t.Fatalf("LLMCustomPromptDir() error: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir custom prompt dir: %v", err)
	}

	customName := "custom-unit"
	customPath := filepath.Join(dir, customName+".md")
	if err := os.WriteFile(customPath, []byte("custom prompt body"), 0o600); err != nil {
		t.Fatalf("write custom prompt file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write non-md file: %v", err)
	}

	segments, err := ListAIPromptSegments()
	if err != nil {
		t.Fatalf("ListAIPromptSegments() error: %v", err)
	}
	if !slices.Contains(segments, customName) {
		t.Fatalf("custom segment %q not found in %v", customName, segments)
	}
	if !slices.Contains(segments, "jsh-runtime") {
		t.Fatalf("embedded segment jsh-runtime not found in %v", segments)
	}

	if err := os.WriteFile(filepath.Join(dir, "jsh-runtime.md"), []byte("custom runtime"), 0o600); err != nil {
		t.Fatalf("write override prompt: %v", err)
	}
	runtimeBody, err := LoadAIPromptSegment("jsh-runtime")
	if err != nil {
		t.Fatalf("LoadAIPromptSegment(jsh-runtime) error: %v", err)
	}
	if runtimeBody != "custom runtime" {
		t.Fatalf("expected custom override content, got %q", runtimeBody)
	}

	if _, err := LoadAIPromptSegment("segment-that-does-not-exist"); err == nil {
		t.Fatal("expected error for missing segment")
	}
}

func TestBuildAndResolveSystemPrompt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	prompt := BuildAISystemPrompt([]string{"jsh-runtime", "segment-that-does-not-exist"}, "extra ctx")
	if !strings.Contains(prompt, "## jsh-runtime") {
		t.Fatalf("prompt does not include jsh-runtime section: %q", prompt)
	}
	if !strings.Contains(prompt, "## context") {
		t.Fatalf("prompt does not include context section: %q", prompt)
	}

	if got := ResolveSystemPrompt(PromptOptions{SystemPrompt: "manual prompt"}); got != "manual prompt" {
		t.Fatalf("ResolveSystemPrompt() should prefer explicit prompt, got %q", got)
	}

	resolved := ResolveSystemPrompt(PromptOptions{Segments: []string{"jsh-runtime"}, ExtraContext: "ctx"})
	if !strings.Contains(resolved, "## jsh-runtime") || !strings.Contains(resolved, "## context") {
		t.Fatalf("ResolveSystemPrompt() returned unexpected prompt: %q", resolved)
	}
}

func TestExecuteWithFSTabs_BootstrapRequired(t *testing.T) {
	prev := agentExecRuntimeBootstrap
	defer func() { agentExecRuntimeBootstrap = prev }()
	SetAgentExecRuntimeBootstrap(nil)

	_, err := ExecuteWithFSTabs(context.Background(), nil, "1+1", Options{})
	if err == nil {
		t.Fatal("expected bootstrap configuration error")
	}
	if !strings.Contains(err.Error(), "bootstrap is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStreamLLM_RequiresMessages(t *testing.T) {
	_, err := StreamLLM(context.Background(), LLMStreamRequest{}, nil)
	if err == nil {
		t.Fatal("expected messages required error")
	}
	if !strings.Contains(err.Error(), "messages are required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
