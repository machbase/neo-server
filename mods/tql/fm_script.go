package tql

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native"
	mod_dbms "github.com/machbase/neo-server/v8/jsh/native/db"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/machbase/neo-server/v8/mods/logging"
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
	_, err = ctx.run(node.Inflight())
	return nil, err
}

type JSContext struct {
	context.Context
	node          *Node
	engine        *engine.JSRuntime
	obj           *goja.Object
	sc            *goja.Program
	yieldCount    int64
	onceFinalize  sync.Once
	didSetResult  bool
	onceInterrupt sync.Once
}

func newJSContext(node *Node, initCode string, mainCode string, deinitCode string) (*JSContext, error) {
	// add blank lines to the beginning of the script
	// so that the compiler error message can show the correct line number
	if node.tqlLine != nil && node.tqlLine.line > 1 {
		initCodeLine := strings.Count(initCode, "\n")
		mainCode = strings.Repeat("\n", initCodeLine+node.tqlLine.line-1) + mainCode
	}
	conf := engine.Config{
		Name: "SCRIPT",
		Code: `(()=>{})()`,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			{MountPoint: "/work", Source: "."}, // TODO: /work directory should match with data directory of the process
		},
		Env:    map[string]any{},
		Reader: node.task.inputReader,
		Writer: &JSLog{w: node.task},
	}
	jr, err := engine.New(conf)
	if err != nil {
		return nil, err
	}
	// register native modules
	native.Enable(jr)

	// create ctx
	ctx := &JSContext{
		Context: node.task.ctx,
		engine:  jr,
		node:    node,
	}
	// compile main code
	if s, err := goja.Compile("SCRIPT main", mainCode, false); err != nil {
		return nil, err
	} else {
		ctx.sc = s
	}

	// it should run before the init code, so that the init code can use the native modules.
	if err := jr.Run(); err != nil {
		return nil, fmt.Errorf("SCRIPT runtime, %s", err.Error())
	}

	// init code
	var initErr error
	jr.EventLoop().Run(func(vm *goja.Runtime) {
		// set interrupt trigger
		ctx.node.task.AddShouldStopListener(func() {
			ctx.onceInterrupt.Do(func() {
				vm.Interrupt("interrupt")
			})
		})

		ctx.obj = vm.NewObject()
		// define $
		vm.Set("$", ctx.obj)

		// set $.payload
		if node.task.nodes[0] == node && node.task.inputReader != nil {
			// $.payload will be defined, only when the SCRIPT is the SRC node.
			// If the SCRIPT is not the SRC node, the payload has been using by the previous node.
			// and if the "inputReader" was consumed here, the actual SRC node will see the EOF.
			ctx.obj.Set("payload", ctx.jsPayload())
		}
		// set $.params
		ctx.obj.Set("params", ctx.jsParam())
		// function $.yield(...)
		ctx.obj.Set("yield", ctx.jsFuncYield)
		// function $.yieldKey(key, ...)
		ctx.obj.Set("yieldKey", ctx.jsFuncYieldKey)
		// function $.yieldArray(array)
		ctx.obj.Set("yieldArray", ctx.jsFuncYieldArray)
		// $.db()
		ctx.obj.Set("db", ctx.jsFuncDB(vm))
		// $.request(url, options)
		ctx.obj.Set("request", ctx.jsFuncRequest(vm))
		// $.inflight()
		ctx.obj.Set("inflight", ctx.jsFuncInflight(vm))

		if strings.TrimSpace(initCode) != "" {
			_, initErr = vm.RunString(initCode)
		}
	})
	if initErr != nil {
		return nil, fmt.Errorf("SCRIPT init, %s", initErr.Error())
	}

	node.SetEOF(func(*Node) {
		defer closeJSContext(ctx)
		ctx.onceFinalize.Do(func() {
			ctx.engine.EventLoop().Run(func(vm *goja.Runtime) {
				// intentionally ignore the panic from finalize stage.
				// it will raised to the task level.
				if strings.TrimSpace(deinitCode) == "" {
					if f, ok := goja.AssertFunction(vm.Get("finalize")); ok {
						_, err := f(goja.Undefined())
						if err != nil {
							node.task.LogErrorf("SCRIPT finalize, %s", err.Error())
						}
					}
				} else {
					if node.tqlLine != nil && node.tqlLine.line > 1 {
						mainCodeLine := strings.Count(mainCode, "\n")
						deinitCode = strings.Repeat("\n", mainCodeLine) + deinitCode
					}
					_, err := vm.RunString(deinitCode)
					if err != nil {
						node.task.LogErrorf("SCRIPT finalize, %s", err.Error())
					}
				}
				// set $.result columns if no records are yielded
				if !ctx.didSetResult {
					ctx.doResult()
				}
			})
		})
	})

	return ctx, nil
}

