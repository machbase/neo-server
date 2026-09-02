package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
)

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/db")
	exports := module.Get("exports").(*goja.Object)

	// db = new dbms.Client()
	exports.Set("Client", new_client(ctx, rt))
}

func new_client(ctx context.Context, rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		opts := ClientOptions{}
		if len(call.Arguments) > 0 {
			if err := rt.ExportTo(call.Arguments[0], &opts); err != nil {
				panic(rt.NewGoError(err))
			}
		}
		// Resolve the user scope for this connection, in order of precedence:
		//  1. explicit `user` option: lets the script author declare scope
		//     themselves, which entry points with no wired context (CLI jsh,
		//     cgi-bin) must use since there is no ambient identity to fall back on.
		//  2. ctx-derived scope (TQL SCRIPT({}), CLI machbase-neo shell; see
		//     model.ContextWithUserScope/ContextWithUserScopeFunc).
		//  3. "sys" fallback.
		scope := model.UserScope{User: "sys"}
		if opts.User != "" {
			scope = model.UserScope{User: opts.User}
		} else if ctxScope, ok := model.UserScopeFromContext(ctx); ok && ctxScope.User != "" {
			scope = ctxScope
		}
		client := NewClientWithOptions(ctx, scope, rt, opts)
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
	// User, when set, overrides the ambient (context-derived) user scope for
	// this client's default-pool/append connections. See new_client.
	User string `json:"user"`
}

type Client struct {
	ctx              context.Context `json:"-"`
	rt               *goja.Runtime   `json:"-"`
	db               *sql.DB         `json:"-"`
	supportAppend    bool            `json:"-"`
	scope            model.UserScope `json:"-"`
	BridgeName       string          `json:"bridge"`
	Driver           string          `json:"driver"`
	LowerCaseColumns bool            `json:"lowerCaseColumns"`
}

func NewClient(ctx context.Context, scope model.UserScope, rt *goja.Runtime, optValue []goja.Value) *Client {
	opts := ClientOptions{}
	if len(optValue) > 0 {
		if err := rt.ExportTo(optValue[0], &opts); err != nil {
			panic(rt.NewGoError(err))
		}
	}
	return NewClientWithOptions(ctx, scope, rt, opts)
}

func NewClientWithOptions(ctx context.Context, scope model.UserScope, rt *goja.Runtime, opts ClientOptions) *Client {
	ret := &Client{
		ctx:              context.Background(),
		rt:               rt,
		scope:            scope,
		BridgeName:       opts.BridgeName,
		Driver:           opts.Driver,
		LowerCaseColumns: opts.LowerCaseColumns,
		supportAppend:    true,
	}
	if opts.BridgeName != "" {
		if db, err := bridge.ResolveSqlDB(ctx, scope, opts.BridgeName); err == nil {
			ret.db = db
		} else {
			panic(rt.NewGoError(err))
		}
	} else if opts.Driver != "" {
		if db, err := connector.NewWithDataSource(opts.Driver, opts.DataSource); err == nil {
			ret.db = db
		} else {
			panic(rt.NewGoError(err))
		}
	} else {
		if db, err := spi.Pool(scope.User); err == nil {
			ret.db = db
		} else {
			panic(rt.NewGoError(err))
		}
		if ret.db == nil {
			panic(rt.ToValue("dbms: no database"))
		}
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
	if len(call.Arguments) > 0 {
		panic(c.rt.NewGoError(fmt.Errorf("Connect() does not accept any arguments")))
	}
	conn, err := c.db.Conn(c.ctx)
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
	appender := &APPENDER{
		db: c.db,
	}

	dsn := spi.DefaultDSN(map[string]string{"user": spi.DSNUser(c.db.scope.User), "db": "MACHBASEDB"})
	ap := &client.Appender{}
	err := ap.Connect(c.db.ctx, dsn, tableName, columns...)
	if err != nil {
		panic(c.db.rt.ToValue(err.Error()))
	}
	appender.appender = ap
	ret := c.db.rt.NewObject()
	ret.Set("close", appender.Close)
	ret.Set("append", appender.Append)
	ret.Set("result", appender.Result)
	return ret
}

type APPENDER struct {
	db            *Client
	appender      *client.Appender
	success       int64
	fail          int64
	closer        func() error
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
	if apd.closer != nil {
		if err := apd.closer(); err != nil {
			panic(apd.db.rt.ToValue(err.Error()))
		}
		apd.closer = nil
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
	conn *sql.Conn

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

	meta := client.Meta{}
	ctx := context.WithValue(c.db.ctx, client.MetaKey, &meta)
	result, err := c.conn.ExecContext(ctx, sqlText, params...)
	if err != nil {
		panic(c.db.rt.NewGoError(err))
	}
	affected, err := result.RowsAffected()
	if err != nil {
		panic(c.db.rt.NewGoError(err))
	}
	var message string
	if message = meta.Message(); message == "" {
		sqlType := spi.DetectSQLStatementType(sqlText)
		message = spi.MakeUserMessage(sqlType, affected)
	}
	return c.db.rt.ToValue(map[string]any{
		"message":      message,
		"rowsAffected": affected,
	})
}

func (c *CONN) jsQueryRow(call goja.FunctionCall) goja.Value {
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

	ret := c.db.rt.NewObject()
	rows, err := c.conn.QueryContext(c.db.ctx, sqlText, params...)
	if err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", goja.Undefined())
		return ret
	}
	defer rows.Close()
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("columns", func() goja.Value { return goja.Null() })
		ret.Set("columnNames", func() goja.Value { return goja.Null() })
		ret.Set("columnTypes", func() goja.Value { return goja.Null() })
		ret.Set("values", goja.Null())
		return ret
	}

	names := make([]string, len(columnTypes))
	types := make([]string, len(columnTypes))
	for i, col := range columnTypes {
		names[i] = col.Name()
		types[i] = col.DatabaseTypeName()
		if c.db.LowerCaseColumns {
			names[i] = strings.ToLower(col.Name())
		} else {

		}
	}

	ret.Set("columns", func() goja.Value {
		return c.db.rt.ToValue(map[string]any{
			"columns": names,
			"types":   types,
		})
	})
	ret.Set("columnNames", func() goja.Value { return c.db.rt.ToValue(names) })
	ret.Set("columnTypes", func() goja.Value { return c.db.rt.ToValue(types) })

	buff := spi.MakeBuffer(columnTypes)
	if !rows.Next() {
		ret.Set("error", c.db.rt.ToValue("no rows found"))
		ret.Set("values", goja.Null())
		return ret
	}
	if err := rows.Scan(buff...); err != nil {
		ret.Set("error", c.db.rt.ToValue(err.Error()))
		ret.Set("values", goja.Null())
		return ret
	}
	values := c.db.rt.NewObject()
	for i, v := range buff {
		if v == nil {
			values.Set(names[i], goja.Null())
		} else {
			values.Set(names[i], client.Unbox(v))
		}
	}
	ret.Set("values", values)
	ret.Set("error", goja.Null())
	return ret
}

