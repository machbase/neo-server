package machcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api"
)

func errorWithCause(obj any, cause error) error {
	var handle unsafe.Pointer
	var handleType mach.HandleType
	switch x := obj.(type) {
	case *Database:
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
	} else if msg == "" || code == 0 {
		return cause
	} else {
		if cause == nil {
			if code == 0 {
				return nil // no error
			}
			return fmt.Errorf("MACHCLI-ERR-%d, %s", code, msg)
		} else {
			return fmt.Errorf("MACHCLI-ERR-%d, %s", code, msg)
		}
	}
}

type Config struct {
	Host string
	Port int
}

type Database struct {
	Config
	handle unsafe.Pointer
}

var _ api.Database = (*Database)(nil)

func NewDatabase(conf *Config) (*Database, error) {
	handle := new(unsafe.Pointer)
	if err := mach.CliInitialize(handle); err != nil {
		return nil, err
	}
	ret := &Database{Config: *conf, handle: *handle}
	return ret, nil
}

func (db *Database) Close() error {
	if err := mach.CliFinalize(db.handle); err == nil {
		return nil
	} else {
		return errorWithCause(db, err)
	}
}

func (db *Database) Ping(ctx context.Context) (time.Duration, error) {
	tick := time.Now()
	// conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", db.Host, db.Port))
	// if err != nil {
	// 	return 0, err
	// }
	// if err := conn.Close(); err != nil {
	// 	return 0, err
	// }

	// TODO implement PING
	return time.Since(tick), nil
}

func (db *Database) UserAuth(ctx context.Context, user, password string) (bool, string, error) {
	conn, err := db.Connect(ctx, api.WithPassword(user, password))
	if err != nil {
		return false, "invalid username or password", nil
	}
	defer conn.Close()
	return true, "", nil
}

func (db *Database) connectionString(user string, password string) string {
	return fmt.Sprintf("SERVER=%s;UID=%s;PWD=%s;CONNTYPE=1;PORT_NO=%d",
		db.Host, strings.ToUpper(user), strings.ToUpper(password), db.Port)
}

func (db *Database) Connect(ctx context.Context, opts ...api.ConnectOption) (api.Conn, error) {
	var user, password string
	for _, opt := range opts {
		switch o := opt.(type) {
		case *api.ConnectOptionPassword:
			user = o.User
			password = o.Password
		case *api.ConnectOptionTrustUser:
			return nil, errors.New("trust user option is not supported")
		default:
			return nil, fmt.Errorf("unknown option type-%T", o)
		}
	}
	handle := new(unsafe.Pointer)
	if err := mach.CliConnect(db.handle, db.connectionString(user, password), handle); err != nil {
		return nil, errorWithCause(db, err)
	}
	ret := &Conn{db: db, handle: *handle}
	return ret, nil
}

type Conn struct {
	handle unsafe.Pointer
	db     *Database
}

var _ api.Conn = (*Conn)(nil)

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

func (c *Conn) Explain(ctx context.Context, query string, full bool) (string, error) {
	return "", api.ErrNotImplemented("Explain")
}

func (c *Conn) Exec(ctx context.Context, query string, args ...any) api.Result {
	ret := &Result{}
	var stmt *Stmt
	stmt, ret.err = c.NewStmt()
	if ret.err != nil {
		return ret
	}
	defer stmt.Close()

	stmt.sqlHead = strings.ToUpper(strings.Fields(query)[0])
	defer func() {
		switch stmt.sqlHead {
		case "INSERT":
			ret.rowsAffected = 1
		case "DELETE":
			// TODO implement rowsAffected
			ret.rowsAffected = 1
		default:
			// TODO implement rowsAffected
		}
	}()
	if len(args) == 0 {
		if err := mach.CliExecDirect(stmt.handle, query); err != nil {
			ret.err = errorWithCause(c, err)
		}
		return ret
	} else {
		if ret.err = stmt.prepare(query); ret.err != nil {
			return ret
		}
		if ret.err = stmt.bindParams(args...); ret.err != nil {
			return ret
		}
		ret.err = stmt.execute()
		return ret
	}
}

