package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	shell "github.com/machbase/neo-shell"
)

type SqlCmd struct {
	SqlText    []string `arg:"" optional:"" name:"SQL" passthrough:""`
	ServerAddr string   `name:"server" short:"s"`
	User       string   `name:"user" short:"u"`
}

func doSql(sqlCmd *SqlCmd) {
	clientConf := &shell.Config{
		ServerAddr:   sqlCmd.ServerAddr,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		VimMode:      false,
		QueryTimeout: 30 * time.Second,
	}
	client, err := shell.New(clientConf)
	if err != nil {
		fmt.Fprintln(os.Stdout, "ERR", err.Error())
		return
	}
	defer client.Close()

	if len(sqlCmd.SqlText) > 0 {
		sqlText := strings.TrimSpace(strings.Join(sqlCmd.SqlText, " "))
		if len(sqlText) > 0 {
			client.RunSql(sqlText)
			return
		}
	}

	client.RunInteractive()
}
