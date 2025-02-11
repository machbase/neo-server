package machsvr

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/api"
)

// Appender creates a new Appender for the given table.
// Appender should be closed as soon as finishing work, otherwise it may cause server side resource leak.
//
//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
//	defer cancelFunc()
//
//	app, _ := conn.Appender(ctx, "MY_TABLE")
//	defer app.Close()
//	app.Append("name", time.Now(), 3.14)
func (conn *Conn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	appender := &Appender{}
	appender.conn = conn
	appender.tableName = strings.ToUpper(tableName)

	_, userName, tableName := api.TableName(tableName).Split()

	for _, opt := range opts {
		switch opt.(type) {
		case *api.AppenderOptionBuffer:
			return nil, fmt.Errorf("unsupported option %T", opt)
		default:
			return nil, fmt.Errorf("unknown option type-%T", opt)
		}
	}

	// make a new internal connection to avoid MACH-ERR 2118
	// MACH-ERR 2118 Lock object was already initialized. (Do not use select and append simultaneously in single session.)
	if queryCon, err := conn.db.Connect(ctx, api.WithTrustUser(userName)); err != nil {
		return nil, err
	} else {
		defer queryCon.Close()

		// table type
		var describeTableSql = api.SqlTidy(
			`SELECT
				j.ID as TABLE_ID,
				j.TYPE as TABLE_TYPE,
				j.FLAG as TABLE_FLAG,
				j.COLCOUNT as TABLE_COLCOUNT
			from
				M$SYS_USERS u,
				M$SYS_TABLES j
			where
				u.NAME = ?
			and j.USER_ID = u.USER_ID
			and j.DATABASE_ID = ?
			and j.NAME = ?`)
		row := queryCon.QueryRow(ctx, describeTableSql, userName, -1, tableName)
		var tableId int32
		var tableType = api.TableType(-1)
		var tableFlag int32
		var colCount int32
		if err := row.Scan(&tableId, &tableType, &tableFlag, &colCount); err != nil {
			if err.Error() == "sql: no rows in result set" {
				return nil, fmt.Errorf("table '%s' does not exist", strings.ToUpper(appender.tableName))
			} else {
				return nil, fmt.Errorf("table '%s' does not exist, %s", strings.ToUpper(appender.tableName), err.Error())
			}
		}
		if tableType != api.TableTypeLog && tableType != api.TableTypeTag {
			return nil, fmt.Errorf("%s '%s' doesn't support append", tableType.String(), appender.tableName)
		}
		appender.tableType = api.TableType(tableType)

		// columns
		var columnsSql = api.SqlTidy(
			`SELECT
				name, type, length, id, flag
			FROM
				M$SYS_COLUMNS
			WHERE
				table_id = ?
			AND database_id = ?
			ORDER BY id`)
		rows, err := queryCon.Query(ctx, columnsSql, tableId, -1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			col := &api.Column{}
			if err := rows.Scan(&col.Name, &col.Type, &col.Length, &col.Id, &col.Flag); err != nil {
				return nil, err
			}
			// exclude _RID column
			if col.Name == "_RID" {
				continue
			}
			col.DataType = col.Type.DataType()
			appender.columns = append(appender.columns, col)
		}
	}
	if err := mach.EngAllocStmt(appender.conn.handle, &appender.stmt); err != nil {
		return nil, err
	}
	if err := mach.EngAppendOpen(appender.stmt, tableName); err != nil {
		mach.EngFreeStmt(appender.stmt)
		return nil, err
	}
	api.AllocAppender()

	colCount, err := mach.EngColumnCount(appender.stmt)
	if err != nil {
		mach.EngAppendClose(appender.stmt)
		mach.EngFreeStmt(appender.stmt)
		return nil, err
	}
	if colCount != len(appender.columns) {
		mach.EngAppendClose(appender.stmt)
		mach.EngFreeStmt(appender.stmt)
		return nil, fmt.Errorf("appender for '%s' doesn't match columns %d, %d", appender.tableName, colCount, len(appender.columns))
	}
	dataTypesString := make([]string, len(appender.columns))
	for i := 0; i < colCount; i++ {
		var columnName string
		var columnRawType, columnSize, columnLength int
		if err := mach.EngColumnInfo(appender.stmt, i, &columnName, &columnRawType, &columnSize, &columnLength); err != nil {
			mach.EngAppendClose(appender.stmt)
			mach.EngFreeStmt(appender.stmt)
			return nil, mach.ErrDatabaseWrap("MachColumnInfo %s", err)
		}
		dataType, err := columnRawTypeToDataType(columnRawType)
		if err != nil {
			mach.EngAppendClose(appender.stmt)
			mach.EngFreeStmt(appender.stmt)
			return nil, mach.ErrDatabaseWrap("MachColumnInfo data type %s", err)
		}
		if appender.columns[i].DataType != dataType {
			mach.EngAppendClose(appender.stmt)
			mach.EngFreeStmt(appender.stmt)
			return nil, fmt.Errorf("MachColumnInfo data type mismatch %s %d", appender.columns[i].DataType, columnRawType)
		}
		dataTypesString[i] = string(dataType)
	}
	appender.buffer = mach.EngMakeAppendBuffer(appender.stmt, appender.columns.Names(), dataTypesString)
	return appender, nil
}

