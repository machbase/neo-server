package tql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/spi"
)

type Table struct {
	Name string
}

func (x *Node) fmTable(tableName string) *Table {
	return &Table{Name: tableName}
}

type Tag struct {
	Name   string
	Column string
}

func (x *Node) fmTag(name string, column ...string) *Tag {
	if len(column) == 0 {
		return &Tag{Name: name, Column: "name"}
	} else {
		return &Tag{Name: name, Column: column[0]}
	}
}

func (x *Node) fmInsert(args ...any) (*insert, error) {
	ret := &insert{}
	for _, arg := range args {
		switch v := arg.(type) {
		case *bridgeName:
			ret.bridge = v
		case string:
			ret.columns = append(ret.columns, v)
		case *Table:
			ret.table = v
		case *Tag:
			ret.tag = v
		}
	}
	if ret.table == nil {
		return nil, ErrArgs("INSERT", 0, "table is not specified")
	}
	if ret.bridge == nil && ret.tag != nil {
		ret.columns = append([]string{ret.tag.Column}, ret.columns...)
	}
	ret.node = x
	return ret, nil
}

type insert struct {
	conn      *sql.Conn
	ctx       context.Context
	ctxCancel context.CancelFunc

	rowsAffected int64
	lastInsertId int64

	node    *Node
	bridge  *bridgeName
	columns []string

	table *Table
	tag   *Tag
}

func (ins *insert) Open(task *Task) error {
	ins.ctx, ins.ctxCancel = context.WithCancel(task.ctx)
	if conn, err := spi.Connect(ins.ctx, ins.node.task.consoleUser); err != nil {
		return err
	} else {
		ins.conn = conn
	}
	return nil
}

func (ins *insert) Close() (string, error) {
	ins.conn.Close()
	ins.ctxCancel()

	unit := "rows"
	if ins.rowsAffected <= 1 {
		unit = "row"
	}
	return fmt.Sprintf("%d %s inserted.", ins.rowsAffected, unit), nil
}

func (ins *insert) AddRow(values []any) error {
	if ins.bridge != nil {
		return ins._addRowBridge(values)
	} else {
		return ins._addRow(values)
	}

}
func (ins *insert) _addRowBridge(values []any) error {
	br, err := bridge.GetSqlBridge(ins.bridge.name)
	if err != nil {
		return err
	}

	placeHolders := []string{}
	for idx := range ins.columns {
		placeHolders = append(placeHolders, br.ParameterMarker(idx))
	}
	sqlText := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)",
		ins.table.Name,
		strings.Join(ins.columns, ","),
		strings.Join(placeHolders, ","))
	conn, err := br.Connect(ins.node.task.ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ins.node.task.ctx, sqlText, values...)
	if err != nil {
		return fmt.Errorf("%s, %s", err, sqlText)
	}
	if br.SupportLastInsertId() {
		lastInsertId, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("%s, %s", err, sqlText)
		}
		ins.lastInsertId = lastInsertId
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s, %s", err, sqlText)
	}
	ins.rowsAffected = rowsAffected
	return nil
}

func (ins *insert) _addRow(values []any) error {
	placeHolders := []string{}
	for range ins.columns {
		placeHolders = append(placeHolders, "?")
	}
	sqlText := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)",
		ins.table.Name,
		strings.Join(ins.columns, ","),
		strings.Join(placeHolders, ","))
	if ins.tag == nil {
		if _, err := ins.conn.ExecContext(ins.ctx, sqlText, values...); err != nil {
			return err
		}
	} else {
		if _, err := ins.conn.ExecContext(ins.ctx, sqlText, append([]any{ins.tag.Name}, values...)...); err != nil {
			return err
		}
	}
	ins.rowsAffected++
	return nil
}

func (x *Node) fmAppend(args ...any) (*appender, error) {
	ret := &appender{}
	for i, arg := range args {
		switch v := arg.(type) {
		case *Table:
			ret.table = v
		case *bridgeName:
			return nil, ErrArgs("APPEND", i, "cannot use with a bridge")
		}
	}
	if ret.table == nil {
		return nil, ErrArgs("APPEND", 0, "table is not specified")
	}
	return ret, nil
}

type appender struct {
	nrows      int
	dbAppender api.Appender
	dbColumns  api.Columns
	table      *Table
}

func (app *appender) Open(task *Task) (err error) {
	aw, err := spi.GetAppendWorker(task.ctx, app.table.Name)
	if err != nil {
		return
	}
	app.dbAppender = aw
	return
}

func (app *appender) Close() (string, error) {
	var succ, fail int64
	var err error
	if app.dbAppender != nil {
		succ, fail, err = app.dbAppender.Close()
	}
	_ = succ
	if err != nil {
		return fmt.Sprintf("append fail, %s", err.Error()), err
	} else {
		unit := "rows"
		if app.nrows <= 1 {
			unit = "row"
		}
		// since we are using api.AppendWraper, success is always nrows
		return fmt.Sprintf("append %d %s (success %d, fail %d)", app.nrows, unit, app.nrows, fail), nil
	}
}

