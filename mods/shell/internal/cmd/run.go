package cmd

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "run",
		PcFunc: pcRun,
		Action: doRun,
		Desc:   "Run a script file",
		Usage:  helpRun,
	})
}

const helpRun string = `  run <filename>
  arguments:
    filename                script file path to run
`

type RunCmd struct {
	Help     bool   `kong:"-"`
	Filename string `arg:"" name:"filename"`
}

func pcRun() readline.PrefixCompleterInterface {
	return readline.PcItem("run")
}

func doRun(ctx *client.ActionContext) {
	cmd := &RunCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpRun); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	file, err := os.Open(cmd.Filename)
	if err != nil {
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	lineno := 0

	buff := []byte{}
	lineBuff := []string{}
	for {
		lineno++
		part, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				ctx.Println("ERR", "line", lineno, err.Error())
			}
			break
		}
		buff = append(buff, part...)
		if isPrefix {
			continue
		}
		subline := string(buff)
		buff = buff[:0]

		if strings.HasPrefix(subline, "#") || strings.HasPrefix(subline, "--") {
			continue
		}
		subline = strings.TrimSpace(subline)
		if len(subline) == 0 {
			// skip empty line
			continue
		}

		lineBuff = append(lineBuff, subline)
		if !strings.HasSuffix(subline, ";") {
			continue
		}

		line := strings.Join(lineBuff, " ")
		line = strings.TrimSuffix(line, ";")
		lineBuff = lineBuff[:0]

		ctx.Println(line)
		ctx.Client.Run(line)
		ctx.Println()
	}
}
