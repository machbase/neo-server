package tql

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/paulmach/orb/geojson"
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
	var js_is_es5 bool = true
	if flag, ok := node.pragma["es5"]; ok {
		if b, err := strconv.ParseBool(flag); err == nil {
			js_is_es5 = b
		}
	}

	if len(args) == 1 {
		text, ok := args[0].(string)
		if !ok {
			goto syntaxErr
		}
		return node.fmScriptTengo(text)
	} else if len(args) >= 2 {
		switch name := args[0].(type) {
		case *bridgeName:
			if text, ok := args[1].(string); !ok {
				goto syntaxErr
			} else {
				return node.fmScriptBridge(name, text)
			}
		case string:
			switch name {
			case "js", "javascript":
				initCode, mainCode := "", ""
				if len(args) == 2 { // SCRIPT("js", "main")
					if str, ok := args[1].(string); !ok {
						goto syntaxErr
					} else {
						mainCode = str
					}
				} else if len(args) == 3 { // SCRIPT("js", "init", "main")
					if str, ok := args[1].(string); !ok {
						goto syntaxErr
					} else {
						initCode = str
					}
					if str, ok := args[2].(string); !ok {
						goto syntaxErr
					} else {
						mainCode = str
					}
				} else {
					goto syntaxErr
				}
				if js_is_es5 {
					return node.fmScriptOtto(initCode, mainCode)
				} else {
					return node.fmScriptGoja(initCode, mainCode)
				}
			case "tengo":
				node.task.LogWarn("SCRIPT(\"tengo\") deprecated, use SCRIPT(\"js\") instead.")
				if text, ok := args[1].(string); !ok {
					goto syntaxErr
				} else {
					return node.fmScriptTengo(text)
				}
			default:
				goto syntaxErr
			}
		default:
			goto syntaxErr
		}
	}
syntaxErr:
	return nil, errors.New(`script: wrong syntax, 'SCRIPT( [script_name,] [init_script], script_text )`)
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

type ScriptOttoResultOption struct {
	Key struct {
		Name string `json:"name,omitempty"`
		Type string `json:"type,omitempty"`
	} `json:"key,omitempty"`
	Columns []string `json:"columns"`
	Types   []string `json:"types,omitempty"`
}

func (so *ScriptOttoResultOption) Load(obj otto.Value) error {
	key, _ := obj.Object().Get("key")
	if key.IsDefined() {
		if name, _ := key.Object().Get("name"); name.IsDefined() {
			so.Key.Name, _ = name.ToString()
		}

		if typ, _ := key.Object().Get("type"); typ.IsDefined() {
			so.Key.Type, _ = typ.ToString()
		}
	}
	cols, _ := obj.Object().Get("columns")
	if cols.IsDefined() {
		colsArr, _ := cols.Export()
		if colsArr != nil {
			so.Columns = append(so.Columns, colsArr.([]string)...)
		}
	}
	types, _ := obj.Object().Get("types")
	if types.IsDefined() {
		typesArr, _ := types.Export()
		if typesArr != nil {
			so.Types = append(so.Types, typesArr.([]string)...)
		}
	}
	return nil
}

func (so *ScriptOttoResultOption) ResultColumns() []*api.Column {
	if len(so.Columns) == 0 {
		return nil
	}
	cols := make([]*api.Column, len(so.Columns)+1)
	cols[0] = &api.Column{Name: "key", DataType: api.DataTypeAny}
	if so.Key.Name != "" {
		cols[0].Name = so.Key.Name
	}
	if so.Key.Type != "" {
		cols[0].DataType = api.ParseDataType(so.Key.Type)
	}
	for i, name := range so.Columns {
		cols[i+1] = &api.Column{Name: name, DataType: api.DataTypeAny}
		if len(so.Types) > i {
			cols[i+1].DataType = api.ParseDataType(so.Types[i])
		}
	}
	return cols
}