func closeJSContext(ctx *JSContext) {
	if ctx == nil {
		return
	}
	ctx.onceInterrupt.Do(func() {
		ctx.engine.EventLoop().Run(func(vm *goja.Runtime) {
			vm.Interrupt("interrupt")
		})
	})
}

func (ctx *JSContext) doResult() error {
	if ctx.obj == nil || ctx.didSetResult {
		return nil
	}
	resultObj := ctx.obj.Get("result")
	if resultObj == nil || goja.IsUndefined(resultObj) {
		return nil
	}
	x, _ := json.Marshal(resultObj)
	var opts JSResultOption
	json.Unmarshal(x, &opts)
	if cols := opts.ResultColumns(); cols != nil {
		ctx.node.task.SetResultColumns(cols)
	}
	ctx.didSetResult = true
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

func (ctx *JSContext) run(inflight *Record) (any, error) {
	if inflight != nil {
		ctx.obj.Set("key", inflight.key)
		if arr, ok := inflight.value.([]any); ok {
			ctx.obj.Set("values", arr)
		} else {
			ctx.obj.Set("values", []any{inflight.value})
		}
	}

	var ret any
	var retErr error
	ctx.engine.EventLoop().Run(func(vm *goja.Runtime) {
		if v, err := vm.RunProgram(ctx.sc); err != nil {
			if jsErr, ok := err.(*goja.Exception); ok {
				if uw := jsErr.Unwrap(); uw != nil {
					err = uw
					if stackFrames := jsErr.Stack(); len(stackFrames) > 0 {
						f := stackFrames[len(stackFrames)-1]
						p := f.Position()
						err = fmt.Errorf("%s, %s:%d:%d", err, f.SrcName(), p.Line, p.Column)
					}
				}
				retErr = err
			} else {
				retErr = err
			}
		} else {
			ret = v.Export()
		}
	})
	if retErr != nil {
		ctx.node.task.LogError(retErr.Error())
	}
	return ret, retErr
}

func (ctx *JSContext) jsPayload() any {
	// $.payload is defined, only when the SCRIPT is the SRC node.
	// If the SCRIPT is not the SRC node, the payload has been using by the previous node.
	// and if the "inputReader" was consumed here, the actual SRC node will see the EOF.
	if b, err := io.ReadAll(ctx.node.task.inputReader); err == nil {
		return string(b)
	}
	return goja.Undefined()
}

func (ctx *JSContext) jsParam() any {
	m := make(map[string]any)
	for k, v := range ctx.node.task.params {
		if len(v) == 1 {
			m[k] = v[0]
		} else {
			m[k] = v
		}
	}
	return m
}

func (ctx *JSContext) jsFuncYield(values ...any) {
	var v_key any
	if inflight := ctx.node.Inflight(); inflight != nil {
		v_key = inflight.key
	}
	if v_key == nil {
		v_key = ctx.yieldCount
	}
	ctx.yield(v_key, values)
}

func (ctx *JSContext) jsFuncYieldKey(key any, values ...any) {
	ctx.yield(key, values)
}

func (ctx *JSContext) jsFuncYieldArray(values []any) {
	ctx.jsFuncYield(values...)
}

func (ctx *JSContext) yield(key any, values []any) {
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

func (ctx *JSContext) jsFuncRequest(vm *goja.Runtime) func(reqUrl string, reqOpt map[string]any) goja.Value {
	return func(reqUrl string, reqOpt map[string]any) goja.Value {
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
				return vm.NewGoError(fmt.Errorf("HTTPError requires a method, but got %q", method))
			}
		}
		if headers, ok := reqOpt["headers"]; ok {
			if m, ok := headers.(map[string]any); ok {
				for k, v := range m {
					if s, ok := v.(string); ok {
						option.Headers[k] = s
					} else {
						return vm.NewGoError(fmt.Errorf("HTTPError requires a headers, but got %q", v))
					}
				}
			} else {
				return vm.NewGoError(fmt.Errorf("HTTPError requires a headers, but got %q", headers))
			}
		}
		if body, ok := reqOpt["body"]; ok {
			if s, ok := body.(string); ok {
				option.Body = s
			} else {
				return vm.NewGoError(fmt.Errorf("HTTPError requires a body, but got %q", body))
			}
		}

		if !slices.Contains([]string{"GET", "POST", "PUT", "DELETE"}, option.Method) {
			return vm.NewGoError(fmt.Errorf("HTTPError unsupported method %q", option.Method))
		}

		requestObj := vm.NewObject()
		requestObj.Set("do", func(callback goja.Callable) goja.Value {
			responseObj := vm.NewObject()
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
			responseObj.Set("error", func() goja.Value {
				if httpErr == nil {
					return goja.Undefined()
				}
				return vm.NewGoError(fmt.Errorf("HTTPError %s", httpErr.Error()))
			})
			bodyFunc := func(typ string) func(goja.Callable) goja.Value {
				return func(callback goja.Callable) goja.Value {
					if httpErr != nil {
						return vm.NewGoError(fmt.Errorf("HTTPError %s", httpErr.Error()))
					}
					if httpResponse == nil {
						return goja.Undefined()
					}
					if !slices.Contains([]string{"csv", "json", "text", "blob"}, typ) {
						return vm.NewGoError(fmt.Errorf("HTTPError %s() unknown function", typ))
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
								return vm.NewGoError(fmt.Errorf("HTTPError %s", err.Error()))
							}
							s := make([]any, len(row))
							for i, v := range row {
								s[i] = v
							}
							if _, e := callback(goja.Undefined(), vm.ToValue(s)); e != nil {
								return vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
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
								return vm.NewGoError(fmt.Errorf("HTTPError %s", err.Error()))
							}
							value := vm.ToValue(data)
							if _, e := callback(goja.Undefined(), value); e != nil {
								return vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
							}
						}
					case "text":
						if b, err := io.ReadAll(httpResponse.Body); err == nil {
							s := vm.ToValue(string(b))
							if _, e := callback(goja.Undefined(), s); e != nil {
								return vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
							}
						}
					case "blob":
						if b, err := io.ReadAll(httpResponse.Body); err == nil {
							s := vm.ToValue(string(b))
							if _, e := callback(goja.Undefined(), s); e != nil {
								return vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
							}
						}
					}
					return goja.Undefined()
				}
			}
			responseObj.Set("text", bodyFunc("text"))
			responseObj.Set("blob", bodyFunc("blob"))
			responseObj.Set("json", bodyFunc("json"))
			responseObj.Set("csv", bodyFunc("csv"))

			if _, e := callback(goja.Undefined(), responseObj); e != nil {
				return vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
			}
			return goja.Undefined()
		})
		return requestObj
	}
}

