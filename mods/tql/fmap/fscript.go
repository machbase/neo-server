package fmap

import (
	"errors"
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

	param  *context.Param
	yields []*context.Param
}

func script_tengo(ctx *context.Context, K any, V any, content string) (any, error) {
	var slet *scriptlet
	if obj, ok := ctx.Get(tengo_script_key); ok {
		if sl, ok := obj.(*scriptlet); ok {
			slet = sl
		}
	} else {
		slet = &scriptlet{param: &context.Param{}}
		if s, c, err := script_compile(content, ctx); err != nil {
			fmt.Println("SCRIPT", err.Error())
			return nil, err
		} else {
			slet.script = s
			slet.compiled = c
		}
		ctx.Set(tengo_script_key, slet)
	}
	if slet == nil {
		return nil, errors.New("script internal error - nil script")
	}
	slet.yields = slet.yields[:0]
	slet.param.K, slet.param.V = K, V

	slet.err = slet.compiled.RunContext(ctx)
	if slet.err != nil {
		fmt.Println("SCRIPT", slet.err.Error())
		return nil, slet.err
	}

	if len(slet.yields) == 0 {
		return slet.param, slet.err
	} else {
		return slet.yields, slet.err
	}
}

const tengo_script_key = "$tengo_script"

func script_compile(content string, ctx *context.Context) (*tengo.Script, *tengo.Compiled, error) {
	s := tengo.NewScript([]byte(content))

	modules := stdlib.GetModuleMap([]string{
		"math", "text", "times", "rand", "fmt", "json", "base64", "hex",
	}...)
	modules.AddBuiltinModule("context", map[string]tengo.Object{
		"key": &tengo.UserFunction{
			Name: "key",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				if len(args) != 0 {
					return nil, tengo.ErrWrongNumArguments
				}
				if obj, ok := ctx.Get(tengo_script_key); ok {
					if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
						return anyToTengoObject(slet.param.K), nil
					}
				}
				return nil, nil
			},
		},
		"value": &tengo.UserFunction{
			Name: "value",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				if len(args) != 0 {
					return nil, tengo.ErrWrongNumArguments
				}
				if obj, ok := ctx.Get(tengo_script_key); ok {
					if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
						return anyToTengoObject(slet.param.V), nil
					}
				}
				return nil, nil
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
				}
				if obj, ok := ctx.Get(tengo_script_key); ok {
					if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
						if len(vargs) == 1 { // change key only
							slet.yields = append(slet.yields, &context.Param{K: vargs[0], V: slet.param.V})
						} else { // change key and values
							slet.yields = append(slet.yields, &context.Param{K: vargs[0], V: vargs[1:]})
						}
					}
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
				if obj, ok := ctx.Get(tengo_script_key); ok {
					if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
						slet.yields = append(slet.yields, &context.Param{K: slet.param.K, V: vargs})
					}
				}
				return nil, nil
			},
		},
	})
	s.SetImports(modules)
	compiled, err := s.Compile()
	return s, compiled, err
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
	case []byte:
		return &tengo.Bytes{Value: v}
	case []any:
		arr := &tengo.Array{}
		for _, n := range v {
			arr.Value = append(arr.Value, anyToTengoObject(n))
		}
		return arr
	}
	return nil
}
