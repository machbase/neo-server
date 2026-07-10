package spi

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machbase"
)

func WrapSqlConn(sqlConn *sql.Conn) api.Conn {
	return &WrappedSqlConn{sqlConn: sqlConn}
}

type WrappedSqlConn struct {
	sqlConn *sql.Conn
}

var _ api.Conn = (*WrappedSqlConn)(nil)

func (c *WrappedSqlConn) Close() error {
	return c.sqlConn.Close()
}

func (c *WrappedSqlConn) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	r, err := c.sqlConn.ExecContext(ctx, sqlText, params...)
	return &WrappedSqlResult{sqlType: DetectSQLStatementType(sqlText), sqlResult: r, err: err}
}

func (c *WrappedSqlConn) Prepare(ctx context.Context, sqlText string) (api.Stmt, error) {
	panic("not implemented")
}

func (c *WrappedSqlConn) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	sqlType := DetectSQLStatementType(sqlText)
	if !sqlType.IsFetch() {
		result, err := c.sqlConn.ExecContext(ctx, sqlText, params...)
		if err != nil {
			return nil, err
		}
		rows := &WrappedSqlRows{sqlType: sqlType}
		if result != nil {
			if affected, err := result.RowsAffected(); err != nil {
				return nil, err
			} else {
				rows.rowCount = affected
			}
		}
		return rows, nil
	}
	r, err := c.sqlConn.QueryContext(ctx, sqlText, params...)
	return &WrappedSqlRows{sqlRows: r, sqlType: sqlType}, err
}

func (c *WrappedSqlConn) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	r, err := c.sqlConn.QueryContext(ctx, sqlText, params...)
	if err != nil {
		return &WrappedSqlRow{err: err}
	}
	defer r.Close()

	ret := &WrappedSqlRow{timeLocation: time.UTC}
	rows := &WrappedSqlRows{sqlRows: r}
	ret.columns, ret.columnsErr = rows.Columns()
	if ret.columnsErr != nil {
		ret.err = ret.columnsErr
		return ret
	}
	ret.values, ret.err = ret.columns.MakeBuffer()
	if ret.err != nil {
		return ret
	}
	if !rows.Next() {
		ret.err = sql.ErrNoRows
		return ret
	}
	ret.err = rows.Scan(ret.values...)
	return ret
}

func (c *WrappedSqlConn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	return nil, api.ErrNotImplemented("Appender")
}

func (c *WrappedSqlConn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	var ret string
	var err error

	c.sqlConn.Raw(func(driverConn any) error {
		conn, isMachbase := driverConn.(*machbase.Conn)
		if !isMachbase {
			err = api.ErrNotImplemented("Explain")
			return nil
		}
		ret, err = conn.Explain(ctx, sqlText, full)
		return nil
	})
	return ret, err
}

type WrappedSqlResult struct {
	sqlType   SQLStatementType
	sqlResult sql.Result
	err       error
}

var _ api.Result = (*WrappedSqlResult)(nil)

func (r *WrappedSqlResult) Err() error {
	return r.err
}

func (r *WrappedSqlResult) Message() string {
	if r.err != nil {
		return r.err.Error()
	}
	switch r.sqlType {
	case SQLStatementTypeInsert:
		rowsCount := r.RowsAffected()
		switch rowsCount {
		case 0:
			return "no rows inserted."
		case 1:
			return "a row inserted."
		default:
			return fmt.Sprintf("%d rows inserted.", rowsCount)
		}
	case SQLStatementTypeUpdate:
		rowsCount := r.RowsAffected()
		switch rowsCount {
		case 0:
			return "no rows updated."
		case 1:
			return "a row updated."
		default:
			return fmt.Sprintf("%d rows updated.", rowsCount)
		}
	case SQLStatementTypeDelete:
		rowsCount := r.RowsAffected()
		switch rowsCount {
		case 0:
			return "no rows deleted."
		case 1:
			return "a row deleted."
		default:
			return fmt.Sprintf("%d rows deleted.", rowsCount)
		}
	case SQLStatementTypeCreate:
		return "Created successfully."
	case SQLStatementTypeDrop:
		return "Dropped successfully."
	case SQLStatementTypeAlter:
		return "Altered successfully."
	case SQLStatementTypeSelect:
		return "Select successfully."
	default:
		return "executed."
	}
}

func (r *WrappedSqlResult) RowsAffected() int64 {
	ret, err := r.sqlResult.RowsAffected()
	r.err = err
	return ret
}

