package main

import (
	"flag"
	"os"

	"github.com/machbase/neo-server/v8/jsh/cmd"
)

func main() {
	self, err := os.Executable()
	if err != nil {
		panic(err)
	}
	os.Exit(cmd.Main(flag.CommandLine, []string{self}, os.Args[1:]))
}
