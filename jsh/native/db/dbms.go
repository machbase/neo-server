package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/db")
	o := module.Get("exports").(*goja.Object)

	// db = new dbms.Client()
	o.Set("Client", new_client(rt))
}

func new_client(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		client := NewClient(rt, call.Arguments)
		ret := rt.NewObject()
		ret.Set("connect", client.jsConnect)
		ret.Set("supportAppend", client.supportAppend)
		return ret
	}
}

type ClientOptions struct {
	BridgeName       string `json:"bridge"`
	LowerCaseColumns bool   `json:"lowerCaseColumns"`
	Driver           string `json:"driver"`
	DataSource       string `json:"dataSource"`
}

type Client struct {
	ctx              context.Context     `json:"-"`
	rt               *goja.Runtime       `json:"-"`
	db               api.Database        `json:"-"`
	supportAppend    bool                `json:"-"`
	BridgeName       string              `json:"bridge"`
	Driver           string              `json:"driver"`
	ConnectOptions   []api.ConnectOption `json:"-"`
	LowerCaseColumns bool                `json:"lowerCaseColumns"`
}

func NewClient(rt *goja.Runtime, optValue []goja.Value) *Client {
	opts := ClientOptions{}
	if len(optValue) > 0 {
		if err := rt.ExportTo(optValue[0], &opts); err != nil {
			panic(rt.NewGoError(err))
		}
	}
	return NewClientWithOptions(rt, opts)
}

