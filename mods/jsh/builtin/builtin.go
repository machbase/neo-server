package builtin

import "strings"

//go:generate go run generate.go

func Code(cmd string) (string, bool) {
	cmd = strings.TrimSuffix(cmd, ".js")
	src, exists := cmds[cmd]
	return src, exists
}
