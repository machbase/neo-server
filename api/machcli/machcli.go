package machcli

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api/types"
)

func errorWithCause(obj any, cause error) error {
	var handle unsafe.Pointer
	var handleType mach.HandleType
	switch x := obj.(type) {
	case *Env:
		handle = x.handle
		handleType = mach.MACHCLI_HANDLE_ENV
	case *Conn:
		handle = x.handle
		handleType = mach.MACHCLI_HANDLE_DBC
	case *Stmt:
		handle = x.handle
		handleType = mach.MACHCLI_HANDLE_STMT
	default:
		return cause
	}
	var code int
	var msg string
	if reErr := mach.CliError(handle, handleType, &code, &msg); reErr != nil {
		if cause == nil {
			return fmt.Errorf("MACHCLI Fail to get error, %s", reErr.Error())
		} else {
			return fmt.Errorf("MACHCLI Fail to get error code of %s, %s", cause.Error(), reErr.Error())
		}
	} else if msg == "" {
		return cause
	} else {
		if cause == nil {
			if code == 0 {
				return nil // no error
			}
			return fmt.Errorf("MACHCLI ERR-%d, %s", code, msg)
		} else {
			return fmt.Errorf("MACHCLI ERR-%d, %s", code, msg)
		}
	}
}

type Env struct {
	handle unsafe.Pointer
}

func NewEnv() (*Env, error) {
	var h unsafe.Pointer
	if err := mach.CliInitialize(&h); err != nil {
		return nil, err
	}
	return &Env{handle: h}, nil
}

func (env *Env) Close() error {
	if err := mach.CliFinalize(env.handle); err == nil {
		return nil
	} else {
		return errorWithCause(env, err)
	}
}

func (env *Env) Connect(ctx context.Context, opts ...ConnectOption) (*Conn, error) {
	conf := &ConnConfig{
		host:     "127.0.0.1",
		port:     5656,
		user:     "sys",
		password: "manager",
	}
	for _, opt := range opts {
		opt(conf)
	}
	var h unsafe.Pointer
	if err := mach.CliConnect(env.handle, conf.ConnectionString(), &h); err == nil {
		ret := &Conn{
			handle: h,
			ctx:    ctx,
			env:    env,
		}
		return ret, nil
	} else {
		return nil, errorWithCause(env, err)
	}
}

type ConnConfig struct {
	host     string
	port     int
	user     string
	password string
}

func (c *ConnConfig) ConnectionString() string {
	return fmt.Sprintf("SERVER=%s;UID=%s;PWD=%s;CONNTYPE=1;PORT_NO=%d",
		c.host, strings.ToUpper(c.user), strings.ToUpper(c.password), c.port)
}

type ConnectOption func(*ConnConfig)

func WithHost(host string, port int) ConnectOption {
	return func(c *ConnConfig) {
		c.host = host
		c.port = port
	}
}

func WithPassword(user string, password string) ConnectOption {
	return func(conf *ConnConfig) {
		conf.user = user
		conf.password = password
	}
}

type Conn struct {
	handle unsafe.Pointer
	ctx    context.Context
	env    *Env
}

func (c *Conn) Close() error {
	if err := mach.CliDisconnect(c.handle); err == nil {
		return nil
	} else {
		return errorWithCause(c, err)
	}
}

func (c *Conn) Error() error {
	return errorWithCause(c, nil)
}

func (c *Conn) ExecDirectContext(ctx context.Context, query string) *Result {
	ret := &Result{}
	stmt, err := c.NewStmt()
	if err != nil {
		ret.err = err
		return ret
	}
	defer stmt.Close()
	if err := mach.CliExecDirect(stmt.handle, query); err == nil {
		// TODO implement rowsAffected
		return ret
	} else {
		ret.err = errorWithCause(c, err)
		return ret
	}
}

func (c *Conn) ExecContext(ctx context.Context, query string, args ...any) *Result {
	ret := &Result{}
	stmt, err := c.NewStmt()
	if err != nil {
		ret.err = err
		return ret
	}
	defer stmt.Close()

	if err := stmt.prepare(query); err != nil {
		ret.err = err
		return ret
	}
	if err := stmt.bindParams(args...); err != nil {
		ret.err = err
		return ret
	}
	if err := stmt.execute(); err != nil {
		ret.err = err
		return ret
	}

	// TODO implement rowsAffected
	return &Result{}
}

