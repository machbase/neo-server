package cmd

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Config struct {
	Topic   string
	MsgID   int64
	Session string
}

type Processor struct {
	Config
}

func NewProcessor(cfg Config) *Processor {
	return &Processor{
		Config: cfg,
	}
}

func (p *Processor) Process(ctx context.Context, line string) {
	p.SendMessage(eventbus.BodyTypeAnswerStart, nil)
	p.SendMessage(eventbus.BodyTypeStreamMessageStart, nil)

	defer func() {
		p.SendMessage(eventbus.BodyTypeStreamMessageStop, nil)
		p.SendMessage(eventbus.BodyTypeAnswerStop, nil)
	}()

	fields := util.SplitFields(line, true)
	if len(fields) == 0 {
		return
	}
	if runtime.GOOS == "windows" {
		// on windows, command line keeps the trailing ';'
		fields[len(fields)-1] = strings.TrimSuffix(fields[len(fields)-1], ";")
	}
	cmd := findCommand(fields[0])
	switch cmd {
	case "sql":
		p.Printf("Executing SQL: %s\n", line)
		for i := range 10 {
			p.Printf("[%d] %v\n", i, time.Now())
			time.Sleep(time.Second)
		}
	default:
		p.Printf("ECHO: %s\n", line)
	}
}

func findCommand(cmdName string) string {
	if IsSqlCommand(cmdName) {
		return "sql"
	}
	return strings.ToLower(cmdName)
}

var sqlCommands = []string{
	"select", "insert", "update", "delete", "alter",
	"create", "drop", "truncate", "exec",
	"mount", "unmount", "backup",
	"grant", "revoke",
}

func IsSqlCommand(cmd string) bool {
	cmd = strings.ToLower(cmd)
	for _, c := range sqlCommands {
		if c == cmd {
			return true
		}
	}
	return false
}
