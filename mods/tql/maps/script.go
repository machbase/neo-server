package maps

import (
	"fmt"
	"strconv"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/gofrs/uuid"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/pkg/errors"
)

type scriptlet struct {
	script   *tengo.Script
	compiled *tengo.Compiled
	err      error

	drop   bool
	param  *context.Param
	yields []*context.Param
}

func ScriptTengo(ctx *context.Context, K any, V any, content string) (any, error) {
	var slet *scriptlet
	if obj, ok := ctx.Get(tengo_script_key); ok {
		if sl, ok := obj.(*scriptlet); ok {
			slet = sl
		}
	} else {
		slet = &scriptlet{param: &context.Param{}}
		if s, c, err := script_compile(content, ctx); err != nil {
			// script compile error
			fmt.Println("SCRIPT", err.Error())
			fallbackCode := fmt.Sprintf(`import("context").yield(%s)`, strconv.Quote(err.Error()))
			s, c, _ = script_compile(fallbackCode, ctx)
			slet.script = s
			slet.compiled = c
		} else {
			slet.script = s
			slet.compiled = c
		}
		ctx.Set(tengo_script_key, slet)
	}
	if slet == nil {
		return nil, errors.New("script internal error - nil script")
	}
	slet.drop = false
	slet.yields = slet.yields[:0]
	slet.param.K, slet.param.V = K, V

	slet.err = slet.compiled.RunContext(ctx)
	if slet.err != nil {
		fmt.Println("SCRIPT", slet.err.Error())
		return nil, slet.err
	}

	if slet.drop {
		return nil, slet.err
	} else if len(slet.yields) == 0 {
		return slet.param, slet.err
	} else {
		return slet.yields, slet.err
	}
}

const tengo_script_key = "$tengo_script"
const tengo_uuid_key = "$tengo_uuid"

func script_compile(content string, ctx *context.Context) (*tengo.Script, *tengo.Compiled, error) {
	modules := stdlib.GetModuleMap([]string{
		"math", "text", "times", "rand", "fmt", "json", "base64", "hex",
	}...)
	modules.AddBuiltinModule("context", map[string]tengo.Object{
		"key": &tengo.UserFunction{
			Name: "key", Value: tengof_key(ctx),
		},
		"value": &tengo.UserFunction{
			Name: "value", Value: tengof_value(ctx),
		},
		"drop": &tengo.UserFunction{
			Name: "drop", Value: tengof_drop(ctx),
		},
		"yieldKey": &tengo.UserFunction{
			Name: "yieldKey", Value: tengof_yieldKey(ctx),
		},
		"yield": &tengo.UserFunction{
			Name: "yield", Value: tengof_yield(ctx),
		},
		"uuid": &tengo.UserFunction{
			Name: "uuid", Value: tengof_uuid(ctx),
		},
		"nil": &tengo.UserFunction{
			Name: "nil", Value: tengof_nil(ctx),
		},
		"bridge": &tengo.UserFunction{
			Name: "bridge", Value: tengof_bridge(ctx),
		},
	})
	s := tengo.NewScript([]byte(content))
	s.SetImports(modules)
	compiled, err := s.Compile()
	return s, compiled, err
}

func tengof_key(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) != 0 {
			return nil, tengo.ErrWrongNumArguments
		}
		if obj, ok := ctx.Get(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				return anyToTengoObject(slet.param.K), nil
			}
		}
		return nil, nil
	}
}

func tengof_value(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) != 0 {
			return nil, tengo.ErrWrongNumArguments
		}
		if obj, ok := ctx.Get(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				return anyToTengoObject(slet.param.V), nil
			}
		}
		return nil, nil
	}
}

func tengof_drop(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if obj, ok := ctx.Get(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				slet.drop = true
			}
		}
		return nil, nil
	}
}

func tengof_yieldKey(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
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
	}
}

func tengof_yield(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
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
	}
}

func tengof_uuid(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		ver := 4
		if len(args) == 1 {
			if v, ok := args[0].(*tengo.Int); ok {
				ver = int(v.Value)
			}
		}
		var gen uuid.Generator
		if obj, ok := ctx.Get(tengo_uuid_key); ok {
			if id, ok := obj.(uuid.Generator); ok {
				gen = id
			}
		}

		if gen == nil {
			gen = uuid.NewGen()
			ctx.Set(tengo_uuid_key, gen)
		}

		var uid uuid.UUID
		var err error
		switch ver {
		case 1:
			uid, err = gen.NewV1()
		case 4:
			uid, err = gen.NewV4()
		case 6:
			uid, err = gen.NewV6()
		case 7:
			uid, err = gen.NewV7()
		default:
			return nil, tengo.ErrInvalidArgumentType{Name: "uuid version", Expected: "1,4,6,7", Found: fmt.Sprintf("%d", ver)}
		}
		if err != nil {
			return nil, err
		}
		return anyToTengoObject(uid.String()), nil
	}
}

func tengof_nil(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		return tengo.UndefinedValue, nil
	}
}

func tengoObjectToString(obj tengo.Object) (string, error) {
	if o, ok := obj.(*tengo.String); ok {
		return o.Value, nil
	} else {
		return "", errors.New("not a string")
	}
}

func tengoSliceToAnySlice(arr []tengo.Object) []any {
	ret := make([]any, len(arr))
	for i, o := range arr {
		ret[i] = tengoObjectToAny(o)
	}
	return ret
}

//lint:ignore U1000 keep it for the future uses
func anySliceToTengoSlice(arr []any) []tengo.Object {
	ret := make([]tengo.Object, len(arr))
	for i, o := range arr {
		ret[i] = anyToTengoObject(o)
	}
	return ret
}

func tengoObjectToAny(obj tengo.Object) any {
	switch o := obj.(type) {
	case *tengo.String:
		return o.Value
	case *tengo.Float:
		return o.Value
	case *tengo.Int:
		return o.Value
	case *tengo.Bool:
		return !o.IsFalsy()
	default:
		return obj.String()
	}
}

func anyToTengoObject(av any) tengo.Object {
	switch v := av.(type) {
	case int:
		return &tengo.Int{Value: int64(v)}
	case *int:
		return &tengo.Int{Value: int64(*v)}
	case int16:
		return &tengo.Int{Value: int64(v)}
	case *int16:
		return &tengo.Int{Value: int64(*v)}
	case int32:
		return &tengo.Int{Value: int64(v)}
	case *int32:
		return &tengo.Int{Value: int64(*v)}
	case int64:
		return &tengo.Int{Value: v}
	case *int64:
		return &tengo.Int{Value: *v}
	case float64:
		return &tengo.Float{Value: v}
	case *float64:
		return &tengo.Float{Value: *v}
	case bool:
		if v {
			return tengo.TrueValue
		} else {
			return tengo.FalseValue
		}
	case *bool:
		if *v {
			return tengo.TrueValue
		} else {
			return tengo.FalseValue
		}
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
