package internal

import (
	_ "embed"
	"strings"

	"github.com/dop251/goja"
)

// Run executes a built-in internal command in the provided Goja VM.
// It returns the result value and a boolean indicating whether the command was found.
// If the command is not found, the boolean will be false.
func Run(vm *goja.Runtime, cmd string, args ...string) (goja.Value, bool) {
	var returnValue goja.Value
	var script string
	switch cmd {
	case "cd":
		script = strings.TrimSpace(cdJS) + "(" + formatArgs(args) + ");"
	default:
		return goja.Undefined(), false
	}

	if v, err := vm.RunString(script); err != nil {
		returnValue = vm.NewGoError(err)
	} else {
		returnValue = v
	}

	return returnValue, true
}

func formatArgs(args []string) string {
	parts := []string{}
	for _, arg := range args {
		parts = append(parts, `"`+arg+`"`)
	}
	return joinArgs(parts)
}

func joinArgs(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ", "
		}
		result += part
	}
	return result
}

//go:embed cd.js
var cdJS string