func (app *appender) AddRow(values []any) error {
	if app.dbAppender == nil {
		return errors.New("f(APPEND) no appender exists")
	}
	if app.dbColumns == nil {
		if columns, err := app.dbAppender.Columns(); err != nil {
			return fmt.Errorf("failed to get appender columns, %s", err.Error())
		} else {
			app.dbColumns = columns
		}
	}

	var timeformat string = "ns"
	var timeLocation *time.Location
	for idx, col := range app.dbColumns {
		if idx >= len(values) {
			return fmt.Errorf("missing value for column %s", col.Name)
		}
		if values[idx] == nil {
			continue
		}
		val, err := col.DataType.Apply(values[idx], timeformat, timeLocation)
		if err != nil {
			return fmt.Errorf("invalid value for column %s: %v, error: %s", col.Name, values[idx], err.Error())
		} else {
			values[idx] = val
		}
	}

	err := app.dbAppender.Append(values...)
	if err == nil {
		app.nrows++
	}
	return err
}

func (x *Node) fmSqlSink(args ...any) (*sqlSink, error) {
	if len(args) == 0 {
		return nil, ErrInvalidNumOfArgs("SQL", 1, 0)
	}

	ret := &sqlSink{node: x}
	var paramStart int

	switch v := args[0].(type) {
	case string:
		if conn, err := spi.Connect(x.task.ctx, x.task.consoleUser); err != nil {
			return nil, err
		} else {
			ret.conn = conn
		}
		ret.sqlText = strings.TrimSuffix(strings.TrimSpace(v), ";")
		paramStart = 1
	case *bridgeName:
		db, err := connector.Database(v.name)
		if err != nil {
			return nil, err
		}
		if conn, err := db.Conn(x.task.ctx); err != nil {
			return nil, err
		} else {
			ret.conn = conn
		}
		if len(args) < 2 {
			return nil, ErrInvalidNumOfArgs("SQL", 2, len(args))
		}
		sqlText, ok := args[1].(string)
		if !ok {
			return nil, ErrWrongTypeOfArgs("SQL", 1, "sql text", args[1])
		}
		ret.sqlText = strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
		paramStart = 2
	default:
		return nil, ErrWrongTypeOfArgs("SQL", 0, "sql text or bridge('name')", args[0])
	}

	if len(ret.sqlText) == 0 {
		return nil, fmt.Errorf("f(SQL) Empty SQL text")
	}
	ret.stmtType = spi.DetectSQLStatementType(ret.sqlText)
	if err := validateSqlVerbForSink(ret.sqlText); err != nil {
		return nil, err
	}

	ret.rawParams = make([]any, 0, len(args)-paramStart)
	for i := paramStart; i < len(args); i++ {
		ret.rawParams = append(ret.rawParams, args[i])
	}
	return ret, nil
}

type sqlSink struct {
	node      *Node
	sqlText   string
	stmtType  spi.SQLStatementType
	rawParams []any

	ctx       context.Context
	ctxCancel context.CancelFunc
	conn      *sql.Conn

	affectedRows int64
	resultMsg    string
}

func (s *sqlSink) Open(task *Task) error {
	s.ctx, s.ctxCancel = context.WithCancel(task.ctx)
	if s.conn == nil {
		return fmt.Errorf("f(SQL) no connection exists")
	}
	return nil
}

func (s *sqlSink) Close() (string, error) {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	return spi.MakeUserMessage(s.stmtType, s.affectedRows), nil
}

func (s *sqlSink) AddRow(values []any) error {
	params := make([]any, 0, len(s.rawParams))
	for _, p := range s.rawParams {
		switch v := p.(type) {
		case *recordValueRef:
			if v == nil {
				params = append(params, nil)
				continue
			}
			if v.index < 0 || v.index >= len(values) {
				return fmt.Errorf("f(SQL) value(%d) is out of range of input tuple(len:%d)", v.index, len(values))
			}
			params = append(params, values[v.index])
		default:
			params = append(params, p)
		}
	}
	result, err := s.conn.ExecContext(s.ctx, s.sqlText, params...)
	if err != nil {
		return err
	}
	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	s.resultMsg = spi.MakeUserMessage(s.stmtType, affectedRows)
	if n, ok := parseRowsAffectedFromMessage(s.resultMsg); ok {
		s.affectedRows += n
	} else {
		s.affectedRows++
	}
	return nil
}

func validateSqlVerbForSink(sqlText string) error {
	stmtType := spi.DetectSQLStatementType(sqlText)
	if stmtType.IsFetch() {
		verb := strings.ToUpper(strings.Fields(sqlText)[0])
		return fmt.Errorf("f(SQL) sink does not allow fetch verb %q", verb)
	}
	return nil
}

func parseRowsAffectedFromMessage(msg string) (int64, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(msg))
	if trimmed == "" {
		return 0, false
	}
	if strings.HasPrefix(trimmed, "a row ") {
		return 1, true
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 3 {
		return 0, false
	}
	if fields[1] != "row" && fields[1] != "rows" {
		return 0, false
	}
	var n int64
	if _, err := fmt.Sscan(fields[0], &n); err != nil {
		return 0, false
	}
	return n, true
}