func (ctx *JSContext) jsFuncDB(vm *goja.Runtime) func(call map[string]any) goja.Value {
	return func(opt map[string]any) goja.Value {
		defer func() {
			if r := recover(); r != nil {
				ctx.node.task.LogErrorf("SCRIPT db====, %s", r)
			}
		}()
		var node = ctx.node
		var dbObj = vm.NewObject()

		dbOpts := mod_dbms.ClientOptions{
			BridgeName:       "",
			LowerCaseColumns: false,
			Driver:           "",
			DataSource:       "",
		}
		if bridge, ok := opt["bridge"]; ok {
			if s, ok := bridge.(string); ok {
				dbOpts.BridgeName = s
			} else {
				return vm.NewGoError(fmt.Errorf("DB requires a bridge name, but got %q", bridge))
			}
		}
		if lc, ok := opt["lowerCaseColumns"]; ok {
			if b, ok := lc.(bool); ok {
				dbOpts.LowerCaseColumns = b
			} else {
				return vm.NewGoError(fmt.Errorf("DB requires a lowerCaseColumns bool, but got %q", lc))
			}
		}
		if driver, ok := opt["driver"]; ok {
			if s, ok := driver.(string); ok {
				dbOpts.Driver = s
			} else {
				return vm.NewGoError(fmt.Errorf("DB requires a driver name, but got %q", driver))
			}
		}
		if dataSource, ok := opt["dataSource"]; ok {
			if s, ok := dataSource.(string); ok {
				dbOpts.DataSource = s
			} else {
				return vm.NewGoError(fmt.Errorf("DB requires a dataSource string, but got %q", dataSource))
			}
		}

		// $.db().query(sql, params...).next(function(row) {...})
		dbObj.Set("query", func(call goja.FunctionCall) goja.Value {
			queryObj := vm.NewObject()
			queryArgs := make([]goja.Value, len(call.Arguments))
			copy(queryArgs, call.Arguments)

			queryObj.Set("yield", func(call goja.FunctionCall) goja.Value {
				client := mod_dbms.NewClientWithOptions(vm, dbOpts)
				conn := client.Connect(goja.FunctionCall{})
				defer conn.Close(goja.FunctionCall{})
				rows := conn.Query(goja.FunctionCall{Arguments: queryArgs})
				defer rows.Close(goja.FunctionCall{})

				var resultOpt = JSResultOption{
					"columns": rows.ColumnNames(goja.FunctionCall{}),
					"types":   rows.ColumnTypes(goja.FunctionCall{}),
				}
				if cols := resultOpt.ResultColumns(); cols != nil {
					node.task.SetResultColumns(cols)
				}
				// yield rows
				count := 0
				for {
					values := rows.Next(goja.FunctionCall{})
					if len(values) == 0 {
						break
					}
					count++
					NewRecord(count, values).Tell(node.next)
				}
				return goja.Undefined()
			})

			queryObj.Set("forEach", func(callback goja.Callable) goja.Value {
				client := mod_dbms.NewClientWithOptions(vm, dbOpts)
				conn := client.Connect(goja.FunctionCall{})
				defer conn.Close(goja.FunctionCall{})
				rows := conn.Query(goja.FunctionCall{Arguments: queryArgs})
				defer rows.Close(goja.FunctionCall{})

				// ensure the columns are set
				_ = rows.ColumnNames(goja.FunctionCall{})
				for {
					values := rows.Next(goja.FunctionCall{})
					if len(values) == 0 {
						break
					}
					names := rows.ColumnNames(goja.FunctionCall{})

					var rec = map[string]any{}
					for i, col := range names {
						if i < len(values) {
							rec[col] = vm.ToValue(api.Unbox(values[i]))
						} else {
							rec[col] = goja.Null()
						}
					}
					if flag, e := callback(goja.Undefined(), vm.ToValue(values), vm.ToValue(rec)); e != nil {
						return vm.NewGoError(e)
					} else {
						if goja.IsUndefined(flag) {
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
				return goja.Undefined()
			})

			return queryObj
		})

		// $.db().exec(sql, params...)
		dbObj.Set("exec", func(call goja.FunctionCall) goja.Value {
			client := mod_dbms.NewClientWithOptions(vm, dbOpts)
			conn := client.Connect(goja.FunctionCall{})
			defer conn.Close(goja.FunctionCall{})
			return conn.Exec(call)
		})

		return dbObj
	}
}

func (ctx *JSContext) jsFuncInflight(vm *goja.Runtime) func() goja.Value {
	return func() goja.Value {
		ret := vm.NewObject()
		ret.Set("set", func(name string, value goja.Value) goja.Value {
			if inf := ctx.node.Inflight(); inf != nil {
				inf.SetVariable(name, value.Export())
			}
			return goja.Undefined()
		})
		ret.Set("get", func(name string) goja.Value {
			if inf := ctx.node.Inflight(); inf != nil {
				if v, err := inf.GetVariable("$" + name); err != nil {
					return vm.NewGoError(fmt.Errorf("SCRIPT %s", err.Error()))
				} else {
					return vm.ToValue(v)
				}
			}
			return goja.Undefined()
		})
		return ret
	}
}

type JSLog struct {
	w io.Writer
}

func (l *JSLog) Write(p []byte) (n int, err error) {
	return l.w.Write(p)
}

func (l *JSLog) Log(lvl slog.Level, args ...any) {
	if v, ok := l.w.(logging.Log); ok {
		switch lvl {
		case slog.LevelDebug:
			v.Debug(args...)
		case slog.LevelInfo:
			v.Info(args...)
		case slog.LevelWarn:
			v.Warn(args...)
		case slog.LevelError:
			v.Error(args...)
		default:
			v.Info(args...)
		}
	} else if v, ok := l.w.(*Task); ok {
		switch lvl {
		case slog.LevelDebug:
			v.LogDebug(args...)
		case slog.LevelInfo:
			v.LogInfo(args...)
		case slog.LevelWarn:
			v.LogWarn(args...)
		case slog.LevelError:
			v.LogError(args...)
		default:
			v.LogInfo(args...)
		}
	} else {
		prefix := "[" + lvl.String() + "]"
		prefix = prefix + strings.Repeat(" ", 8-len(prefix))
		fmt.Fprintln(l.w, prefix, fmt.Sprint(args...))
	}
}

func (l *JSLog) Print(args ...any) {
	if v, ok := l.w.(*Task); ok {
		v.Log(args...)
	} else {
		fmt.Fprint(l.w, args...)
	}
}
func (l *JSLog) Println(args ...any) {
	if v, ok := l.w.(*Task); ok {
		v.Log(args...)
	} else {
		fmt.Fprintln(l.w, args...)
	}
}
func (l *JSLog) Printf(format string, args ...any) {
	if v, ok := l.w.(*Task); ok {
		v.Log(fmt.Sprintf(format, args...))
	} else {
		fmt.Fprintf(l.w, format, args...)
	}
}
