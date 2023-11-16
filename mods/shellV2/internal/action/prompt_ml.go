package action

import (
	"fmt"
	"io"
	"strings"

	multiline "github.com/hymkor/go-multiline-ny"
	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/keys"
)

func (act *Actor) PromptMultiLine() {
	history := NewHistory(500)

	ed := multiline.Editor{}
	ed.LineEditor.Writer = colorable.NewColorableStdout()
	ed.LineEditor.History = history
	ed.LineEditor.HistoryCycling = true
	ed.LineEditor.Coloring = &ColorHandler{}
	ed.BindKey(keys.CtrlI, &AutoComplete{BuildPrefixCompleter()})
	ed.BindKey(keys.Up, readline.AnonymousCommand(ed.CmdPreviousHistory))
	ed.BindKey(keys.Down, readline.AnonymousCommand(ed.CmdNextHistory))
	ed.BindKey(keys.AltJ, readline.AnonymousCommand(ed.CmdNextLine))
	ed.BindKey(keys.AltK, readline.AnonymousCommand(ed.CmdPreviousLine))
	ed.SetPrompt(func(w io.Writer, n int) (int, error) {
		if n > 0 {
			return io.WriteString(w, act.conf.PromptCont)
		} else {
			return io.WriteString(w, act.conf.Prompt)
		}
	})
	ed.SubmitOnEnterWhen(func(lines []string, _ int) bool {
		if strings.HasSuffix(strings.TrimSpace(lines[len(lines)-1]), ";") {
			return true
		}
		line := strings.Join(lines, "\n")
		switch trimLine(line) {
		case "exit", "quit", "clear":
			return true
		}
		if strings.HasPrefix(line, "help") {
			return true
		} else if line == "set" || strings.HasPrefix(line, "set ") {
			return true
		}

		return false
	})
	for {
		lines, err := ed.Read(act.ctx)
		if err != nil {
			if err == io.EOF {
				break
			} else if err == readline.CtrlC {
				// when user send input '^C'
				// clear multi-line buffer and recover origin prompt
				continue
			} else {
				fmt.Println("ERR", err.Error())
				continue
			}
		}
		line := strings.Join(lines, "\n")
		if trimLine(line) == "exit" || trimLine(line) == "quit" {
			break
		} else if trimLine(line) == "clear" {
			fmt.Print("\033\143")
			continue
		}
		history.Add(line)
		line = strings.TrimSuffix(line, ";")
		act.Process(line)
	}
}
