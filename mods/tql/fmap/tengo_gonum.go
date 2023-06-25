package fmap

import (
	"github.com/d5/tengo/v2"
)

func TengoModuleForGonumFloats() (string, map[string]tengo.Object) {
	return "gonum.floats", map[string]tengo.Object{
		"add": &tengo.UserFunction{
			Name: "add", Value: tengof_gonum_floats_add,
		},
	}
}

func tengof_gonum_floats_add(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 2 {
		return nil, tengo.ErrInvalidArgumentType{Name: "add", Expected: "two array of floats"}
	}
	//if aa, ok := args[0].(*tengo.Array); ok {
	// aa.Value
	// floats.Add()
	return nil, nil
	//}
	//return nil, tengo.ErrInvalidArgumentType{Name: "add", Expected: "two array of floats"}
}
