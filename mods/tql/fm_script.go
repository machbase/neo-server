package tql

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/machbase/neo-server/v8/mods/jsh/builtin"
	mod_dbms "github.com/machbase/neo-server/v8/mods/jsh/db"
	"github.com/machbase/neo-server/v8/mods/logging"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type bridgeName struct {
	name string
}

// bridge('name')
func (x *Node) fmBridge(name string) *bridgeName {
	return &bridgeName{name: name}
}

func (node *Node) fmScript(args ...any) (any, error) {
	if len(args) > 0 && args[0] == "js" {
		args = args[1:]
	}
	initCode, mainCode, deinitCode := "", "", ""
	if len(args) == 1 { // SCRIPT("main")
		if str, ok := args[0].(string); !ok {
			goto syntaxErr
		} else {
			mainCode = str
		}
	} else if len(args) == 2 { // SCRIPT("init", "main")
		if str, ok := args[0].(string); !ok {
			goto syntaxErr
		} else {
			initCode = str
		}
		if str, ok := args[1].(string); !ok {
			goto syntaxErr
		} else {
			mainCode = str
		}
	} else if len(args) == 3 { // SCRIPT("init", "main", "deinit")
		if str, ok := args[0].(string); !ok {
			goto syntaxErr
		} else {
			initCode = str
		}
		if str, ok := args[1].(string); !ok {
			goto syntaxErr
		} else {
			mainCode = str
		}
		if str, ok := args[2].(string); !ok {
			goto syntaxErr
		} else {
			deinitCode = str
		}
	} else {
		goto syntaxErr
	}
	return node.fmScriptJS(initCode, mainCode, deinitCode)
syntaxErr:
	return nil, errors.New(`script: wrong syntax, 'SCRIPT( init_script, main_script, deinit_script )`)
}

const js_ctx_key = "$js_ctx$"

func (node *Node) fmScriptJS(initCode string, mainCode string, deinitCode string) (any, error) {
	var ctx *JSContext
	var err error

	defer func() {
		if r := recover(); r != nil {
			code := "{" + strings.TrimSpace(strings.TrimPrefix(initCode, "//")) + "}\n" +
				"{" + strings.TrimSpace(strings.TrimPrefix(mainCode, "//")) + "}"
			node.task.LogWarnf("script panic; %v\n%s", r, code)
		}
	}()

	if obj, ok := node.GetValue(js_ctx_key); ok {
		if o, ok := obj.(*JSContext); ok {
			ctx = o
		}
	}

	if ctx == nil {
		ctx, err = newJSContext(node, initCode, mainCode, deinitCode)
		if err != nil {
			return nil, err
		}
		node.SetValue(js_ctx_key, ctx)
	}
	if inflight := node.Inflight(); inflight != nil {
		ctx.obj.Set("key", ctx.vm.ToValue(inflight.key))
		if arr, ok := inflight.value.([]any); ok {
			ctx.obj.Set("values", ctx.vm.ToValue(arr))
		} else {
			ctx.obj.Set("values", ctx.vm.ToValue([]any{inflight.value}))
		}
	}
	_, err = ctx.Run()
	return nil, err
}

type JSContext struct {
	context.Context
	*jsh.Cleaner

	vm           *js.Runtime
	sc           *js.Program
	node         *Node
	obj          *js.Object
	yieldCount   int64
	onceFinalize sync.Once
	didSetResult bool

	onceInterrupt sync.Once
}

// predefined modules, primarily for testing purpose
var predefModules map[string][]byte

func RegisterPredefModule(name string, content []byte) {
	if predefModules == nil {
		predefModules = map[string][]byte{}
	}
	predefModules[name] = content
}

func UnregisterPredefModule(name string) {
	if predefModules != nil {
		delete(predefModules, name)
	}
}

func ClearPredefModules() {
	if predefModules != nil {
		for k := range predefModules {
			delete(predefModules, k)
		}
	}
}

func jsSourceLoad(path string) ([]byte, error) {
	if predefModules != nil {
		if content, ok := predefModules[path]; ok {
			return content, nil
		}
	}
	ss := ssfs.Default()
	ent, err := ss.Get("/" + strings.TrimPrefix(path, "/"))
	if err != nil || ent.IsDir {
		return nil, require.ModuleFileDoesNotExistError
	}
	return ent.Content, nil
}