func (node *Node) fmScriptOtto(initCode string, mainCode string) (any, error) {
	var ctx *OttoContext
	var err error

	defer func() {
		if r := recover(); r != nil {
			code := "{" + strings.TrimSpace(strings.TrimPrefix(initCode, "//")) + "}\n" +
				"{" + strings.TrimSpace(strings.TrimPrefix(mainCode, "//")) + "}"
			if r == errOttoInterrupt {
				node.task.LogWarnf("script is interrupted; %s", code)
			} else {
				node.task.LogWarnf("script panic; %v\n%s", r, code)
			}
		}
	}()

	if obj, ok := node.GetValue(js_ctx_key); ok {
		if o, ok := obj.(*OttoContext); ok {
			ctx = o
		}
	}
	if ctx == nil {
		// if script is interrupted whilethe init stage of the newOtterContext()
		// ctx might be nil
		ctx, err = newOttoContext(node, initCode, mainCode)
		if err != nil {
			return nil, err
		}
		node.SetValue(js_ctx_key, ctx)
	}
	if inflight := node.Inflight(); inflight != nil {
		ctx.obj.Set("key", inflight.key)
		if arr, ok := inflight.value.([]any); ok {
			ctx.obj.Set("values", arr)
		} else {
			ctx.obj.Set("values", []any{inflight.value})
		}
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
	didSetResult bool

	watchdogCleanup chan struct{}
	waitCleanup     sync.WaitGroup
}

func (ctx *OttoContext) Run() (any, error) {
	v, err := ctx.vm.Run(ctx.sc)
	if err != nil {
		return nil, err
	}
	return v.Export()
}

func closeOttoContext(ctx *OttoContext) {
	close(ctx.watchdogCleanup)
	ctx.waitCleanup.Wait()
	close(ctx.vm.Interrupt)
}

var errOttoInterrupt = errors.New("script execution is interrupted")

func newOttoContext(node *Node, initCode string, mainCode string) (*OttoContext, error) {
	ctx := &OttoContext{
		node: node,
		vm:   otto.New(),
	}

	// add blank lines to the beginning of the script
	// so that the compiler error message can show the correct line number
	if node.tqlLine != nil && node.tqlLine.line > 1 {
		initCodeLine := strings.Count(initCode, "\n")
		mainCode = strings.Repeat("\n", initCodeLine+node.tqlLine.line-1) + mainCode
	}
	if s, err := ctx.vm.Compile("", mainCode); err != nil {
		return nil, err
	} else {
		ctx.sc = s
	}

	node.SetEOF(func(*Node) {
		defer closeOttoContext(ctx)
		// set $.result columns if no records are yielded
		if !ctx.didSetResult {
			ctx.doResult()
		}
		ctx.onceFinalize.Do(func() {
			// intentionally ignore the panic from finalize stage.
			// it will raised to the task level.
			// do not use "recover()" here.
			// The related test case : TestScriptInterrupt()/js-timeout-finalize
			f, _ := ctx.vm.Get("finalize")
			if f.IsDefined() && f.IsFunction() {
				ctx.vm.Call("finalize", nil)
			}
		})
	})
	con, _ := ctx.vm.Get("console")
	con.Object().Set("log", ctx.consoleLog(INFO))
	con.Object().Set("warn", ctx.consoleLog(WARN))
	con.Object().Set("error", ctx.consoleLog(ERROR))

	// define $
	ctx.obj, _ = ctx.vm.Object(`($ = {})`)

	// set $.payload
	var payload = otto.UndefinedValue()
	if node.task.nodes[0] == node && node.task.inputReader != nil {
		// $.payload is defined, only when the SCRIPT is the SRC node.
		// If the SCRIPT is not the SRC node, the payload has been using by the previous node.
		// and if the "inputReader" was consumed here, the actual SRC node will see the EOF.
		if b, err := io.ReadAll(node.task.inputReader); err == nil {
			if v, err := otto.ToValue(string(b)); err == nil && len(b) > 0 {
				payload = v
			}
		}
	}
	ctx.obj.Set("payload", payload)

	// set $.params[]
	var param = otto.UndefinedValue()
	if node.task.params != nil {
		values := map[string]any{}
		for k, v := range node.task.params {
			if len(v) == 1 {
				values[k] = v[0]
			} else {
				values[k] = v
			}
		}
		param, _ = ctx.vm.ToValue(values)
	}
	ctx.obj.Set("params", param)

	// function $.yield(...)
	ctx.obj.Set("yield", ottoFuncYield(ctx))
	// function $.yieldKey(key, ...)
	ctx.obj.Set("yieldKey", ottoFuncYieldKey(ctx))
	// function $.yieldArray(array)
	ctx.obj.Set("yieldArray", ottoFuncYieldArray(ctx))
	// $.db()
	ctx.obj.Set("db", ottoFuncDB(ctx))
	// $.request()
	ctx.obj.Set("request", ottoFuncRequest(ctx))
	// $.geojson()
	ctx.obj.Set("geojson", ottoFuncGeoJSON(ctx))
	// $.system()
	ctx.obj.Set("system", ottoFuncSystem(ctx))
	ctx.vm.Interrupt = make(chan func(), 3) // 1 is for non-blocking
	ctx.watchdogCleanup = make(chan struct{})

	ctx.waitCleanup.Add(1)
	go func() {
		defer ctx.waitCleanup.Done()
		for {
			select {
			case <-time.After(1 * time.Second):
				if ctx.node.task.shouldStop() {
					ctx.vm.Interrupt <- func() {
						panic(errOttoInterrupt)
					}
					return
				}
			case <-ctx.watchdogCleanup:
				return
			}
		}
	}()

	// init code
	if initCode != "" {
		if node.tqlLine != nil && node.tqlLine.line > 1 {
			initCode = strings.Repeat("\n", node.tqlLine.line-1) + initCode
		}
		_, err := ctx.vm.Run(initCode)
		if err != nil {
			return nil, fmt.Errorf("SCRIPT init, %s", err.Error())
		}
	}

	return ctx, nil
}

func (ctx *OttoContext) doResult() error {
	resultObj, err := ctx.obj.Get("result")
	if err != nil {
		return fmt.Errorf("SCRIPT result, %s", err.Error())
	}
	if resultObj.IsDefined() {
		var opts ScriptOttoResultOption
		if err := opts.Load(resultObj); err != nil {
			msg := strings.TrimPrefix(err.Error(), "json: ")
			return fmt.Errorf("line %d, SCRIPT option, %s", ctx.node.tqlLine.line, msg)
		}
		if cols := opts.ResultColumns(); cols != nil {
			ctx.node.task.SetResultColumns(cols)
		}
		ctx.didSetResult = true
	}
	return nil
}

func (ctx *OttoContext) consoleLog(level Level) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		params := []any{}
		for _, v := range call.ArgumentList {
			params = append(params, v.String())
		}
		ctx.node.task._log(level, params...)
		return otto.UndefinedValue()
	}
}

