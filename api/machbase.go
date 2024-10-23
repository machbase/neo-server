package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/api/machcli"
	"github.com/machbase/neo-server/api/machrpc"
	"github.com/machbase/neo-server/api/machsvr"
	"github.com/machbase/neo-server/api/types"
)

var (
	_ Database = (*machrpcDatabase)(nil)
	_ Conn     = (*machrpcConn)(nil)
	_ Result   = (*machrpc.Result)(nil)
	_ Rows     = (*machrpc.Rows)(nil)
	_ Row      = (*machrpc.Row)(nil)
	_ Appender = (*machrpc.Appender)(nil)
)

var (
	_ Database = (*machsvrDatabase)(nil)
	_ Conn     = (*machsvrConn)(nil)
	_ Result   = (*machsvr.Result)(nil)
	_ Rows     = (*machsvr.Rows)(nil)
	_ Row      = (*machsvr.Row)(nil)
	_ Appender = (*machsvr.Appender)(nil)
)

var (
	_ Database = (*machcliDatabase)(nil)
	_ Conn     = (*machcliConn)(nil)
	_ Result   = (*machcli.Result)(nil)
	_ Rows     = (*machcli.Rows)(nil)
	_ Row      = (*machcli.Row)(nil)
	_ Appender = (*machcli.Appender)(nil)
)

func NewDatabase[T *machsvr.Database | *machrpc.Client | *machcli.Env](underlying T) Database {
	database := any(underlying)
	switch raw := database.(type) {
	case *machsvr.Database:
		return &machsvrDatabase{raw: raw}
	case *machrpc.Client:
		return &machrpcDatabase{raw: raw}
	case *machcli.Env:
		return &machcliDatabase{raw: raw}
	default:
		panic(fmt.Errorf("invalid underlying db %T", underlying))
	}
}

type Database interface {
	Connect(ctx context.Context, options ...ConnectOption) (Conn, error)
}

type AuthServer interface {
	UserAuth(string, string) (bool, error)
}

type machsvrDatabase struct {
	raw *machsvr.Database
}

type machrpcDatabase struct {
	raw *machrpc.Client
}

type machcliDatabase struct {
	raw *machcli.Env
}

type ConnectOption func(db any) any

