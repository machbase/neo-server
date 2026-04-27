package server

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSSH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping SSH tests on Windows")
	}
	tests := []struct {
		name   string
		user   string
		cmd    string
		expect []string
	}{
		{
			name: "shell_show_tables",
			user: "sys",
			cmd:  "show tables --format csv",
			expect: []string{
				"ROWNUM,DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG",
				"1,MACHBASEDB,SYS,EXAMPLE,15,Tag,",
				"2,MACHBASEDB,SYS,LOG_DATA,8,Log,",
				"3,MACHBASEDB,SYS,TAG_DATA,7,Tag,",
			},
		},
		{
			name: "jsh_echo",
			user: "sys:jsh",
			cmd:  "echo ssh-ok",
			expect: []string{
				"ssh-ok",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", shellPort), &ssh.ClientConfig{
				User: tt.user,
				Auth: []ssh.AuthMethod{
					ssh.Password("manager"),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         5 * time.Second,
			})
			if err != nil {
				t.Fatalf("Failed to dial SSH server: %v", err)
			}
			defer client.Close()

			session, err := client.NewSession()
			if err != nil {
				t.Fatalf("Failed to create SSH session: %v", err)
			}
			defer session.Close()

			var stdout lockedBuffer
			var stderr lockedBuffer
			session.Stdout = &stdout
			session.Stderr = &stderr

			stdin, err := session.StdinPipe()
			if err != nil {
				t.Fatalf("Failed to get SSH stdin pipe: %v", err)
			}

			if err := session.RequestPty("xterm", 40, 120, ssh.TerminalModes{}); err != nil {
				t.Fatalf("Failed to request PTY: %v", err)
			}

			if err := session.Shell(); err != nil {
				t.Fatalf("Failed to start SSH shell: %v", err)
			}

			if _, err := stdin.Write([]byte(tt.cmd + "\n")); err != nil {
				t.Fatalf("Failed to write SSH command: %v", err)
			}
			if !waitForExpectedOutput(&stdout, tt.expect, 5*time.Second) {
				t.Fatalf("Timed out waiting for SSH output, got %q", stdout.String())
			}
			if _, err := stdin.Write([]byte("exit\n")); err != nil {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("Failed to write SSH exit command: %v", err)
				}
			}
			stdin.Close()

			if err := session.Wait(); err != nil {
				if !strings.Contains(err.Error(), "remote command exited without exit status or exit signal") {
					t.Fatalf("SSH shell failed: %v, stderr: %s", err, stderr.String())
				}
			}

			rawOutput := stdout.String()
			outputStr := removeTerminalControlCharacters(rawOutput)
			if strings.TrimSpace(outputStr) == "" {
				t.Fatalf("Expected SSH command to produce output")
			}
			if strings.TrimSpace(stderr.String()) != "" {
				t.Fatalf("Expected empty stderr, got %q", stderr.String())
			}
			if strings.Contains(rawOutput, "panic:") {
				t.Fatalf("Unexpected panic in SSH shell output: %s", rawOutput)
			}
			for _, expected := range tt.expect {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain '%s', got '%s'", expected, outputStr)
				}
			}
			require.Contains(t, outputStr, strings.Join(tt.expect, "\n"))
		})
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForExpectedOutput(buf *lockedBuffer, expects []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output := buf.String()
		matched := true
		for _, expected := range expects {
			if !strings.Contains(output, expected) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func removeTerminalControlCharacters(s string) string {
	var lines []string
	line := make([]rune, 0, 128)
	cursor := 0

	ensureCursor := func() {
		for len(line) < cursor {
			line = append(line, ' ')
		}
	}
	writeRune := func(r rune) {
		ensureCursor()
		if cursor == len(line) {
			line = append(line, r)
		} else {
			line[cursor] = r
		}
		cursor++
	}
	finishLine := func() {
		lines = append(lines, strings.TrimRight(string(line), " "))
		line = line[:0]
		cursor = 0
	}
	clearLineFromCursor := func() {
		ensureCursor()
		line = line[:cursor]
	}
	clearEntireLine := func() {
		line = line[:0]
		cursor = 0
	}

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\x1b':
			if i+1 >= len(s) {
				continue
			}
			switch s[i+1] {
			case '[':
				j := i + 2
				for j < len(s) && (s[j] < '@' || s[j] > '~') {
					j++
				}
				if j >= len(s) {
					i = len(s)
					break
				}
				params := s[i+2 : j]
				final := s[j]
				paramValues := parseTerminalParams(params)
				switch final {
				case 'm', 'h', 'l':
					// Ignore SGR and mode switches.
				case 'K':
					mode := firstTerminalParam(paramValues, 0)
					switch mode {
					case 0:
						clearLineFromCursor()
					case 1:
						ensureCursor()
						for pos := 0; pos < cursor && pos < len(line); pos++ {
							line[pos] = ' '
						}
					case 2:
						clearEntireLine()
					}
				case 'G':
					cursor = max(0, firstTerminalParam(paramValues, 1)-1)
				case 'C':
					cursor += firstTerminalParam(paramValues, 1)
				case 'D':
					cursor -= firstTerminalParam(paramValues, 1)
					if cursor < 0 {
						cursor = 0
					}
				case 'P':
					count := firstTerminalParam(paramValues, 1)
					ensureCursor()
					if cursor < len(line) {
						end := cursor + count
						if end > len(line) {
							end = len(line)
						}
						line = append(line[:cursor], line[end:]...)
					}
				case '@':
					count := firstTerminalParam(paramValues, 1)
					ensureCursor()
					spaces := make([]rune, count)
					for idx := range spaces {
						spaces[idx] = ' '
					}
					line = append(line[:cursor], append(spaces, line[cursor:]...)...)
				}
				i = j
			case ']':
				j := i + 2
				for j < len(s) && s[j] != '\a' {
					if s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
						j++
						break
					}
					j++
				}
				i = j
			default:
				i++
			}
		case '\r':
			cursor = 0
		case '\n':
			finishLine()
		case '\b', 0x7f:
			if cursor > 0 {
				cursor--
			}
		case '\t':
			nextTabStop := ((cursor / 8) + 1) * 8
			for cursor < nextTabStop {
				writeRune(' ')
			}
		default:
			if s[i] >= 0x20 {
				writeRune(rune(s[i]))
			}
		}
	}
	if len(line) > 0 || cursor > 0 {
		finishLine()
	}
	return strings.Join(compactRepeatedLines(lines), "\n")
}

func parseTerminalParams(params string) []int {
	if params == "" {
		return nil
	}
	parts := strings.Split(params, ";")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		value := 0
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				continue
			}
			value = value*10 + int(ch-'0')
		}
		values = append(values, value)
	}
	return values
}

func firstTerminalParam(params []int, fallback int) int {
	if len(params) == 0 || params[0] == 0 {
		return fallback
	}
	return params[0]
}

func compactRepeatedLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if len(result) > 0 && result[len(result)-1] == trimmed {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func isTerminalPromptLine(line string, user string) bool {
	baseUser := strings.ToLower(user)
	if idx := strings.Index(baseUser, ":"); idx >= 0 {
		baseUser = baseUser[:idx]
	}
	trimmed := strings.ToLower(strings.TrimSpace(line))
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, baseUser+" ") && strings.Contains(trimmed, " machbase-neo ") {
		return true
	}
	return false
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
