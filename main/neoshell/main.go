package main

import (
	"github.com/alecthomas/kong"
	"github.com/machbase/neo-server/mods/shell"
)

func main() {
	var cli shell.ShellCmd
	_ = kong.Parse(&cli,
		kong.HelpOptions{NoAppSummary: false, Compact: true, FlagsLast: true},
		kong.UsageOnError(),
		kong.Help(shell.HelpKong),
	)
	shell.Shell(&cli)
}
