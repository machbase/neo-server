package main

import (
	"os"

	shell "github.com/machbase/neo-server/mods/shell"
)

func main() {
	os.Exit(shell.Main())
}
