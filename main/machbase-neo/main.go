package main

import (
	"fmt"
	"os"

	"github.com/machbase/booter"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
	shell "github.com/machbase/neo-shell"
)

func main() {
	cli, err := ParseCommand(os.Args)
	if err != nil {
		if cli != nil {
			doHelp(cli.Command, "")
		} else {
			doHelp("", "")
		}
		fmt.Println("ERR", err.Error())
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		doServe()
		return
	}
	switch cli.Command {
	case "gen-config":
		fmt.Println(string(server.DefaultFallbackConfig))
	case "version":
		fmt.Println(server.GenBanner())
	case "help":
		doHelp(cli.Help.Command, cli.Help.SubCommand)
	case "serve":
		doServe()
	case "shell":
		shell.Shell(&cli.Shell)
	case "help <command> <sub-command>":
	}
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