func (c *Conn) QueryRow(ctx context.Context, query string, args ...any) api.Row {
	ret := &Row{}
	stmt, err := c.NewStmt()
	if err != nil {
		ret.err = err
		return ret
	}
	defer stmt.Close()

	stmt.sqlHead = strings.ToUpper(strings.Fields(query)[0])

	if ret.err = stmt.prepare(query); ret.err != nil {
		return ret
	}
	if ret.err = stmt.bindParams(args...); ret.err != nil {
		return ret
	}
	if ret.err = stmt.execute(); ret.err != nil {
		return ret
	}
	if values, err := stmt.fetch(); err != nil {
		ret.err = err
		return ret
	} else {
		ret.values = values
	}
	ret.columns = make(api.Columns, len(stmt.columnDescs))
	for i, desc := range stmt.columnDescs {
		ret.columns[i] = &api.Column{
			Name:     desc.Name,
			Length:   desc.Size,
			Type:     desc.Type.ColumnType(),
			DataType: desc.Type.DataType(),
		}
	}
	return ret
}

func (c *Conn) Query(ctx context.Context, query string, args ...any) (api.Rows, error) {
	stmt, err := c.NewStmt()
	if err != nil {
		return nil, err
	}
	stmt.sqlHead = strings.ToUpper(strings.Fields(query)[0])
	if err := stmt.prepare(query); err != nil {
		stmt.Close()
		return nil, err
	}
	if err := stmt.bindParams(args...); err != nil {
		fmt.Printf("bind error: %s args:%d\n", err.Error(), len(args))
		stmt.Close()
		return nil, err
	}
	if err := stmt.execute(); err != nil {
		stmt.Close()
		fmt.Printf("execute error: %s args:%d\n", err.Error(), len(args))
		return nil, err
	}
	ret := &Rows{
		stmt: stmt,
	}
	if stmt.sqlHead == "INSERT" {
		ret.rowsAffected = 1
	} else {
		// TODO implement rowsAffected
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
		return api.ErrParamCount(numParam, len(args))
	}

	for idx, arg := range args {
		var value unsafe.Pointer
		var valueLen int
		var cType mach.CType
		var sqlType mach.SqlType
		switch val := arg.(type) {
		default:
			pd, _ := mach.CliDescribeParam(stmt.handle, idx)
			if val == nil {
				cType = mach.MACHCLI_C_TYPE_CHAR
				sqlType = pd.Type
				value = nil
				valueLen = 0
			} else {
				return api.ErrDatabaseBindUnknownType(idx, fmt.Sprintf("%T, expect: %d", val, pd.Type))
			}
		case int16:
			cType = mach.MACHCLI_C_TYPE_INT16
			sqlType = mach.MACHCLI_SQL_TYPE_INT16
			value = unsafe.Pointer(&val)
			valueLen = 2
		case *int16:
			cType = mach.MACHCLI_C_TYPE_INT16
			sqlType = mach.MACHCLI_SQL_TYPE_INT16
			value = unsafe.Pointer(val)
			valueLen = 2
		case int32:
			cType = mach.MACHCLI_C_TYPE_INT32
			sqlType = mach.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(&val)
			valueLen = 4
		case *int32:
			cType = mach.MACHCLI_C_TYPE_INT32
			sqlType = mach.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(val)
			valueLen = 4
		case int:
			cType = mach.MACHCLI_C_TYPE_INT32
			sqlType = mach.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(&val)
			valueLen = 4
		case *int:
			cType = mach.MACHCLI_C_TYPE_INT32
			sqlType = mach.MACHCLI_SQL_TYPE_INT32
			value = unsafe.Pointer(val)
			valueLen = 4
		case int64:
			cType = mach.MACHCLI_C_TYPE_INT64
			sqlType = mach.MACHCLI_SQL_TYPE_INT64
			value = unsafe.Pointer(&val)
			valueLen = 8
		case *int64:
			cType = mach.MACHCLI_C_TYPE_INT64
			sqlType = mach.MACHCLI_SQL_TYPE_INT64
			value = unsafe.Pointer(val)
			valueLen = 8
		case time.Time:
			cType = mach.MACHCLI_C_TYPE_INT64
			sqlType = mach.MACHCLI_SQL_TYPE_DATETIME
			n := new(int64)
			*n = val.UnixNano()
			value = unsafe.Pointer(n)
			valueLen = 8
		case *time.Time:
			cType = mach.MACHCLI_C_TYPE_INT64
			sqlType = mach.MACHCLI_SQL_TYPE_DATETIME
			n := new(int64)
			*n = val.UnixNano()
			value = unsafe.Pointer(n)
			valueLen = 8
		case float32:
			cType = mach.MACHCLI_C_TYPE_FLOAT
			sqlType = mach.MACHCLI_SQL_TYPE_FLOAT
			value = unsafe.Pointer(&val)
			valueLen = 4
		case *float32:
			cType = mach.MACHCLI_C_TYPE_FLOAT
			sqlType = mach.MACHCLI_SQL_TYPE_FLOAT
			value = unsafe.Pointer(val)
			valueLen = 4
		case float64:
			cType = mach.MACHCLI_C_TYPE_DOUBLE
			sqlType = mach.MACHCLI_SQL_TYPE_DOUBLE
			value = unsafe.Pointer(&val)
			valueLen = 8
		case *float64:
			cType = mach.MACHCLI_C_TYPE_DOUBLE
			sqlType = mach.MACHCLI_SQL_TYPE_DOUBLE
			value = unsafe.Pointer(val)
			valueLen = 8
		case net.IP:
			if len(val) == 4 {
				cType = mach.MACHCLI_C_TYPE_CHAR
				sqlType = mach.MACHCLI_SQL_TYPE_IPV4
				value = unsafe.Pointer(&(val.To4()[0]))
				valueLen = 4
			} else {
				cType = mach.MACHCLI_C_TYPE_CHAR
				sqlType = mach.MACHCLI_SQL_TYPE_IPV6
				value = unsafe.Pointer(&(val.To16()[0]))
				valueLen = 16
			}
		case string:
			cType = mach.MACHCLI_C_TYPE_CHAR
			sqlType = mach.MACHCLI_SQL_TYPE_STRING
			bStr := []byte(val)
			value = (unsafe.Pointer)(&bStr[0])
			valueLen = len(bStr)
		case *string:
			cType = mach.MACHCLI_C_TYPE_CHAR
			sqlType = mach.MACHCLI_SQL_TYPE_STRING
			bStr := []byte(*val)
			value = (unsafe.Pointer)(&bStr[0])
			valueLen = len(bStr)
		case []byte:
			cType = mach.MACHCLI_C_TYPE_CHAR
			sqlType = mach.MACHCLI_SQL_TYPE_BINARY
			value = (unsafe.Pointer)(&val[0])
			valueLen = len(val)
		}
		if err := mach.CliBindParam(stmt.handle, idx, cType, sqlType, value, valueLen); err != nil {
			return errorWithCause(stmt, err)
		}
	}
	return nil
}

