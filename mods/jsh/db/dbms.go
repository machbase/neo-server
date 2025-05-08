package db

import (
	"context"
	"fmt"
	"io"
	"strings"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/mods/jsh/builtin"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/db")
		o := module.Get("exports").(*js.Object)

		// db = new dbms.Client()
		o.Set("Client", new_client(ctx, rt))
	}
}

func new_client(ctx context.Context, rt *js.Runtime) func(call js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		client := NewClient(ctx, rt, call.Arguments)
		ret := rt.NewObject()
		ret.Set("connect", client.jsConnect)
		ret.Set("supportAppend", client.supportAppend)
		return ret
	}
}

type Client struct {
	ctx              context.Context     `json:"-"`
	rt               *js.Runtime         `json:"-"`
	db               api.Database        `json:"-"`
	supportAppend    bool                `json:"-"`
	BridgeName       string              `json:"bridge"`
	Driver           string              `json:"driver"`
	ConnectOptions   []api.ConnectOption `json:"-"`
	LowerCaseColumns bool                `json:"lowerCaseColumns"`
}

func NewClient(ctx context.Context, rt *js.Runtime, optValue []js.Value) *Client {
	opts := struct {
		BridgeName       string `json:"bridge"`
		LowerCaseColumns bool   `json:"lowerCaseColumns"`
		Driver           string `json:"driver"`
		DataSource       string `json:"dataSource"`
	}{
		BridgeName: "",
	}
	if len(optValue) > 0 {
		if err := rt.ExportTo(optValue[0], &opts); err != nil {
			panic(rt.NewGoError(err))
		}
	}
	ret := &Client{
		ctx:              ctx,
		rt:               rt,
		BridgeName:       opts.BridgeName,
		Driver:           opts.Driver,
		LowerCaseColumns: opts.LowerCaseColumns,
	}
	if opts.BridgeName != "" {
		if db, err := connector.New(opts.BridgeName); err == nil {
			ret.db = db
		} else {
			panic(rt.NewGoError(err))
		}
	} else if opts.Driver != "" {
		if db, opts, err := connector.NewWithDataSource(opts.Driver, opts.DataSource); err == nil {
			ret.db = db
			ret.ConnectOptions = opts
		} else {
			panic(rt.NewGoError(err))
		}
	} else {
		ret.db = api.Default()
		if ret.db == nil {
			panic(rt.ToValue("dbms: no database"))
		}
	}

	switch ret.db.(type) {
	case *machrpc.Client:
		ret.supportAppend = true
	case *machsvr.Database:
		ret.supportAppend = true
	case *machcli.Database:
		ret.supportAppend = true
	}
	return ret
}

func (c *Client) jsConnect(call js.FunctionCall) js.Value {
	connection := c.Connect(call)
	ret := c.rt.NewObject()
	ret.Set("close", connection.Close)
	ret.Set("exec", connection.Exec)
	ret.Set("query", connection.jsQuery)
	ret.Set("queryRow", connection.jsQueryRow)
	if c.supportAppend {
		ret.Set("appender", connection.Appender)
	}

	return ret
}

func (c *Client) Connect(call js.FunctionCall) *CONN {
	conf := struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}{}
	if len(call.Arguments) > 0 {
		if err := c.rt.ExportTo(call.Arguments[0], &conf); err != nil {
			panic(c.rt.NewGoError(err))
		}
	}
	var conn api.Conn
	var err error
	if c.BridgeName == "" && c.Driver == "" {
		var username string
		if jshCtx, ok := c.ctx.(builtin.JshContext); ok {
			username = jshCtx.Username()
		}
		if username == "" {
			username = "sys"
		}
		conn, err = c.db.Connect(c.ctx, api.WithTrustUser(username))
	} else {
		opts := append([]api.ConnectOption{}, c.ConnectOptions...)
		if conf.User != "" {
			opts = append(opts, api.WithPassword(conf.User, conf.Password))
		}
		conn, err = c.db.Connect(c.ctx, opts...)
	}
	if err != nil {
		panic(c.rt.NewGoError(err))
	}

	connection := &CONN{
		db:   c,
		conn: conn,
	}
	if cleaner, ok := c.ctx.(builtin.JshContext); ok {
		tok := cleaner.AddCleanup(func(out io.Writer) {
			if connection.conn != nil {
				io.WriteString(out, "forced db connection to close by cleanup\n")
				connection.conn.Close()
			}
		})
		connection.cancelCleaner = func() {
			cleaner.RemoveCleanup(tok)
		}
	}
	return connection
}

