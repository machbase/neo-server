package machsvr

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api/types"
)

type AppenderOption func(*Appender)

// Appender creates a new Appender for the given table.
// Appender should be closed as soon as finishing work, otherwise it may cause server side resource leak.
//
//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
//	defer cancelFunc()
//
//	app, _ := conn.Appender(ctx, "MY_TABLE")
//	defer app.Close()
//	app.Append("name", time.Now(), 3.14)
func (conn *Conn) Appender(ctx context.Context, tableName string, opts ...AppenderOption) (*Appender, error) {
	appender := &Appender{}
	appender.conn = conn
	appender.tableName = strings.ToUpper(tableName)

	for _, opt := range opts {
		opt(appender)
	}

	// table type
	// make a new internal connection to avoid MACH-ERR 2118
	// MACH-ERR 2118 Lock object was already initialized. (Do not use select and append simultaneously in single session.)
	if queryCon, err := conn.db.Connect(ctx, WithTrustUser("sys")); err != nil {
		return nil, err
	} else {
		defer queryCon.Close()
		row := queryCon.QueryRow(ctx, "select type from M$SYS_TABLES where name = ?", appender.tableName)
		var typ int32 = -1
		if err := row.Scan(&typ); err != nil {
			if err.Error() == "sql: no rows in result set" {
				return nil, fmt.Errorf("table '%s' not found", appender.tableName)
			} else {
				return nil, fmt.Errorf("table '%s' not found, %s", appender.tableName, err.Error())
			}
		}
		if typ < 0 || typ > 6 {
			return nil, fmt.Errorf("table '%s' not found", tableName)
		}
		appender.tableType = types.TableType(typ)
	}
	if err := mach.EngAllocStmt(appender.conn.handle, &appender.stmt); err != nil {
		return nil, err
	}
	if err := mach.EngAppendOpen(appender.stmt, tableName); err != nil {
		mach.EngFreeStmt(appender.stmt)
		return nil, err
	}
	statz.AllocAppender()

	colCount, err := mach.EngColumnCount(appender.stmt)
	if err != nil {
		mach.EngAppendClose(appender.stmt)
		mach.EngFreeStmt(appender.stmt)
		return nil, err
	}
	appender.columnNames = make([]string, colCount)
	appender.columnTypes = make([]types.DataType, colCount)
	columnTypesString := make([]string, colCount)
	for i := 0; i < colCount; i++ {
		var columnName string
		var columnType, columnSize, columnLength int
		if err := mach.EngColumnInfo(appender.stmt, i, &columnName, &columnType, &columnSize, &columnLength); err != nil {
			mach.EngAppendClose(appender.stmt)
			mach.EngFreeStmt(appender.stmt)
			return nil, err
		}
		typ, err := columnRawTypeToDataType(columnType)
		if err != nil {
			mach.EngAppendClose(appender.stmt)
			mach.EngFreeStmt(appender.stmt)
			return nil, mach.ErrDatabaseWrap("MachColumnInfo %s", err)
		}
		appender.columnNames[i] = columnName
		appender.columnTypes[i] = typ
		columnTypesString[i] = string(typ)
	}
	appender.buffer = mach.EngMakeAppendBuffer(appender.stmt, appender.columnNames, columnTypesString)
	return appender, nil
}

type Appender struct {
	conn        *Conn
	stmt        unsafe.Pointer
	tableName   string
	tableType   types.TableType
	closed      bool
	columnNames []string
	columnTypes []types.DataType

	buffer *mach.AppendBuffer

	successCount int64
	failCount    int64
}

func (ap *Appender) Close() (int64, int64, error) {
	if ap.closed {
		return ap.successCount, ap.failCount, nil
	}
	ap.closed = true
	var err error
	statz.FreeAppender()
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

func (ap *Appender) Columns() ([]string, []types.DataType, error) {
	return ap.columnNames, ap.columnTypes, nil
}

func (ap *Appender) TableType() types.TableType {
	return ap.tableType
}

func (ap *Appender) Append(values ...any) error {
	if ap.tableType == types.TableTypeTag {
		return ap.append(values...)
	} else if ap.tableType == types.TableTypeLog {
		colsWithTime := append([]any{time.Time{}}, values...)
		return ap.append(colsWithTime...)
	} else {
		return fmt.Errorf("%s can not be appended", ap.tableName)
	}
}

func (ap *Appender) AppendWithTimestamp(ts time.Time, cols ...any) error {
	if ap.tableType == types.TableTypeLog {
		colsWithTime := append([]any{ts}, cols...)
		return ap.append(colsWithTime...)
	} else {
		return fmt.Errorf("%s is not a log table, use Append() instead", ap.tableName)
	}
}

func (ap *Appender) append(values ...any) error {
	if len(ap.columnTypes) == 0 {
		return types.ErrDatabaseNoColumns(ap.tableName)
	}
	if len(ap.columnNames) != len(values) {
		return types.ErrDatabaseLengthOfColumns(ap.tableName, len(ap.columnNames), len(values))
	}
	if ap.closed {
		return types.ErrDatabaseClosedAppender
	}
	if ap.conn == nil || !ap.conn.Connected() {
		return types.ErrDatabaseNoConnection
	}

	return ap.buffer.Append(values...)
}