func NewClientWithOptions(rt *goja.Runtime, opts ClientOptions) *Client {
	ret := &Client{
		ctx:              context.Background(),
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

func (c *Client) WithContext(ctx context.Context) *Client {
	c.ctx = ctx
	return c
}

func (c *Client) jsConnect(call goja.FunctionCall) goja.Value {
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

func (c *Client) Connect(call goja.FunctionCall) *CONN {
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
		var username = "sys"
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
	return connection
}

// conn.append(table_name, columns...)
func (c *CONN) Appender(call goja.FunctionCall) goja.Value {
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
	rawAppender, err := c.conn.Appender(c.db.ctx, tableName)
	if err != nil {
		c.Close(goja.FunctionCall{})
		panic(c.db.rt.ToValue(err.Error()))
	}
	if len(columns) > 0 {
		rawAppender = rawAppender.WithInputColumns(columns...)
	}

	appender := &APPENDER{
		db:       c.db,
		appender: rawAppender,
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

func (apd *APPENDER) Close(call goja.FunctionCall) goja.Value {
	if apd.appender != nil {
		if s, f, err := apd.appender.Close(); err != nil {
			panic(apd.db.rt.ToValue(err.Error()))
		} else {
			apd.success = s
			apd.fail = f
		}
		apd.appender = nil
	}
	return goja.Undefined()
}

func (apd *APPENDER) Append(call goja.FunctionCall) goja.Value {
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
	return goja.Undefined()
}

func (apd *APPENDER) Result(call goja.FunctionCall) goja.Value {
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

func (c *CONN) Close(call goja.FunctionCall) goja.Value {
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
	return goja.Undefined()
}

func (c *CONN) Exec(call goja.FunctionCall) goja.Value {
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

func (c *CONN) jsQueryRow(call goja.FunctionCall) goja.Value {
	row := c.QueryRow(call)
	ret := c.db.rt.NewObject()
	if err := row.Err(); err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", goja.Undefined())
		return ret
	}

	columns, err := row.Columns()
	if err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("columns", func() goja.Value { return goja.Null() })
		ret.Set("columnNames", func() goja.Value { return goja.Null() })
		ret.Set("columnTypes", func() goja.Value { return goja.Null() })
		ret.Set("values", goja.Null())
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

	ret.Set("columns", func() goja.Value {
		return c.db.rt.ToValue(map[string]any{
			"columns": names,
			"types":   types,
		})
	})
	ret.Set("columnNames", func() goja.Value { return c.db.rt.ToValue(names) })
	ret.Set("columnTypes", func() goja.Value { return c.db.rt.ToValue(types) })

	buff, err := columns.MakeBuffer()
	if err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", goja.Null())
		return ret
	}
	if err := row.Scan(buff...); err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", goja.Null())
		return ret
	}
	values := c.db.rt.NewObject()
	for i, v := range buff {
		if v == nil {
			values.Set(names[i], goja.Null())
		} else {
			values.Set(names[i], api.Unbox(v))
		}
	}
	ret.Set("values", values)
	ret.Set("error", goja.Null())
	return ret
}

func (c *CONN) QueryRow(call goja.FunctionCall) api.Row {
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

func (c *CONN) jsQuery(call goja.FunctionCall) goja.Value {
	rows := c.Query(call)
	ret := c.db.rt.NewObject()
	ret.Set("close", rows.Close)
	ret.Set("next", rows.jsNext)
	ret.Set("columns", rows.jsColumns)
	ret.Set("columnNames", rows.jsColumnNames)
	ret.Set("columnTypes", rows.jsColumnTypes)
	ret.SetSymbol(goja.SymIterator, rows.jsIterator)
	return ret
}

func (c *CONN) Query(call goja.FunctionCall) *ROWS {
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

func (r *ROWS) Close(call goja.FunctionCall) goja.Value {
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
	return goja.Undefined()
}

func (r *ROWS) jsIterator(call goja.FunctionCall) goja.Value {
	ret := r.db.rt.NewObject()
	ret.Set("next", func(call goja.FunctionCall) goja.Value {
		r.ensureColumns()
		item := r.db.rt.NewObject()
		val := r.jsNext(goja.FunctionCall{})
		if goja.IsNull(val) {
			r.Close(goja.FunctionCall{})
			item.Set("done", true)
		} else {
			item.Set("value", val)
			item.Set("done", false)
		}
		return item
	})
	ret.Set("return", func(call goja.FunctionCall) goja.Value {
		r.Close(goja.FunctionCall{})
		return r.db.rt.ToValue(map[string]any{
			"done": true,
		})
	})
	ret.Set("throw", func(call goja.FunctionCall) goja.Value {
		r.Close(goja.FunctionCall{})
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

func (r *ROWS) jsColumns(call goja.FunctionCall) goja.Value {
	if r.rows == nil {
		panic(r.db.rt.ToValue("invalid rows"))
	}
	r.ensureColumns()
	return r.db.rt.ToValue(map[string]any{
		"columns": r.names,
		"types":   r.types,
	})
}

func (r *ROWS) jsColumnNames(call goja.FunctionCall) goja.Value {
	return r.db.rt.ToValue(r.ColumnNames(call))
}

func (r *ROWS) ColumnNames(call goja.FunctionCall) []string {
	r.ensureColumns()
	return r.names
}

func (r *ROWS) jsColumnTypes(call goja.FunctionCall) goja.Value {
	return r.db.rt.ToValue(r.ColumnTypes(call))
}

func (r *ROWS) ColumnTypes(call goja.FunctionCall) []string {
	r.ensureColumns()
	return r.types
}

func (r *ROWS) jsNext(call goja.FunctionCall) goja.Value {
	var values []any
	if values = r.Next(call); len(values) == 0 {
		return goja.Null()
	}
	r.ensureColumns()

	var vm = r.db.rt
	var rec = vm.NewObject()
	for i, col := range r.names {
		if i < len(values) {
			rec.Set(col, vm.ToValue(api.Unbox(values[i])))
		} else {
			rec.Set(col, goja.Null())
		}
	}

	rec.SetSymbol(goja.SymIterator, func(call goja.FunctionCall) goja.Value {
		iter := newIterableObject(vm, values)
		return iter.obj
	})

	return rec
}

func (r *ROWS) Next(call goja.FunctionCall) []any {
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
	vm     *goja.Runtime
	values []any
	obj    *goja.Object
}

func newIterableObject(vm *goja.Runtime, values []any) *iterableObject {
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

func (it *iterableObject) Next(call goja.FunctionCall) goja.Value {
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

func (it *iterableObject) Return(call goja.FunctionCall) goja.Value {
	it.values = nil
	return it.vm.ToValue(map[string]any{
		"done": true,
	})
}

func (it *iterableObject) Throw(call goja.FunctionCall) goja.Value {
	it.values = nil
	return it.vm.ToValue(map[string]any{
		"done": true,
	})
}
