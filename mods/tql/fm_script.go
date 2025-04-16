package tql

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/mods/util"
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
	if len(args) == 1 {
		text, ok := args[0].(string)
		if !ok {
			goto syntaxErr
		}
		return node.fmScriptGoja("", text, "")
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
				initCode, mainCode, deinitCode := "", "", ""
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
				} else if len(args) == 4 { // SCRIPT("js", "init", "main", "deinit")
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
					if str, ok := args[3].(string); !ok {
						goto syntaxErr
					} else {
						deinitCode = str
					}
				} else {
					goto syntaxErr
				}
				return node.fmScriptGoja(initCode, mainCode, deinitCode)
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

const goja_ctx_key = "$goja_ctx$"

func (node *Node) fmScriptGoja(initCode string, mainCode string, deinitCode string) (any, error) {
	var ctx *GojaContext
	var err error

	defer func() {
		if r := recover(); r != nil {
			code := "{" + strings.TrimSpace(strings.TrimPrefix(initCode, "//")) + "}\n" +
				"{" + strings.TrimSpace(strings.TrimPrefix(mainCode, "//")) + "}"
			node.task.LogWarnf("script panic; %v\n%s", r, code)
		}
	}()

	if obj, ok := node.GetValue(goja_ctx_key); ok {
		if o, ok := obj.(*GojaContext); ok {
			ctx = o
		}
	}

	if ctx == nil {
		ctx, err = newGojaContext(node, initCode, mainCode, deinitCode)
		if err != nil {
			return nil, err
		}
		node.SetValue(goja_ctx_key, ctx)
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

type GojaContext struct {
	vm           *goja.Runtime
	sc           *goja.Program
	node         *Node
	obj          *goja.Object
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
	if !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".mjs") {
		return nil, require.ModuleFileDoesNotExistError
	}
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

func newGojaContext(node *Node, initCode string, mainCode string, deinitCode string) (*GojaContext, error) {
	ctx := &GojaContext{
		node: node,
		vm:   goja.New(),
	}
	ctx.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", false))

	enableModuleRegistry(ctx)

	// add blank lines to the beginning of the script
	// so that the compiler error message can show the correct line number
	if node.tqlLine != nil && node.tqlLine.line > 1 {
		initCodeLine := strings.Count(initCode, "\n")
		mainCode = strings.Repeat("\n", initCodeLine+node.tqlLine.line-1) + mainCode
	}

	if s, err := goja.Compile("", mainCode, false); err != nil {
		return nil, err
	} else {
		ctx.sc = s
	}

	node.SetEOF(func(*Node) {
		defer closeGojaContext(ctx)
		// set $.result columns if no records are yielded
		if !ctx.didSetResult {
			ctx.doResult()
		}
		ctx.onceFinalize.Do(func() {
			// intentionally ignore the panic from finalize stage.
			// it will raised to the task level.
			if strings.TrimSpace(deinitCode) == "" {
				if f, ok := goja.AssertFunction(ctx.vm.Get("finalize")); ok {
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
				_, err := ctx.vm.RunString(deinitCode)
				if err != nil {
					node.task.LogErrorf("SCRIPT finalize, %s", err.Error())
				}
			}
		})
	})

	// define console
	con := ctx.vm.NewObject()
	con.Set("log", ctx.consoleLog(INFO))
	con.Set("debug", ctx.consoleLog(DEBUG))
	con.Set("info", ctx.consoleLog(INFO))
	con.Set("warn", ctx.consoleLog(WARN))
	con.Set("error", ctx.consoleLog(ERROR))
	ctx.vm.Set("console", con)

	// define $
	ctx.obj = ctx.vm.NewObject()
	ctx.vm.Set("$", ctx.obj)

	// set $.payload
	var payload = goja.Undefined()
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
	ctx.obj.Set("yield", ctx.gojaFuncYield)
	// function $.yieldKey(key, ...)
	ctx.obj.Set("yieldKey", ctx.gojaFuncYieldKey)
	// function $.yieldArray(array)
	ctx.obj.Set("yieldArray", ctx.gojaFuncYieldArray)
	// $.db()
	ctx.obj.Set("db", ctx.gojaFuncDB)
	// $.publisher()
	ctx.obj.Set("publisher", ctx.gojaFuncPublisher)
	// $.request()
	ctx.obj.Set("request", ctx.gojaFuncRequest)
	// $.set()
	ctx.obj.Set("set", ctx.gojaFuncSet)
	// $.get()
	ctx.obj.Set("get", ctx.gojaFuncGet)

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
			return nil, fmt.Errorf("SCRIPT init, %s", err.Error())
		}
	}

	return ctx, nil
}

func closeGojaContext(ctx *GojaContext) {
	if ctx == nil {
		return
	}
	ctx.onceInterrupt.Do(func() {
		ctx.vm.Interrupt("interrupt")
	})
}

func (ctx *GojaContext) doResult() error {
	if ctx.obj == nil {
		fmt.Println("ctx.obj is nil")
		return nil
	}
	resultObj := ctx.obj.Get("result")
	if resultObj != nil && !goja.IsUndefined(resultObj) {
		var opts ScriptGojaResultOption
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

type ScriptGojaResultOption map[string]any

func (so ScriptGojaResultOption) ResultColumns() api.Columns {
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

func (ctx *GojaContext) Run() (any, error) {
	if v, err := ctx.vm.RunProgram(ctx.sc); err != nil {
		return nil, err
	} else {
		return v.Export(), nil
	}
}

func (ctx *GojaContext) consoleLog(level Level) func(args ...goja.Value) {
	return func(args ...goja.Value) {
		params := []any{}
		for _, value := range args {
			val := value.Export()
			if v, ok := val.(map[string]any); ok {
				m, _ := json.Marshal(v)
				params = append(params, string(m))
			} else {
				params = append(params, val)
			}
		}
		ctx.node.task._log(level, params...)
	}
}

func (ctx *GojaContext) gojaFuncYield(values ...goja.Value) {
	var v_key goja.Value
	if inflight := ctx.node.Inflight(); inflight != nil {
		v_key = ctx.vm.ToValue(inflight.key)
	}
	if v_key == nil {
		v_key = ctx.vm.ToValue(ctx.yieldCount)
	}
	ctx.yield(v_key, values)
}

func (ctx *GojaContext) gojaFuncYieldKey(key goja.Value, values ...goja.Value) {
	ctx.yield(key, values)
}

func (ctx *GojaContext) gojaFuncYieldArray(values goja.Value) {
	obj := values.ToObject(ctx.vm)
	var arr []goja.Value
	ctx.vm.ExportTo(obj, &arr)
	ctx.gojaFuncYield(arr...)
}

func (ctx *GojaContext) yield(argKey goja.Value, args []goja.Value) {
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

func (ctx *GojaContext) gojaFuncRequest(reqUrl string, reqOpt map[string]any) goja.Value {
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
	requestObj.Set("do", func(callback goja.Callable) goja.Value {
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
		responseObj.Set("error", func() goja.Value {
			if httpErr == nil {
				return goja.Undefined()
			}
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", httpErr.Error()))
		})
		bodyFunc := func(typ string) func(goja.Callable) goja.Value {
			return func(callback goja.Callable) goja.Value {
				if httpErr != nil {
					return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", httpErr.Error()))
				}
				if httpResponse == nil {
					return goja.Undefined()
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
						if _, e := callback(goja.Undefined(), ctx.vm.ToValue(s)); e != nil {
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
						if _, e := callback(goja.Undefined(), value); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
						}
					}
				case "text":
					if b, err := io.ReadAll(httpResponse.Body); err == nil {
						s := ctx.vm.ToValue(string(b))
						if _, e := callback(goja.Undefined(), s); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
						}
					}
				case "blob":
					if b, err := io.ReadAll(httpResponse.Body); err == nil {
						s := ctx.vm.ToValue(string(b))
						if _, e := callback(goja.Undefined(), s); e != nil {
							return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
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
			return ctx.vm.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
		}
		return goja.Undefined()
	})
	return requestObj
}

func (ctx *GojaContext) gojaFuncPublisher(optObj map[string]any) goja.Value {
	var cname string
	if len(optObj) > 0 {
		// parse db options `$.publisher({bridge: "name"})`
		if br, ok := optObj["bridge"]; ok {
			cname = br.(string)
		}
	}
	br, err := bridge.GetBridge(cname)
	if err != nil || br == nil {
		return ctx.vm.NewGoError(fmt.Errorf("publisher: bridge '%s' not found", cname))
	}

	ret := ctx.vm.NewObject()
	if mqttC, ok := br.(*bridge.MqttBridge); ok {
		ret.Set("publish", func(topic string, payload any) goja.Value {
			flag, err := mqttC.Publish(topic, payload)
			if err != nil {
				return ctx.vm.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
			}
			return ctx.vm.ToValue(flag)
		})
	} else if natsC, ok := br.(*bridge.NatsBridge); ok {
		ret.Set("publish", func(subject string, payload any) goja.Value {
			flag, err := natsC.Publish(subject, payload)
			if err != nil {
				return ctx.vm.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
			}
			return ctx.vm.ToValue(flag)
		})
	} else {
		return ctx.vm.NewGoError(fmt.Errorf("publisher: bridge '%s' not supported", cname))
	}

	return ret
}

func (ctx *GojaContext) gojaFuncDB(optObj map[string]any) goja.Value {
	var node = ctx.node
	var db = ctx.vm.NewObject()

	var bridgeName string
	if len(optObj) > 0 {
		// parse db options `$.db({bridge: "name"})`
		if br, ok := optObj["bridge"]; ok {
			bridgeName = br.(string)
		}
	}

	// $.db().query(sql, params...).next(function(row) {...})
	db.Set("query", func(sqlText string, params ...any) goja.Value {
		queryObj := ctx.vm.NewObject()
		queryObj.Set("yield", func() error {
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
				return fmt.Errorf("DBError %s", err.Error())
			}
			defer conn.Close()

			rows, err := conn.Query(node.task.ctx, sqlText, params...)
			if err != nil {
				node.task.Cancel()
				return fmt.Errorf("DBError %s", err.Error())
			}
			defer rows.Close()

			cols, _ := rows.Columns()
			// set headers
			types := []string{}
			for _, col := range cols {
				types = append(types, string(col.DataType))
			}
			var opts = ScriptGojaResultOption{
				"columns": cols.Names(),
				"types":   types,
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
			return nil
		})

		queryObj.Set("forEach", func(callback goja.Callable) goja.Value {
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
				return ctx.vm.NewGoError(fmt.Errorf("DBError %s", err.Error()))
			}
			defer conn.Close()

			rows, err := conn.Query(node.task.ctx, sqlText, params...)
			if err != nil {
				node.task.Cancel()
				return ctx.vm.NewGoError(fmt.Errorf("DBError %s", err.Error()))
			}
			defer rows.Close()
			for rows.Next() {
				cols, _ := rows.Columns()
				values, _ := cols.MakeBuffer()
				rows.Scan(values...)
				if flag, e := callback(goja.Undefined(), ctx.vm.ToValue(values)); e != nil {
					return ctx.vm.NewGoError(fmt.Errorf("DBError %s", e.Error()))
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
	db.Set("exec", func(sqlText string, params ...any) goja.Value {
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
			return ctx.vm.NewGoError(fmt.Errorf("DBError %s", err.Error()))
		}
		defer conn.Close()

		result := conn.Exec(node.task.ctx, sqlText, params...)
		if err = result.Err(); err != nil {
			return ctx.vm.NewGoError(fmt.Errorf("DBError %s", err.Error()))
		}
		ret := result.RowsAffected()
		return ctx.vm.ToValue(ret)
	})

	return db
}

func (ctx *GojaContext) gojaFuncSet(name string, value goja.Value) goja.Value {
	if inf := ctx.node.Inflight(); inf != nil {
		inf.SetVariable(name, value.Export())
	}
	return goja.Undefined()
}

func (ctx *GojaContext) gojaFuncGet(name string) goja.Value {
	if inf := ctx.node.Inflight(); inf != nil {
		if v, err := inf.GetVariable("$" + name); err != nil {
			return ctx.vm.NewGoError(fmt.Errorf("SCRIPT %s", err.Error()))
		} else {
			return ctx.vm.ToValue(v)
		}
	}
	return goja.Undefined()
}