type WrappedSqlRow struct {
	err          error
	values       []any
	columns      api.Columns
	columnsErr   error
	timeLocation *time.Location
}

var _ api.Row = (*WrappedSqlRow)(nil)

func (r *WrappedSqlRow) Err() error {
	return r.err
}

func (r *WrappedSqlRow) RowsAffected() int64 {
	return 0
}

func (r *WrappedSqlRow) Message() string {
	// TODO: implement
	return "success"
}

func (r *WrappedSqlRow) Scan(values ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(values) > len(r.values) {
		return api.ErrDatabaseScanIndex(len(values), len(r.values))
	}
	for i := range values {
		if r.values[i] == nil {
			values[i] = nil
			continue
		}
		if err := api.Scan(r.values[i], values[i], r.timeLocation); err != nil {
			return err
		}
	}
	return nil
}

func (r *WrappedSqlRow) Columns() (api.Columns, error) {
	return r.columns, nil
}

type WrappedSqlRows struct {
	sqlRows  *sql.Rows
	sqlType  SQLStatementType
	rowCount int64
	err      error
}

var _ api.Rows = (*WrappedSqlRows)(nil)

func (r *WrappedSqlRows) Next() bool {
	if r.sqlRows == nil {
		return false
	}
	if !r.sqlRows.Next() {
		return false
	}
	r.rowCount++
	return true
}

func (r *WrappedSqlRows) Scan(values ...any) error {
	if r.sqlRows == nil {
		return nil
	}
	if err := r.sqlRows.Scan(values...); err != nil {
		return err
	}

	for i, val := range values {
		switch v := val.(type) {
		case *sql.NullFloat64:
			if v.Valid {
				values[i] = v.Float64
			} else {
				values[i] = nil
			}
		case *sql.NullInt64:
			if v.Valid {
				values[i] = v.Int64
			} else {
				values[i] = nil
			}
		case *sql.NullInt32:
			if v.Valid {
				values[i] = v.Int32
			} else {
				values[i] = nil
			}
		case *sql.NullInt16:
			if v.Valid {
				values[i] = v.Int16
			} else {
				values[i] = nil
			}
		case *sql.NullString:
			if v.Valid {
				values[i] = v.String
			} else {
				values[i] = nil
			}
		case *sql.Null[api.JSONString]:
			if v.Valid {
				values[i] = v.V
			} else {
				values[i] = nil
			}
		case *sql.NullBool:
			if v.Valid {
				values[i] = v.Bool
			} else {
				values[i] = nil
			}
		case *sql.NullTime:
			if v.Valid {
				values[i] = v.Time
			} else {
				values[i] = nil
			}
		}
	}
	return nil
}

func (r *WrappedSqlRows) Close() error {
	if r.sqlRows == nil {
		return nil
	}
	return r.sqlRows.Close()
}

func (r *WrappedSqlRows) Columns() (api.Columns, error) {
	if r.sqlRows == nil {
		return nil, nil
	}
	cols, err := r.sqlRows.ColumnTypes()
	ret := make([]*api.Column, len(cols))
	for i, col := range cols {
		nullable, ok := col.Nullable()
		ret[i] = &api.Column{
			Name:     col.Name(),
			DataType: SqlColumnTypeToDataType(col),
			Nullable: ok && nullable,
		}
		if length, ok := col.Length(); ok {
			if length <= math.MaxInt {
				ret[i].Length = int(length)
			} else {
				ret[i].Length = math.MaxInt
			}
		}
	}
	return ret, err
}

func (r *WrappedSqlRows) IsFetchable() bool {
	return r.sqlType.IsFetch()
}

func (r *WrappedSqlRows) RowsAffected() int64 {
	return r.rowCount
}