type Appender struct {
	conn      *Conn
	stmt      unsafe.Pointer
	tableName string
	tableType api.TableType
	closed    bool
	columns   api.Columns

	inputColumns []AppenderInputColumn

	buffer *mach.AppendBuffer

	successCount int64
	failCount    int64
}

var _ api.Appender = (*Appender)(nil)

type AppenderInputColumn struct {
	Name string
	Idx  int
}

func (ap *Appender) WithInputColumns(columns ...string) api.Appender {
	ap.inputColumns = nil
	for _, col := range columns {
		ap.inputColumns = append(ap.inputColumns, AppenderInputColumn{Name: strings.ToUpper(col), Idx: -1})
	}
	if len(ap.inputColumns) > 0 {
		for idx, col := range ap.columns {
			for inIdx, inputCol := range ap.inputColumns {
				if col.Name == inputCol.Name {
					ap.inputColumns[inIdx].Idx = idx
				}
			}
		}
	}
	return ap
}

func (ap *Appender) Close() (int64, int64, error) {
	if ap.closed {
		return ap.successCount, ap.failCount, nil
	}
	ap.closed = true
	var err error
	api.FreeAppender()
	ap.successCount, ap.failCount, err = mach.EngAppendClose(ap.stmt)
	if err != nil {
		return ap.successCount, ap.failCount, err
	}

	if err := mach.EngFreeStmt(ap.stmt); err != nil {
		return ap.successCount, ap.failCount, err
	}
	return ap.successCount, ap.failCount, nil
}

func (ap *Appender) String() string {
	return fmt.Sprintf("appender %s %v", ap.tableName, ap.stmt)
}

func (ap *Appender) TableName() string {
	return ap.tableName
}

func (ap *Appender) Columns() (api.Columns, error) {
	return ap.columns, nil
}

func (ap *Appender) TableType() api.TableType {
	return ap.tableType
}

func (ap *Appender) Append(values ...any) error {
	if ap.tableType == api.TableTypeTag {
		return ap.append(values...)
	} else if ap.tableType == api.TableTypeLog {
		var colsWithTime []any
		if len(values) == len(ap.columns) {
			colsWithTime = values
		} else if len(values) == len(ap.columns)-1 {
			colsWithTime = append([]any{time.Time{}}, values...)
		}
		return ap.append(colsWithTime...)
	} else {
		return fmt.Errorf("%s can not be appended", ap.tableName)
	}
}

func (ap *Appender) AppendLogTime(ts time.Time, cols ...any) error {
	if ap.tableType != api.TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", ap.tableName)
	}
	colsWithTime := append([]any{ts}, cols...)
	return ap.append(colsWithTime...)
}

func (ap *Appender) append(values ...any) error {
	if len(ap.columns) == 0 {
		return api.ErrDatabaseNoColumns(ap.tableName)
	}
	if len(ap.inputColumns) > 0 {
		if len(ap.inputColumns) != len(values) {
			return api.ErrDatabaseLengthOfColumns(ap.tableName, len(ap.columns), len(values))
		}
		newValues := make([]any, len(ap.columns))
		for i, inputCol := range ap.inputColumns {
			newValues[inputCol.Idx] = values[i]
		}
		values = newValues
	} else {
		if len(ap.columns) != len(values) {
			return api.ErrDatabaseLengthOfColumns(ap.tableName, len(ap.columns), len(values))
		}
	}
	if ap.closed {
		return api.ErrDatabaseClosedAppender
	}
	if ap.conn == nil || !ap.conn.Connected() {
		return api.ErrDatabaseNoConnection
	}

	return ap.buffer.Append(values...)
}