func (c *CONN) QueryRow(call goja.FunctionCall) *sql.Row {
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

	return c.conn.QueryRowContext(c.db.ctx, sqlText, params...)
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
	if dbRows, dbErr := c.conn.QueryContext(c.db.ctx, sqlText, params...); dbErr != nil {
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
	conn   *sql.Conn
	rows   *sql.Rows
	cols   []*sql.ColumnType
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
		types, _ := r.rows.ColumnTypes()
		r.cols = types
	}
}

func (r *ROWS) jsColumns(call goja.FunctionCall) goja.Value {
	if r.rows == nil {
		panic(r.db.rt.ToValue("invalid rows"))
	}
	r.ensureColumns()
	names := make([]string, len(r.cols))
	types := make([]string, len(r.cols))
	for i, col := range r.cols {
		names[i] = col.Name()
		types[i] = col.DatabaseTypeName()
	}

	return r.db.rt.ToValue(map[string]any{
		"columns": names,
		"types":   types,
	})
}

func (r *ROWS) jsColumnNames(call goja.FunctionCall) goja.Value {
	return r.db.rt.ToValue(r.ColumnNames(call))
}

func (r *ROWS) ColumnNames(call goja.FunctionCall) []string {
	r.ensureColumns()
	names := make([]string, len(r.cols))
	for i, col := range r.cols {
		names[i] = col.Name()
	}
	return names
}

func (r *ROWS) jsColumnTypes(call goja.FunctionCall) goja.Value {
	return r.db.rt.ToValue(r.ColumnTypes(call))
}

func (r *ROWS) ColumnTypes(call goja.FunctionCall) []string {
	r.ensureColumns()
	types := make([]string, len(r.cols))
	for i, col := range r.cols {
		types[i] = col.DatabaseTypeName()
	}
	return types
}

func (r *ROWS) jsNext(call goja.FunctionCall) goja.Value {
	var values []any
	if values = r.Next(call); len(values) == 0 {
		return goja.Null()
	}
	r.ensureColumns()

	var vm = r.db.rt
	var rec = vm.NewObject()
	for i, col := range r.cols {
		if i < len(values) {
			rec.Set(col.Name(), vm.ToValue(client.Unbox(values[i])))
		} else {
			rec.Set(col.Name(), goja.Null())
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
	values := spi.MakeBuffer(r.cols)
	r.rows.Scan(values...)
	r.rownum++
	for i, v := range values {
		if v == nil {
			continue
		}
		values[i] = client.Unbox(v)
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
