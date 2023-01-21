package client

import (
	"strings"

	"github.com/chzyer/readline"
)

func (cli *client) pcSet() *readline.PrefixCompleter {
	return readline.PcItem("set",
		readline.PcItem("key",
			readline.PcItem("vi"),
			readline.PcItem("emacs"),
		),
	)
}

func (cli *client) doSet(args ...string) {
	if len(args) <= 2 || strings.ToLower(args[0]) != "set" {
		return
	}
	switch strings.ToLower(args[1]) {
	case "key":
		if strings.ToLower(args[2]) == "vi" {
			cli.conf.VimMode = true
		} else {
			cli.conf.VimMode = false
		}
	}
}
