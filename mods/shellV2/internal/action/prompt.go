package action

import (
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/coloring"
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
		Coloring:       &coloring.VimBatch{},
		HistoryCycling: true,
	}

	editor.BindKey(keys.CtrlI, &AutoComplete{BuildPrefixCompleter()})

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
			fmt.Println("\033\143")
			continue
		} else if strings.HasPrefix(line, "help") {
			goto madeline
		} else if line == "set" || strings.HasPrefix(line, "set ") {
			goto madeline
		}

		parts = append(parts, strings.Clone(line))
		if !strings.HasSuffix(line, ";") {
			onPromptCont = true
			continue
		}
		line = strings.Join(parts, " ")
	madeline:
		history.Add(line)
		line = strings.TrimSuffix(line, ";")
		parts = parts[:0]
		onPromptCont = false
		act.Process(line)
		// TODO there is a timing issue between prompt and stdout
		// without sleep, sometimes the prompt does not display on client's terminal.
		//time.Sleep(50 * time.Millisecond)
	}
}

func trimLine(line string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";"))
}
