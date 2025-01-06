package api

import (
	"context"
	"fmt"
	"strings"
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
	// Ping checks if the database is alive and returns the round-trip time.
	Ping(ctx context.Context) (time.Duration, error)
}

type Conn interface {
	// Close closes the connection.
	Close() error

	// ExecContext executes SQL statements that do not return results,
	// such as 'ALTER', 'CREATE TABLE', 'DROP TABLE', etc.
	Exec(ctx context.Context, sqlText string, params ...any) Result

	// Query executes SQL statements that are expected to return multiple rows.
	// Commonly used to execute 'SELECT * FROM <TABLE>'.
	//
	// Rows returned by Query() must be closed to prevent server-side resource leaks.
	//
	// Example:
	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	//	defer cancelFunc()
	//
	//	rows, err := conn.Query(ctx, "SELECT * FROM my_table WHERE name = ?", my_name)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer rows.Close()
	Query(ctx context.Context, sqlText string, params ...any) (Rows, error)

	// QueryRow executes a SQL statement that expects a single row result.
	//
	// Example:
	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	//	defer cancelFunc()
	//
	//	var cnt int
	//	row := conn.QueryRow(ctx, "SELECT count(*) FROM my_table WHERE name = ?", "my_name")
	//	row.Scan(&cnt)
	QueryRow(ctx context.Context, sqlText string, params ...any) Row

	// Appender creates a new Appender for the given table.
	// The Appender should be closed as soon as the work is finished to prevent server-side resource leaks.
	//
	// Example:
	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	//	defer cancelFunc()
	//
	//	app, _ := conn.Appender(ctx, "MY_TABLE")
	//	defer app.Close()
	//	app.Append("name", time.Now(), 3.14)
	Appender(ctx context.Context, tableName string, opts ...AppenderOption) (Appender, error)

	// Explain returns the execution plan for the given SQL statement.
	// If full is true, it returns a detailed execution plan.
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

type Rows interface {
	// Next prepares the next row for reading. It returns true if there is another row available.
	// It must be called before any Scan operations.
	//
	// Example:
	//	rows, _ := db.Query("SELECT name, value FROM my_table")
	//	for rows.Next() {
	//		var name string
	//		var value float64
	//		rows.Scan(&name, &value)
	//	}
	Next() bool

	// Scan copies the columns from the current row into the values pointed at by cols.
	// The number of values in cols must be the same as the number of columns in the result set.
	//
	// Example:
	//	for rows.Next() {
	//		var name string
	//		var value float64
	//		rows.Scan(&name, &value)
	//	}
	Scan(cols ...any) error

	// Close releases any resources associated with the Rows. It is important to call Close
	// after finishing with the Rows to prevent resource leaks.
	//
	// Example:
	//	rows, _ := db.Query("SELECT name, value FROM my_table")
	//	defer rows.Close()
	Close() error

	// IsFetchable returns true if the statement that produced this Rows is fetchable (e.g., a SELECT statement).
	IsFetchable() bool

	// RowsAffected returns the number of rows affected by the query.
	RowsAffected() int64

	// Message returns a message associated with the result of the query.
	Message() string

	// Columns returns a list of column information for the result set.
	//
	// Example:
	//	columns, err := rows.Columns()
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	for _, col := range columns {
	//		fmt.Println(col.Name)
	//	}
	Columns() (Columns, error)
}

type Result interface {
	// Err returns the error, if any, that was encountered during the execution of the query.
	Err() error

	// RowsAffected returns the number of rows affected by the query.
	RowsAffected() int64

	// Message returns a message associated with the result of the query.
	Message() string
}

type Row interface {
	// Err returns the error, if any, that was encountered during the execution of the query.
	Err() error

	// RowsAffected returns the number of rows affected by the query.
	RowsAffected() int64

	// Message returns a message associated with the result of the query.
	Message() string

	// Scan copies the columns from the current row into the values pointed at by cols.
	// The number of values in cols must be the same as the number of columns in the result set.
	//
	// Example:
	//	var name string
	//	var value float64
	//	row.Scan(&name, &value)
	Scan(cols ...any) error

	// Columns returns a list of column information for the result set.
	//
	// Example:
	//	columns, err := row.Columns()
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	for _, col := range columns {
	//		fmt.Println(col.Name)
	//	}
	Columns() (Columns, error)
}

type Appender interface {
	// TableName returns the name of the table to which the Appender is appending data.
	TableName() string

	// Append adds a new row with the specified values to the table.
	// The number of values must match the number of columns in the table.
	//
	// Example:
	//	appender.Append("name", time.Now(), 3.14)
	Append(values ...any) error

	// AppendLogTime adds a new row with the specified timestamp and values to the table.
	// This is applicable only for log tables, where the timestamp is applied to _ARRIVAL_TIME instead of the current system time.
	//
	// Example:
	//	appender.AppendLogTime(time.Now(), "name", 3.14)
	AppendLogTime(ts time.Time, values ...any) error

	// Close finalizes the appending process and releases any resources associated with the Appender.
	// It returns the number of rows successfully appended and the number of rows that failed to append.
	//
	// Example:
	//	rowsAppended, rowsFailed, err := appender.Close()
	Close() (int64, int64, error)

	// Columns returns a list of column information for the table.
	//
	// Example:
	//	columns, err := appender.Columns()
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	for _, col := range columns {
	//		fmt.Println(col.Name)
	//	}
	Columns() (Columns, error)

	// TableType returns the type of the table to which the Appender is appending data.
	TableType() TableType

	// WithInputColumns sets the input column names for the Appender.
	WithInputColumns(columns ...string) Appender
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

func (typ TableType) ShortString() string {
	switch typ {
	case TableTypeLog:
		return "Log"
	case TableTypeFixed:
		return "Fixed"
	case TableTypeVolatile:
		return "Volatile"
	case TableTypeLookup:
		return "Lookup"
	case TableTypeKeyValue:
		return "KeyValue"
	case TableTypeTag:
		return "Tag"
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

type SQLStatementType int

const (
	SQLStatementTypeOther SQLStatementType = iota
	SQLStatementTypeSelect
	SQLStatementTypeInsert
	SQLStatementTypeUpdate
	SQLStatementTypeDelete
	SQLStatementTypeCreate
	SQLStatementTypeDrop
	SQLStatementTypeAlter
	SQLStatementTypeDescribe
)

func DetectSQLStatementType(sqlText string) SQLStatementType {
	toks := strings.Fields(sqlText)
	if len(toks) == 0 {
		return SQLStatementTypeOther
	}
	verb := strings.ToUpper(toks[0])
	switch verb {
	case "SELECT":
		return SQLStatementTypeSelect
	case "INSERT":
		return SQLStatementTypeInsert
	case "UPDATE":
		return SQLStatementTypeUpdate
	case "DELETE":
		return SQLStatementTypeDelete
	case "CREATE":
		return SQLStatementTypeCreate
	case "DROP":
		return SQLStatementTypeDrop
	case "ALTER":
		return SQLStatementTypeAlter
	case "DESCRIBE":
		return SQLStatementTypeDescribe
	default:
		return SQLStatementTypeOther
	}
}

func (st SQLStatementType) IsFetch() bool {
	return st == SQLStatementTypeSelect || st == SQLStatementTypeDescribe
}