func (c *Conn) QueryRowContext(ctx context.Context, query string, args ...any) *Row {
	ret := &Row{}

	stmt, err := c.NewStmt()
	if err != nil {
		ret.err = err
		return ret
	}
	defer stmt.Close()

	if err := stmt.prepare(query); err != nil {
		ret.err = err
		return ret
	}
	if err := stmt.bindParams(args...); err != nil {
		ret.err = err
		return ret
	}
	if err := stmt.execute(); err != nil {
		ret.err = err
		return ret
	}
	if values, _, err := stmt.fetch(); err != nil {
		ret.err = err
		return ret
	} else {
		ret.values = values
	}
	return ret
}

func (c *Conn) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	stmt, err := c.NewStmt()
	if err != nil {
		return nil, err
	}
	if err := stmt.prepare(query); err != nil {
		stmt.Close()
		return nil, err
	}
	if err := stmt.bindParams(args...); err != nil {
		stmt.Close()
		return nil, err
	}
	if err := stmt.execute(); err != nil {
		stmt.Close()
		return nil, err
	}
	ret := &Rows{
		stmt: stmt,
	}
	return ret, nil
}

func (stmt *Stmt) prepare(query string) error {
	if err := mach.CliPrepare(stmt.handle, query); err != nil {
		return errorWithCause(stmt, err)
	}
	return nil
}

