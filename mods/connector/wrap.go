package connector

import (
	"context"
	"database/sql"
	"errors"

	spi "github.com/machbase/neo-spi"
)

type sqlWrap struct {
	SqlConnector
}

var _ spi.Database = &sqlWrap{}

var ErrNotImplmemented = errors.New("not implemented")

func (sw *sqlWrap) GetServerInfo() (*spi.ServerInfo, error) {
	return nil, ErrNotImplmemented
}

func (sw *sqlWrap) GetServicePorts(service string) ([]*spi.ServicePort, error) {
	return nil, ErrNotImplmemented
}

func (sw *sqlWrap) Explain(sqlText string, full bool) (string, error) {
	return "", ErrNotImplmemented
}

func (sw *sqlWrap) Appender(tableName string, opts ...spi.AppendOption) (spi.Appender, error) {
	return nil, ErrNotImplmemented
}

func (sw *sqlWrap) Exec(sqlText string, params ...any) spi.Result {
	return sw.ExecContext(context.TODO(), sqlText, params...)
}

func (sw *sqlWrap) Query(sqlText string, params ...any) (spi.Rows, error) {
	return sw.QueryContext(context.TODO(), sqlText, params...)
}

func (sw *sqlWrap) QueryRow(sqlText string, params ...any) spi.Row {
	return sw.QueryRowContext(context.TODO(), sqlText, params...)
}

func (sw *sqlWrap) ExecContext(ctx context.Context, sqlText string, params ...any) spi.Result {
	conn, err := sw.Connect(ctx)
	if err != nil {
		return &sqlWrapResult{err: err}
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, sqlText, params...)
	if err != nil {
		return &sqlWrapResult{err: err}
	}
	ra, err := result.RowsAffected()
	if err != nil {
		return &sqlWrapResult{err: err}
	}
	var message = "executed."
	return &sqlWrapResult{rowsAffected: ra, message: message}
}

func (sw *sqlWrap) QueryRowContext(ctx context.Context, sqlText string, params ...any) spi.Row {
	conn, err := sw.Connect(ctx)
	if err != nil {
		return &sqlWrapRow{err: err}
	}
	defer conn.Close()

	row := conn.QueryRowContext(ctx, sqlText, params)
	if row.Err() != nil {
		return &sqlWrapRow{err: row.Err()}
	}
	return &sqlWrapRow{success: true, row: row}
}

func (sw *sqlWrap) QueryContext(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
	conn, err := sw.Connect(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, sqlText, params...)
	if err != nil {
		return nil, err
	}
	return &sqlWrapRows{conn: conn, rows: rows}, nil
}

type sqlWrapResult struct {
	err          error
	rowsAffected int64
	message      string
}

func (r *sqlWrapResult) Err() error {
	return r.err
}

func (r *sqlWrapResult) RowsAffected() int64 {
	return r.rowsAffected
}

func (r *sqlWrapResult) Message() string {
	return r.message
}

type sqlWrapRow struct {
	err     error
	success bool
	row     *sql.Row
}

func (r *sqlWrapRow) Err() error {
	return r.err
}

func (r *sqlWrapRow) Success() bool {
	return r.success
}

func (r *sqlWrapRow) Scan(cols ...any) error {
	if r.row == nil {
		return errors.New("invalid state of row")
	}
	return r.row.Scan(cols...)
}

func (r *sqlWrapRow) Values() []any {
	return nil
}

func (r *sqlWrapRow) RowsAffected() int64 {
	return 0
}

func (r *sqlWrapRow) Message() string {
	return "executed."
}

type sqlWrapRows struct {
	conn *sql.Conn
	rows *sql.Rows
}

func (r *sqlWrapRows) Next() bool {
	if r.rows == nil {
		return false
	}
	return r.rows.Next()
}

func (r *sqlWrapRows) Scan(cols ...any) error {
	return r.rows.Scan(cols)
}

func (r *sqlWrapRows) Close() error {
	if r.rows == nil || r.conn == nil {
		return errors.New("invalid state of rows")
	}
	err := r.rows.Close()
	if err != nil {
		return err
	}
	return r.conn.Close()
}

func (r *sqlWrapRows) IsFetchable() bool {
	return false
}

func (r *sqlWrapRows) RowsAffected() int64 {
	return 0
}

func (r *sqlWrapRows) Message() string {
	return "success."
}

func (r *sqlWrapRows) Columns() (spi.Columns, error) {
	types, err := r.rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	names, err := r.rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(types) != len(names) {
		return nil, errors.New("invalid state of rows")
	}
	cols := make([]*spi.Column, len(names))
	for i := range names {
		length, ok := types[i].Length()
		if !ok {
			length = 0
		}
		cols[i] = &spi.Column{
			Name:   names[i],
			Type:   types[i].Name(),
			Size:   int(length),
			Length: int(length),
		}
	}
	return cols, nil
}