func (db *machsvrDatabase) Connect(ctx context.Context, options ...ConnectOption) (Conn, error) {
	opts := make([]machsvr.ConnectOption, 0, len(options))
	for _, opt := range options {
		o := opt(db)
		if o == nil {
			continue
		} else if connOpt, ok := o.(machsvr.ConnectOption); ok {
			opts = append(opts, connOpt)
		}
	}
	c, err := db.raw.Connect(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &machsvrConn{raw: c}, nil
}

func (db *machsvrDatabase) UserAuth(username string, password string) (bool, error) {
	return db.raw.UserAuth(username, password)
}

func (db *machrpcDatabase) Connect(ctx context.Context, options ...ConnectOption) (Conn, error) {
	opts := make([]machrpc.ConnectOption, 0, len(options))
	for _, opt := range options {
		o := opt(db)
		if o == nil {
			continue
		} else if connOpt, ok := o.(machrpc.ConnectOption); ok {
			opts = append(opts, connOpt)
		}
	}
	c, err := db.raw.Connect(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &machrpcConn{raw: c}, nil
}

func (db *machcliDatabase) Connect(ctx context.Context, options ...ConnectOption) (Conn, error) {
	opts := make([]machcli.ConnectOption, 0, len(options))
	for _, opt := range options {
		o := opt(db)
		if o == nil {
			continue
		} else if connOpt, ok := o.(machcli.ConnectOption); ok {
			opts = append(opts, connOpt)
		}
	}
	c, err := db.raw.Connect(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &machcliConn{raw: c}, nil
}

func WithTrustUser(username string) ConnectOption {
	return func(db any) any {
		switch db.(type) {
		case *machsvr.Database:
			return machsvr.WithTrustUser(username)
		case *machsvrDatabase:
			return machsvr.WithTrustUser(username)
		default:
			return nil
		}
	}
}

func WithPassword(username, password string) ConnectOption {
	return func(db any) any {
		switch db.(type) {
		case *machsvrConn:
			return machsvr.WithPassword(username, password)
		case *machrpc.Client:
			return machrpc.WithPassword(username, password)
		case *machcliDatabase:
			return machcli.WithPassword(username, password)
		default:
			return nil
		}
	}
}

type machsvrConn struct {
	raw *machsvr.Conn
}

func ConnMach(raw *machsvr.Conn) Conn {
	return &machsvrConn{raw: raw}
}

func ConnRpc(raw *machrpc.Conn) Conn {
	return &machrpcConn{raw: raw}
}

func (c *machsvrConn) Close() error {
	return c.raw.Close()
}

func (c *machsvrConn) Exec(ctx context.Context, sqlText string, params ...any) Result {
	ret := c.raw.Exec(ctx, sqlText, params...)
	return ret
}

func (c *machsvrConn) Query(ctx context.Context, sqlText string, params ...any) (Rows, error) {
	ret, err := c.raw.Query(ctx, sqlText, params...)
	return ret, err
}

func (c *machsvrConn) QueryRow(ctx context.Context, sqlText string, params ...any) Row {
	ret := c.raw.QueryRow(ctx, sqlText, params...)
	return ret
}

func (c *machsvrConn) Appender(ctx context.Context, tableName string, options ...AppenderOption) (Appender, error) {
	opts := make([]machsvr.AppenderOption, len(options))
	for i, o := range options {
		opts[i] = func(a *machsvr.Appender) {
			o(a)
		}
	}
	return c.raw.Appender(ctx, tableName, opts...)
}

type machrpcConn struct {
	raw *machrpc.Conn
}

func (c *machrpcConn) Close() error {
	return c.raw.Close()
}

func (c *machrpcConn) Exec(ctx context.Context, sqlText string, params ...any) Result {
	ret := c.raw.Exec(ctx, sqlText, params...)
	return ret
}

func (c *machrpcConn) Query(ctx context.Context, sqlText string, params ...any) (Rows, error) {
	ret, err := c.raw.Query(ctx, sqlText, params...)
	return ret, err
}

func (c *machrpcConn) QueryRow(ctx context.Context, sqlText string, params ...any) Row {
	ret := c.raw.QueryRow(ctx, sqlText, params...)
	return ret
}

func (c *machrpcConn) Appender(ctx context.Context, tableName string, options ...AppenderOption) (Appender, error) {
	opts := make([]machrpc.AppenderOption, len(options))
	for i, o := range options {
		opts[i] = func(a *machrpc.Appender) {
			o(a)
		}
	}
	ap, err := c.raw.Appender(ctx, tableName, opts...)
	if err != nil {
		return nil, err
	}
	return ap, nil
}

type machcliConn struct {
	raw *machcli.Conn
}

func (c *machcliConn) Close() error {
	return c.raw.Close()
}

func (c *machcliConn) Exec(ctx context.Context, sqlText string, params ...any) Result {
	if len(params) > 0 {
		return c.raw.ExecDirectContext(ctx, sqlText)
	}
	return c.raw.ExecContext(ctx, sqlText, params...)
}

func (c *machcliConn) Query(ctx context.Context, sqlText string, params ...any) (Rows, error) {
	return nil, nil
}

func (c *machcliConn) QueryRow(ctx context.Context, sqlText string, params ...any) Row {
	return c.raw.QueryRowContext(ctx, sqlText, params...)
}

func (c *machcliConn) Appender(ctx context.Context, tableName string, options ...AppenderOption) (Appender, error) {
	return nil, nil
}

type Conn interface {
	// Close closes connection
	Close() error

	// ExecContext executes SQL statements that does not return result
	// like 'ALTER', 'CREATE TABLE', 'DROP TABLE', ...
	Exec(ctx context.Context, sqlText string, params ...any) Result

	// Query executes SQL statements that are expected multiple rows as result.
	// Commonly used to execute 'SELECT * FROM <TABLE>'
	//
	// Rows returned by Query() must be closed to prevent server-side-resource leaks.
	//
	//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
	//	defer cancelFunc()
	//
	//	rows, err := conn.Query(ctx, "select * from my_table where name = ?", my_name)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer rows.Close()
	Query(ctx context.Context, sqlText string, params ...any) (Rows, error)

	// QueryRow executes a SQL statement that expects a single row result.
	//
	//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
	//	defer cancelFunc()
	//
	//	var cnt int
	//	row := conn.QueryRow(ctx, "select count(*) from my_table where name = ?", "my_name")
	//	row.Scan(&cnt)
	QueryRow(ctx context.Context, sqlText string, params ...any) Row

	// Appender creates a new Appender for the given table.
	// Appender should be closed as soon as finishing work, otherwise it may cause server side resource leak.
	//
	//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
	//	defer cancelFunc()
	//
	//	app, _ := conn.Appender(ctx, "MY_TABLE")
	//	defer app.Close()
	//	app.Append("name", time.Now(), 3.14)
	Appender(ctx context.Context, tableName string, opts ...AppenderOption) (Appender, error)
}

type Result interface {
	Err() error
	RowsAffected() int64
	Message() string
}

type Rows interface {
	// Next returns true if there are at least one more fetch-able record remained.
	//
	//  rows, _ := db.Query("select name, value from my_table")
	//	for rows.Next(){
	//		var name string
	//		var value float64
	//		rows.Scan(&name, &value)
	//	}
	Next() bool

	// Scan retrieve values of columns in a row
	//
	//	for rows.Next(){
	//		var name string
	//		var value float64
	//		rows.Scan(&name, &value)
	//	}
	Scan(cols ...any) error

	// Close release all resources that assigned to the Rows
	Close() error

	// IsFetchable returns true if statement that produced this Rows was fetch-able (e.g was select?)
	IsFetchable() bool

	RowsAffected() int64
	Message() string

	// Columns returns list of column info that consists of result of query statement.
	Columns() ([]string, []types.DataType, error)
}

type Row interface {
	Success() bool
	Err() error
	Scan(cols ...any) error
	Values() []any
	RowsAffected() int64
	Message() string
}

type Appender interface {
	TableName() string
	Append(values ...any) error
	AppendWithTimestamp(ts time.Time, values ...any) error
	Close() (int64, int64, error)
	Columns() ([]string, []types.DataType, error)
}

type AppenderOption func(Appender)

func AppenderTableType(app Appender) types.TableType {
	switch a := app.(type) {
	case *machsvr.Appender:
		return types.TableType(a.TableType())
	case *machrpc.Appender:
		return types.TableType(a.TableType())
	}
	return types.TableType(-1)
}

// RowsColumns returns list of column info that consists of result of query statement.
func RowsColumns(rows Rows) (types.Columns, error) {
	columnNames, columnTypes, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(columnNames) != len(columnTypes) {
		return nil, fmt.Errorf("internal error names %d types %d", len(columnNames), len(columnTypes))
	}
	ret := make([]*types.Column, len(columnNames))
	for i := range columnNames {
		ret[i] = &types.Column{
			Name:     columnNames[i],
			DataType: columnTypes[i],
		}
	}
	return ret, nil
}

func AppenderColumns(rows Appender) (types.Columns, error) {
	columnNames, columnTypes, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(columnNames) != len(columnTypes) {
		return nil, fmt.Errorf("internal error names %d types %d", len(columnNames), len(columnTypes))
	}
	ret := make([]*types.Column, len(columnNames))
	for i := range columnNames {
		ret[i] = &types.Column{
			Name:     columnNames[i],
			DataType: columnTypes[i],
		}
	}
	return ret, nil
}

func SqlTidy(sqlTextLines ...string) string {
	sqlText := strings.Join(sqlTextLines, "\n")
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.Join(lines, " ")
}
