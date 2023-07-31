package fx

import (
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/fcom"
)

type Definition struct {
	Name string
	Func any
}

func GetFunction(name string) expression.Function {
	ret := GenFunctions[name]
	if ret == nil {
		ret = fcom.Functions[name]
	}
	return ret
}
