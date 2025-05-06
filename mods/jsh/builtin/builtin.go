package builtin

import (
	"context"
	"io"
	"strings"
)

//go:generate go run generate.go

func Code(cmd string) (string, bool) {
	cmd = strings.TrimSuffix(cmd, ".js")
	src, exists := cmds[cmd]
	return src, exists
}

type JshContext interface {
	context.Context
	Signal() <-chan string
	AddCleanup(func(io.Writer)) int64
	RemoveCleanup(int64)
}
