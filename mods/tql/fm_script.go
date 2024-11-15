package tql

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/util"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

type bridgeName struct {
	name string
}

// bridge('name')
func (x *Node) fmBridge(name string) *bridgeName {
	return &bridgeName{name: name}
}

func (node *Node) fmScript(args ...any) (any, error) {
	if len(args) == 2 {
		text, ok := args[1].(string)
		if !ok {
			goto syntaxErr
		}
		switch name := args[0].(type) {
		case *bridgeName:
			return node.fmScriptBridge(name, text)
		case string:
			switch name {
			case "js", "javascript":
				return node.fmScriptOtto(text)
			case "tengo":
				return node.fmScriptTengo(text)
			default:
				goto syntaxErr
			}
		default:
			goto syntaxErr
		}
	} else if len(args) == 1 {
		text, ok := args[0].(string)
		if !ok {
			goto syntaxErr
		}
		return node.fmScriptTengo(text)
	}
syntaxErr:
	return nil, errors.New(`script: wrong syntax, 'SCRIPT( [bridge("name"),] script_text )`)
}

func (node *Node) fmScriptBridge(name *bridgeName, content string) (any, error) {
	br, err := bridge.GetBridge(name.name)
	if err != nil || br == nil {
		return nil, fmt.Errorf(`script: bridge '%s' not found`, name.name)
	}
	switch engine := br.(type) {
	case bridge.PythonBridge:
		var input []byte
		rec := node.Inflight()
		if rec != nil {
			b := &bytes.Buffer{}
			w := csv.NewWriter(b)
			if rec.IsArray() {
				for _, r := range rec.Array() {
					fields := util.StringFields(r.Fields(), "ns", nil, -1)
					w.Write(fields)
				}
			} else {
				fields := util.StringFields(rec.Fields(), "ns", nil, -1)
				w.Write(fields)
			}
			w.Flush()
			input = b.Bytes()
		}
		exitCode, stdout, stderr, err := engine.Invoke(node.task.ctx, []string{"-c", content}, input)
		if err != nil {
			if len(stdout) > 0 {
				node.task.Log(string(stderr))
			}
			if len(stderr) > 0 {
				node.task.LogWarn(string(stderr))
			}
			return nil, err
		}
		if len(stderr) > 0 {
			node.task.LogWarn(string(stderr))
		}
		if exitCode != 0 {
			node.task.LogWarn(fmt.Sprintf("script: exit %d", exitCode))
		}
		if len(stdout) > 0 {
			if isPng(stdout) {
				return NewImageRecord(stdout, "image/png"), nil
			} else if isJpeg(stdout) {
				return NewImageRecord(stdout, "image/jpeg"), nil
			} else {
				// yield the output from python's stdout as bytes chunk
				//fmt.Println("output", string(stdout))
				return NewBytesRecord(stdout), nil
			}
		}
	default:
		return nil, fmt.Errorf(`script: bridge '%s' is not support for SCRIPT()`, name.name)
	}
	return nil, nil
}

func isJpeg(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	matched := true
	for i, b := range []byte{0xFF, 0xD8, 0xFF} { // jpg
		if data[i] != b {
			matched = false
			break
		}
	}
	return matched
}

