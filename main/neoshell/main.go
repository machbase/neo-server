package main

import (
	"os"

	shell "github.com/machbase/neo-server/v8/mods/shell"
)

func main() {
	os.Exit(shell.Main())
}