type Result struct {
	message      string
	err          error
	rowsAffected int64
}

var _ api.Result = (*Result)(nil)

func (rs *Result) Message() string {
	return rs.message
}

func (rs *Result) Err() error {
	return rs.err
}

func (rs *Result) LastInsertId() (int64, error) {
	return 0, api.ErrNotImplemented("LastInsertId")
}

func (rs *Result) RowsAffected() int64 {
	return rs.rowsAffected
}

func (c *Conn) NewStmt() (*Stmt, error) {
	handle := new(unsafe.Pointer)
	if err := mach.CliAllocStmt(c.handle, handle); err != nil {
		return nil, errorWithCause(c, err)
	}
	ret := &Stmt{conn: c, handle: *handle}
	return ret, nil
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

func (st SqlType) ColumnType() api.ColumnType {
	switch st {
	default:
		return api.ColumnTypeUnknown
	case MACHCLI_SQL_TYPE_INT16:
		return api.ColumnTypeShort
	case MACHCLI_SQL_TYPE_INT32:
		return api.ColumnTypeInteger
	case MACHCLI_SQL_TYPE_INT64:
		return api.ColumnTypeLong
	case MACHCLI_SQL_TYPE_DATETIME:
		return api.ColumnTypeDatetime
	case MACHCLI_SQL_TYPE_FLOAT:
		return api.ColumnTypeFloat
	case MACHCLI_SQL_TYPE_DOUBLE:
		return api.ColumnTypeDouble
	case MACHCLI_SQL_TYPE_IPV4:
		return api.ColumnTypeIPv4
	case MACHCLI_SQL_TYPE_IPV6:
		return api.ColumnTypeIPv6
	case MACHCLI_SQL_TYPE_STRING:
		return api.ColumnTypeVarchar
	case MACHCLI_SQL_TYPE_BINARY:
		return api.ColumnTypeBinary
	}
}

func (st SqlType) DataType() api.DataType {
	switch st {
	default:
		return api.DataTypeAny
	case MACHCLI_SQL_TYPE_INT16:
		return api.DataTypeInt16
	case MACHCLI_SQL_TYPE_INT32:
		return api.DataTypeInt32
	case MACHCLI_SQL_TYPE_INT64:
		return api.DataTypeInt64
	case MACHCLI_SQL_TYPE_DATETIME:
		return api.DataTypeDatetime
	case MACHCLI_SQL_TYPE_FLOAT:
		return api.DataTypeFloat32
	case MACHCLI_SQL_TYPE_DOUBLE:
		return api.DataTypeFloat64
	case MACHCLI_SQL_TYPE_IPV4:
		return api.DataTypeIPv4
	case MACHCLI_SQL_TYPE_IPV6:
		return api.DataTypeIPv6
	case MACHCLI_SQL_TYPE_STRING:
		return api.DataTypeString
	case MACHCLI_SQL_TYPE_BINARY:
		return api.DataTypeBinary
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
	reachEOF    bool
	sqlHead     string
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
	if stmt.sqlHead != "SELECT" {
		return nil
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
func (stmt *Stmt) fetch() ([]any, error) {
	if stmt.reachEOF {
		return nil, api.ErrDatabaseFetch(fmt.Errorf("reached end of the result set"))
	}
	end, err := mach.CliFetch(stmt.handle)
	if err != nil {
		return nil, err
	}
	stmt.reachEOF = end
	if stmt.reachEOF {
		return nil, io.EOF
	}

	values := make([]any, len(stmt.columnDescs))
	for i, desc := range stmt.columnDescs {
		switch desc.Type {
		case MACHCLI_SQL_TYPE_INT16:
			var v = new(int16)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT16, unsafe.Pointer(v), 2); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = *v
			}
		case MACHCLI_SQL_TYPE_INT32:
			var v = new(int32)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT32, unsafe.Pointer(v), 4); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = *v
			}
		case MACHCLI_SQL_TYPE_INT64:
			var v = new(int64)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT64, unsafe.Pointer(v), 8); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = *v
			}
		case MACHCLI_SQL_TYPE_DATETIME:
			var v = new(int64)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_INT64, unsafe.Pointer(v), 8); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = time.Unix(0, *v)
			}
		case MACHCLI_SQL_TYPE_FLOAT:
			var v = new(float32)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_FLOAT, unsafe.Pointer(v), 4); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = *v
			}
		case MACHCLI_SQL_TYPE_DOUBLE:
			var v = new(float64)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_DOUBLE, unsafe.Pointer(v), 8); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = *v
			}
		case MACHCLI_SQL_TYPE_IPV4:
			var v = []byte{0, 0, 0, 0}
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), 4); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = net.IP(v)
			}
		case MACHCLI_SQL_TYPE_IPV6:
			var v = make([]byte, 16)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), 16); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = net.IP(v)
			}
		case MACHCLI_SQL_TYPE_STRING:
			var v = make([]byte, desc.Size+1)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), len(v)); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = string(v[0:n])
			}
		case MACHCLI_SQL_TYPE_BINARY:
			var v = make([]byte, desc.Size+1)
			if n, err := mach.CliGetData(stmt.handle, i, mach.MACHCLI_C_TYPE_CHAR, unsafe.Pointer(&v[0]), len(v)); err != nil {
				return nil, errorWithCause(stmt, err)
			} else if n == -1 {
				values[i] = nil
			} else {
				values[i] = v[0:n]
			}
		}
	}
	return values, nil
}