// conn.append(tablename, columns...)
func (c *CONN) Appender(call js.FunctionCall) js.Value {
	if !c.db.supportAppend {
		panic(c.db.rt.ToValue(fmt.Sprintf("%T append not supported", c.db)))
	}
	var tableName string
	if len(call.Arguments) > 0 {
		if err := c.db.rt.ExportTo(call.Arguments[0], &tableName); err != nil {
			panic(c.db.rt.ToValue(err.Error()))
		}
	}
	var columns = make([]string, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		if err := c.db.rt.ExportTo(call.Arguments[i], &columns[i-1]); err != nil {
			panic(c.db.rt.ToValue(err.Error()))
		}
	}
	appd, err := c.conn.Appender(c.db.ctx, tableName)
	if err != nil {
		c.Close(js.FunctionCall{})
		panic(c.db.rt.ToValue(err.Error()))
	}
	if len(columns) > 0 {
		appd = appd.WithInputColumns(columns...)
	}

	appender := &APPENDER{
		db:       c.db,
		appender: appd,
	}
	if cleaner, ok := c.db.ctx.(builtin.JshContext); ok {
		tok := cleaner.AddCleanup(func(out io.Writer) {
			if appender.appender != nil {
				io.WriteString(out, "forced db appender to close by cleanup\n")
				appender.Close(js.FunctionCall{})
			}
		})
		appender.cancelCleaner = func() {
			cleaner.RemoveCleanup(tok)
		}
	}
	ret := c.db.rt.NewObject()
	ret.Set("close", appender.Close)
	ret.Set("append", appender.Append)
	ret.Set("result", appender.Result)
	return ret
}

type APPENDER struct {
	db            *Client
	appender      api.Appender
	success       int64
	fail          int64
	cancelCleaner func()
}

func (apd *APPENDER) Close(call js.FunctionCall) js.Value {
	if apd.appender != nil {
		if s, f, err := apd.appender.Close(); err != nil {
			panic(apd.db.rt.ToValue(err.Error()))
		} else {
			apd.success = s
			apd.fail = f
		}
		apd.appender = nil
	}
	return js.Undefined()
}

func (apd *APPENDER) Append(call js.FunctionCall) js.Value {
	if apd.appender == nil {
		panic(apd.db.rt.ToValue("invalid appender"))
	}
	values := make([]any, len(call.Arguments))
	for i := 0; i < len(call.Arguments); i++ {
		if err := apd.db.rt.ExportTo(call.Arguments[i], &values[i]); err != nil {
			panic(apd.db.rt.ToValue(err.Error()))
		}
	}
	err := apd.appender.Append(values...)
	if err != nil {
		panic(apd.db.rt.ToValue(err.Error()))
	}
	return js.Undefined()
}

func (apd *APPENDER) Result(call js.FunctionCall) js.Value {
	ret := apd.db.rt.NewObject()
	ret.Set("success", apd.success)
	ret.Set("fail", apd.fail)
	return ret
}

type CONN struct {
	db   *Client
	conn api.Conn

	cancelCleaner func()
}

func (c *CONN) Close(call js.FunctionCall) js.Value {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return c.db.rt.NewGoError(err)
		}
		c.conn = nil
		if c.cancelCleaner != nil {
			c.cancelCleaner()
			c.cancelCleaner = nil
		}
	}
	return js.Undefined()
}

func (c *CONN) Exec(call js.FunctionCall) js.Value {
	var sqlText string
	var params []any

	if len(call.Arguments) == 0 {
		panic(c.db.rt.ToValue("missing arguments"))
	}
	sqlText = call.Arguments[0].String()
	params = make([]any, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		if err := c.db.rt.ExportTo(call.Arguments[i], &params[i-1]); err != nil {
			panic(c.db.rt.NewGoError(err))
		}
	}

	result := c.conn.Exec(c.db.ctx, sqlText, params...)
	if err := result.Err(); err != nil {
		panic(c.db.rt.NewGoError(err))
	}
	return c.db.rt.ToValue(map[string]any{
		"message":      result.Message(),
		"rowsAffected": result.RowsAffected(),
	})
}

