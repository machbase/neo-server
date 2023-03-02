package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	shell "github.com/machbase/neo-shell"
	"github.com/machbase/neo-shell/client"
	"github.com/machbase/neo-shell/util"
)

func doHelp(command string, subcommand string) error {
	showShellHelp := true
	showServeHelp := true

	switch command {
	case "serve":
		fmt.Println(os.Args[0] + " serve [args...]")
		showShellHelp = false
	case "shell":
		fmt.Println(os.Args[0] + " shell [flags] <sub-command> [args...]")
		showServeHelp = false
	case "timeformat":
		fmt.Println("  timeformats:")
		fmt.Printf("%s\n", util.HelpTimeformats())
		return nil
	case "tz":
		fmt.Println("  timezones:")
		fmt.Printf("%s\n", util.HelpTimeZones())
		return nil
	default:
		fmt.Println(filepath.Base(os.Args[0]) + helpRootText)
	}

	if showServeHelp {
		fmt.Println(helpServeText)
	}

	if showShellHelp {
		serverAddr := shell.DefaultServerAddress
		if shellPref, err := client.LoadPref(); err == nil {
			serverAddr = shellPref.Server().Value()
		}

		if subcommand != "" {
			fmt.Printf(helpShellText, serverAddr)
			if subcommand == "timeformat" {
				fmt.Println("  timeformats:")
				fmt.Printf("%s\n", util.HelpTimeformats())
			} else if subcommand == "tz" {
				fmt.Println("  timezones:")
				fmt.Printf("%s\n", util.HelpTimeZones())
			} else {
				targetCmd := client.FindCmd(subcommand)
				if targetCmd == nil {
					fmt.Printf("unknown sub-command %s\n\n", subcommand)
					return nil
				}
				fmt.Printf("%s shell %s\n", filepath.Base(os.Args[0]), targetCmd.Usage)
			}
		} else {
			fmt.Printf("\nshell "+helpShellText, serverAddr)
			fmt.Println("shell sub-commands:")
			cmds := client.Commands()
			for _, cmd := range cmds {
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
	}
	return nil
}

const helpRootText = ` <command> [args...]

Commands:
  serve <falgs>               start machbase-neo server process
  shell <flags> <sub-command> run neoshell client
  gen-config                  show config template
  version                     show version`

const helpServeText = `
serve flags:
      --config-dir=<dir>  config directory path
  -c, --config=<file>     config file path
      --pname=<pname>     assign process name
      --pid=<path>        pid file path
      --bootlog=<path>    boot log file path
  -d, --daemon            run process in background, daemonize`

const helpShellText = `flags:
  -s, --server=<addr>     server address (default %s)

`
