package fmap

import (
	"errors"
	"fmt"

	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/conv"

	scriptLoader "github.com/machbase/neo-server/mods/script"
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

	return script_tengo(ctx, k, v, []byte(content))
}

func script_tengo(ctx *context.Context, K any, V any, content []byte) (any, error) {
	loader := scriptLoader.NewLoader()

	engine, err := loader.Parse(content)
	if err != nil {
		fmt.Println("ERR Tengo1", err.Error())
		return nil, err
	}

	engine.SetVar("CTX", ctx)
	engine.SetVar("K", K)
	engine.SetVar("V", V)

	ret := &context.Param{K: K, V: V}
	engine.SetFunc("yield", func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, errors.New("missing key-value argument")
		} else if len(args) == 1 {
			switch v := args[0].(type) {
			case []any:
				ret.K = v[0]
				ret.V = v[1:]
			default:
				fmt.Printf("-->%T len:%+v\n", args[0], args[0])
			}
		} else {
			ret.K = args[0]
			ret.V = args[1:]
		}
		return nil, nil
	})

	if err = engine.Run(); err != nil {
		fmt.Println("ERR Tengo run", err.Error())
		return nil, err
	}

	return ret, nil
}