func isPng(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	matched := true
	for i, b := range []byte{0x89, 0x50, 0x4E, 0x47} { // png
		if data[i] != b {
			matched = false
			break
		}
	}
	return matched
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
		"math", "text", "times", "rand", "fmt", "json", "base64", "hex", "os", "enum",
	}...)
	modules.AddBuiltinModule("context", map[string]tengo.Object{
		"key":      &tengo.UserFunction{Name: "key", Value: tengof_key(node)},
		"value":    &tengo.UserFunction{Name: "value", Value: tengof_value(node)},
		"drop":     &tengo.UserFunction{Name: "drop", Value: tengof_drop(node)},
		"yieldKey": &tengo.UserFunction{Name: "yieldKey", Value: tengof_yieldKey(node)},
		"yield":    &tengo.UserFunction{Name: "yield", Value: tengof_yield(node)},
		"param":    &tengo.UserFunction{Name: "param", Value: tengof_param(node)},
		"uuid":     &tengo.UserFunction{Name: "uuid", Value: tengof_uuid(node)},
		"nil":      &tengo.UserFunction{Name: "nil", Value: tengof_nil(node)},
		"bridge":   &tengo.UserFunction{Name: "bridge", Value: tengof_bridge(node)},
		"print":    &tengo.UserFunction{Name: "print", Value: tengof_print(node)},
		"println":  &tengo.UserFunction{Name: "println", Value: tengof_print(node)},
		"printf":   &tengo.UserFunction{Name: "printf", Value: tengof_printf(node)},
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
		indexed := -1
		if len(args) != 0 {
			if i, ok := tengo.ToInt(args[0]); ok {
				indexed = i
			} else {
				return nil, tengo.ErrInvalidIndexType
			}
		}
		scriptObj, ok := node.GetValue(tengo_script_key)
		if !ok {
			return nil, nil
		}
		slet, ok := scriptObj.(*scriptlet)
		if !ok || slet.param == nil {
			return nil, nil
		}

		obj := anyToTengoObject(slet.param.value)
		// value of a record should be always a tuple (= array).
		if arr, ok := obj.(*tengo.Array); ok {
			if indexed >= 0 {
				if indexed >= len(arr.Value) {
					return nil, tengo.ErrIndexOutOfBounds
				}
				return arr.Value[indexed], nil
			} else {
				return obj, nil
			}
		} else {
			if indexed == 0 {
				return obj, nil
			} else if indexed > 0 {
				return nil, tengo.ErrIndexOutOfBounds
			}
			return &tengo.Array{Value: []tengo.Object{obj}}, nil
		}
	}
}

func tengof_param(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		paramName := ""
		defaultValue := ""
		if len(args) > 0 {
			if str, ok := tengo.ToString(args[0]); ok {
				paramName = str
			}
		}
		if len(args) > 1 {
			if str, ok := tengo.ToString(args[1]); ok {
				defaultValue = str
			}
		}
		if paramName == "" {
			return anyToTengoObject(defaultValue), nil
		}
		if v, ok := node.task.params[paramName]; ok {
			if len(v) == 1 {
				return anyToTengoObject(v[0]), nil
			} else if len(v) > 1 {
				ret := &tengo.Array{}
				for _, elm := range v {
					ret.Value = append(ret.Value, anyToTengoObject(elm))
				}
				return ret, nil
			}
		} else {
			return anyToTengoObject(defaultValue), nil
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
		v_args := make([]any, len(args))
		for i, v := range args {
			v_args[i] = tengo.ToInterface(v)
		}
		if len(v_args) == 0 {
			return nil, nil // yield no changes
		}
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				if len(v_args) == 1 { // change key only
					slet.yields = append(slet.yields, NewRecord(v_args[0], slet.param.value))
				} else { // change key and values
					slet.yields = append(slet.yields, NewRecord(v_args[0], v_args[1:]))
				}
			}
		}
		return nil, nil
	}
}