func (c *CONN) jsQueryRow(call js.FunctionCall) js.Value {
	row := c.QueryRow(call)
	ret := c.db.rt.NewObject()
	if err := row.Err(); err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", js.Undefined())
		return ret
	}

	columns, err := row.Columns()
	if err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("columns", func() js.Value { return js.Null() })
		ret.Set("columnNames", func() js.Value { return js.Null() })
		ret.Set("columnTypes", func() js.Value { return js.Null() })
		ret.Set("values", js.Null())
		return ret
	}
	names := columns.Names()
	if c.db.LowerCaseColumns {
		for i, col := range names {
			names[i] = strings.ToLower(col)
		}
	}
	types := make([]string, len(columns))
	for i, col := range columns {
		types[i] = string(col.DataType)
	}

	ret.Set("columns", func() js.Value {
		return c.db.rt.ToValue(map[string]any{
			"columns": names,
			"types":   types,
		})
	})
	ret.Set("columnNames", func() js.Value { return c.db.rt.ToValue(names) })
	ret.Set("columnTypes", func() js.Value { return c.db.rt.ToValue(types) })

	buff, err := columns.MakeBuffer()
	if err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", js.Null())
		return ret
	}
	if err := row.Scan(buff...); err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", js.Null())
		return ret
	}
	values := c.db.rt.NewObject()
	for i, v := range buff {
		if v == nil {
			values.Set(names[i], js.Null())
		} else {
			values.Set(names[i], api.Unbox(v))
		}
	}
	ret.Set("values", values)
	ret.Set("error", js.Null())
	return ret
}

func (c *CONN) QueryRow(call js.FunctionCall) api.Row {
	var sqlText string
	var params []any

	if len(call.Arguments) == 0 {
		panic(c.db.rt.ToValue("missing arguments"))
	}
	sqlText = call.Arguments[0].String()
	params = make([]any, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		if err := c.db.rt.ExportTo(call.Arguments[i], &params[i-1]); err != nil {
			panic(c.db.rt.NewGoError(err))
		}
	}

	return c.conn.QueryRow(c.db.ctx, sqlText, params...)
}

func (c *CONN) jsQuery(call js.FunctionCall) js.Value {
	rows := c.Query(call)
	ret := c.db.rt.NewObject()
	ret.Set("close", rows.Close)
	ret.Set("next", rows.jsNext)
	ret.Set("columns", rows.jsColumns)
	ret.Set("columnNames", rows.jsColumnNames)
	ret.Set("columnTypes", rows.jsColumnTypes)
	ret.SetSymbol(js.SymIterator, rows.jsIterator)
	return ret
}

func (c *CONN) Query(call js.FunctionCall) *ROWS {
	var sqlText string
	var params []any

	if len(call.Arguments) == 0 {
		panic(c.db.rt.ToValue("missing arguments"))
	}
	sqlText = call.Arguments[0].String()
	params = make([]any, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		if err := c.db.rt.ExportTo(call.Arguments[i], &params[i-1]); err != nil {
			panic(c.db.rt.NewGoError(err))
		}
	}

	var rows *ROWS
	if dbRows, dbErr := c.conn.Query(c.db.ctx, sqlText, params...); dbErr != nil {
		panic(c.db.rt.NewGoError(dbErr))
	} else {
		rows = &ROWS{
			db:   c.db,
			conn: c.conn,
			rows: dbRows,
		}
	}

	if cleaner, ok := c.db.ctx.(builtin.JshContext); ok {
		tok := cleaner.AddCleanup(func(out io.Writer) {
			if rows.rows != nil {
				io.WriteString(out, "forced db rows to close by cleanup\n")
				rows.rows.Close()
			}
		})
		rows.cancelCleaner = func() {
			cleaner.RemoveCleanup(tok)
		}
	}

	return rows
}

type ROWS struct {
	db     *Client
	conn   api.Conn
	rows   api.Rows
	cols   api.Columns
	names  []string
	types  []string
	rownum int

	cancelCleaner func()
}

func (r *ROWS) Close(call js.FunctionCall) js.Value {
	if r.rows != nil {
		if err := r.rows.Close(); err != nil {
			return r.db.rt.NewGoError(err)
		}
		r.rows = nil
		if r.cancelCleaner != nil {
			r.cancelCleaner()
			r.cancelCleaner = nil
		}
	}
	return js.Undefined()
}

