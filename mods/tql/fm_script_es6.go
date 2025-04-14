package tql

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
	"github.com/paulmach/orb/geojson"
	"gonum.org/v1/gonum/stat"
)

const goja_ctx_key = "$goja_ctx$"

func (node *Node) fmScriptGoja(initCode string, mainCode string) (any, error) {
	var ctx *GojaContext
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

	if obj, ok := node.GetValue(goja_ctx_key); ok {
		if o, ok := obj.(*GojaContext); ok {
			ctx = o
		}
	}

	if ctx == nil {
		ctx, err = newGojaContext(node, initCode, mainCode)
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

func newGojaContext(node *Node, initCode string, mainCode string) (*GojaContext, error) {
	ctx := &GojaContext{
		node: node,
		vm:   goja.New(),
	}
	ctx.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", false))

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
			// do not use "recover()" here.
			// The related test case : TestScriptInterrupt()/js-timeout-finalize
			if f, ok := goja.AssertFunction(ctx.vm.Get("finalize")); ok {
				f(goja.Undefined())
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

	// set $.params[]
	var param any
	if node.task.params != nil {
		values := map[string]any{}
		for k, v := range node.task.params {
			if len(v) == 1 {
				values[k] = v[0]
			} else {
				values[k] = v
			}
		}
		param = ctx.vm.ToValue(values)
	} else {
		param = goja.Undefined()
	}
	ctx.obj.Set("params", param)

	// function $.yield(...)
	ctx.obj.Set("yield", ctx.gojaFuncYield)
	// function $.yieldKey(key, ...)
	ctx.obj.Set("yieldKey", ctx.gojaFuncYieldKey)
	// function $.yieldArray(array)
	ctx.obj.Set("yieldArray", ctx.gojaFuncYieldArray)
	// $.db()
	ctx.obj.Set("db", ctx.gojaFuncDB)
	// $.request()
	ctx.obj.Set("request", ctx.gojaFuncRequest)
	// $.geojson()
	ctx.obj.Set("geojson", ctx.gojaFuncGeoJSON)
	// $.system()
	ctx.obj.Set("system", ctx.gojaFuncSystem)
	// $.set()
	ctx.obj.Set("set", ctx.gojaFuncSet)
	// $.get()
	ctx.obj.Set("get", ctx.gojaFuncGet)
	// $.num()
	ctx.obj.Set("num", ctx.gojaFuncNum)

	ctx.node.task.AddShouldStopListener(func() {
		ctx.onceInterrupt.Do(func() {
			ctx.vm.Interrupt("interrupt")
		})
	})

	// init code
	if initCode != "" {
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

func (ctx *GojaContext) gojaFuncGeoJSON(value goja.Value) goja.Value {
	obj := value.ToObject(ctx.vm)
	if obj == nil {
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError requires a GeoJSON object, but got %q", value.ExportType()))
	}
	typeString := obj.Get("type")
	if typeString == nil {
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError missing a GeoJSON type"))
	}
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
	}
	var geoObj any
	switch typeString.String() {
	case "FeatureCollection":
		if geo, err := geojson.UnmarshalFeatureCollection(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	case "Feature":
		if geo, err := geojson.UnmarshalFeature(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
		if geo, err := geojson.UnmarshalGeometry(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	default:
		return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", "unsupported GeoJSON type"))
	}
	var _ = geoObj
	return obj
}

func (ctx *GojaContext) gojaFuncSystem() goja.Value {
	ret := ctx.vm.NewObject()
	// $.system().free_os_memory()
	ret.Set("free_os_memory", func() {
		debug.FreeOSMemory()
	})
	// $.system().gc()
	ret.Set("gc", func() {
		runtime.GC()
	})
	// $.system().now()
	ret.Set("now", func() goja.Value {
		return ctx.vm.ToValue(time.Now())
	})
	return ret
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

func (ctx *GojaContext) gojaFuncNum() goja.Value {
	ret := ctx.vm.NewObject()
	ret.Set("fft", gojaNumFFT(ctx))
	ret.Set("mean", gojaNumMean(ctx))
	ret.Set("quantile", gojaNumQuantile(ctx))
	ret.Set("simplex", gojaNumSimplex(ctx))
	ret.Set("stdDev", gojaNumStdDev(ctx))
	return ret
}

func gojaNumFFT(ctx *GojaContext) func(times []any, values []any) goja.Value {
	return func(times []any, values []any) goja.Value {
		ts := make([]time.Time, len(times))
		vs := make([]float64, len(values))
		for i, val := range times {
			switch v := val.(type) {
			case time.Time:
				ts[i] = v
			case *time.Time:
				ts[i] = *v
			default:
				return ctx.vm.NewGoError(fmt.Errorf("FFTError invalid %dth sample time, but %T", i, val))
			}
		}
		for i, val := range values {
			switch v := val.(type) {
			case float64:
				vs[i] = v
			case *float64:
				vs[i] = *v
			default:
				return ctx.vm.NewGoError(fmt.Errorf("FFTError invalid %dth sample value, but %T", i, val))
			}
		}
		xs, ys := fft.FastFourierTransform(ts, vs)
		return ctx.vm.ToValue(map[string]any{"x": xs, "y": ys})
	}
}

func gojaNumSimplex(ctx *GojaContext) func(seed int64) goja.Value {
	return func(seed int64) goja.Value {
		simplex := &GojaSimpleX{seed: seed}
		return ctx.vm.ToValue(simplex)
	}
}

type GojaSimpleX struct {
	seed int64
	gen  *opensimplex.Generator
}

func (sx *GojaSimpleX) Eval(dim ...float64) float64 {
	if sx.gen == nil {
		sx.gen = opensimplex.New(sx.seed)
	}
	return sx.gen.Eval(dim...)
}

func gojaNumQuantile(_ *GojaContext) func(p float64, x []float64) float64 {
	return func(p float64, x []float64) float64 {
		return stat.Quantile(p, stat.Empirical, x, nil)
	}
}

func gojaNumMean(_ *GojaContext) func(x []float64) float64 {
	return func(x []float64) float64 {
		return stat.Mean(x, nil)
	}
}

func gojaNumStdDev(_ *GojaContext) func(x []float64) float64 {
	return func(x []float64) float64 {
		return stat.StdDev(x, nil)
	}
}
