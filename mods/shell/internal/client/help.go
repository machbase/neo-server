package client

import (
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	RegisterCmd(&Cmd{
		Name:         "help",
		PcFunc:       pcHelp,
		Action:       doHelp,
		Desc:         "Display this message, use 'help [command]'",
		ClientAction: true,
	})
}

func pcHelp() readline.PrefixCompleterInterface {
	return readline.PcItem("help", readline.PcItemDynamic(func(line string) []string {
		lst := make([]string, 0)
		for k := range commands {
			lst = append(lst, k)
		}
		lst = append(lst, "timeformat")
		lst = append(lst, "tz")
		lst = append(lst, "exit")
		return lst
	}))
}

func doHelp(ctx *ActionContext) {
	fields := util.SplitFields(ctx.Line, true)
	if len(fields) > 0 {
		if cmd, ok := commands[strings.ToLower(fields[0])]; ok {
			ctx.Println(cmd.Desc)

			if len(cmd.Usage) > 0 {
				ctx.Println("Usage:")
				lines := strings.Split(cmd.Usage, "\n")
				for _, l := range lines {
					ctx.Println(l)
				}
			}
			return
		}
		switch fields[0] {
		case "timeformat":
			ctx.Println("\n  timeformats:\n" + util.HelpTimeformats())
			return
		case "tz":
			ctx.Println("\n  timezones:\n" + util.HelpTimeZones())
			return
		}
	}
	ctx.Println("commands")
	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "help" {
			return false
		} else if keys[j] == "help" {
			return true
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		cmd := commands[k]
		if cmd.Experimental {
			// do not expose experimental command
			continue
		}
		ctx.Printfln("    %-*s %s", 10, cmd.Name, cmd.Desc)
	}
	ctx.Printfln("    %-*s %s", 10, "exit", "Exit shell")
}
