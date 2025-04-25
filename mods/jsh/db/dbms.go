package db

import (
	"context"
	"io"
	"strings"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
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
		db := NewClient(ctx, rt, call.Arguments)
		ret := rt.NewObject()
		ret.Set("connect", db.jsConnect)
		return ret
	}
}

type Client struct {
	ctx              context.Context `json:"-"`
	rt               *js.Runtime     `json:"-"`
	db               api.Database    `json:"-"`
	BridgeName       string          `json:"bridge"`
	LowerCaseColumns bool            `json:"lowerCaseColumns"`
}

func NewClient(ctx context.Context, rt *js.Runtime, optValue []js.Value) *Client {
	opts := struct {
		BridgeName       string `json:"bridge"`
		LowerCaseColumns bool   `json:"lowerCaseColumns"`
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
		LowerCaseColumns: opts.LowerCaseColumns,
	}
	if opts.BridgeName == "" {
		ret.db = api.Default()
		if ret.db == nil {
			panic(rt.ToValue("dbms: no database"))
		}
	} else {
		if db, err := connector.New(opts.BridgeName); err == nil {
			ret.db = db
		} else {
			panic(rt.NewGoError(err))
		}
	}

	return ret
}

func (c *Client) jsConnect(call js.FunctionCall) js.Value {
	connection := c.Connect(call)
	ret := c.rt.NewObject()
	ret.Set("close", connection.Close)
	ret.Set("exec", connection.Exec)
	ret.Set("query", connection.jsQuery)
	return ret
}

func (c *Client) Connect(call js.FunctionCall) *CONN {
	opts := struct {
		ConnStr string `json:"conn"`
	}{
		ConnStr: "",
	}
	if len(call.Arguments) > 0 {
		if err := c.rt.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(c.rt.NewGoError(err))
		}
	}
	var conn api.Conn
	var err error
	if c.BridgeName == "" {
		// TODO: fix this with actual user name
		conn, err = c.db.Connect(c.ctx, api.WithTrustUser("sys"))
	} else {
		conn, err = c.db.Connect(c.ctx)
	}
	if err != nil {
		panic(c.rt.NewGoError(err))
	}

	connection := &CONN{
		db:   c,
		conn: conn,
	}
	if cleaner, ok := c.ctx.(Cleaner); ok {
		tok := cleaner.AddCleanup(func(out io.Writer) {
			if connection.conn != nil {
				io.WriteString(out, "WARNING: db connection not closed!!!\n")
				connection.conn.Close()
			}
		})
		connection.cancelCleaner = func() {
			cleaner.RemoveCleanup(tok)
		}
	}
	return connection
}

type Cleaner interface {
	AddCleanup(func(io.Writer)) int64
	RemoveCleanup(int64)
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

	if cleaner, ok := c.db.ctx.(Cleaner); ok {
		tok := cleaner.AddCleanup(func(out io.Writer) {
			if rows.rows != nil {
				io.WriteString(out, "WARNING: db rows not closed!!!\n")
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