func tengof_yield(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		v_args := make([]any, len(args))
		for i, v := range args {
			v_args[i] = tengo.ToInterface(v)
		}
		if obj, ok := node.GetValue(tengo_script_key); ok {
			if slet, ok := obj.(*scriptlet); ok && slet.param != nil {
				slet.yields = append(slet.yields, NewRecord(slet.param.key, v_args))
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

func tengof_nil(_ *Node) func(args ...tengo.Object) (tengo.Object, error) {
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

const js_ctx_key = "$js_otto_ctx$"

func (node *Node) fmScriptOtto(content string) (any, error) {
	var ctx *OttoContext
	var err error

	if obj, ok := node.GetValue(js_ctx_key); ok {
		if o, ok := obj.(*OttoContext); ok {
			ctx = o
		}
	}
	if ctx == nil {
		ctx, err = newOttoContext(node, content)
		if err != nil {
			return nil, err
		}
		node.SetValue(js_ctx_key, ctx)
	}
	if inflight := node.Inflight(); inflight != nil {
		ctx.obj.Set("key", inflight.key)
		ctx.obj.Set("values", inflight.value)
	}
	_, err = ctx.Run()
	return nil, err
}

type OttoContext struct {
	vm           *otto.Otto
	sc           *otto.Script
	node         *Node
	obj          *otto.Object
	yieldCount   int64
	onceFinalize sync.Once
}

func (ctx *OttoContext) Run() (any, error) {
	done := make(chan bool)
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if ctx.node.task.shouldStop() {
					ctx.vm.Interrupt <- func() {
						ctx.node.task.LogWarn("script execution is interrupted")
					}
				}
			}
		}
	}()
	v, err := ctx.vm.Run(ctx.sc)
	done <- true
	if err != nil {
		return nil, err
	}
	return v.Export()
}

func newOttoContext(node *Node, code string) (*OttoContext, error) {
	ctx := &OttoContext{
		node: node,
		vm:   otto.New(),
	}

	// add blank lines to the beginning of the script
	// so that the compiler error message can show the correct line number
	if node.tqlLine != nil && node.tqlLine.line > 1 {
		code = strings.Repeat("\n", node.tqlLine.line-1) + code
	}
	if s, err := ctx.vm.Compile("", code); err != nil {
		return nil, err
	} else {
		ctx.sc = s
	}
	node.SetEOF(func(*Node) {
		ctx.onceFinalize.Do(func() {
			ctx.vm.Call("finalize", nil)
		})
	})
	consoleLog := func(level Level) func(call otto.FunctionCall) otto.Value {
		return func(call otto.FunctionCall) otto.Value {
			params := []any{}
			for _, v := range call.ArgumentList {
				params = append(params, v.String())
			}
			node.task._log(level, params...)
			return otto.UndefinedValue()
		}
	}
	con, _ := ctx.vm.Get("console")
	con.Object().Set("log", consoleLog(INFO))
	con.Object().Set("warn", consoleLog(WARN))
	con.Object().Set("error", consoleLog(ERROR))

	// define $
	ctx.obj, _ = ctx.vm.Object(`($ = {})`)

	// set $.payload
	var payload = otto.UndefinedValue()
	if node.task.inputReader != nil {
		if b, err := io.ReadAll(node.task.inputReader); err == nil {
			if v, err := otto.ToValue(string(b)); err == nil {
				payload = v
			}
		}
	}
	ctx.obj.Set("payload", payload)

	// set $.params[]
	var param = otto.UndefinedValue()
	if node.task.params != nil {
		param, _ = ctx.vm.ToValue(node.task.params)
	}
	ctx.obj.Set("params", param)

	var err error
	yield := func(key any, args []otto.Value) otto.Value {
		var values = make([]any, len(args))
		for i, v := range args {
			values[i], err = v.Export()
			if err != nil {
				values[i] = v.String()
			}
		}
		NewRecord(key, values).Tell(node.next)
		ctx.yieldCount++
		return otto.TrueValue()
	}
	// function $.yield(...)
	ctx.obj.Set("yield", func(call otto.FunctionCall) otto.Value {
		var v_key any
		if inflight := node.Inflight(); inflight != nil {
			v_key = inflight.key
		}
		if v_key == nil {
			v_key = ctx.yieldCount
		}
		return yield(v_key, call.ArgumentList)
	})
	// function $.yieldKey(key, ...)
	ctx.obj.Set("yieldKey", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 2 {
			return otto.FalseValue()
		}
		v_key, _ := call.ArgumentList[0].Export()
		if v_key == nil {
			v_key = ctx.yieldCount
		}
		return yield(v_key, call.ArgumentList[1:])
	})
	return ctx, nil
}