func newJSContext(node *Node, initCode string, mainCode string, deinitCode string) (*JSContext, error) {
	ctx := &JSContext{
		Context: node.task.ctx,
		Cleaner: &jsh.Cleaner{},
		node:    node,
		vm:      js.New(),
	}
	ctx.vm.SetFieldNameMapper(js.TagFieldNameMapper("json", false))
	builtin.EnableConsole(ctx.vm, func(level logging.Level, args ...any) error {
		node.task._log(LogginLevelFrom(level), args...)
		return nil
	})

	registry := require.NewRegistry(require.WithLoader(jsSourceLoad))
	registry.Enable(ctx.vm)
	jsh.RegisterNativeModules(ctx, registry, jsh.NativeModuleNamesExcludes("@jsh/process")...)

	// add blank lines to the beginning of the script
	// so that the compiler error message can show the correct line number
	if node.tqlLine != nil && node.tqlLine.line > 1 {
		initCodeLine := strings.Count(initCode, "\n")
		mainCode = strings.Repeat("\n", initCodeLine+node.tqlLine.line-1) + mainCode
	}

	if s, err := js.Compile("", mainCode, false); err != nil {
		return nil, err
	} else {
		ctx.sc = s
	}

	node.SetEOF(func(*Node) {
		defer closeJSContext(ctx)
		// set $.result columns if no records are yielded
		if !ctx.didSetResult {
			ctx.doResult()
		}
		ctx.onceFinalize.Do(func() {
			// intentionally ignore the panic from finalize stage.
			// it will raised to the task level.
			if strings.TrimSpace(deinitCode) == "" {
				if f, ok := js.AssertFunction(ctx.vm.Get("finalize")); ok {
					_, err := f(js.Undefined())
					if err != nil {
						node.task.LogErrorf("SCRIPT finalize, %s", err.Error())
					}
				}
			} else {
				if node.tqlLine != nil && node.tqlLine.line > 1 {
					mainCodeLine := strings.Count(mainCode, "\n")
					deinitCode = strings.Repeat("\n", mainCodeLine) + deinitCode
				}
				_, err := ctx.vm.RunString(deinitCode)
				if err != nil {
					node.task.LogErrorf("SCRIPT finalize, %s", err.Error())
				}
			}

			ctx.RunCleanup(ctx.node.task)
		})
	})

	// define $
	ctx.obj = ctx.vm.NewObject()
	ctx.vm.Set("$", ctx.obj)

	// set $.payload
	var payload = js.Undefined()
	if node.task.nodes[0] == node && node.task.inputReader != nil {
		// $.payload is defined, only when the SCRIPT is the SRC node.
		// If the SCRIPT is not the SRC node, the payload has been using by the previous node.
		// and if the "inputReader" was consumed here, the actual SRC node will see the EOF.
		if b, err := io.ReadAll(node.task.inputReader); err == nil {
			payload = ctx.vm.ToValue(string(b))
		}
	}
	ctx.obj.Set("payload", payload)

	// set $.params
	if pv, err := ctx.vm.RunString("()=>{return new Map()}"); err != nil {
		return nil, fmt.Errorf("SCRIPT params, %s", err.Error())
	} else {
		values := pv.ToObject(ctx.vm)
		for k, v := range node.task.params {
			if len(v) == 1 {
				values.Set(k, ctx.vm.ToValue(v[0]))
			} else {
				values.Set(k, ctx.vm.ToValue(v))
			}
		}
		ctx.obj.Set("params", values)
	}

	// function $.yield(...)
	ctx.obj.Set("yield", ctx.jsFuncYield)
	// function $.yieldKey(key, ...)
	ctx.obj.Set("yieldKey", ctx.jsFuncYieldKey)
	// function $.yieldArray(array)
	ctx.obj.Set("yieldArray", ctx.jsFuncYieldArray)
	// $.db()
	ctx.obj.Set("db", ctx.jsFuncDB)
	// $.request()
	ctx.obj.Set("request", ctx.jsFuncRequest)
	// $.inflight()
	ctx.obj.Set("inflight", ctx.jsFuncInflight)

	ctx.node.task.AddShouldStopListener(func() {
		ctx.onceInterrupt.Do(func() {
			ctx.vm.Interrupt("interrupt")
		})
	})

	// init code
	if strings.TrimSpace(initCode) != "" {
		if node.tqlLine != nil && node.tqlLine.line > 1 {
			initCode = strings.Repeat("\n", node.tqlLine.line-1) + initCode
		}
		_, err := ctx.vm.RunString(initCode)
		if err != nil {
			if jsErr, ok := err.(*js.Exception); ok {
				return nil, fmt.Errorf("SCRIPT init, %s", strings.ReplaceAll(jsErr.String(), "github.com/dop251/goja_nodejs/", ""))
			} else {
				return nil, fmt.Errorf("SCRIPT init, %s", err.Error())
			}
		}
	}

	return ctx, nil
}