func (stmt *Stmt) bindParams(args ...any) error {
	numParam, err := mach.CliNumParam(stmt.handle)
	if err != nil {
		return errorWithCause(stmt, err)
	}
	if len(args) != numParam {
		return types.ErrParamCount(numParam, len(args))
	}

	paramsDesc := make([]mach.CliParamDesc, numParam)
	for i := 0; i < numParam; i++ {
		desc, err := mach.CliDescribeParam(stmt.handle, i)
		if err != nil {
			return errorWithCause(stmt, err)
		}
		paramsDesc[i] = desc
	}

	if len(args) != numParam {
		return types.ErrParamCount(numParam, len(args))
	}

	for paramNo, pd := range paramsDesc {
		var value unsafe.Pointer
		var valueLen int
		var cType mach.CType
		arg := args[paramNo]
		switch pd.Type {
		default:
			return types.ErrDatabaseBindUnknownType(paramNo, int(pd.Type))
		case mach.MACHCLI_SQL_TYPE_INT16:
			switch val := arg.(type) {
			case int16:
				cType = mach.MACHCLI_C_TYPE_INT16
				value = unsafe.Pointer(&val)
				valueLen = 2
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), arg)
			}
		case mach.MACHCLI_SQL_TYPE_INT32:
			switch val := arg.(type) {
			case int32, int:
				cType = mach.MACHCLI_C_TYPE_INT32
				value = unsafe.Pointer(&val)
				valueLen = 4
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_INT64:
			switch val := arg.(type) {
			case int64:
				cType = mach.MACHCLI_C_TYPE_INT64
				value = unsafe.Pointer(&val)
				valueLen = 8
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_DATETIME:
			switch val := arg.(type) {
			case int64:
				cType = mach.MACHCLI_C_TYPE_INT64
				value = unsafe.Pointer(&val)
				valueLen = 8
			case time.Time:
				cType = mach.MACHCLI_C_TYPE_INT64
				v := val.UnixNano()
				value = unsafe.Pointer(&v)
				valueLen = 8
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_FLOAT:
			switch val := arg.(type) {
			case float32:
				cType = mach.MACHCLI_C_TYPE_FLOAT
				value = unsafe.Pointer(&val)
				valueLen = 4
			case float64:
				cType = mach.MACHCLI_C_TYPE_FLOAT
				v := float32(val)
				value = unsafe.Pointer(&v)
				valueLen = 4
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_DOUBLE:
			switch val := arg.(type) {
			case float32:
				cType = mach.MACHCLI_C_TYPE_FLOAT
				value = unsafe.Pointer(&val)
				valueLen = 4
			case float64:
				cType = mach.MACHCLI_C_TYPE_DOUBLE
				value = unsafe.Pointer(&val)
				valueLen = 8
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_IPV4:
			switch val := arg.(type) {
			case net.IP:
				cType = mach.MACHCLI_C_TYPE_CHAR
				v := val.To4()
				value = unsafe.Pointer(&v[0])
				valueLen = 4
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_IPV6:
			switch val := arg.(type) {
			case net.IP:
				cType = mach.MACHCLI_C_TYPE_CHAR
				v := val.To16()
				value = unsafe.Pointer(&v[0])
				valueLen = 16
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_STRING:
			switch val := arg.(type) {
			case string:
				cType = mach.MACHCLI_C_TYPE_CHAR
				bStr := []byte(val)
				value = (unsafe.Pointer)(&bStr[0])
				valueLen = len(bStr)
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		case mach.MACHCLI_SQL_TYPE_BINARY:
			switch val := arg.(type) {
			case []byte:
				cType = mach.MACHCLI_C_TYPE_CHAR
				value = (unsafe.Pointer)(&val[0])
				valueLen = len(val)
			default:
				return types.ErrDatabaseBindWrongType(paramNo, int(pd.Type), value)
			}
		}

		if err := mach.CliBindParam(stmt.handle, paramNo, cType, pd.Type, value, valueLen); err != nil {
			return errorWithCause(stmt, err)
		}
	}
	return nil
}

type Result struct {
	message string
	err     error
	// rowsAffected int64
}

func (rs *Result) Message() string {
	return rs.message
}

func (rs *Result) Err() error {
	return rs.err
}

func (rs *Result) LastInsertId() (int64, error) {
	return 0, types.ErrNotImplemented("LastInsertId")
}

// func (rs *Result) RowsAffected() (int64, error) {
// 	return 0, types.ErrNotImplemented("RowsAffected")
// 	// return rs.rowsAffected, nil
// }

func (rs *Result) RowsAffected() int64 {
	return 0
}

func (c *Conn) PrepareContext(ctx context.Context, query string) (*Stmt, error) {
	ret := &Stmt{
		conn: c,
	}
	if err := mach.CliAllocStmt(c.handle, &ret.handle); err == nil {
		return ret, nil
	} else {
		return nil, errorWithCause(c, err)
	}
}

func (c *Conn) NewStmt() (*Stmt, error) {
	ret := &Stmt{}
	if err := mach.CliAllocStmt(c.handle, &ret.handle); err == nil {
		ret.conn = c
		return ret, nil
	} else {
		return nil, errorWithCause(c, err)
	}
}

type SqlType int

const (
	MACHCLI_SQL_TYPE_INT16    SqlType = 0
	MACHCLI_SQL_TYPE_INT32    SqlType = 1
	MACHCLI_SQL_TYPE_INT64    SqlType = 2
	MACHCLI_SQL_TYPE_DATETIME SqlType = 3
	MACHCLI_SQL_TYPE_FLOAT    SqlType = 4
	MACHCLI_SQL_TYPE_DOUBLE   SqlType = 5
	MACHCLI_SQL_TYPE_IPV4     SqlType = 6
	MACHCLI_SQL_TYPE_IPV6     SqlType = 7
	MACHCLI_SQL_TYPE_STRING   SqlType = 8
	MACHCLI_SQL_TYPE_BINARY   SqlType = 9
)

func (st SqlType) String() string {
	switch st {
	case MACHCLI_SQL_TYPE_INT16:
		return "INT16"
	case MACHCLI_SQL_TYPE_INT32:
		return "INT32"
	case MACHCLI_SQL_TYPE_INT64:
		return "INT64"
	case MACHCLI_SQL_TYPE_DATETIME:
		return "DATETIME"
	case MACHCLI_SQL_TYPE_FLOAT:
		return "FLOAT"
	case MACHCLI_SQL_TYPE_DOUBLE:
		return "DOUBLE"
	case MACHCLI_SQL_TYPE_IPV4:
		return "IPV4"
	case MACHCLI_SQL_TYPE_IPV6:
		return "IPV6"
	case MACHCLI_SQL_TYPE_STRING:
		return "STRING"
	case MACHCLI_SQL_TYPE_BINARY:
		return "BINARY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", st)
	}
}

type ColumnDesc struct {
	Name     string
	Type     SqlType
	Size     int
	Scale    int
	Nullable bool
}

type Stmt struct {
	handle      unsafe.Pointer
	conn        *Conn
	columnDescs []ColumnDesc
}

func (stmt *Stmt) Close() error {
	if err := mach.CliFreeStmt(stmt.handle); err == nil {
		return nil
	} else {
		return errorWithCause(stmt, err)
	}
}

func (stmt *Stmt) Error() error {
	return errorWithCause(stmt, nil)
}

func (stmt *Stmt) execute() error {
	if err := mach.CliExecute(stmt.handle); err != nil {
		return errorWithCause(stmt, err)
	}
	num, err := mach.CliNumResultCol(stmt.handle)
	if err != nil {
		return errorWithCause(stmt, err)
	}
	stmt.columnDescs = make([]ColumnDesc, num)
	for i := 0; i < num; i++ {
		d := ColumnDesc{}
		if err := mach.CliDescribeCol(stmt.handle, i, &d.Name, (*mach.SqlType)(&d.Type), &d.Size, &d.Scale, &d.Nullable); err != nil {
			return errorWithCause(stmt, err)
		}
		stmt.columnDescs[i] = d
	}
	return nil
}

// fetch fetches the next row from the result set.
// It returns true if it reaches end of the result, otherwise false.
func (stmt *Stmt) fetch() ([]any, bool, error) {
	end, err := mach.CliFetch(stmt.handle)
	if err != nil {
		return nil, end, err
	}
	if end {
		return nil, true, nil
	}

	row := make([]any, len(stmt.columnDescs))
	for i, desc := range stmt.columnDescs {
		switch desc.Type {
		case MACHCLI_SQL_TYPE_INT16:
			var v = new(int16)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT16, unsafe.Pointer(v), 2); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = *v
			}
		case MACHCLI_SQL_TYPE_INT32:
			var v = new(int32)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT32, unsafe.Pointer(v), 4); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = *v
			}
		case MACHCLI_SQL_TYPE_INT64:
			var v = new(int64)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT64, unsafe.Pointer(v), 8); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = *v
			}
		case MACHCLI_SQL_TYPE_DATETIME:
			var v = new(int64)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT64, unsafe.Pointer(v), 8); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = time.Unix(0, *v)
			}
		case MACHCLI_SQL_TYPE_FLOAT:
			var v = new(float32)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_FLOAT, unsafe.Pointer(v), 4); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = *v
			}
		case MACHCLI_SQL_TYPE_DOUBLE:
			var v = new(float64)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_DOUBLE, unsafe.Pointer(v), 8); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = *v
			}
		case MACHCLI_SQL_TYPE_IPV4:
			var v = []byte{0, 0, 0, 0}
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), 4); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = net.IP(v)
			}
		case MACHCLI_SQL_TYPE_IPV6:
			var v = make([]byte, 16)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), 16); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = net.IP(v)
			}
		case MACHCLI_SQL_TYPE_STRING:
			var v = make([]byte, desc.Size)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), desc.Size); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = string(v[0:n])
			}
		case MACHCLI_SQL_TYPE_BINARY:
			var v = make([]byte, desc.Size)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), desc.Size); err != nil {
				return nil, end, errorWithCause(stmt, err)
			} else if n == -1 {
				row[i] = nil
			} else {
				row[i] = v[0:n]
			}
		}
	}
	return row, end, nil
}

