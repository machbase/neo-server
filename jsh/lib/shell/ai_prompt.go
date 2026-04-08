package shell

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed prompts
var promptsEmbed embed.FS

var promptSubFS fs.ReadFileFS

func init() {
	sub, _ := fs.Sub(promptsEmbed, "prompts")
	if rfs, ok := sub.(fs.ReadFileFS); ok {
		promptSubFS = rfs
	}
}

func DefaultAIPromptSegments() []string {
	cfg, err := LoadLLMConfig()
	if err != nil || cfg == nil {
		cfg = DefaultLLMConfig()
	}
	segments := cfg.Prompt.Segments
	if len(segments) == 0 {
		segments = []string{"jsh-runtime", "jsh-modules", "agent-api", "machbase-sql"}
	}
	out := make([]string, len(segments))
	copy(out, segments)
	return out
}

func ListAIPromptSegments() ([]string, error) {
	seen := map[string]bool{}
	var result []string

	if promptSubFS != nil {
		entries, err := fs.ReadDir(promptSubFS, ".")
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				base := strings.TrimSuffix(e.Name(), ".md")
				if !seen[base] {
					seen[base] = true
					result = append(result, base)
				}
			}
		}
	}

	customDir, err := LLMCustomPromptDir()
	if err == nil {
		entries, err := os.ReadDir(customDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				base := strings.TrimSuffix(e.Name(), ".md")
				if !seen[base] {
					seen[base] = true
					result = append(result, base)
				}
			}
		}
	}
	return result, nil
}

func LoadAIPromptSegment(name string) (string, error) {
	customDir, err := LLMCustomPromptDir()
	if err == nil {
		path := filepath.Join(customDir, name+".md")
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}

	if promptSubFS != nil {
		data, err := fs.ReadFile(promptSubFS, name+".md")
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("segment %q not found", name)
}

func BuildAISystemPrompt(segmentNames []string, extraContext string) string {
	parts := make([]string, 0, len(segmentNames)+1)
	for _, name := range segmentNames {
		content, err := LoadAIPromptSegment(name)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			continue
		}
		parts = append(parts, "## "+name+"\n\n"+trimmed)
	}
	if strings.TrimSpace(extraContext) != "" {
		parts = append(parts, "## context\n\n"+strings.TrimSpace(extraContext))
	}
	return strings.Join(parts, "\n\n---\n\n")
}

type PromptOptions struct {
	SystemPrompt string
	Segments     []string
	ExtraContext string
}

func ResolveSystemPrompt(opts PromptOptions) string {
	if strings.TrimSpace(opts.SystemPrompt) != "" {
		return opts.SystemPrompt
	}
	segments := opts.Segments
	if len(segments) == 0 {
		segments = DefaultAIPromptSegments()
	}
	return BuildAISystemPrompt(segments, opts.ExtraContext)
}
