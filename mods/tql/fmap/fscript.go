package fmap

import (
	"fmt"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/conv"
)

// SCRIPT(CTX, K, V, {block})
func mapf_SCRIPT(args ...any) (any, error) {
	if len(args) != 4 {
		return nil, conv.ErrInvalidNumOfArgs("SCRIPT", 4, len(args))
	}
	ctx, ok := args[0].(*context.Context)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("SCRIPT", 0, "context", args[0])
	}
	k := args[1]
	v := args[2]

	content, err := conv.String(args, 3, "SCRIPT", "block")
	if err != nil {
		return nil, err
	}

	return script_tengo(ctx, k, v, content)
}

type scriptlet struct {
	script   *tengo.Script
	compiled *tengo.Compiled
	err      error
}

func anyToTengoObject(av any) tengo.Object {
	switch v := av.(type) {
	case float64:
		return &tengo.Float{Value: v}
	case *float64:
		return &tengo.Float{Value: *v}
	case string:
		return &tengo.String{Value: v}
	case *string:
		return &tengo.String{Value: *v}
	case time.Time:
		return &tengo.Time{Value: v}
	case *time.Time:
		return &tengo.Time{Value: *v}
	case []any:
		arr := &tengo.Array{}
		for _, n := range v {
			arr.Value = append(arr.Value, anyToTengoObject(n))
		}
		return arr
	}
	return nil
}

func script_tengo(ctx *context.Context, K any, V any, content string) (any, error) {
	ret := &context.Param{K: K, V: V}

	slet := &scriptlet{}
	slet.script = tengo.NewScript([]byte(content))

	modules := stdlib.GetModuleMap(stdlib.AllModuleNames()...)
	modules.AddBuiltinModule("context", map[string]tengo.Object{
		"key": &tengo.UserFunction{
			Name: "key",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				if len(args) != 0 {
					return nil, tengo.ErrWrongNumArguments
				}
				return anyToTengoObject(K), nil
			},
		},
		"value": &tengo.UserFunction{
			Name: "value",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				if len(args) != 0 {
					return nil, tengo.ErrWrongNumArguments
				}
				return anyToTengoObject(V), nil
			},
		},
		"yieldKey": &tengo.UserFunction{
			Name: "yieldKey",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				vargs := make([]any, len(args))
				for i, v := range args {
					vargs[i] = tengo.ToInterface(v)
				}
				if len(vargs) == 0 {
					return nil, nil // yield no changes
				} else if len(vargs) == 1 {
					ret.K = []any{vargs[0]} // change key only
				} else {
					ret.K = vargs[0] // change key and values
					ret.V = vargs[1:]
				}
				return nil, nil
			},
		},
		"yield": &tengo.UserFunction{
			Name: "yield",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				vargs := make([]any, len(args))
				for i, v := range args {
					vargs[i] = tengo.ToInterface(v)
				}
				ret.V = vargs
				return nil, nil
			},
		},
	})
	slet.script.SetImports(modules)
	slet.compiled, slet.err = slet.script.Compile()
	if slet.err != nil {
		fmt.Println("SCRIPT", slet.err.Error())
		return nil, slet.err
	}
	slet.err = slet.compiled.RunContext(ctx)
	if slet.err != nil {
		fmt.Println("SCRIPT", slet.err.Error())
		return nil, slet.err
	}

	return ret, slet.err
}