func closeJSContext(ctx *JSContext) {
	if ctx == nil {
		return
	}
	ctx.onceInterrupt.Do(func() {
		ctx.vm.Interrupt("interrupt")
	})
}

func (ctx *JSContext) doResult() error {
	if ctx.obj == nil {
		return nil
	}
	resultObj := ctx.obj.Get("result")
	if resultObj != nil && !js.IsUndefined(resultObj) {
		var opts JSResultOption
		if err := ctx.vm.ExportTo(resultObj, &opts); err != nil {
			return fmt.Errorf("line %d, SCRIPT option, %s", ctx.node.tqlLine.line, err.Error())
		}
		if cols := opts.ResultColumns(); cols != nil {
			ctx.node.task.SetResultColumns(cols)
		}
		ctx.didSetResult = true
	}
	return nil
}

type JSResultOption map[string]any

func (so JSResultOption) ResultColumns() api.Columns {
	var columns []string
	var types []string
	if c, ok := so["columns"]; !ok {
		return nil
	} else if s, ok := c.([]string); ok {
		columns = s
	} else {
		for _, v := range c.([]any) {
			if s, ok := v.(string); ok {
				columns = append(columns, s)
			} else {
				columns = append(columns, fmt.Sprintf("%v", v))
			}
		}
	}
	if t, ok := so["types"]; !ok {
		return nil
	} else if s, ok := t.([]string); ok {
		types = s
	} else {
		for _, v := range t.([]any) {
			if s, ok := v.(string); ok {
				types = append(types, s)
			} else {
				types = append(types, fmt.Sprintf("%v", v))
			}
		}
	}

	cols := make([]*api.Column, len(columns)+1)
	cols[0] = &api.Column{Name: "key", DataType: api.DataTypeAny}
	for i, name := range columns {
		cols[i+1] = &api.Column{Name: name, DataType: api.DataTypeAny}
		if len(types) > i {
			cols[i+1].DataType = api.ParseDataType(types[i])
		}
	}
	return cols
}

func (ctx *JSContext) Run() (any, error) {
	if v, err := ctx.vm.RunProgram(ctx.sc); err != nil {
		if jsErr, ok := err.(*js.Exception); ok {
			if uw := jsErr.Unwrap(); uw != nil {
				err = uw
				if stackFrames := jsErr.Stack(); len(stackFrames) > 0 {
					f := stackFrames[len(stackFrames)-1]
					p := f.Position()
					err = fmt.Errorf("%s:%d:%d", err, p.Line, p.Column)
				}
			}
		}
		return nil, err
	} else {
		return v.Export(), nil
	}
}

func (ctx *JSContext) jsFuncYield(values ...js.Value) {
	var v_key js.Value
	if inflight := ctx.node.Inflight(); inflight != nil {
		v_key = ctx.vm.ToValue(inflight.key)
	}
	if v_key == nil {
		v_key = ctx.vm.ToValue(ctx.yieldCount)
	}
	ctx.yield(v_key, values)
}

func (ctx *JSContext) jsFuncYieldKey(key js.Value, values ...js.Value) {
	ctx.yield(key, values)
}

func (ctx *JSContext) jsFuncYieldArray(values js.Value) {
	obj := values.ToObject(ctx.vm)
	var arr []js.Value
	ctx.vm.ExportTo(obj, &arr)
	ctx.jsFuncYield(arr...)
}

