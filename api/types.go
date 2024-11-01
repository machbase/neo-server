package api

import (
	"context"
	"fmt"
	"time"
)

type Database interface {
	// Connect creates a new connection to the database.
	Connect(ctx context.Context, options ...ConnectOption) (Conn, error)
	// UserAuth checks user authentication.
	// If user is authenticated, it returns true and no error
	// If user is not authenticated, it returns false and the reason of failure.
	// If error occurred, it returns error.
	UserAuth(ctx context.Context, user string, password string) (bool, string, error)
	// Ping checks if the database is alive.
	Ping(ctx context.Context) (time.Duration, error)
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

	Explain(ctx context.Context, sqlText string, full bool) (string, error)
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
	Columns() (Columns, error)
}

type Result interface {
	Err() error
	RowsAffected() int64
	Message() string
}

type Row interface {
	Err() error
	RowsAffected() int64
	Message() string
	Scan(cols ...any) error
	// Columns returns list of column info that consists of result of query statement.
	Columns() (Columns, error)
}

type Appender interface {
	TableName() string
	Append(values ...any) error
	AppendLogTime(ts time.Time, values ...any) error
	Close() (int64, int64, error)
	Columns() (Columns, error)
	TableType() TableType
}

type Flusher interface {
	Flush() error
}

// 0: Log Table, 1: Fixed Table, 3: Volatile Table,
// 4: Lookup Table, 5: KeyValue Table, 6: Tag Table
type TableType int

const (
	TableTypeLog      TableType = iota + 0
	TableTypeFixed    TableType = 1
	TableTypeVolatile TableType = 3
	TableTypeLookup   TableType = 4
	TableTypeKeyValue TableType = 5
	TableTypeTag      TableType = 6
)

func (typ TableType) String() string {
	switch typ {
	case TableTypeLog:
		return "LogTable"
	case TableTypeFixed:
		return "FixedTable"
	case TableTypeVolatile:
		return "VolatileTable"
	case TableTypeLookup:
		return "LookupTable"
	case TableTypeKeyValue:
		return "KeyValueTable"
	case TableTypeTag:
		return "TagTable"
	default:
		return fmt.Sprintf("UndefinedTable-%d", typ)
	}
}

func (typ TableType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, typ.String())), nil
}

type TableFlag int

const (
	TableFlagNone   TableFlag = 0
	TableFlagData   TableFlag = 1
	TableFlagRollup TableFlag = 2
	TableFlagMeta   TableFlag = 4
	TableFlagStat   TableFlag = 8
)

func (flag TableFlag) String() string {
	switch flag {
	case TableFlagNone:
		return ""
	case TableFlagData:
		return "Data"
	case TableFlagRollup:
		return "Rollup"
	case TableFlagMeta:
		return "Meta"
	case TableFlagStat:
		return "Stat"
	default:
		return fmt.Sprintf("UndefinedTableFlag-%d", flag)
	}
}

func (flag TableFlag) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, flag.String())), nil
}

type IndexType int

const (
	IndexTypeBitmap   IndexType = iota + 6
	IndexTypeRedBlack IndexType = 8
	IndexTypeKeyword  IndexType = 9
	IndexTypeTag      IndexType = 11
)

func (typ IndexType) String() string {
	switch typ {
	case IndexTypeBitmap:
		return "BITMAP (LSM)"
	case IndexTypeRedBlack:
		return "REDBLACK"
	case IndexTypeKeyword:
		return "KEYWORD (LSM)"
	case IndexTypeTag:
		return "TAG"
	default:
		return fmt.Sprintf("UndefinedIndex-%d", typ)
	}
}
