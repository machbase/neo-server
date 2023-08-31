package tql

import (
	"context"
	"database/sql"

	"github.com/d5/tengo/v2"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/pkg/errors"
)

func tengof_print(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		printArgs, err := tengof_getPrintArgs(args...)
		if err != nil {
			return nil, err
		}
		node.task.Log(printArgs...)
		return nil, nil
	}
}

func tengof_printf(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		numArgs := len(args)
		if numArgs == 0 {
			return nil, tengo.ErrWrongNumArguments
		}
		format, ok := args[0].(*tengo.String)
		if !ok {
			return nil, tengo.ErrInvalidArgumentType{
				Name:     "format",
				Expected: "string",
				Found:    args[0].TypeName(),
			}
		}
		if numArgs == 1 {
			node.task.Log(format)
			return nil, nil
		}
		s, err := tengo.Format(format.Value, args[1:]...)
		if err != nil {
			return nil, err
		}
		node.task.Log(s)
		return nil, nil
	}
}

func tengof_getPrintArgs(args ...tengo.Object) ([]interface{}, error) {
	var printArgs []interface{}
	l := 0
	for _, arg := range args {
		s, _ := tengo.ToString(arg)
		slen := len(s)
		// make sure length does not exceed the limit
		if l+slen > tengo.MaxStringLen {
			return nil, tengo.ErrStringLimit
		}
		l += slen
		printArgs = append(printArgs, s)
	}
	return printArgs, nil
}

func tengof_bridge(node *Node) func(args ...tengo.Object) (tengo.Object, error) {
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
			var ctx context.Context
			if node.task != nil && node.task.ctx != nil {
				ctx = node.task.ctx
			} else {
				ctx = context.TODO()
			}
			conn, err := sqlC.Connect(ctx)
			if err != nil {
				return nil, err
			}
			node.AddCloser(conn)
			return &scSqlBridge{node: node, conn: conn, name: cname}, nil
		} else if mqttC, ok := br.(bridge.MqttBridge); ok {
			return &pubBridge{
				node:      node,
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
	node      *Node
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
	return &pubBridge{node: c.node, name: c.name, publisher: c.publisher}
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

type scSqlBridge struct {
	tengo.ObjectImpl
	node *Node
	name string
	conn *sql.Conn
}

func (c *scSqlBridge) TypeName() string {
	return "bridge:sql"
}

func (c *scSqlBridge) String() string {
	return "bridge:sql:" + c.name
}

func (c *scSqlBridge) Copy() tengo.Object {
	return &scSqlBridge{node: c.node, conn: c.conn, name: c.name}
}

func (c *scSqlBridge) IndexGet(index tengo.Object) (tengo.Object, error) {
	if o, ok := index.(*tengo.String); ok {
		switch o.Value {
		case "exec":
			return &tengo.UserFunction{
				Name: "exec", Value: scSqlBridge_exec(c),
			}, nil
		case "query":
			return &tengo.UserFunction{
				Name: "query", Value: scSqlBridge_query(c),
			}, nil
		case "queryRow":
			return &tengo.UserFunction{
				Name: "queryRow", Value: scSqlBridge_queryRow(c),
			}, nil
		case "close":
			return &tengo.UserFunction{
				Name: "close",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if c.conn == nil {
						return nil, nil
					}
					defer c.node.CancelCloser(c.conn)
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

func scSqlBridge_exec(c *scSqlBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "sqlText", Expected: "string", Found: args[0].TypeName()}
		}
		params := tengoSliceToAnySlice(args[1:])
		var ctx context.Context
		if c.node != nil && c.node.task != nil && c.node.task.ctx != nil {
			ctx = c.node.task.ctx
		} else {
			ctx = context.TODO()
		}
		result, err := c.conn.ExecContext(ctx, queryText, params...)
		if err != nil {
			return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
		}
		return &scSqlResult{ctx: c.node, result: result}, nil
	}
}

func scSqlBridge_query(c *scSqlBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "sqlText", Expected: "string", Found: args[0].TypeName()}
		}
		params := tengoSliceToAnySlice(args[1:])
		var ctx context.Context
		if c.node != nil && c.node.task != nil && c.node.task.ctx != nil {
			ctx = c.node.task.ctx
		} else {
			ctx = context.TODO()
		}
		rows, err := c.conn.QueryContext(ctx, queryText, params...)
		if err != nil {
			return nil, errors.Wrap(err, "query failed")
		}
		c.node.AddCloser(rows)
		return &scSqlRows{ctx: c.node, rows: rows}, nil
	}
}

func scSqlBridge_queryRow(c *scSqlBridge) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, tengo.ErrInvalidArgumentType{Name: "sqlText", Expected: "string", Found: args[0].TypeName()}
		}
		params := tengoSliceToAnySlice(args[1:])
		var ctx context.Context
		if c.node != nil && c.node.task != nil && c.node.task.ctx != nil {
			ctx = c.node.task.ctx
		} else {
			ctx = context.TODO()
		}
		rows, err := c.conn.QueryContext(ctx, queryText, params...)
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

type scSqlResult struct {
	tengo.ObjectImpl
	ctx    *Node
	result sql.Result
}

func (r *scSqlResult) TyepName() string {
	return "connector:sql-result"
}

func (r *scSqlResult) String() string {
	return "connector:sql-result"
}

func (r *scSqlResult) Copy() tengo.Object {
	return &scSqlResult{ctx: r.ctx, result: r.result}
}

func (r *scSqlResult) IndexGet(index tengo.Object) (tengo.Object, error) {
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

type scSqlRows struct {
	tengo.ObjectImpl
	ctx  *Node
	rows *sql.Rows
}

func (c *scSqlRows) TypeName() string {
	return "connector:sql-rows"
}

func (c *scSqlRows) String() string {
	return "connector:sql-rows"
}

func (c *scSqlRows) Copy() tengo.Object {
	return &scSqlRows{ctx: c.ctx, rows: c.rows}
}

func (c *scSqlRows) IndexGet(index tengo.Object) (tengo.Object, error) {
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
			defer c.ctx.CancelCloser(c.rows)
			return nil, err
		}}, nil
	default:
		return nil, tengo.ErrInvalidIndexOnError
	}
}