func (r *WrappedSqlRows) Message() string {
	rowsCount := r.RowsAffected()
	switch r.sqlType {
	case SQLStatementTypeSelect, SQLStatementTypeDescribe:
		switch rowsCount {
		case 0:
			return "no rows selected."
		case 1:
			return "a row selected."
		default:
			return fmt.Sprintf("%d rows selected.", rowsCount)
		}
	case SQLStatementTypeInsert:
		switch rowsCount {
		case 0:
			return "no rows inserted."
		case 1:
			return "a row inserted."
		default:
			return fmt.Sprintf("%d rows inserted.", rowsCount)
		}
	case SQLStatementTypeUpdate:
		switch rowsCount {
		case 0:
			return "no rows updated."
		case 1:
			return "a row updated."
		default:
			return fmt.Sprintf("%d rows updated.", rowsCount)
		}
	case SQLStatementTypeDelete:
		switch rowsCount {
		case 0:
			return "no rows deleted."
		case 1:
			return "a row deleted."
		default:
			return fmt.Sprintf("%d rows deleted.", rowsCount)
		}
	case SQLStatementTypeCreate:
		return "Created successfully."
	case SQLStatementTypeDrop:
		return "Dropped successfully."
	case SQLStatementTypeAlter:
		return "Altered successfully."
	default:
		return "executed."
	}
}

func (r *WrappedSqlRows) Err() error {
	if r.err != nil {
		return r.err
	}
	if r.sqlRows == nil {
		return nil
	}
	return r.sqlRows.Err()
}

func SqlColumnTypeToDataType(col *sql.ColumnType) api.DataType {
	switch col.DatabaseTypeName() {
	case "VARCHAR", "TEXT", "NCHAR", "NVARCHAR":
		return api.DataTypeString
	case "JSON":
		return api.DataTypeJSON
	}
	switch col.ScanType().String() {
	case "bool", "sql.NullBool":
		return api.DataTypeBoolean
	case "int8", "sql.NullByte":
		return api.DataTypeInt16
	case "int16", "sql.NullInt16":
		return api.DataTypeInt16
	case "int32", "sql.NullInt32":
		return api.DataTypeInt32
	case "int64", "sql.NullInt64":
		return api.DataTypeInt64
	case "float32":
		return api.DataTypeFloat32
	case "float64", "sql.NullFloat64":
		return api.DataTypeFloat64
	case "string", "sql.NullString":
		return api.DataTypeString
	case "time.Time", "sql.NullTime":
		return api.DataTypeDatetime
	case "[]byte", "[]uint8", "sql.RawBytes":
		return api.DataTypeBinary
	case "*interface {}":
		// FIXME: SQLite binds `count(*)` field as `*interface {}`
		return api.DataTypeString
	default:
		return api.DataTypeAny
	}
}

type SqlBridgeBase struct {
}

func (b *SqlBridgeBase) Conn(c *sql.Conn) api.Conn {
	return &WrappedSqlConn{sqlConn: c}
}

func (b *SqlBridgeBase) NewScanType(reflectType string, databaseTypeName string) any {
	switch reflectType {
	case "sql.NullBool":
		return new(bool)
	case "sql.NullByte":
		return new(uint8)
	case "sql.NullFloat64":
		return new(float64)
	case "sql.NullInt16":
		return new(int16)
	case "sql.NullInt32":
		return new(int32)
	case "sql.NullInt64":
		return new(int64)
	case "sql.NullString":
		return new(string)
	case "sql.NullTime":
		return new(time.Time)
	case "sql.RawBytes":
		return new([]byte)
	case "[]uint8":
		return new([]byte)
	case "bool":
		return new(bool)
	case "int32":
		return new(int32)
	case "int64":
		return new(int64)
	case "string":
		return new(string)
	case "time.Time":
		return new(time.Time)
	}
	return nil
}

func (c *SqlBridgeBase) NormalizeType(values []any) []any {
	for i, val := range values {
		switch v := val.(type) {
		case sql.RawBytes:
			values[i] = []byte(v)
		case *sql.NullBool:
			if v.Valid {
				values[i] = v.Bool
			} else {
				values[i] = nil
			}
		case *sql.NullByte:
			if v.Valid {
				values[i] = v.Byte
			} else {
				values[i] = nil
			}
		case *sql.NullFloat64:
			if v.Valid {
				values[i] = v.Float64
			} else {
				values[i] = nil
			}
		case *sql.NullInt16:
			if v.Valid {
				values[i] = v.Int16
			} else {
				values[i] = nil
			}
		case *sql.NullInt32:
			if v.Valid {
				values[i] = v.Int32
			} else {
				values[i] = nil
			}
		case *sql.NullInt64:
			if v.Valid {
				values[i] = v.Int64
			} else {
				values[i] = nil
			}
		case *sql.NullString:
			if v.Valid {
				values[i] = v.String
			} else {
				values[i] = nil
			}
		case *sql.NullTime:
			if v.Valid {
				values[i] = v.Time
			} else {
				values[i] = nil
			}
		}
	}
	return values
}
