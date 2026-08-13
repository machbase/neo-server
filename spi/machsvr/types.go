package machsvr

// type Database interface {
// 	// Connect creates a new connection to the database.
// 	Connect(ctx context.Context, options ...ConnectOption) (Conn, error)
// 	// UserAuth checks user authentication.
// 	// If user is authenticated, it returns true and no error
// 	// If user is not authenticated, it returns false and the reason of failure.
// 	// If error occurred, it returns error.
// 	UserAuth(ctx context.Context, user string, password string) (bool, string, error)
// 	// Ping checks if the database is alive and returns the round-trip time.
// 	Ping(ctx context.Context) (time.Duration, error)
// }

// type Conn interface {
// 	// Close closes the connection.
// 	Close() error

// 	// ExecContext executes SQL statements that do not return results,
// 	// such as 'ALTER', 'CREATE TABLE', 'DROP TABLE', etc.
// 	Exec(ctx context.Context, sqlText string, params ...any) Result

// 	// Query executes SQL statements that are expected to return multiple rows.
// 	// Commonly used to execute 'SELECT * FROM <TABLE>'.
// 	//
// 	// Rows returned by Query() must be closed to prevent server-side resource leaks.
// 	//
// 	// Example:
// 	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
// 	//	defer cancelFunc()
// 	//
// 	//	rows, err := conn.Query(ctx, "SELECT * FROM my_table WHERE name = ?", my_name)
// 	//	if err != nil {
// 	//		panic(err)
// 	//	}
// 	//	defer rows.Close()
// 	Query(ctx context.Context, sqlText string, params ...any) (Rows, error)

// 	// QueryRow executes a SQL statement that expects a single row result.
// 	//
// 	// Example:
// 	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
// 	//	defer cancelFunc()
// 	//
// 	//	var cnt int
// 	//	row := conn.QueryRow(ctx, "SELECT count(*) FROM my_table WHERE name = ?", "my_name")
// 	//	row.Scan(&cnt)
// 	QueryRow(ctx context.Context, sqlText string, params ...any) Row

// 	// Prepare creates a prepared statement for later executions.
// 	//
// 	// Caution: Only machcli.Conn supports prepared statements. The other implementations panic when this method is called.
// 	//
// 	// Caution: Prepared statements must be closed as soon as the work is finished to prevent server-side resource leaks.
// 	//
// 	// Example:
// 	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
// 	//	defer cancelFunc()
// 	//
// 	//	stmt, err := conn.Prepare(ctx, "SELECT * FROM my_table WHERE name = ?")
// 	//	if err != nil {
// 	//		panic(err)
// 	//	}
// 	//	defer stmt.Close()
// 	//
// 	//	rows, err := stmt.Query(ctx, "my_name")
// 	//	if err != nil {
// 	//		panic(err)
// 	//	}
// 	//	defer rows.Close()
// 	Prepare(ctx context.Context, query string) (Stmt, error)

// 	// Appender creates a new Appender for the given table.
// 	// The Appender should be closed as soon as the work is finished to prevent server-side resource leaks.
// 	//
// 	// Example:
// 	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
// 	//	defer cancelFunc()
// 	//
// 	//	app, _ := conn.Appender(ctx, "MY_TABLE")
// 	//	defer app.Close()
// 	//	app.Append("name", time.Now(), 3.14)
// 	Appender(ctx context.Context, tableName string, opts ...AppenderOption) (Appender, error)

// 	// Explain returns the execution plan for the given SQL statement.
// 	// If full is true, it returns a detailed execution plan.
// 	Explain(ctx context.Context, sqlText string, full bool) (string, error)
// }

// type Stmt interface {
// 	// Close closes the prepared statement.
// 	Close() error

// 	// ExecContext executes a prepared statement that does not return results,
// 	// such as 'ALTER', 'CREATE TABLE', 'DROP TABLE', etc.
// 	Exec(ctx context.Context, params ...any) Result
// 	// Query executes a prepared statement that is expected to return multiple rows.
// 	// Commonly used to execute 'SELECT * FROM <TABLE>'.
// 	//
// 	// Rows returned by Query() must be closed to prevent server-side resource leaks.
// 	//
// 	// Example:
// 	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
// 	//	defer cancelFunc()
// 	//
// 	//	rows, err := stmt.Query(ctx, my_name)
// 	//	if err != nil {
// 	//		panic(err)
// 	//	}
// 	//	defer rows.Close()
// 	Query(ctx context.Context, params ...any) (Rows, error)

// 	// QueryRow executes a prepared statement that expects a single row result.
// 	//
// 	// Example:
// 	//	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
// 	//	defer cancelFunc()
// 	//
// 	//	var cnt int
// 	//	row := stmt.QueryRow(ctx, "my_name")
// 	//	row.Scan(&cnt)
// 	QueryRow(ctx context.Context, params ...any) Row
// }

// type Rows interface {
// 	// Next prepares the next row for reading. It returns true if there is another row available.
// 	// It must be called before any Scan operations.
// 	//
// 	// Example:
// 	//	rows, _ := db.Query("SELECT name, value FROM my_table")
// 	//	for rows.Next() {
// 	//		var name string
// 	//		var value float64
// 	//		rows.Scan(&name, &value)
// 	//	}
// 	Next() bool

