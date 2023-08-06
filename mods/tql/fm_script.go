package tql

import (
	"fmt"
	"strconv"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/gofrs/uuid"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/pkg/errors"
)

func (node *Node) fmScript(args ...any) (any, error) {
	if len(args) == 2 {
		name, ok := args[0].(*bridgeName)
		if !ok {
			goto syntaxerr
		}
		text, ok := args[1].(string)
		if !ok {
			goto syntaxerr
		}
		return node.fmScriptBridge(name, text)
	} else if len(args) == 1 {
		text, ok := args[0].(string)
		if !ok {
			goto syntaxerr
		}
		return node.fmScriptTengo(text)
	}
syntaxerr:
	return nil, errors.New(`script: wrong syntax, 'SCRIPT( [bridge("name"),] script_text )`)
}

func (node *Node) fmScriptBridge(name *bridgeName, content string) (any, error) {
	br, err := bridge.GetBridge(name.name)
	if err != nil || br == nil {
		return nil, fmt.Errorf(`script: bridge '%s' not found`, name.name)
	}
	switch engine := br.(type) {
	case bridge.PythonBridge:
		exitCode, stdout, stderr, err := engine.Invoke(node.task.ctx, []string{"-c", content}, nil)
		if err != nil {
			return nil, err
		}
		if len(stderr) > 0 {
			node.task.LogWarn(string(stderr))
		}
		if exitCode != 0 {
			node.task.LogWarn(fmt.Sprintf("script: exit %d", exitCode))
		}
		text := string(stdout)
		node.tellNext(NewRecord("stdout", text))
	default:
		return nil, fmt.Errorf(`script: bridge '%s' is not support for SCRIPT()`, name.name)
	}
	return nil, nil
}

type scriptlet struct {
	script   *tengo.Script
	compiled *tengo.Compiled
	err      error

	drop   bool
	param  *Record
	yields []*Record
}

func (node *Node) fmScriptTengo(content string) (any, error) {
	var slet *scriptlet
	if obj, ok := node.GetValue(tengo_script_key); ok {
		if sl, ok := obj.(*scriptlet); ok {
			slet = sl
		}
	} else {
		slet = &scriptlet{param: &Record{}}
		if s, c, err := script_compile(content, node); err != nil {
			// script compile error
			node.task.LogError("SCRIPT", err.Error())
			fallbackCode := fmt.Sprintf(`import("context").yield(%s)`, strconv.Quote(err.Error()))
			s, c, _ = script_compile(fallbackCode, node)
			slet.script = s
			slet.compiled = c
		} else {
			slet.script = s
			slet.compiled = c
		}
		node.SetValue(tengo_script_key, slet)
	}
	if slet == nil {
		return nil, errors.New("script internal error - nil script")
	}
	slet.drop = false
	slet.yields = slet.yields[:0]
	if inflight := node.Inflight(); inflight != nil {
		slet.param.key, slet.param.value = inflight.key, inflight.value
	}

	slet.err = slet.compiled.RunContext(node.task.ctx)
	if slet.err != nil {
		node.task.LogError("SCRIPT", slet.err.Error())
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

func script_compile(content string, node *Node) (*tengo.Script, *tengo.Compiled, error) {
	modules := stdlib.GetModuleMap([]string{
		"math", "text", "times", "rand", "fmt", "json", "base64", "hex",
	}...)
	modules.AddBuiltinModule("context", map[string]tengo.Object{
		"key": &tengo.UserFunction{
			Name: "key", Value: tengof_key(node),
		},
		"value": &tengo.UserFunction{
			Name: "value", Value: tengof_value(node),
		},
		"drop": &tengo.UserFunction{
			Name: "drop", Value: tengof_drop(node),
		},
		"yieldKey": &tengo.UserFunction{
			Name: "yieldKey", Value: tengof_yieldKey(node),
		},
		"yield": &tengo.UserFunction{
			Name: "yield", Value: tengof_yield(node),
		},
		"uuid": &tengo.UserFunction{
			Name: "uuid", Value: tengof_uuid(node),
		},
		"nil": &tengo.UserFunction{
			Name: "nil", Value: tengof_nil(node),
		},
		"bridge": &tengo.UserFunction{
			Name: "bridge", Value: tengof_bridge(node),
		},
	})
	s := tengo.NewScript([]byte(content))
	s.SetImports(modules)
	compiled, err := s.Compile()
	return s, compiled, err
}

func tengof_key(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) != 0 {
			return nil, tengo.ErrWrongNumArguments
		}
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				return anyToTengoObject(slet.param.key), nil
			}
		}
		return nil, nil
	}
}

func tengof_value(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) != 0 {
			return nil, tengo.ErrWrongNumArguments
		}
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				return anyToTengoObject(slet.param.value), nil
			}
		}
		return nil, nil
	}
}

func tengof_drop(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				slet.drop = true
			}
		}
		return nil, nil
	}
}

func tengof_yieldKey(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		vargs := make([]any, len(args))
		for i, v := range args {
			vargs[i] = tengo.ToInterface(v)
		}
		if len(vargs) == 0 {
			return nil, nil // yield no changes
		}
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				if len(vargs) == 1 { // change key only
					slet.yields = append(slet.yields, NewRecord(vargs[0], slet.param.value))
				} else { // change key and values
					slet.yields = append(slet.yields, NewRecord(vargs[0], vargs[1:]))
				}
			}
		}
		return nil, nil
	}
}

func tengof_yield(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		vargs := make([]any, len(args))
		for i, v := range args {
			vargs[i] = tengo.ToInterface(v)
		}
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				slet.yields = append(slet.yields, NewRecord(slet.param.key, vargs))
			}
		}
		return nil, nil
	}
}

func tengof_uuid(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		ver := 4
		if len(args) == 1 {
			if v, ok := args[0].(*tengo.Int); ok {
				ver = int(v.Value)
			}
		}
		var gen uuid.Generator
		if obj, ok := node.GetValue(tengo_uuid_key); ok {
			if id, ok := obj.(uuid.Generator); ok {
				gen = id
			}
		}

		if gen == nil {
			gen = uuid.NewGen()
			node.SetValue(tengo_uuid_key, gen)
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

func tengof_nil(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
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
