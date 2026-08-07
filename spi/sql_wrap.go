package spi

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/machbase/neo-client/api"
)

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
		ret[i] = api.NewColumnWithType(col)
		ret[i].Nullable = ok && nullable
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
