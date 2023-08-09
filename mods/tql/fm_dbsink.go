package tql

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/bridge"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

// Deprecated: no more required
func (node *Node) fmOUTPUT(args ...any) (any, error) {
	node.task.LogWarnf("OUTPUT() is deprecated.")
	if len(args) != 1 {
		return nil, ErrInvalidNumOfArgs("OUTPUT", 1, len(args))
	}
	return args[0], nil
}

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
	db spi.Database

	rowsAffected int64
	lastInsertId int64

	node    *Node
	bridge  *bridgeName
	columns []string

	table *Table
	tag   *Tag
}

func (ins *insert) Open(db spi.Database) error {
	ins.db = db
	return nil
}

func (ins *insert) Close() string {
	return fmt.Sprintf("%d rows inserted.", ins.rowsAffected)
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
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)",
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
		return errors.Wrapf(err, sqlText)
	}
	if br.SupportLastInsertId() {
		lastInsertId, err := result.LastInsertId()
		if err != nil {
			return errors.Wrapf(err, sqlText)
		}
		ins.lastInsertId = lastInsertId
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, sqlText)
	}
	ins.rowsAffected = rowsAffected
	return nil
}

func (ins *insert) _addRow(values []any) error {
	placeHolders := []string{}
	for range ins.columns {
		placeHolders = append(placeHolders, "?")
	}
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)",
		ins.table.Name,
		strings.Join(ins.columns, ","),
		strings.Join(placeHolders, ","))
	var err error
	if ins.tag == nil {
		if result := ins.db.Exec(sqlText, values...); result.Err() != nil {
			err = result.Err()
		}
	} else {
		if result := ins.db.Exec(sqlText, append([]any{ins.tag.Name}, values...)...); result.Err() != nil {
			err = result.Err()
		}
	}
	if err == nil {
		ins.rowsAffected++
	}
	return err
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
	db    spi.Database
	nrows int

	dbAppender spi.Appender

	table *Table
}

func (app *appender) Open(db spi.Database) (err error) {
	app.db = db
	app.dbAppender, err = app.db.Appender(app.table.Name)
	return
}

func (app *appender) Close() string {
	var succ, fail int64
	var err error
	if app.dbAppender != nil {
		succ, fail, err = app.dbAppender.Close()
	}
	if err != nil {
		return fmt.Sprintf("append fail, %s", err.Error())
	} else {
		return fmt.Sprintf("append %d rows (success %d, fail %d).", app.nrows, succ, fail)
	}
}

func (app *appender) AddRow(values []any) error {
	if app.dbAppender == nil {
		return errors.New("f(APPEND) no appender exists")
	}
	err := app.dbAppender.Append(values...)
	if err == nil {
		app.nrows++
	}
	return err
}