func (ctx *OttoContext) yield(key any, args []otto.Value) otto.Value {
	var values []any
	var err error
	if len(args) == 1 && args[0].IsObject() && args[0].Object().Class() == "Array" {
		arr, _ := args[0].Object().Value().Export()
		if v, ok := arr.([][]any); ok {
			values = make([]any, len(v))
			for i, v := range v {
				values[i] = v
			}
		} else if v, ok := arr.([]any); ok {
			values = v
		} else {
			values = []any{arr}
		}
	} else {
		values = make([]any, len(args))
		for i, v := range args {
			values[i], err = v.Export()
			if err != nil {
				values[i] = v.String()
			}
		}
	}
	// set $.result columns before the first yield
	if ctx.yieldCount == 0 && !ctx.didSetResult {
		ctx.doResult()
	}
	NewRecord(key, values).Tell(ctx.node.next)
	ctx.yieldCount++
	return otto.TrueValue()
}

func ottoFuncYield(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		var v_key any
		if inflight := ctx.node.Inflight(); inflight != nil {
			v_key = inflight.key
		}
		if v_key == nil {
			v_key = ctx.yieldCount
		}
		return ctx.yield(v_key, call.ArgumentList)
	}
}

func ottoFuncYieldKey(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 2 {
			return otto.FalseValue()
		}
		v_key, _ := call.ArgumentList[0].Export()
		if v_key == nil {
			v_key = ctx.yieldCount
		}
		return ctx.yield(v_key, call.ArgumentList[1:])
	}
}

func ottoFuncYieldArray(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		var v_key any
		if inflight := ctx.node.Inflight(); inflight != nil {
			v_key = inflight.key
		}
		if v_key == nil {
			v_key = ctx.yieldCount
		}
		if len(call.ArgumentList) != 1 || !slices.Contains([]string{"Array", "GoSlice"}, call.ArgumentList[0].Class()) {
			return ctx.vm.MakeCustomError("SCRIPT", "argument should be an array")
		}
		obj, _ := call.ArgumentList[0].Export()
		args := []otto.Value{}
		switch arr := obj.(type) {
		case []any:
			for _, v := range arr {
				ov, _ := ctx.vm.ToValue(v)
				args = append(args, ov)
			}
		case []bool:
			for _, v := range arr {
				ov, _ := ctx.vm.ToValue(v)
				args = append(args, ov)
			}
		case []string:
			for _, v := range arr {
				ov, _ := ctx.vm.ToValue(v)
				args = append(args, ov)
			}
		case []int64:
			for _, v := range arr {
				ov, _ := ctx.vm.ToValue(v)
				args = append(args, ov)
			}
		case []float64:
			for _, v := range arr {
				ov, _ := ctx.vm.ToValue(v)
				args = append(args, ov)
			}
		}
		return ctx.yield(v_key, args)
	}
}

