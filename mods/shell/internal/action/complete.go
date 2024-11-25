package action

import (
	"context"

	"github.com/nyaosorg/go-box/v2"
	"github.com/nyaosorg/go-readline-ny"
)

type AutoComplete struct {
	PrefixCompleterInterface
}

var _ readline.Command = (*AutoComplete)(nil)

func (c *AutoComplete) String() string {
	return "COMPLETION_SHELL"
}

func (C *AutoComplete) Call(ctx context.Context, B *readline.Buffer) readline.Result {
	line := []rune(B.String())
	newLines, length := C.Do(line, B.Cursor)
	list := []string{}
	for _, line := range newLines {
		list = append(list, string(line))
	}
	if len(list) == 1 {
		str := list[0]
		B.InsertAndRepaint(str)
	} else if len(list) > 0 {
		prefix := ""
		if len(line) >= length {
			prefix = string(line[len(line)-length:])
		}
		fullnames := []string{}
		for i := range list {
			fullnames = append(fullnames, prefix+list[i])
		}
		B.Out.WriteByte('\n')
		selected := box.Choose(fullnames, B.Out)
		B.Out.WriteByte('\n')
		B.RepaintLastLine()
		if selected >= 0 {
			B.InsertAndRepaint(list[selected])
		}
	}
	return readline.CONTINUE
}