type Row struct {
	err     error
	values  []any
	columns api.Columns
}

var _ api.Row = (*Row)(nil)

func (r *Row) Success() bool {
	return r.err == nil
}

func (r *Row) Err() error {
	return r.err
}

func (r *Row) Columns() (api.Columns, error) {
	return r.columns, nil
}

func (r *Row) Scan(dest ...any) error {
	if len(dest) > len(r.values) {
		return api.ErrParamCount(len(r.values), len(dest))
	}
	for i, d := range dest {
		if r.values[i] == nil {
			dest[i] = nil
			continue
		}
		if err := api.Scan(r.values[i], d); err != nil {
			return err
		}
	}
	return nil
}

func (r *Row) Values() []any {
	return r.values
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
	stmt         *Stmt
	err          error
	row          []any
	rowsAffected int64
}

var _ api.Rows = (*Rows)(nil)

func (r *Rows) Err() error {
	return r.err
}

func (r *Rows) Close() error {
	if r.stmt != nil {
		return r.stmt.Close()
	}
	return nil
}

func (r *Rows) IsFetchable() bool {
	return r.stmt.sqlHead == "SELECT"
}

func (r *Rows) Columns() (api.Columns, error) {
	ret := make(api.Columns, len(r.stmt.columnDescs))
	for i, desc := range r.stmt.columnDescs {
		ret[i] = &api.Column{
			Name:     desc.Name,
			Length:   desc.Size,
			Type:     desc.Type.ColumnType(),
			DataType: desc.Type.DataType(),
		}
	}
	return ret, nil
}

