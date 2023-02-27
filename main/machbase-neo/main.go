package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/machbase/booter"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
	shell "github.com/machbase/neo-shell"
	"github.com/machbase/neo-shell/client"
)

func main() {
	var cli struct {
		Serve     ServeCmd       `cmd:"" name:"serve" help:"start machbase-neo server process"`
		Shell     shell.ShellCmd `cmd:"" name:"shell" help:"run neoshell client"`
		GenConfig struct{}       `cmd:"" name:"gen-config" help:"show config template"`
		Version   struct{}       `cmd:"" name:"version" help:"show version"`
		Help      struct {
			Command    string `arg:""`
			SubCommand string `arg:"" optional:""`
		} `cmd:"" name:"help"`
	}

	kongParser := kong.Parse(&cli,
		kong.HelpOptions{NoAppSummary: true, Compact: true, FlagsLast: true},
		kong.TypeMapper(reflect.TypeOf((*time.Location)(nil)), &client.TimezoneParser{}),
		kong.Help(func(options kong.HelpOptions, ctx *kong.Context) error {
			if len(ctx.Args) > 0 {
				return doHelp(ctx.Args[0])
			} else {
				return doHelp("")
			}
		}),
		kong.UsageOnError(),
	)
	command := kongParser.Command()
	switch command {
	case "gen-config":
		fmt.Println(string(server.DefaultFallbackConfig))
		return
	case "version":
		fmt.Println(server.GenBanner())
		return
	case "help <command>":
		switch cli.Help.Command {
		case "serve":
			doHelp("serve")
		case "shell":
			doHelp("shell")
		case "timeformat":
			fmt.Printf("%s\n", client.HelpTimeFormat)
		default:
			doHelp("")
		}
		return
	case "help <command> <sub-command>":
		switch cli.Help.Command {
		case "shell":
			if len(cli.Help.SubCommand) > 0 {
				if cli.Help.SubCommand == "timeformat" {
					fmt.Printf("%s\n", client.HelpTimeFormat)
				} else {
					targetCmd := client.FindCmd(cli.Help.SubCommand)
					if targetCmd == nil {
						fmt.Printf("unknown sub-command %s\n\n", cli.Help.SubCommand)
						return
					}
					fmt.Printf("%s\n", targetCmd.Usage)
				}
			} else {
				doHelp("shell")
			}
		}
		return
	}

	switch {
	default:
		kongParser.PrintUsage(false)
	case strings.HasPrefix(command, "shell"):
		shell.Shell(&cli.Shell)
	case command == "serve":
		doServe()
	}
}

type ServeCmd struct {
	ConfigDir   string
	ConfigFile  string
	Pname       string
	PidFile     string
	BootlogFile string
	Daemon      bool
	Args        []string
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

func doHelp(ctx string) error {
	showShellHelp := true
	showServeHelp := true
	switch ctx {
	case "serve":
		fmt.Println(os.Args[0] + " serve [args...]")
		showShellHelp = false
	case "shell":
		fmt.Println(os.Args[0] + " shell [flags] <sub-command> [args...]")
		showServeHelp = false
	default:
		fmt.Println(os.Args[0] + ` <command> [args...]
Commands:
  serve                   start machbase-neo server process
  shell <flags> <sub-command> [args...]  run neoshell client
  gen-config              show config template
  version                 show version
`)
	}

	if showServeHelp {
		fmt.Println(`
serve flags:
      --config-dir=<dir>  config directory path
  -c, --config=<file>     config file path
      --pname=<pname>     assign process name
      --pid=<path>        pid file path
      --bootlog=<path>    boot log file path
  -d, --daemon            run process in background, daemonize`)
	}

	if showShellHelp {
		fmt.Println(`
shell flags:
  -s, --server            server address (default tcp://127.0.0.1:5655)
  -u, --user              machbase user (default sys)

shell sub-commands:`)
		cmds := client.Commands()
		for _, cmd := range cmds {
			// fmt.Printf("  %-*s %s\n", 10, cmd.Name, cmd.Desc)
			lns := strings.Split(cmd.Usage, "\n")
			for i, l := range lns {
				if i == 0 {
					fmt.Printf("%s\n", l)
				} else {
					fmt.Printf("      %s\n", l)
				}
			}
		}

	}

	return nil
}
