package api

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-client/machrpc"
	mach "github.com/machbase/neo-engine"
)

var (
	_ Database = &machrpcClient{}
	_ Conn     = &machrpcConn{}
	_ Result   = &machrpc.Result{}
	_ Rows     = &machrpc.Rows{}
	_ Row      = &machrpc.Row{}
	_ Appender = &machrpc.Appender{}
)

var (
	_ Database = &machDatabase{}
	_ Conn     = &machConn{}
	_ Result   = &mach.Result{}
	_ Rows     = &mach.Rows{}
	_ Row      = &mach.Row{}
	_ Appender = &mach.Appender{}
)

func NewDatabase(underlying any) Database {
	switch raw := underlying.(type) {
	case *mach.Database:
		return &machDatabase{raw: raw}
	case *machrpc.Client:
		return &machrpcClient{raw: raw}
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

type machDatabase struct {
	raw *mach.Database
}

type machrpcClient struct {
	raw *machrpc.Client
}

type ConnectOption func(db any) any

func (db *machDatabase) Connect(ctx context.Context, options ...ConnectOption) (Conn, error) {
	opts := make([]mach.ConnectOption, len(options))
	for i, o := range options {
		if f := o(db); f != nil {
			opts[i] = f.(mach.ConnectOption)
		}
	}
	c, err := db.raw.Connect(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &machConn{raw: c}, nil
}

func (db *machDatabase) UserAuth(username string, password string) (bool, error) {
	return db.raw.UserAuth(username, password)
}

func (db *machrpcClient) Connect(ctx context.Context, options ...ConnectOption) (Conn, error) {
	opts := make([]machrpc.ConnectOption, len(options))
	for i, o := range options {
		opts[i] = func(c *machrpc.Conn) {
			if f := o(db); f != nil {
				opts[i] = f.(machrpc.ConnectOption)
			}
		}
	}
	c, err := db.raw.Connect(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &machrpcConn{raw: c}, nil
}

func WithTrustUser(username string) ConnectOption {
	return func(db any) any {
		switch db.(type) {
		case *mach.Database:
			return mach.WithTrustUser(username)
		case *machDatabase:
			return mach.WithTrustUser(username)
		default:
			return nil
		}
	}
}

func WithPassword(username, password string) ConnectOption {
	return func(db any) any {
		switch db.(type) {
		case *machConn:
			return mach.WithPassword(username, password)
		case *machrpc.Client:
			return machrpc.WithPassword(username, password)
		default:
			return nil
		}
	}
}

type machConn struct {
	raw *mach.Conn
}

func ConnMach(raw *mach.Conn) Conn {
	return &machConn{raw: raw}
}

func ConnRpc(raw *machrpc.Conn) Conn {
	return &machrpcConn{raw: raw}
}

func (c *machConn) Close() error {
	return c.raw.Close()
}

func (c *machConn) Exec(ctx context.Context, sqlText string, params ...any) Result {
	ret := c.raw.Exec(ctx, sqlText, params...)
	return ret
}

func (c *machConn) Query(ctx context.Context, sqlText string, params ...any) (Rows, error) {
	ret, err := c.raw.Query(ctx, sqlText, params...)
	return ret, err
}

func (c *machConn) QueryRow(ctx context.Context, sqlText string, params ...any) Row {
	ret := c.raw.QueryRow(ctx, sqlText, params...)
	return ret
}

func (c *machConn) Appender(ctx context.Context, tableName string, options ...AppenderOption) (Appender, error) {
	opts := make([]mach.AppenderOption, len(options))
	for i, o := range options {
		opts[i] = func(a *mach.Appender) {
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
	Columns() ([]string, []string, error)
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
	Columns() ([]string, []string, error)
}

type AppenderOption func(Appender)

func AppenderTableType(app Appender) TableType {
	switch a := app.(type) {
	case *mach.Appender:
		return TableType(a.TableType())
	case *machrpc.Appender:
		return TableType(a.TableType())
	}
	return TableType(-1)
}

type Columns []*Column

type Column struct {
	Name string
	Type string
}

func (cols Columns) Names() []string {
	names := make([]string, len(cols))
	for i := range cols {
		names[i] = cols[i].Name
	}
	return names
}

func (cols Columns) NamesWithTimeLocation(tz *time.Location) []string {
	names := make([]string, len(cols))
	for i := range cols {
		if cols[i].Type == "datetime" {
			names[i] = fmt.Sprintf("%s(%s)", cols[i].Name, tz.String())
		} else {
			names[i] = cols[i].Name
		}
	}
	return names
}

func (cols Columns) Types() []string {
	types := make([]string, len(cols))
	for i := range cols {
		types[i] = cols[i].Type
	}
	return types
}

// RowsColumns returns list of column info that consists of result of query statement.
func RowsColumns(rows Rows) (Columns, error) {
	names, types, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(names) != len(types) {
		return nil, fmt.Errorf("internal error names %d types %d", len(names), len(types))
	}
	ret := make([]*Column, len(names))
	for i := range names {
		ret[i] = &Column{
			Name: names[i],
			Type: types[i],
		}
	}
	return ret, nil
}

func AppenderColumns(rows Appender) (Columns, error) {
	names, types, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(names) != len(types) {
		return nil, fmt.Errorf("internal error names %d types %d", len(names), len(types))
	}
	ret := make([]*Column, len(names))
	for i := range names {
		ret[i] = &Column{
			Name: names[i],
			Type: types[i],
		}
	}
	return ret, nil
}