// 	// Scan copies the columns from the current row into the values pointed at by cols.
// 	// The number of values in cols must be the same as the number of columns in the result set.
// 	//
// 	// Example:
// 	//	for rows.Next() {
// 	//		var name string
// 	//		var value float64
// 	//		rows.Scan(&name, &value)
// 	//	}
// 	Scan(cols ...any) error

// 	// Close releases any resources associated with the Rows. It is important to call Close
// 	// after finishing with the Rows to prevent resource leaks.
// 	//
// 	// Example:
// 	//	rows, _ := db.Query("SELECT name, value FROM my_table")
// 	//	defer rows.Close()
// 	Close() error

// 	// Err returns the error, if any, that was encountered during the execution of the query.
// 	Err() error

// 	// IsFetchable returns true if the statement that produced this Rows is fetchable (e.g., a SELECT statement).
// 	IsFetchable() bool

// 	// RowsAffected returns the number of rows affected by the query.
// 	RowsAffected() int64

// 	// Message returns a message associated with the result of the query.
// 	Message() string

// 	// Columns returns a list of column information for the result set.
// 	//
// 	// Example:
// 	//	columns, err := rows.Columns()
// 	//	if err != nil {
// 	//		log.Fatal(err)
// 	//	}
// 	//	for _, col := range columns {
// 	//		fmt.Println(col.Name)
// 	//	}
// 	Columns() (Columns, error)
// }

// type Result interface {
// 	// Err returns the error, if any, that was encountered during the execution of the query.
// 	Err() error

// 	// RowsAffected returns the number of rows affected by the query.
// 	RowsAffected() int64

// 	// Message returns a message associated with the result of the query.
// 	Message() string
// }

// type Row interface {
// 	// Err returns the error, if any, that was encountered during the execution of the query.
// 	Err() error

// 	// RowsAffected returns the number of rows affected by the query.
// 	RowsAffected() int64

// 	// Message returns a message associated with the result of the query.
// 	Message() string

// 	// Scan copies the columns from the current row into the values pointed at by cols.
// 	// The number of values in cols must be the same as the number of columns in the result set.
// 	//
// 	// Example:
// 	//	var name string
// 	//	var value float64
// 	//	row.Scan(&name, &value)
// 	Scan(cols ...any) error

// 	// Columns returns a list of column information for the result set.
// 	//
// 	// Example:
// 	//	columns, err := row.Columns()
// 	//	if err != nil {
// 	//		log.Fatal(err)
// 	//	}
// 	//	for _, col := range columns {
// 	//		fmt.Println(col.Name)
// 	//	}
// 	Columns() (Columns, error)
// }

// type Appender interface {
// 	// TableName returns the name of the table to which the Appender is appending data.
// 	TableName() string

// 	// Append adds a new row with the specified values to the table.
// 	// The number of values must match the number of columns in the table.
// 	//
// 	// Example:
// 	//	appender.Append("name", time.Now(), 3.14)
// 	Append(values ...any) error

// 	// AppendLogTime adds a new row with the specified timestamp and values to the table.
// 	// This is applicable only for log tables, where the timestamp is applied to _ARRIVAL_TIME instead of the current system time.
// 	//
// 	// Example:
// 	//	appender.AppendLogTime(time.Now(), "name", 3.14)
// 	AppendLogTime(ts time.Time, values ...any) error

// 	// Close finalizes the appending process and releases any resources associated with the Appender.
// 	// It returns the number of rows successfully appended and the number of rows that failed to append.
// 	//
// 	// Example:
// 	//	rowsAppended, rowsFailed, err := appender.Close()
// 	Close() (int64, int64, error)

// 	// Columns returns a list of column information for the table.
// 	//
// 	// Example:
// 	//	columns, err := appender.Columns()
// 	//	if err != nil {
// 	//		log.Fatal(err)
// 	//	}
// 	//	for _, col := range columns {
// 	//		fmt.Println(col.Name)
// 	//	}
// 	Columns() (Columns, error)

// 	// TableType returns the type of the table to which the Appender is appending data.
// 	TableType() TableType

// 	// WithInputColumns sets the input column names for the Appender.
// 	WithInputColumns(columns ...string) Appender

// 	// WithInputFormats sets the input formats for the Appender.
// 	WithInputFormats(formats ...string) Appender

// 	// WithBatchMaxRows sets the maximum batch size in rows for batch append. If the batch size exceeds the limit, it will be flushed immediately.
// 	// The default value is 512 rows. The minimum value is 1 row.
// 	WithBatchMaxRows(rows int) Appender

// 	// WithBatchMaxBytes sets the maximum batch size in bytes for batch append. If the batch size exceeds the limit, it will be flushed immediately.
// 	// The default value is 512KB. The minimum value is 4KB.
// 	WithBatchMaxBytes(bytes int) Appender

// 	// WithBatchMaxDelay sets the maximum delay for batch append. If the batch is not full, it will be flushed when the delay is reached.
// 	// The default value is 5 milliseconds. The minimum value is 1 millisecond.
// 	WithBatchMaxDelay(duration time.Duration) Appender
// }

// type Flusher interface {
// 	Flush() error
// }
