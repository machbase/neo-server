package machsvr

import (
	"database/sql"
	"fmt"
	"strings"
	"unsafe"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Result struct {
	err          error
	affectedRows int64
	stmtType     StmtType
}

var _ api.Result = (*Result)(nil)

func (r *Result) RowsAffected() int64 {
	return r.affectedRows
}

func (r *Result) Err() error {
	return r.err
}

func (r *Result) Message() string {
	if r.err != nil {
		return r.err.Error()
	}

	rows := "no row"
	if r.affectedRows == 1 {
		rows = "a row"
	} else if r.affectedRows > 1 {
		p := message.NewPrinter(language.English)
		rows = p.Sprintf("%d rows", r.affectedRows)
	}
	if r.stmtType.IsSelect() {
		return rows + " selected."
	} else if r.stmtType.IsInsert() {
		return rows + " inserted."
	} else if r.stmtType.IsUpdate() {
		return rows + " updated."
	} else if r.stmtType.IsDelete() {
		return rows + " deleted."
	} else if r.stmtType.IsAlterSystem() {
		return "system altered."
	} else if r.stmtType.IsDDL() {
		return "ok."
	}
	return fmt.Sprintf("ok.(%d)", r.stmtType)
}

// * DDL: 1-255
// * ALTER SYSTEM: 256-511
// * SELECT: 512
// * INSERT: 513
// * DELETE: 514-518
// * INSERT_SELECT: 519
// * UPDATE: 520
// * EXEC_ROLLUP: 522-524

type StmtType int

func (typ StmtType) IsSelect() bool {
	return typ == 512
}

func (typ StmtType) IsDDL() bool {
	return typ >= 1 && typ <= 255
}

func (typ StmtType) IsAlterSystem() bool {
	return typ >= 256 && typ <= 511
}

func (typ StmtType) IsInsert() bool {
	return typ == 513
}

func (typ StmtType) IsDelete() bool {
	return typ >= 514 && typ <= 518
}

func (typ StmtType) IsInsertSelect() bool {
	return typ == 519
}

func (typ StmtType) IsUpdate() bool {
	return typ == 520
}

func (typ StmtType) IsExecRollup() bool {
	return typ >= 522 && typ <= 524
}

type Row struct {
	ok      bool
	err     error
	columns api.Columns
	values  []any

	affectedRows int64
	stmtType     StmtType
}

var _ api.Row = (*Row)(nil)

func (row *Row) Success() bool {
	return row.ok
}

func (row *Row) Err() error {
	return row.err
}

func (row *Row) Values() []any {
	return row.values
}

func (row *Row) Columns() (api.Columns, error) {
	return row.columns, nil
}

func (row *Row) RowsAffected() int64 {
	return row.affectedRows
}

func (r *Row) Message() string {
	if r.err != nil {
		return r.err.Error()
	}

	rows := "no row"
	if r.affectedRows == 1 {
		rows = "a row"
	} else if r.affectedRows > 1 {
		p := message.NewPrinter(language.English)
		rows = p.Sprintf("%d rows", r.affectedRows)
	}
	if r.stmtType.IsSelect() {
		return rows + " selected."
	} else if r.stmtType.IsInsert() {
		return rows + " inserted."
	} else if r.stmtType.IsUpdate() {
		return rows + " updated."
	} else if r.stmtType.IsDelete() {
		return rows + " deleted."
	} else if r.stmtType.IsAlterSystem() {
		return "system altered."
	} else if r.stmtType.IsDDL() {
		return "ok."
	}
	return fmt.Sprintf("ok.(%d)", r.stmtType)
}

func (row *Row) Scan(cols ...any) error {
	if row.err != nil {
		return row.err
	}
	if !row.ok {
		return sql.ErrNoRows
	}
	for i := range cols {
		if i >= len(row.values) {
			return api.ErrDatabaseScanIndex(i, len(row.values))
		}
		var isNull = row.values[i] == nil
		if isNull {
			cols[i] = nil
		} else if row.err = api.Scan(row.values[i], cols[i]); row.err != nil {
			return row.err
		}
	}
	return nil
}

type Rows struct {
	stmt       unsafe.Pointer
	stmtType   StmtType
	sqlText    string
	columns    api.Columns
	fetchError error
}

// Close release all resources that assigned to the Rows
func (rows *Rows) Close() error {
	var err error
	if rows.stmt != nil {
		statz.FreeStmt()
		err = mach.EngFreeStmt(rows.stmt)
		rows.stmt = nil
	}
	rows.sqlText = ""
	return err
}

// IsFetchable returns true if statement that produced this Rows was fetch-able (e.g was select?)
func (rows *Rows) IsFetchable() bool {
	return rows.stmtType.IsSelect()
}

func (rows *Rows) StatementType() StmtType {
	return rows.stmtType
}

func (rows *Rows) RowsAffected() int64 {
	if rows.IsFetchable() {
		return 0
	}
	nrow, err := mach.EngEffectRows(rows.stmt)
	if err != nil {
		return 0
	}
	return nrow
}

func (rows *Rows) Columns() (api.Columns, error) {
	return rows.columns, nil
}

func (rows *Rows) definedMessage() (string, bool) {
	fields := strings.Fields(rows.sqlText)
	if len(fields) > 0 {
		head := strings.ToLower(fields[0])
		switch head {
		case "create":
			return "Created successfully.", true
		case "drop":
			return "Dropped successfully.", true
		case "truncate":
			return "Truncated successfully.", true
		case "alter":
			return "Altered successfully.", true
		case "connect":
			return "Connected successfully.", true
		}
	}
	return "", false
}

func (rows *Rows) Message() string {
	numRows := rows.RowsAffected()
	stmtType := rows.stmtType
	var verb = ""

	if stmtType >= 1 && stmtType <= 255 {
		if msg, ok := rows.definedMessage(); ok {
			return msg
		}
		return "executed."
	} else if stmtType >= 256 && stmtType <= 511 {
		if msg, ok := rows.definedMessage(); ok {
			return msg
		}
		return "system altered."
	} else if stmtType.IsSelect() {
		verb = "selected."
	} else if stmtType.IsInsert() {
		verb = "inserted."
	} else if stmtType.IsDelete() {
		verb = "deleted."
	} else if stmtType.IsInsertSelect() {
		verb = "select and inserted."
	} else if stmtType.IsUpdate() {
		verb = "updated."
	} else if stmtType.IsExecRollup() {
		return "rollup executed."
	} else {
		return fmt.Sprintf("executed (%d).", stmtType)
	}
	if numRows == 0 {
		return fmt.Sprintf("no rows %s", verb)
	} else if numRows == 1 {
		return fmt.Sprintf("a row %s", verb)
	} else {
		p := message.NewPrinter(language.English)
		return p.Sprintf("%d rows %s", numRows, verb)
	}
}

// internal use only from machrpc server
func (rows *Rows) Fetch() ([]any, bool, error) {
	// Do not proceed if the statement is not a SELECT
	if !rows.IsFetchable() {
		return nil, false, sql.ErrNoRows
	}

	next, err := mach.EngFetch(rows.stmt)
	if err != nil {
		return nil, next, api.ErrDatabaseFetch(err)
	}
	if !next {
		return nil, false, nil
	}

	values, err := rows.columns.MakeBuffer()
	if err != nil {
		return nil, next, fmt.Errorf("Fetch %s", err.Error())
	}
	for i := range values {
		if i >= len(rows.columns) {
			return values, next, api.ErrDatabaseScanIndex(i, len(rows.columns))
		}
		rawType, err := columnDataTypeToRawType(rows.columns[i].DataType)
		if err != nil {
			return values, next, err
		}
		if err = readColumnData(rows.stmt, rawType, i, values[i]); err != nil {
			return nil, next, err
		}
	}
	return values, next, nil
}

// Next returns true if there are at least one more fetchable record remained.
//
// rows, _ := db.Query("select name, value from my_table")
//
//	for rows.Next(){
//		var name string
//		var value float64
//		rows.Scan(&name, &value)
//	}
func (rows *Rows) Next() bool {
	// the statement is not SELECT
	if !rows.IsFetchable() {
		return false
	}
	next, err := mach.EngFetch(rows.stmt)
	if err != nil {
		rows.fetchError = err
		return false
	}
	return next
}

func (rows *Rows) FetchError() error {
	return rows.fetchError
}

// Scan retrieve values of columns in a row
//
//	for rows.Next(){
//		var name string
//		var value float64
//		rows.Scan(&name, &value)
//	}
func (rows *Rows) Scan(cols ...any) error {
	if !rows.IsFetchable() {
		return sql.ErrNoRows
	}
	for i := range cols {
		if i >= len(rows.columns) {
			return api.ErrDatabaseScanIndex(i, len(rows.columns))
		}
		rawType, err := columnDataTypeToRawType(rows.columns[i].DataType)
		if err != nil {
			return err
		}
		if err := readColumnData(rows.stmt, rawType, i, cols[i]); err != nil {
			return err
		}
	}
	return nil
}

func readColumnData(stmt unsafe.Pointer, rawType int, idx int, dst any) error {
	if dst == nil {
		return nil
	}
	switch rawType {
	case ColumnRawTypeInt16:
		v, nonNull, err := mach.EngColumnDataInt16(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("int16", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeInt32:
		v, nonNull, err := mach.EngColumnDataInt32(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("int32", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeInt64:
		v, nonNull, err := mach.EngColumnDataInt64(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("int64", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeDatetime:
		v, nonNull, err := mach.EngColumnDataDateTime(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("datetime", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeFloat32:
		v, nonNull, err := mach.EngColumnDataFloat32(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("float32", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeFloat64:
		v, nonNull, err := mach.EngColumnDataFloat64(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("float64", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeIPv4:
		v, nonNull, err := mach.EngColumnDataIPv4(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("IPv4", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeIPv6:
		v, nonNull, err := mach.EngColumnDataIPv6(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("IPv6", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeString:
		v, nonNull, err := mach.EngColumnDataString(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("string", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	case ColumnRawTypeBinary:
		v, nonNull, err := mach.EngColumnDataBinary(stmt, idx)
		if err != nil {
			return api.ErrDatabaseScanTypeName("binary", err)
		}
		if nonNull {
			return api.Scan(v, dst)
		}
	default:
		return api.ErrDatabaseScanUnsupportedType(dst)
	}
	return nil
}
