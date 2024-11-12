package connector

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/machbase/neo-server/api"
)

func NewConn(sqlConn *sql.Conn) api.Conn {
	return &Conn{sqlConn: sqlConn}
}

type Conn struct {
	sqlConn *sql.Conn
}

var _ api.Conn = (*Conn)(nil)

func (c *Conn) Close() error {
	return c.sqlConn.Close()
}

func (c *Conn) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	r, err := c.sqlConn.ExecContext(ctx, sqlText, params...)
	return &Result{sqlType: api.DetectSQLStatementType(sqlText), sqlResult: r, err: err}
}

func (c *Conn) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	r, err := c.sqlConn.QueryContext(ctx, sqlText, params...)
	return &Rows{sqlRows: r}, err
}

func (c *Conn) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	r, err := c.sqlConn.QueryContext(ctx, sqlText, params...)
	if err != nil {
		return &Row{err: err}
	}
	defer r.Close()

	ret := &Row{}
	rows := &Rows{sqlRows: r}
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

func (c *Conn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	return nil, api.ErrNotImplemented("Appender")
}

func (c *Conn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	return "", api.ErrNotImplemented("Explain")
}

type Result struct {
	sqlType   api.SQLStatementType
	sqlResult sql.Result
	err       error
}

var _ api.Result = (*Result)(nil)

func (r *Result) Err() error {
	return r.err
}

func (r *Result) Message() string {
	if r.err != nil {
		return r.err.Error()
	}
	switch r.sqlType {
	case api.SQLStatementTypeInsert:
		rowsCount := r.RowsAffected()
		if rowsCount == 0 {
			return "no rows inserted."
		} else if rowsCount == 1 {
			return "a row inserted."
		} else {
			return fmt.Sprintf("%d rows inserted.", rowsCount)
		}
	case api.SQLStatementTypeUpdate:
		rowsCount := r.RowsAffected()
		if rowsCount == 0 {
			return "no rows updated."
		} else if rowsCount == 1 {
			return "a row updated."
		} else {
			return fmt.Sprintf("%d rows updated.", rowsCount)
		}
	case api.SQLStatementTypeDelete:
		rowsCount := r.RowsAffected()
		if rowsCount == 0 {
			return "no rows deleted."
		} else if rowsCount == 1 {
			return "a row deleted."
		} else {
			return fmt.Sprintf("%d rows deleted.", rowsCount)
		}
	case api.SQLStatementTypeCreate:
		return "Created successfully."
	case api.SQLStatementTypeDrop:
		return "Dropped successfully."
	case api.SQLStatementTypeAlter:
		return "Altered successfully."
	case api.SQLStatementTypeSelect:
		return "Select successfully."
	default:
		return "executed."
	}
}

func (r *Result) RowsAffected() int64 {
	ret, err := r.sqlResult.RowsAffected()
	r.err = err
	return ret
}

type Row struct {
	err        error
	values     []any
	columns    api.Columns
	columnsErr error
}

var _ api.Row = (*Row)(nil)

func (r *Row) Err() error {
	return r.err
}

func (r *Row) RowsAffected() int64 {
	return 0
}

func (r *Row) Message() string {
	// TODO: implement
	return "success"
}

func (r *Row) Scan(values ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(values) > len(r.values) {
		return api.ErrDatabaseScanIndex(len(values), len(r.values))
	}
	for i := range values {
		if err := api.Scan(r.values[i], values[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Row) Columns() (api.Columns, error) {
	return r.columns, nil
}

type Rows struct {
	sqlRows *sql.Rows
}

var _ api.Rows = (*Rows)(nil)

func (r *Rows) Next() bool {
	return r.sqlRows.Next()
}

func (r *Rows) Scan(values ...any) error {
	return r.sqlRows.Scan(values...)
}

func (r *Rows) Close() error {
	return r.sqlRows.Close()
}

func (r *Rows) Columns() (api.Columns, error) {
	cols, err := r.sqlRows.ColumnTypes()
	ret := make([]*api.Column, len(cols))
	for i, col := range cols {
		ret[i] = &api.Column{
			Name:     col.Name(),
			DataType: scanTypeToDataType(col.ScanType().String()),
		}
		if nullable, ok := col.Nullable(); ok {
			ret[i].Nullable = nullable
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

func (r *Rows) IsFetchable() bool {
	return true
}

func (r *Rows) RowsAffected() int64 {
	return 0
}

func (r *Rows) Message() string {
	// TODO: implement
	return "success"
}

func scanTypeToDataType(sqlType string) api.DataType {
	switch sqlType {
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
	case "[]byte", "sql.RawBytes":
		return api.DataTypeBinary
	case "*interface {}":
		// SQLite binds `count(*)` field as `*interface {}`
		return api.DataTypeString
	default:
		return api.DataTypeAny
	}
}

type SqlBridgeBase struct {
}

func (b *SqlBridgeBase) Conn(c *sql.Conn) api.Conn {
	return &Conn{sqlConn: c}
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