func (r *ROWS) jsIterator(call js.FunctionCall) js.Value {
	ret := r.db.rt.NewObject()
	ret.Set("next", func(call js.FunctionCall) js.Value {
		r.ensureColumns()
		item := r.db.rt.NewObject()
		val := r.jsNext(js.FunctionCall{})
		if js.IsNull(val) {
			r.Close(js.FunctionCall{})
			item.Set("done", true)
		} else {
			item.Set("value", val)
			item.Set("done", false)
		}
		return item
	})
	ret.Set("return", func(call js.FunctionCall) js.Value {
		r.Close(js.FunctionCall{})
		return r.db.rt.ToValue(map[string]any{
			"done": true,
		})
	})
	ret.Set("throw", func(call js.FunctionCall) js.Value {
		r.Close(js.FunctionCall{})
		return r.db.rt.ToValue(map[string]any{
			"done": true,
		})
	})
	return ret
}

func (r *ROWS) ensureColumns() {
	if r.cols == nil {
		if cols, err := r.rows.Columns(); err != nil {
			panic(r.db.rt.NewGoError(err))
		} else {
			r.cols = cols
		}
	}
	if r.names == nil {
		r.names = r.cols.Names()
		if r.db.LowerCaseColumns {
			for i, col := range r.names {
				r.names[i] = strings.ToLower(col)
			}
		}
		r.types = make([]string, len(r.cols))
		for i, col := range r.cols {
			r.types[i] = string(col.DataType)
		}
	}
}

func (r *ROWS) jsColumns(call js.FunctionCall) js.Value {
	if r.rows == nil {
		panic(r.db.rt.ToValue("invalid rows"))
	}
	r.ensureColumns()
	return r.db.rt.ToValue(map[string]any{
		"columns": r.names,
		"types":   r.types,
	})
}

func (r *ROWS) jsColumnNames(call js.FunctionCall) js.Value {
	return r.db.rt.ToValue(r.ColumnNames(call))
}

func (r *ROWS) ColumnNames(call js.FunctionCall) []string {
	r.ensureColumns()
	return r.names
}

func (r *ROWS) jsColumnTypes(call js.FunctionCall) js.Value {
	return r.db.rt.ToValue(r.ColumnTypes(call))
}

func (r *ROWS) ColumnTypes(call js.FunctionCall) []string {
	r.ensureColumns()
	return r.types
}

func (r *ROWS) jsNext(call js.FunctionCall) js.Value {
	var values []any
	if values = r.Next(call); len(values) == 0 {
		return js.Null()
	}
	r.ensureColumns()

	var vm = r.db.rt
	var rec = vm.NewObject()
	for i, col := range r.names {
		if i < len(values) {
			rec.Set(col, vm.ToValue(api.Unbox(values[i])))
		} else {
			rec.Set(col, js.Null())
		}
	}

	rec.SetSymbol(js.SymIterator, func(call js.FunctionCall) js.Value {
		iter := newIterableObject(vm, values)
		return iter.obj
	})

	return rec
}

func (r *ROWS) Next(call js.FunctionCall) []any {
	if r.rows == nil {
		panic(r.db.rt.ToValue("invalid rows"))
	}
	if !r.rows.Next() {
		return nil
	}
	values, err := r.cols.MakeBuffer()
	if err != nil {
		panic(r.db.rt.NewGoError(err))
	}
	r.rows.Scan(values...)
	r.rownum++
	for i, v := range values {
		if v == nil {
			continue
		}
		values[i] = api.Unbox(v)
	}
	return values
}

type iterableObject struct {
	vm     *js.Runtime
	values []any
	obj    *js.Object
}

func newIterableObject(vm *js.Runtime, values []any) *iterableObject {
	ret := &iterableObject{
		vm:     vm,
		values: values,
		obj:    vm.NewObject(),
	}
	ret.obj.Set("next", ret.Next)
	ret.obj.Set("return", ret.Return)
	ret.obj.Set("throw", ret.Throw)
	return ret
}

func (it *iterableObject) Next(call js.FunctionCall) js.Value {
	item := it.vm.NewObject()
	if len(it.values) == 0 {
		item.Set("done", true)
	} else {
		val := it.values[0]
		it.values = it.values[1:]
		item.Set("value", val)
		item.Set("done", false)
	}
	return item
}

func (it *iterableObject) Return(call js.FunctionCall) js.Value {
	it.values = nil
	return it.vm.ToValue(map[string]any{
		"done": true,
	})
}

func (it *iterableObject) Throw(call js.FunctionCall) js.Value {
	it.values = nil
	return it.vm.ToValue(map[string]any{
		"done": true,
	})
}