func (ctx *JSContext) yield(argKey js.Value, args []js.Value) {
	var values []any
	var key = argKey.Export()
	values = make([]any, len(args))
	for i, val := range args {
		values[i] = val.Export()
		if v, ok := values[i].(int64); ok {
			values[i] = float64(v)
		}
	}
	// set $.result columns before the first yield
	if ctx.yieldCount == 0 && !ctx.didSetResult {
		ctx.doResult()
	}

	var vars map[string]any
	if inf := ctx.node.Inflight(); inf != nil && inf.vars != nil {
		vars = inf.vars
	}
	NewRecordVars(key, values, vars).Tell(ctx.node.next)
	ctx.yieldCount++
}

func (ctx *JSContext) jsFuncRequest(reqUrl string, reqOpt map[string]any) js.Value {
	// $.request(url, option).do(function(response) {...})
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
		Url:     reqUrl,
		Method:  "GET",
		Headers: map[string]string{},
	}

	if method, ok := reqOpt["method"]; ok {
		if s, ok := method.(string); ok {
			option.Method = strings.ToUpper(s)
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError requires a method, but got %q", method))
		}
	}
	if headers, ok := reqOpt["headers"]; ok {
		if m, ok := headers.(map[string]any); ok {
			for k, v := range m {
				if s, ok := v.(string); ok {
					option.Headers[k] = s
				} else {
					return ctx.vm.NewGoError(fmt.Errorf("HTTPError requires a headers, but got %q", v))
				}
			}
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError requires a headers, but got %q", headers))
		}
	}
	if body, ok := reqOpt["body"]; ok {
		if s, ok := body.(string); ok {
			option.Body = s
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError requires a body, but got %q", body))
		}
	}

	if !slices.Contains([]string{"GET", "POST", "PUT", "DELETE"}, option.Method) {
		return ctx.vm.NewGoError(fmt.Errorf("HTTPError unsupported method %q", option.Method))
	}

	requestObj := ctx.vm.NewObject()
	requestObj.Set("do", func(callback js.Callable) js.Value {
		responseObj := ctx.vm.NewObject()
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
		responseObj.Set("error", func() js.Value {
			if httpErr == nil {
				return js.Undefined()
			}
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", httpErr.Error()))
		})
		bodyFunc := func(typ string) func(js.Callable) js.Value {
			return func(callback js.Callable) js.Value {
				if httpErr != nil {
					return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", httpErr.Error()))
				}
				if httpResponse == nil {
					return js.Undefined()
				}
				if !slices.Contains([]string{"csv", "json", "text", "blob"}, typ) {
					return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s() unknown function", typ))
				}

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
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", err.Error()))
						}
						s := make([]any, len(row))
						for i, v := range row {
							s[i] = v
						}
						if _, e := callback(js.Undefined(), ctx.vm.ToValue(s)); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
						}
					}
				case "json":
					dec := json.NewDecoder(httpResponse.Body)
					for {
						data := map[string]any{}
						err := dec.Decode(&data)
						if err == io.EOF {
							break
						} else if err != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", err.Error()))
						}
						value := ctx.vm.ToValue(data)
						if _, e := callback(js.Undefined(), value); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
						}
					}
				case "text":
					if b, err := io.ReadAll(httpResponse.Body); err == nil {
						s := ctx.vm.ToValue(string(b))
						if _, e := callback(js.Undefined(), s); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
						}
					}
				case "blob":
					if b, err := io.ReadAll(httpResponse.Body); err == nil {
						s := ctx.vm.ToValue(string(b))
						if _, e := callback(js.Undefined(), s); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
						}
					}
				}
				return js.Undefined()
			}
		}
		responseObj.Set("text", bodyFunc("text"))
		responseObj.Set("blob", bodyFunc("blob"))
		responseObj.Set("json", bodyFunc("json"))
		responseObj.Set("csv", bodyFunc("csv"))

		if _, e := callback(js.Undefined(), responseObj); e != nil {
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
		}
		return js.Undefined()
	})
	return requestObj
}

