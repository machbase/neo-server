package maps

import (
	"database/sql"

	"github.com/d5/tengo/v2"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/pkg/errors"
)

func tengof_bridge(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		var cname string
		if len(args) == 1 {
			if v, ok := args[0].(*tengo.String); ok {
				cname = v.Value
			}
		}
		if len(cname) == 0 {
			return nil, tengo.ErrInvalidArgumentType{Name: "bridge name", Expected: "string"}
		}
		br, err := bridge.GetBridge(cname)
		if err != nil {
			return tengo.UndefinedValue, err
		}
		if sqlC, ok := br.(bridge.SqlBridge); ok {
			conn, err := sqlC.Connect(ctx)
			if err != nil {
				return nil, err
			}
			ctx.LazyClose(conn)
			return &sqlBridge{ctx: ctx, conn: conn, name: cname}, nil
		} else if mqttC, ok := br.(bridge.MqttBridge); ok {
			return &pubBridge{
				ctx:       ctx,
				name:      cname,
				publisher: mqttC,
			}, nil
		}
		return nil, nil
	}
}

type Publisher interface {
	Publish(topic string, payload any) (bool, error)
}

type pubBridge struct {
	tengo.ObjectImpl
	ctx       *context.Context
	name      string
	publisher Publisher
}

func (c *pubBridge) TypeName() string {
	return "bridge:publisher"
}

func (c *pubBridge) String() string {
	return "bridge:publisher:" + c.name
}

func (c *pubBridge) Copy() tengo.Object {
	return &pubBridge{ctx: c.ctx, name: c.name, publisher: c.publisher}
}

func (c *pubBridge) IndexGet(index tengo.Object) (tengo.Object, error) {
	if o, ok := index.(*tengo.String); ok {
		switch o.Value {
		case "publish":
			return &tengo.UserFunction{
				Name: "publish", Value: pubBridge_publish(c),
			}, nil
		default:
			return nil, tengo.ErrInvalidIndexOnError
		}
	} else {
		return nil, nil
	}
}

func pubBridge_publish(c *pubBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) != 2 {
			return nil, tengo.ErrWrongNumArguments
		}
		topic, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "topic", Expected: "string", Found: args[0].TypeName()}
		}
		payload := tengoObjectToAny(args[1])

		ok, err := c.publisher.Publish(topic, payload)
		if err != nil {
			return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
		}
		return tengo.FromInterface(ok)
	}
}

type sqlBridge struct {
	tengo.ObjectImpl
	ctx  *context.Context
	name string
	conn *sql.Conn
}

func (c *sqlBridge) TypeName() string {
	return "bridge:sql"
}

func (c *sqlBridge) String() string {
	return "bridge:sql:" + c.name
}

func (c *sqlBridge) Copy() tengo.Object {
	return &sqlBridge{ctx: c.ctx, conn: c.conn, name: c.name}
}

func (c *sqlBridge) IndexGet(index tengo.Object) (tengo.Object, error) {
	if o, ok := index.(*tengo.String); ok {
		switch o.Value {
		case "exec":
			return &tengo.UserFunction{
				Name: "exec", Value: sqlBridge_exec(c),
			}, nil
		case "query":
			return &tengo.UserFunction{
				Name: "query", Value: sqlBridge_query(c),
			}, nil
		case "queryRow":
			return &tengo.UserFunction{
				Name: "queryRow", Value: sqlBridge_queryRow(c),
			}, nil
		case "close":
			return &tengo.UserFunction{
				Name: "close",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if c.conn == nil {
						return nil, nil
					}
					defer c.ctx.CancelClose(c.conn)
					if err := c.conn.Close(); err != nil {
						return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
					}
					return nil, nil
				},
			}, nil
		default:
			return nil, tengo.ErrInvalidIndexOnError
		}
	} else {
		return nil, nil
	}
}

func sqlBridge_exec(c *sqlBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "sqlText", Expected: "string", Found: args[0].TypeName()}
		}
		params := tengoSliceToAnySlice(args[1:])
		result, err := c.conn.ExecContext(c.ctx, queryText, params...)
		if err != nil {
			return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
		}
		return &sqlResult{ctx: c.ctx, result: result}, nil
	}
}

