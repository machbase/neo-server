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
			Sql SqlCmd `cmd:""`
		}
		cmd := kong.Parse(&cli)
		switch cmd.Command() {
		default:
			doServe()
		case "sql":
			doSql(&cli.Sql)
		case "sql <SQL>":
			doSql(&cli.Sql)
		}
	}
}
