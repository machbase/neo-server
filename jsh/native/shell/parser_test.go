package shell

import (
	"reflect"
	"testing"
)

func TestParseCommand_Simple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "simple command",
			input: "ls",
			expected: &Command{
				Raw: "ls",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "ls",
								Args:    []string{},
							},
						},
					},
				},
			},
		},
		{
			name:  "command with arguments",
			input: "ls -la /tmp",
			expected: &Command{
				Raw: "ls -la /tmp",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "ls",
								Args:    []string{"-la", "/tmp"},
							},
						},
					},
				},
			},
		},
		{
			name:  "command with quoted argument",
			input: `echo "hello world"`,
			expected: &Command{
				Raw: `echo "hello world"`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"hello world"},
							},
						},
					},
				},
			},
		},
		{
			name:  "empty command",
			input: "",
			expected: &Command{
				Raw:        "",
				Statements: []*Statement{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if !commandEqual(result, tt.expected) {
				t.Errorf("parseCommand(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCommand_Pipes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "simple pipe",
			input: "cat file.txt | grep test",
			expected: &Command{
				Raw: "cat file.txt | grep test",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "cat",
								Args:    []string{"file.txt"},
							},
							{
								Command: "grep",
								Args:    []string{"test"},
							},
						},
					},
				},
			},
		},
		{
			name:  "multiple pipes",
			input: "ps aux | grep node | wc -l",
			expected: &Command{
				Raw: "ps aux | grep node | wc -l",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "ps",
								Args:    []string{"aux"},
							},
							{
								Command: "grep",
								Args:    []string{"node"},
							},
							{
								Command: "wc",
								Args:    []string{"-l"},
							},
						},
					},
				},
			},
		},
		{
			name:  "pipe with quoted string containing pipe",
			input: `echo "a | b" | cat`,
			expected: &Command{
				Raw: `echo "a | b" | cat`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"a | b"},
							},
							{
								Command: "cat",
								Args:    []string{},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if !commandEqual(result, tt.expected) {
				t.Errorf("parseCommand(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCommand_Redirections(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "output redirection",
			input: "echo hello > output.txt",
			expected: &Command{
				Raw: "echo hello > output.txt",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"hello"},
								Stdout: &Redirect{
									Type:   ">",
									Target: "output.txt",
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "append redirection",
			input: "echo hello >> output.txt",
			expected: &Command{
				Raw: "echo hello >> output.txt",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"hello"},
								Stdout: &Redirect{
									Type:   ">>",
									Target: "output.txt",
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "input redirection",
			input: "cat < input.txt",
			expected: &Command{
				Raw: "cat < input.txt",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "cat",
								Args:    []string{},
								Stdin: &Redirect{
									Type:   "<",
									Target: "input.txt",
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "multiple redirections",
			input: "sort < input.txt > output.txt",
			expected: &Command{
				Raw: "sort < input.txt > output.txt",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "sort",
								Args:    []string{},
								Stdin: &Redirect{
									Type:   "<",
									Target: "input.txt",
								},
								Stdout: &Redirect{
									Type:   ">",
									Target: "output.txt",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if !commandEqual(result, tt.expected) {
				t.Errorf("parseCommand(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCommand_Statements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "semicolon separator",
			input: "cd /tmp; ls",
			expected: &Command{
				Raw: "cd /tmp; ls",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "cd",
								Args:    []string{"/tmp"},
							},
						},
						Operator: ";",
					},
					{
						Pipelines: []*Pipeline{
							{
								Command: "ls",
								Args:    []string{},
							},
						},
					},
				},
			},
		},
		{
			name:  "and operator",
			input: "mkdir test && cd test",
			expected: &Command{
				Raw: "mkdir test && cd test",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "mkdir",
								Args:    []string{"test"},
							},
						},
						Operator: "&&",
					},
					{
						Pipelines: []*Pipeline{
							{
								Command: "cd",
								Args:    []string{"test"},
							},
						},
					},
				},
			},
		},
		{
			name:  "multiple statements",
			input: "echo a; echo b; echo c",
			expected: &Command{
				Raw: "echo a; echo b; echo c",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"a"},
							},
						},
						Operator: ";",
					},
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"b"},
							},
						},
						Operator: ";",
					},
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"c"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if !commandEqual(result, tt.expected) {
				t.Errorf("parseCommand(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCommand_Complex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "pipe with redirection",
			input: "cat input.txt | grep test > output.txt",
			expected: &Command{
				Raw: "cat input.txt | grep test > output.txt",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "cat",
								Args:    []string{"input.txt"},
							},
							{
								Command: "grep",
								Args:    []string{"test"},
								Stdout: &Redirect{
									Type:   ">",
									Target: "output.txt",
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "statements with pipes",
			input: "cat a.txt | grep x; cat b.txt | grep y",
			expected: &Command{
				Raw: "cat a.txt | grep x; cat b.txt | grep y",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "cat",
								Args:    []string{"a.txt"},
							},
							{
								Command: "grep",
								Args:    []string{"x"},
							},
						},
						Operator: ";",
					},
					{
						Pipelines: []*Pipeline{
							{
								Command: "cat",
								Args:    []string{"b.txt"},
							},
							{
								Command: "grep",
								Args:    []string{"y"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if !commandEqual(result, tt.expected) {
				t.Errorf("parseCommand(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single statement",
			input:    "echo hello",
			expected: []string{"echo hello"},
		},
		{
			name:     "semicolon separator",
			input:    "echo a; echo b",
			expected: []string{"echo a", "echo b"},
		},
		{
			name:     "and operator",
			input:    "cd /tmp && ls",
			expected: []string{"cd /tmp", "ls"},
		},
		{
			name:     "mixed operators",
			input:    "echo a; echo b && echo c",
			expected: []string{"echo a", "echo b", "echo c"},
		},
		{
			name:     "quoted semicolon",
			input:    `echo "a;b"; echo c`,
			expected: []string{`echo "a;b"`, "echo c"},
		},
		{
			name:     "quoted and",
			input:    `echo "a&&b" && echo c`,
			expected: []string{`echo "a&&b"`, "echo c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitStatements(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitStatements(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitPipes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single command",
			input:    "echo hello",
			expected: []string{"echo hello"},
		},
		{
			name:     "two commands",
			input:    "cat file | grep test",
			expected: []string{"cat file", "grep test"},
		},
		{
			name:     "multiple pipes",
			input:    "cat file | grep test | wc -l",
			expected: []string{"cat file", "grep test", "wc -l"},
		},
		{
			name:     "quoted pipe",
			input:    `echo "a|b" | cat`,
			expected: []string{`echo "a|b"`, "cat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitPipes(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitPipes(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple tokens",
			input:    "echo hello world",
			expected: []string{"echo", "hello", "world"},
		},
		{
			name:     "quoted string",
			input:    `echo "hello world"`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "single quoted",
			input:    `echo 'hello world'`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "output redirection",
			input:    "echo test > file.txt",
			expected: []string{"echo", "test", ">", "file.txt"},
		},
		{
			name:     "append redirection",
			input:    "echo test >> file.txt",
			expected: []string{"echo", "test", ">>", "file.txt"},
		},
		{
			name:     "input redirection",
			input:    "cat < file.txt",
			expected: []string{"cat", "<", "file.txt"},
		},
		{
			name:     "mixed quotes and operators",
			input:    `sort "data file.txt" > output.txt`,
			expected: []string{"sort", "data file.txt", ">", "output.txt"},
		},
		{
			name:     "multiple spaces",
			input:    "echo    hello     world",
			expected: []string{"echo", "hello", "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParsePipeline(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Pipeline
	}{
		{
			name:  "simple command",
			input: "ls",
			expected: &Pipeline{
				Command: "ls",
				Args:    []string{},
			},
		},
		{
			name:  "command with args",
			input: "ls -la /tmp",
			expected: &Pipeline{
				Command: "ls",
				Args:    []string{"-la", "/tmp"},
			},
		},
		{
			name:  "with output redirection",
			input: "echo hello > file.txt",
			expected: &Pipeline{
				Command: "echo",
				Args:    []string{"hello"},
				Stdout: &Redirect{
					Type:   ">",
					Target: "file.txt",
				},
			},
		},
		{
			name:  "with input redirection",
			input: "cat < input.txt",
			expected: &Pipeline{
				Command: "cat",
				Args:    []string{},
				Stdin: &Redirect{
					Type:   "<",
					Target: "input.txt",
				},
			},
		},
		{
			name:  "with append redirection",
			input: "echo test >> log.txt",
			expected: &Pipeline{
				Command: "echo",
				Args:    []string{"test"},
				Stdout: &Redirect{
					Type:   ">>",
					Target: "log.txt",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePipeline(tt.input)
			if !pipelineEqual(result, tt.expected) {
				t.Errorf("parsePipeline(%q) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper functions for comparing structures

func commandEqual(a, b *Command) bool {
	if a.Raw != b.Raw {
		return false
	}
	if len(a.Statements) != len(b.Statements) {
		return false
	}
	for i := range a.Statements {
		if !statementEqual(a.Statements[i], b.Statements[i]) {
			return false
		}
	}
	return true
}

func statementEqual(a, b *Statement) bool {
	if a.Operator != b.Operator {
		return false
	}
	if len(a.Pipelines) != len(b.Pipelines) {
		return false
	}
	for i := range a.Pipelines {
		if !pipelineEqual(a.Pipelines[i], b.Pipelines[i]) {
			return false
		}
	}
	return true
}

func pipelineEqual(a, b *Pipeline) bool {
	if a.Command != b.Command {
		return false
	}
	if !reflect.DeepEqual(a.Args, b.Args) {
		return false
	}
	if !redirectEqual(a.Stdin, b.Stdin) {
		return false
	}
	if !redirectEqual(a.Stdout, b.Stdout) {
		return false
	}
	if !redirectEqual(a.Stderr, b.Stderr) {
		return false
	}
	return true
}

func redirectEqual(a, b *Redirect) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type && a.Target == b.Target
}

// TestParseCommand_Unicode tests parsing of Unicode characters including Korean
func TestParseCommand_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Command
	}{
		{
			name:  "Korean characters in argument",
			input: `echo "안녕하세요"`,
			expected: &Command{
				Raw: `echo "안녕하세요"`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"안녕하세요"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Korean characters without quotes",
			input: "echo 안녕하세요",
			expected: &Command{
				Raw: "echo 안녕하세요",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"안녕하세요"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Korean filename in redirection",
			input: "cat < 한글파일.txt",
			expected: &Command{
				Raw: "cat < 한글파일.txt",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "cat",
								Args:    []string{},
								Stdin: &Redirect{
									Type:   "<",
									Target: "한글파일.txt",
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "Korean with pipe",
			input: `echo "안녕" | grep 안녕`,
			expected: &Command{
				Raw: `echo "안녕" | grep 안녕`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"안녕"},
							},
							{
								Command: "grep",
								Args:    []string{"안녕"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Korean with semicolon separator",
			input: "echo 첫번째; echo 두번째",
			expected: &Command{
				Raw: "echo 첫번째; echo 두번째",
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"첫번째"},
							},
						},
						Operator: ";",
					},
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"두번째"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Mixed Korean and English",
			input: `echo "Hello 세계"`,
			expected: &Command{
				Raw: `echo "Hello 세계"`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"Hello 세계"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Japanese characters",
			input: `echo "こんにちは"`,
			expected: &Command{
				Raw: `echo "こんにちは"`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"こんにちは"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Chinese characters",
			input: `echo "你好世界"`,
			expected: &Command{
				Raw: `echo "你好世界"`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"你好世界"},
							},
						},
					},
				},
			},
		},
		{
			name:  "Emoji characters",
			input: `echo "Hello 👋 World 🌍"`,
			expected: &Command{
				Raw: `echo "Hello 👋 World 🌍"`,
				Statements: []*Statement{
					{
						Pipelines: []*Pipeline{
							{
								Command: "echo",
								Args:    []string{"Hello 👋 World 🌍"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if !commandEqual(result, tt.expected) {
				t.Errorf("parseCommand(%q) failed", tt.input)
				t.Errorf("  Got:  %+v", result)
				t.Errorf("  Want: %+v", tt.expected)
				if len(result.Statements) > 0 && len(result.Statements[0].Pipelines) > 0 {
					gotPipeline := result.Statements[0].Pipelines[0]
					expPipeline := tt.expected.Statements[0].Pipelines[0]
					if len(gotPipeline.Args) > 0 && len(expPipeline.Args) > 0 {
						t.Errorf("  Got Args[0]:  %q (bytes: %v)", gotPipeline.Args[0], []byte(gotPipeline.Args[0]))
						t.Errorf("  Want Args[0]: %q (bytes: %v)", expPipeline.Args[0], []byte(expPipeline.Args[0]))
					}
				}
			}
		})
	}
}
