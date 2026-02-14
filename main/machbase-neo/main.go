package main

import (
	"flag"
	"os"

	"github.com/machbase/neo-server/v8/jsh/cmd"
	"github.com/machbase/neo-server/v8/mods/args"
	"github.com/machbase/neo-server/v8/shell/session"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "shell" {
		// handling "machbase-neo shell ..."
		self, err := os.Executable()
		if err != nil {
			panic(err)
		}
		flagSet := flag.NewFlagSet("shell", flag.ExitOnError)
		session.Main(flagSet, []string{self, "shell"}, os.Args[2:])
	} else if len(os.Args) > 1 && os.Args[1] == "jsh" {
		// handling "machbase-neo jsh ..."
		self, err := os.Executable()
		if err != nil {
			panic(err)
		}
		flagSet := flag.NewFlagSet("jsh", flag.ExitOnError)
		cmd.Main(flagSet, []string{self, "jsh"}, os.Args[2:])
	} else {
		// handling "machbase-neo serve ..." or others
		os.Exit(args.Main())
	}
}
