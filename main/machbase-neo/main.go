package main

import (
	"flag"
	"os"

	"github.com/machbase/neo-server/v8/mods/args"
	"github.com/machbase/neo-server/v8/shell/entry"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "shell" {
		// handling "machbase-neo shell ..."
		self, err := os.Executable()
		if err != nil {
			panic(err)
		}
		flagSet := flag.NewFlagSet("shell", flag.ExitOnError)
		entry.Main(flagSet, []string{self, "shell"}, os.Args[2:])
	} else {
		// handling "machbase-neo serve ..." or others
		os.Exit(args.Main())
	}
}