func ottoFuncRequest(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	// $.request(url, option).do(function(response) {...})
	return func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) == 0 {
			return ctx.vm.MakeCustomError("HTTPError", "missing a URL")
		}
		option := struct {
			Url     string            `json:"url"`
			Method  string            `json:"method"` // GET, POST, PUT, DELETE
			Body    string            `json:"body"`   // be-aware the type of body should be co-response to "Content-Type"
			Headers map[string]string `json:"headers"`
			// mode: "cors", // no-cors, *cors, same-origin
			// cache: "no-cache", // *default, no-cache, reload, force-cache, only-if-cached
			// credentials: "same-origin", // include, *same-origin, omit
			// redirect: "follow", // manual, *follow, error
			// referrerPolicy: "no-referrer", // no-referrer, *no-referrer-when-downgrade, origin, origin-when-cross-origin, same-origin, strict-origin, strict-origin-when-cross-origin, unsafe-url
		}{
			Method:  "GET",
			Headers: map[string]string{},
		}

		if url := call.ArgumentList[0]; !url.IsString() {
			return ctx.vm.MakeCustomError("HTTPError", fmt.Sprintf("requires a URL, but got %q", url.Class()))
		} else {
			option.Url = url.String()
		}

		if len(call.ArgumentList) > 1 {
			for _, key := range call.ArgumentList[1].Object().Keys() {
				if v, err := call.ArgumentList[1].Object().Get(key); err == nil {
					switch key {
					case "method":
						if !v.IsString() {
							return ctx.vm.MakeCustomError("HTTPError", fmt.Sprintf("requires a method, but got %q", v.Class()))
						}
						option.Method, _ = v.ToString()
					case "headers":
						if !v.IsObject() {
							return ctx.vm.MakeCustomError("HTTPError", fmt.Sprintf("requires a headers object, but got %q", v.Class()))
						}
						for _, k := range v.Object().Keys() {
							if h, err := v.Object().Get(k); err == nil {
								option.Headers[k], _ = h.ToString()
							}
						}
					case "body":
						if !v.IsString() {
							return ctx.vm.MakeCustomError("HTTPError", fmt.Sprintf("requires a body, but got %q", v.Class()))
						}
						option.Body, _ = v.ToString()
					}
				}
			}
		}
		if !slices.Contains([]string{"GET", "POST", "PUT", "DELETE"}, strings.ToUpper(option.Method)) {
			return ctx.vm.MakeCustomError("InvalidOption", fmt.Sprintf("unsupported method %q", option.Method))
		}

		requestObj, _ := ctx.vm.Object(`({})`)
		requestObj.Set("do", func(call otto.FunctionCall) otto.Value {
			if len(call.ArgumentList) != 1 {
				return ctx.vm.MakeCustomError("HTTPError", "do() requires a callback function")
			}
			if !call.ArgumentList[0].IsFunction() {
				return ctx.vm.MakeCustomError("HTTPError",
					fmt.Sprintf("do() requires a callback function, but got %s", call.ArgumentList[0].Class()))
			}
			callback := call.ArgumentList[0]
			responseObj, _ := ctx.vm.Object(`({})`)

			httpClient := ctx.node.task.NewHttpClient()
			httpRequest, httpErr := http.NewRequest(strings.ToUpper(option.Method), option.Url, strings.NewReader(option.Body))
			var httpResponse *http.Response
			if httpErr == nil {
				for k, v := range option.Headers {
					httpRequest.Header.Set(k, v)
				}
				if option.Method == "POST" || option.Method == "PUT" {
					httpRequest.Body = io.NopCloser(strings.NewReader(option.Body))
				}
				if rsp, err := httpClient.Do(httpRequest); err != nil {
					httpErr = err
				} else {
					defer rsp.Body.Close()
					httpResponse = rsp
					responseObj.Set("status", rsp.StatusCode)
					responseObj.Set("statusText", rsp.Status)
					hdr := map[string]any{}
					for k, v := range rsp.Header {
						if len(v) == 1 {
							hdr[k] = v[0]
						} else {
							hdr[k] = v
						}
					}
					// TODO: implement get(), forEach(), has(), keys()
					responseObj.Set("headers", hdr)
				}
			}
			responseObj.Set("url", option.Url)
			responseObj.Set("ok", httpResponse != nil && httpResponse.StatusCode >= 200 && httpResponse.StatusCode < 300)
			responseObj.Set("error", func(call otto.FunctionCall) otto.Value {
				if httpErr == nil {
					return otto.UndefinedValue()
				}
				return ctx.vm.MakeCustomError("HTTPError", httpErr.Error())
			})
			bodyFunc := func(typ string) func(call otto.FunctionCall) otto.Value {
				return func(call otto.FunctionCall) otto.Value {
					if httpErr != nil {
						return ctx.vm.MakeCustomError("HTTPError", httpErr.Error())
					}
					if httpResponse == nil {
						return otto.UndefinedValue()
					}
					if !slices.Contains([]string{"csv", "json", "text", "blob"}, typ) {
						return ctx.vm.MakeCustomError("HTTPError", fmt.Sprintf("%s() unknown function", typ))
					}
					if len(call.ArgumentList) == 0 || !call.ArgumentList[0].IsFunction() {
						return ctx.vm.MakeCustomError("HTTPError", fmt.Sprintf("%s() requires a callback function", typ))
					}
					cb := call.ArgumentList[0]

					switch typ {
					case "csv":
						dec := csv.NewReader(httpResponse.Body)
						dec.FieldsPerRecord = -1
						dec.TrimLeadingSpace = true
						dec.ReuseRecord = true
						for {
							row, err := dec.Read()
							if err == io.EOF {
								break
							} else if err != nil {
								return ctx.vm.MakeCustomError("HTTPError", err.Error())
							}
							s := make([]any, len(row))
							for i, v := range row {
								s[i] = v
							}
							if _, e := cb.Call(otto.UndefinedValue(), s); e != nil {
								return ctx.vm.MakeCustomError("HTTPError", e.Error())
							}
						}
					case "json":
						dec := json.NewDecoder(httpResponse.Body)
						for {
							data := new(any)
							err := dec.Decode(data)
							if err == io.EOF {
								break
							} else if err != nil {
								return ctx.vm.MakeCustomError("HTTPError", err.Error())
							}
							var value otto.Value
							value, err = ottoValue(ctx, data)
							if err != nil {
								return ctx.vm.MakeCustomError("HTTPError", err.Error())
							}
							if _, e := cb.Call(otto.UndefinedValue(), value); e != nil {
								return ctx.vm.MakeCustomError("HTTPError", e.Error())
							}
						}
					case "text":
						if b, err := io.ReadAll(httpResponse.Body); err == nil {
							if s, err := otto.ToValue(string(b)); err == nil {
								if _, e := cb.Call(otto.UndefinedValue(), s); e != nil {
									return ctx.vm.MakeCustomError("HTTPError", e.Error())
								}
							} else {
								return ctx.vm.MakeCustomError("HTTPError", err.Error())
							}
						}
					case "blob":
						if b, err := io.ReadAll(httpResponse.Body); err == nil {
							if s, err := otto.ToValue(b); err == nil {
								if _, e := cb.Call(otto.UndefinedValue(), s); e != nil {
									return ctx.vm.MakeCustomError("HTTPError", e.Error())
								}
							} else {
								return ctx.vm.MakeCustomError("HTTPError", err.Error())
							}
						}
					}
					return otto.UndefinedValue()
				}
			}
			responseObj.Set("text", bodyFunc("text"))
			responseObj.Set("blob", bodyFunc("blob"))
			responseObj.Set("json", bodyFunc("json"))
			responseObj.Set("csv", bodyFunc("csv"))

			if _, e := callback.Call(otto.UndefinedValue(), responseObj.Value()); e != nil {
				return ctx.vm.MakeCustomError("HTTPError", e.Error())
			}
			return otto.UndefinedValue()
		})
		return requestObj.Value()
	}
}