func sqlBridge_query(c *sqlBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "sqlText", Expected: "string", Found: args[0].TypeName()}
		}
		params := tengoSliceToAnySlice(args[1:])
		rows, err := c.conn.QueryContext(c.ctx, queryText, params...)
		if err != nil {
			return nil, errors.Wrap(err, "query failed")
		}
		c.ctx.LazyClose(rows)
		return &sqlRows{ctx: c.ctx, rows: rows}, nil
	}
}

func sqlBridge_queryRow(c *sqlBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "sqlText", Expected: "string", Found: args[0].TypeName()}
		}
		params := tengoSliceToAnySlice(args[1:])
		rows, err := c.conn.QueryContext(c.ctx, queryText, params...)
		if err != nil {
			return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
		}
		defer rows.Close()
		if !rows.Next() {
			return tengo.UndefinedValue, nil
		}
		columns, err := rows.Columns()
		if err != nil {
			return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
		}
		values := make([]any, len(columns))
		for i := range values {
			values[i] = new(string)
		}
		err = rows.Scan(values...)
		if err != nil {
			return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
		}
		ret := &tengo.ImmutableMap{Value: map[string]tengo.Object{}}
		for i, val := range values {
			col := columns[i]
			ret.Value[col] = &tengo.String{Value: *(val.(*string))}
		}
		return ret, nil
	}
}

type sqlResult struct {
	tengo.ObjectImpl
	ctx    *context.Context
	result sql.Result
}

func (r *sqlResult) TyepName() string {
	return "connector:sql-result"
}

func (r *sqlResult) String() string {
	return "connector:sql-result"
}

func (r *sqlResult) Copy() tengo.Object {
	return &sqlResult{ctx: r.ctx, result: r.result}
}

func (r *sqlResult) IndexGet(index tengo.Object) (tengo.Object, error) {
	s, ok := index.(*tengo.String)
	if !ok {
		return nil, tengo.ErrIndexOutOfBounds
	}
	switch s.Value {
	case "lastInsertId":
		if id, err := r.result.LastInsertId(); err != nil {
			return nil, err
		} else {
			return &tengo.Int{Value: id}, nil
		}
	case "rowsAffected":
		if num, err := r.result.RowsAffected(); err != nil {
			return nil, err
		} else {
			return &tengo.Int{Value: num}, nil
		}
	default:
		return nil, tengo.ErrInvalidIndexOnError
	}
}

type sqlRows struct {
	tengo.ObjectImpl
	ctx  *context.Context
	rows *sql.Rows
}

func (c *sqlRows) TypeName() string {
	return "connector:sql-rows"
}

func (c *sqlRows) String() string {
	return "connector:sql-rows"
}

func (c *sqlRows) Copy() tengo.Object {
	return &sqlRows{ctx: c.ctx, rows: c.rows}
}

func (c *sqlRows) IndexGet(index tengo.Object) (tengo.Object, error) {
	s, ok := index.(*tengo.String)
	if !ok {
		return nil, tengo.ErrIndexOutOfBounds
	}

	switch s.Value {
	case "next":
		return &tengo.UserFunction{Name: "next", Value: func(args ...tengo.Object) (tengo.Object, error) {
			if c.rows.Next() {
				return tengo.TrueValue, nil
			} else {
				return tengo.FalseValue, nil
			}
		}}, nil
	case "scan":
		columns, err := c.rows.Columns()
		if err != nil {
			return nil, err
		}
		values := make([]any, len(columns))
		for i := range values {
			values[i] = new(string)
		}
		return &tengo.UserFunction{Name: "scan", Value: func(args ...tengo.Object) (tengo.Object, error) {
			c.rows.Scan(values...)
			ret := &tengo.ImmutableMap{Value: map[string]tengo.Object{}}
			for i, val := range values {
				col := columns[i]
				ret.Value[col] = &tengo.String{Value: *(val.(*string))}
			}
			return ret, nil
		}}, nil
	case "close":
		return &tengo.UserFunction{Name: "close", Value: func(args ...tengo.Object) (tengo.Object, error) {
			// fmt.Println("rows.close()")
			err := c.rows.Close()
			defer c.ctx.CancelClose(c.rows)
			return nil, err
		}}, nil
	default:
		return nil, tengo.ErrInvalidIndexOnError
	}
}