type Row struct {
	err    error
	values []any
}

func (r *Row) Success() bool {
	return r.err == nil
}

func (r *Row) Err() error {
	return r.err
}

func (r *Row) Scan(dest ...any) error {
	if len(dest) > len(r.values) {
		return types.ErrParamCount(len(r.values), len(dest))
	}
	for i, d := range dest {
		if d == nil {
			continue
		}
		if r.values[i] == nil {
			continue
		}
		if err := types.Scan(r.values[i], d); err != nil {
			return err
		}
	}
	return nil
}

func (r *Row) Values() []any {
	// TODO implement
	return nil
}
func (r *Row) RowsAffected() int64 {
	// TODO implement
	return 0
}

func (r *Row) Message() string {
	// TODO implement
	return ""
}

type Rows struct {
	stmt *Stmt
	err  error
	row  []any
}

func (r *Rows) Err() error {
	return r.err
}

func (r *Rows) Close() error {
	return r.stmt.Close()
}

func (r *Rows) IsFetchable() bool {
	// TODO implement
	return false
}

func (r *Rows) Columns() ([]string, []types.DataType, error) {
	return nil, nil, types.ErrNotImplemented("Columns")
}

func (r *Rows) Message() string {
	// TODO implement
	return ""
}

func (r *Rows) RowsAffected() int64 {
	// TODO implement
	return 0
}