func ottoValue(ctx *OttoContext, value any) (otto.Value, error) {
	switch v := value.(type) {
	case map[string]any:
		obj, _ := ctx.vm.Object(`({})`)
		for k, n := range v {
			if val, err := ottoValue(ctx, n); err == nil {
				obj.Set(k, val)
			} else {
				return otto.UndefinedValue(), err
			}
		}
		return obj.Value(), nil
	default:
		return otto.ToValue(v)
	}
}

func ottoFuncDB(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	var node = ctx.node
	return func(call otto.FunctionCall) otto.Value {
		var bridgeName string
		if len(call.ArgumentList) > 0 {
			// parse db options `$.db({bridge: "name"})`
			for _, key := range call.ArgumentList[0].Object().Keys() {
				if v, err := call.ArgumentList[0].Object().Get(key); err == nil {
					switch key {
					case "bridge":
						if !v.IsString() {
							return ctx.vm.MakeCustomError("DBError", fmt.Sprintf("requires a bridge, but got %q", v.Class()))
						}
						bridgeName, _ = v.ToString()
					default:
						return ctx.vm.MakeCustomError("DBError", fmt.Sprintf("unknown db option %q: %v", key, v.String()))
					}
				}
			}
		}
		db, _ := ctx.vm.Object(`({})`)
		// $.db().query(sql, params...).next(function(row) {...})
		db.Set("query", func(call otto.FunctionCall) otto.Value {
			if len(call.ArgumentList) == 0 {
				return ctx.vm.MakeCustomError("DBError", "missing a SQL text")
			}
			sqlText := call.ArgumentList[0]
			if !sqlText.IsString() {
				return ctx.vm.MakeCustomError("DBError", fmt.Sprintf("requires a SQL text, but got %q", sqlText.Class()))
			}
			var params []any
			if len(call.ArgumentList) > 1 {
				params = make([]any, len(call.ArgumentList)-1)
				for i, v := range call.ArgumentList[1:] {
					params[i], _ = v.Export()
				}
			}
			queryObj, _ := ctx.vm.Object(`({})`)
			queryObj.Set("yield", func(handler otto.FunctionCall) otto.Value {
				var conn api.Conn
				var err error
				if bridgeName == "" {
					conn, err = node.task.ConnDatabase(node.task.ctx)
				} else {
					if db, dbErr := connector.New(bridgeName); dbErr == nil {
						conn, err = db.Connect(node.task.ctx)
					} else {
						err = dbErr
					}
				}
				if err != nil {
					node.task.Cancel()
					return ctx.vm.MakeCustomError("DBError", err.Error())
				}
				defer conn.Close()

				rows, err := conn.Query(node.task.ctx, sqlText.String(), params...)
				if err != nil {
					node.task.Cancel()
					return ctx.vm.MakeCustomError("DBError", err.Error())
				}
				defer rows.Close()
				cols, _ := rows.Columns()

				// set headers
				types := []string{}
				for _, col := range cols {
					types = append(types, string(col.DataType))
				}
				var opts = ScriptOttoResultOption{
					Columns: cols.Names(),
					Types:   types,
				}
				if cols := opts.ResultColumns(); cols != nil {
					node.task.SetResultColumns(cols)
				}
				// yield rows
				count := 0
				for rows.Next() {
					values, _ := cols.MakeBuffer()
					rows.Scan(values...)
					count++
					NewRecord(count, values).Tell(node.next)
				}
				return otto.UndefinedValue()
			})
			queryObj.Set("forEach", func(handler otto.FunctionCall) otto.Value {
				if len(handler.ArgumentList) != 1 {
					return ctx.vm.MakeCustomError("DBError", "forEach() requires a callback function")
				}
				if !handler.ArgumentList[0].IsFunction() {
					return ctx.vm.MakeCustomError("DBError",
						fmt.Sprintf("forEach() requires a callback function, but got %s", handler.ArgumentList[0].Class()))
				}
				callback := handler.ArgumentList[0]
				var conn api.Conn
				var err error
				if bridgeName == "" {
					conn, err = node.task.ConnDatabase(node.task.ctx)
				} else {
					if db, dbErr := connector.New(bridgeName); dbErr == nil {
						conn, err = db.Connect(node.task.ctx)
					} else {
						err = dbErr
					}
				}
				if err != nil {
					node.task.Cancel()
					return ctx.vm.MakeCustomError("DBError", err.Error())
				}
				defer conn.Close()

				rows, err := conn.Query(node.task.ctx, sqlText.String(), params...)
				if err != nil {
					node.task.Cancel()
					return ctx.vm.MakeCustomError("DBError", err.Error())
				}
				defer rows.Close()
				for rows.Next() {
					cols, _ := rows.Columns()
					values, _ := cols.MakeBuffer()
					rows.Scan(values...)
					if flag, e := callback.Call(otto.UndefinedValue(), values); e != nil {
						return ctx.vm.MakeCustomError("DBError", e.Error())
					} else {
						if flag.IsUndefined() {
							// if the callback does not return anything (undefined), continue
							continue
						}
						if !flag.IsBoolean() {
							// if the callback returns a non-boolean value, break
							break
						}
						if b, _ := flag.ToBoolean(); !b {
							// if the callback returns false, break
							break
						}
					}
				}
				return otto.UndefinedValue()
			})
			return queryObj.Value()
		})

		// $.db().exec(sql, params...)
		db.Set("exec", func(call otto.FunctionCall) otto.Value {
			if len(call.ArgumentList) == 0 {
				return ctx.vm.MakeCustomError("DBError", "missing a SQL text")
			}
			sqlText := call.ArgumentList[0]
			if !sqlText.IsString() {
				return ctx.vm.MakeCustomError("DBError", fmt.Sprintf("requires a SQL text, but got %q", sqlText.Class()))
			}
			var params []any
			if len(call.ArgumentList) > 1 {
				params = make([]any, len(call.ArgumentList)-1)
				for i, v := range call.ArgumentList[1:] {
					params[i], _ = v.Export()
				}
			}
			var conn api.Conn
			var err error
			if bridgeName == "" {
				conn, err = node.task.ConnDatabase(node.task.ctx)
			} else {
				if db, dbErr := connector.New(bridgeName); dbErr == nil {
					conn, err = db.Connect(node.task.ctx)
				} else {
					err = dbErr
				}
			}
			if err != nil {
				node.task.Cancel()
				return ctx.vm.MakeCustomError("DBError", err.Error())
			}
			defer conn.Close()

			result := conn.Exec(node.task.ctx, sqlText.String(), params...)
			if err = result.Err(); err != nil {
				return ctx.vm.MakeCustomError("DBError", err.Error())
			}
			ret := result.RowsAffected()
			retValue, _ := otto.ToValue(ret)
			return retValue
		})
		return db.Value()
	}
}

