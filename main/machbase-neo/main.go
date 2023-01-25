package main

import (
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/machbase/booter"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
	shell "github.com/machbase/neo-shell"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		doServe()
	} else {
		var cli struct {
			Serve ServeCmd       `cmd:"" help:"serve machbase-neo"`
			Shell shell.ShellCmd `cmd:"" help:"shell client"`
		}
		cmd := kong.Parse(&cli,
			kong.HelpOptions{NoAppSummary: false, Compact: true, FlagsLast: true},
			kong.UsageOnError(),
		)
		command := cmd.Command()

		switch {
		default:
			cmd.PrintUsage(false)
		case strings.HasPrefix(command, "shell"):
			shell.Shell(&cli.Shell)
		case command == "serve":
			doServe()
		}
	}
}

type ServeCmd struct {
	Args []string `arg:"" optional:"" name:"ARGS" passthrough:""`
}

func doServe() {
	booter.SetConfiFileSuffix(".conf")
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + mods.EngineInfoString())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
}