func (r *Rows) Next() bool {
	row, end, err := r.stmt.fetch()
	if err != nil {
		r.err = err
		return false
	}
	if end {
		return false
	}
	r.row = row
	return true
}

func (r *Rows) Row() []any {
	return r.row
}

func (r *Rows) ColumnDescriptions() []ColumnDesc {
	return r.stmt.columnDescs
}

func (r *Rows) Scan(dest ...any) error {
	if len(dest) > len(r.row) {
		return types.ErrParamCount(len(r.row), len(dest))
	}
	for i, d := range dest {
		if d == nil {
			continue
		}
		if r.row[i] == nil {
			continue
		}
		if err := types.Scan(r.row[i], d); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) Appender(tableName string, opts ...AppenderOption) (*Appender, error) {
	ret := &Appender{tableName: strings.ToUpper(tableName)}
	for _, opt := range opts {
		opt(ret)
	}

	stmt, err := c.NewStmt()
	if err != nil {
		return nil, err
	}
	ret.stmt = stmt

	if err := mach.CliAppendOpen(stmt.handle, ret.tableName, ret.errCheckCount); err != nil {
		return nil, errorWithCause(stmt, err)
	}

	num, err := mach.CliNumResultCol(stmt.handle)
	if err != nil {
		stmt.Close()
		return nil, err
	}

	// XXX
	// TODO temporary solution to skip the first column
	stmt.columnDescs = make([]ColumnDesc, num-1)
	for i := 0; i < num; i++ {
		if i == 0 {
			continue
		}
		d := ColumnDesc{}
		if err := mach.CliDescribeCol(stmt.handle, i, &d.Name, (*mach.SqlType)(&d.Type), &d.Size, &d.Scale, &d.Nullable); err != nil {
			err = errorWithCause(stmt, err)
			stmt.Close()
			return nil, err
		}
		stmt.columnDescs[i-1] = d
		ret.columnTypes = append(ret.columnTypes, mach.SqlType(d.Type))
		ret.columnNames = append(ret.columnNames, d.Name)
		ret.columnDataTypes = append(ret.columnDataTypes, types.ParseDataType(d.Type.String()))
	}

	return ret, nil
}

type Appender struct {
	stmt            *Stmt
	tableName       string
	errCheckCount   int
	columnTypes     []mach.SqlType
	columnNames     []string
	columnDataTypes []types.DataType
}

type AppenderOption func(*Appender)

func WithErrorCheckCount(count int) AppenderOption {
	return func(c *Appender) {
		c.errCheckCount = count
	}
}

// Close returns the number of success and fail rows.
func (a *Appender) Close() (int64, int64, error) {
	if success, fail, err := mach.CliAppendClose(a.stmt.handle); err == nil {
		return success, fail, nil
	} else {
		c := a.stmt.conn
		if err := a.stmt.Close(); err != nil {
			return success, fail, errorWithCause(c, err)
		}
		return success, fail, errorWithCause(a.stmt, err)
	}
}

func (a *Appender) TableName() string {
	return a.tableName
}

func (a *Appender) Columns() ([]string, []types.DataType, error) {
	return a.columnNames, a.columnDataTypes, nil
}

func (a *Appender) Flush() error {
	if err := mach.CliAppendFlush(a.stmt.handle); err == nil {
		return nil
	} else {
		return errorWithCause(a.stmt, err)
	}
}

func (a *Appender) Append(args ...any) error {
	if err := mach.CliAppendData(a.stmt.handle, a.columnTypes, a.columnNames, args); err != nil {
		return errorWithCause(a.stmt, err)
	}
	return nil
}

func (a *Appender) AppendWithTimestamp(ts time.Time, values ...any) error {
	return types.ErrNotImplemented("AppendWithTimestamp")
}
