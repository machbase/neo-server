package action

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/keys"
)

func (act *Actor) Prompt() {
	history := NewHistory(500)

	onPromptCont := false
	editor := &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			if onPromptCont {
				return io.WriteString(w, act.conf.PromptCont)
			} else {
				return io.WriteString(w, act.conf.Prompt)
			}
		},
		Writer:         colorable.NewColorableStdout(),
		History:        history,
		Coloring:       &ColorHandler{},
		HistoryCycling: true,
	}

	editor.BindKey(keys.CtrlI, &AutoComplete{BuildPrefixCompleter()})
	editor.BindKey(keys.CtrlM, readline.AnonymousCommand(
		func(ctx context.Context, buffer *readline.Buffer) readline.Result {
			str := strings.TrimSpace(buffer.String())
			if strings.HasSuffix(str, "\\") {
				return readline.ENTER
			}
			if str == "" ||
				str == "exit" ||
				str == "quit" ||
				str == "clear" ||
				strings.HasPrefix(str, "help") ||
				strings.HasSuffix(str, ";") {
				return readline.ENTER
			}
			return readline.CONTINUE
		}))

	var parts []string
	for {
		line, err := editor.ReadLine(act.ctx)
		if err != nil {
			if err == io.EOF {
				break
			} else if err == readline.CtrlC {
				// when user send input '^C'
				// clear multi-line buffer and recover origin prompt
				parts = parts[:0]
				onPromptCont = false
				continue
			} else {
				fmt.Println("ERR", err.Error())
				continue
			}
		}

		line = strings.TrimSpace(line)
		if line == "" {
			parts = parts[:0]
			onPromptCont = false
			continue
		}

		if trimLine(line) == "exit" || trimLine(line) == "quit" {
			break
		} else if trimLine(line) == "clear" {
			fmt.Print("\033\143")
			continue
		} else if strings.HasPrefix(line, "help") {
			goto madeline
		} else if line == "set" || strings.HasPrefix(line, "set ") {
			goto madeline
		}

		if strings.HasSuffix(line, "\\") {
			parts = append(parts, strings.Clone(strings.TrimSuffix(line, "\\")))
			onPromptCont = true
			continue
		} else {
			parts = append(parts, strings.Clone(line))
		}
		line = strings.Join(parts, " ")
	madeline:
		history.Add(line)
		line = strings.TrimSuffix(line, ";")
		parts = parts[:0]
		onPromptCont = false
		act.Process(line)
	}
}

func trimLine(line string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";"))
}

type ColorHandler struct {
	bits int
}

func (c *ColorHandler) Init() readline.ColorSequence {
	c.bits = 0
	return readline.ColorReset
}

const (
	doubleQuotedArea = 1
	singleQuotedArea = 2

	colorCodeBitSize = 8

	defaultForegroundColor readline.ColorSequence = 3 | ((30 + 9) << colorCodeBitSize) | (49 << (colorCodeBitSize * 2)) // | (1 << (colorCodeBitSize * 3))
)

func (s *ColorHandler) Next(codepoint rune) readline.ColorSequence {
	newbits := s.bits
	if codepoint == '"' {
		newbits ^= doubleQuotedArea
	} else if codepoint == '\'' {
		newbits ^= singleQuotedArea
	}
	color := defaultForegroundColor
	if codepoint == '\u3000' {
		color = readline.SGR3(37, 22, 41)
	} else if ((s.bits | newbits) & doubleQuotedArea) != 0 {
		color = readline.Cyan
	} else if ((s.bits | newbits) & singleQuotedArea) != 0 {
		color = readline.Magenta
	} else if codepoint == '\\' {
		color = readline.DarkYellow
	} else {
		color = defaultForegroundColor
	}
	s.bits = newbits
	return color
}