func (r *Rows) Message() string {
	switch r.stmt.sqlHead {
	case "SELECT":
		return "Select successfully."
	case "INSERT":
		if r.rowsAffected == 0 {
			return "no rows inserted."
		} else if r.rowsAffected == 1 {
			return "a row inserted."
		} else {
			return fmt.Sprintf("%d rows inserted.", r.rowsAffected)
		}
	case "DELETE":
		if r.rowsAffected == 0 {
			return "no rows deleted."
		} else if r.rowsAffected == 1 {
			return "a row deleted."
		} else {
			return fmt.Sprintf("%d rows deleted.", r.rowsAffected)
		}
	case "CREATE":
		return "Created successfully."
	case "DROP":
		return "Dropped successfully."
	case "TRUNCATE":
		return "Truncated successfully."
	case "ALTER":
		return "Altered successfully."
	case "CONNECT":
		return "Connected successfully."
	default:
		return r.stmt.sqlHead + " executed."
	}
}

func (r *Rows) RowsAffected() int64 {
	return r.rowsAffected
}

func (r *Rows) Next() bool {
	if r.stmt.reachEOF {
		return false
	}
	row, err := r.stmt.fetch()
	if err != nil {
		r.err = err
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
	if r.err != nil {
		return r.err
	}
	if len(dest) > len(r.row) {
		return api.ErrParamCount(len(r.row), len(dest))
	}
	for i, d := range dest {
		if d == nil {
			continue
		}
		if r.row[i] == nil {
			if !api.ScanNull(dest[i]) {
				dest[i] = nil
			}
			continue
		}
		if err := api.Scan(r.row[i], d); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	ret := &Appender{tableName: strings.ToUpper(tableName)}
	for _, opt := range opts {
		switch o := opt.(type) {
		case *api.AppenderOptionBuffer:
			ret.errCheckCount = o.Threshold
		default:
			return nil, fmt.Errorf("unknown option type-%T", o)
		}
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

		ret.columns = append(ret.columns, &api.Column{
			Name:     d.Name,
			Length:   d.Size,
			DataType: api.ParseDataType(d.Type.String()),
		})
		ret.columnNames = append(ret.columnNames, d.Name)
		ret.columnTypes = append(ret.columnTypes, mach.SqlType(d.Type))
	}

	return ret, nil
}

type Appender struct {
	stmt          *Stmt
	tableName     string
	errCheckCount int
	columns       api.Columns
	columnNames   []string
	columnTypes   []mach.SqlType
}

var _ api.Appender = (*Appender)(nil)
var _ api.Flusher = (*Appender)(nil)

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

func (a *Appender) TableType() api.TableType {
	// TODO implement
	return api.TableTypeTag
}

func (a *Appender) Columns() (api.Columns, error) {
	return a.columns, nil
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

func (a *Appender) AppendLogTime(ts time.Time, values ...any) error {
	return api.ErrNotImplemented("AppendWithTimestamp")
}
