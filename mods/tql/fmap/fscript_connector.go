package fmap

import (
	"database/sql"
	"strings"

	"github.com/d5/tengo/v2"
	"github.com/machbase/neo-server/mods/connector"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/pkg/errors"
)

func tengof_connector(ctx *context.Context) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		var cname string
		if len(args) == 1 {
			if v, ok := args[0].(*tengo.String); ok {
				cname = v.Value
			}
		}
		if len(cname) == 0 {
			return nil, tengo.ErrInvalidArgumentType{Name: "connector name", Expected: "string"}
		}
		c, err := connector.SharedConnector(cname)
		if err != nil {
			return tengo.UndefinedValue, err
		}
		if sqlC, ok := c.(connector.SqlConnector); ok {
			conn, err := sqlC.Connect(ctx)
			if err != nil {
				return nil, err
			}
			return &sqlConnector{ctx: ctx, conn: conn, name: cname}, nil
		}
		return nil, nil
	}
}

type sqlConnector struct {
	tengo.ObjectImpl
	ctx  *context.Context
	name string
	conn *sql.Conn
}

func (c *sqlConnector) TypeName() string {
	return "connector:sql"
}

func (c *sqlConnector) String() string {
	return "connector:sql:" + c.name
}

func (c *sqlConnector) Copy() tengo.Object {
	return &sqlConnector{ctx: c.ctx, conn: c.conn, name: c.name}
}

func (c *sqlConnector) IndexGet(index tengo.Object) (tengo.Object, error) {
	if o, ok := index.(*tengo.String); ok {
		switch o.Value {
		case "query":
			return &tengo.UserFunction{
				Name: "query", Value: sqlConnector_query(c),
			}, nil
		default:
			return nil, nil
		}
	} else {
		return nil, nil
	}
}

func sqlConnector_query(c *sqlConnector) func(args ...tengo.Object) (tengo.Object, error) {
	return func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) < 1 {
			return nil, tengo.ErrWrongNumArguments
		}
		queryText, err := tengoObjectToString(args[0])
		if err != nil {
			return nil, errors.Wrap(err, "queryText missing")
		}
		params := tengoSliceToAnySlice(args[1:])
		rows, err := c.conn.QueryContext(c.ctx, queryText, params...)
		if err != nil {
			return nil, errors.Wrap(err, "query failed")
		}

		return &sqlResult{ctx: c.ctx, rows: rows}, nil
	}
}

type sqlResult struct {
	tengo.ObjectImpl
	ctx  *context.Context
	rows *sql.Rows
}

func (c *sqlResult) TypeName() string {
	return "connector:sql-rows"
}

func (c *sqlResult) String() string {
	return "connector:sql-rows"
}

func (c *sqlResult) Copy() tengo.Object {
	return &sqlResult{ctx: c.ctx, rows: c.rows}
}

func (c *sqlResult) IndexGet(index tengo.Object) (tengo.Object, error) {
	if o, ok := index.(*tengo.String); ok {
		switch o.Value {
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
				// fmt.Println("scan()", values)
				ret := &tengo.ImmutableMap{Value: map[string]tengo.Object{}}
				for i, col := range columns {
					// fmt.Println("scan()", col, *(values[i].(*string)))
					col = strings.ToLower(col)
					ret.Value[col] = &tengo.String{Value: *(values[i].(*string))}
				}
				return ret, nil
			}}, nil
		default:
			return nil, nil
		}
	} else {
		return nil, nil
	}
}
