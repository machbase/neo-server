package fx

import (
	"github.com/machbase/neo-server/mods/expression"
)

type Definition struct {
	Name string
	Func any
}

func GetFunction(name string) expression.Function {
	return GenFunctions[name]
}
