package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/machbase/neo-server/mods/client"
)

type SqlCmd struct {
	SqlText    []string `arg:"" optional:"" name:"SQL" passthrough:""`
	ServerAddr string   `name:"server" short:"s"`
	User       string   `name:"user" short:"u"`
}

func doSql(sqlCmd *SqlCmd) {
	clientConf := &client.Config{
		ServerAddr: sqlCmd.ServerAddr,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
	}
	client, err := client.New(clientConf)
	if err != nil {
		fmt.Fprintf(os.Stdout, "ERR %s\r\n", err.Error())
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
