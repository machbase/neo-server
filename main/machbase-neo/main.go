package main

import (
	"os"

	"github.com/alecthomas/kong"
)

func main() {
	if len(os.Args) == 1 || (len(os.Args) > 1 && os.Args[1] == "serve") {
		doServe()
	} else {
		var cli struct {
			Serve struct{} `cmd:""`
			Shell ShellCmd `cmd:""`
		}
		cmd := kong.Parse(&cli, kong.HelpOptions{NoAppSummary: false, Compact: true, FlagsLast: true})
		switch cmd.Command() {
		default:
			doServe()
		case "shell":
			doShell(&cli.Shell)
		case "shell <ARGS>":
			doShell(&cli.Shell)
		}
	}
}