func (ctx *JSContext) jsFuncDB(call js.FunctionCall) js.Value {
	defer func() {
		if r := recover(); r != nil {
			ctx.node.task.LogErrorf("SCRIPT db====, %s", r)
		}
	}()
	var node = ctx.node
	var dbObj = ctx.vm.NewObject()

	dbArgs := make([]js.Value, len(call.Arguments))
	for i, arg := range call.Arguments {
		dbArgs[i] = ctx.vm.ToValue(arg.Export())
	}

	// $.db().query(sql, params...).next(function(row) {...})
	dbObj.Set("query", func(call js.FunctionCall) js.Value {
		queryObj := ctx.vm.NewObject()
		queryArgs := make([]js.Value, len(call.Arguments))
		for i, arg := range call.Arguments {
			queryArgs[i] = ctx.vm.ToValue(arg.Export())
		}

		queryObj.Set("yield", func(call js.FunctionCall) js.Value {
			client := mod_dbms.NewClient(ctx, ctx.vm, dbArgs)
			conn := client.Connect(js.FunctionCall{})
			defer conn.Close(js.FunctionCall{})
			rows := conn.Query(js.FunctionCall{Arguments: queryArgs})
			defer rows.Close(js.FunctionCall{})

			var resultOpt = JSResultOption{
				"columns": rows.ColumnNames(js.FunctionCall{}),
				"types":   rows.ColumnTypes(js.FunctionCall{}),
			}
			if cols := resultOpt.ResultColumns(); cols != nil {
				node.task.SetResultColumns(cols)
			}
			// yield rows
			count := 0
			for {
				values := rows.Next(js.FunctionCall{})
				if len(values) == 0 {
					break
				}
				count++
				NewRecord(count, values).Tell(node.next)
			}
			return js.Undefined()
		})

		queryObj.Set("forEach", func(callback js.Callable) js.Value {
			client := mod_dbms.NewClient(ctx, ctx.vm, dbArgs)
			conn := client.Connect(js.FunctionCall{})
			defer conn.Close(js.FunctionCall{})
			rows := conn.Query(js.FunctionCall{Arguments: queryArgs})
			defer rows.Close(js.FunctionCall{})

			// ensure the columns are set
			_ = rows.ColumnNames(js.FunctionCall{})
			for {
				values := rows.Next(js.FunctionCall{})
				if len(values) == 0 {
					break
				}
				names := rows.ColumnNames(js.FunctionCall{})

				var rec = map[string]any{}
				for i, col := range names {
					if i < len(values) {
						rec[col] = ctx.vm.ToValue(api.Unbox(values[i]))
					} else {
						rec[col] = js.Null()
					}
				}
				if flag, e := callback(js.Undefined(), ctx.vm.ToValue(values), ctx.vm.ToValue(rec)); e != nil {
					return ctx.vm.NewGoError(e)
				} else {
					if js.IsUndefined(flag) {
						// if the callback does not return anything (undefined), continue
						continue
					}
					if !flag.ToBoolean() {
						// if the callback returns a non-boolean value, break
						// if the callback returns false, break
						break
					}
				}
			}
			return js.Undefined()
		})

		return queryObj
	})

	// $.db().exec(sql, params...)
	dbObj.Set("exec", func(call js.FunctionCall) js.Value {
		client := mod_dbms.NewClient(ctx, ctx.vm, dbArgs)
		conn := client.Connect(js.FunctionCall{})
		defer conn.Close(js.FunctionCall{})
		return conn.Exec(call)
	})

	return dbObj
}

func (ctx *JSContext) jsFuncInflight() js.Value {
	ret := ctx.vm.NewObject()
	ret.Set("set", func(name string, value js.Value) js.Value {
		if inf := ctx.node.Inflight(); inf != nil {
			inf.SetVariable(name, value.Export())
		}
		return js.Undefined()
	})
	ret.Set("get", func(name string) js.Value {
		if inf := ctx.node.Inflight(); inf != nil {
			if v, err := inf.GetVariable("$" + name); err != nil {
				return ctx.vm.NewGoError(fmt.Errorf("SCRIPT %s", err.Error()))
			} else {
				return ctx.vm.ToValue(v)
			}
		}
		return js.Undefined()
	})
	return ret
}