func ottoFuncGeoJSON(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) != 1 {
			return ctx.vm.MakeCustomError("GeoJSONError", "missing a GeoJSON object")
		}
		value := call.ArgumentList[0]
		if !value.IsObject() {
			return ctx.vm.MakeCustomError("GeoJSONError", fmt.Sprintf("requires a GeoJSON object, but got %q", value.Class()))
		}
		obj := value.Object()
		typeString, err := obj.Get("type")
		if err != nil {
			return ctx.vm.MakeCustomError("GeoJSONError", "missing a GeoJSON type")
		}
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return ctx.vm.MakeCustomError("GeoJSONError", err.Error())
		}
		var geoObj any
		switch typeString.String() {
		case "FeatureCollection":
			if geo, err := geojson.UnmarshalFeatureCollection(jsonBytes); err == nil {
				geoObj = geo
			} else {
				return ctx.vm.MakeCustomError("GeoJSONError", err.Error())
			}
		case "Feature":
			if geo, err := geojson.UnmarshalFeature(jsonBytes); err == nil {
				geoObj = geo
			} else {
				return ctx.vm.MakeCustomError("GeoJSONError", err.Error())
			}
		case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
			if geo, err := geojson.UnmarshalGeometry(jsonBytes); err == nil {
				geoObj = geo
			} else {
				return ctx.vm.MakeCustomError("GeoJSONError", err.Error())
			}
		default:
			return ctx.vm.MakeCustomError("GeoJSONError", fmt.Sprintf("requires a GeoJSON type, but got %q", typeString.String()))
		}
		var _ = geoObj
		return obj.Value()
	}
}

func ottoFuncSystem(ctx *OttoContext) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		sys, _ := ctx.vm.Object(`({})`)
		// $.sys().free_os_memory()
		sys.Set("free_os_memory", func(call otto.FunctionCall) otto.Value {
			debug.FreeOSMemory()
			return otto.UndefinedValue()
		})
		// $.sys().gc()
		sys.Set("gc", func(call otto.FunctionCall) otto.Value {
			runtime.GC()
			return otto.UndefinedValue()
		})
		return sys.Value()
	}
}
