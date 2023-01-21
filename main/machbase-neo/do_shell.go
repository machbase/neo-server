package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	shell "github.com/machbase/neo-shell"
)

type ShellCmd struct {
	Args       []string `arg:"" optional:"" name:"ARGS" passthrough:""`
	ServerAddr string   `name:"server" short:"s" default:"tcp://127.0.0.1:5655"`
	User       string   `name:"user" short:"u" default:"sys"`
}

func doShell(sqlCmd *ShellCmd) {
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

	if len(sqlCmd.Args) > 0 {
		command := strings.TrimSpace(strings.Join(sqlCmd.Args, " "))
		if len(command) > 0 {
			client.Run(command)
			return
		}
	}

	client.RunInteractive()
}
