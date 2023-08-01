package maps

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/conv"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Table struct {
	Name string
}

func ToTable(tableName string) *Table {
	return &Table{Name: tableName}
}

type Tag struct {
	Name   string
	Column string
}

func ToTag(name string, column ...string) *Tag {
	if len(column) == 0 {
		return &Tag{Name: name, Column: "name"}
	} else {
		return &Tag{Name: name, Column: column[0]}
	}
}

func ToInsert(args ...any) (*insert, error) {
	ret := &insert{}
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			ret.columns = append(ret.columns, v)
		case *Table:
			ret.table = v
		case *Tag:
			ret.tag = v
		}
	}
	if ret.table == nil {
		return nil, conv.ErrArgs("INSERT", 0, "table is not specified")
	}
	if ret.tag != nil {
		ret.columns = append([]string{ret.tag.Column}, ret.columns...)
	}
	for range ret.columns {
		ret.placeHolders = append(ret.placeHolders, "?")
	}
	return ret, nil
}

type insert struct {
	db     spi.Database
	writer spec.OutputStream
	nrows  int

	columns      []string
	placeHolders []string

	table *Table
	tag   *Tag
}

func (ins *insert) Open(db spi.Database) error {
	ins.db = db
	return nil
}

func (ins *insert) Close() {
	if ins.writer != nil {
		ins.writer.Write([]byte(fmt.Sprintf("%d rows inserted.", ins.nrows)))
	}
}

func (ins *insert) SetOutputStream(w spec.OutputStream) {
	ins.writer = w
}

func (ins *insert) AddRow(values []any) error {
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)", ins.table.Name, strings.Join(ins.columns, ","), strings.Join(ins.placeHolders, ","))
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
		ins.nrows++
	}
	return err
}

func ToAppend(args ...any) (*appender, error) {
	ret := &appender{}
	for _, arg := range args {
		switch v := arg.(type) {
		case *Table:
			ret.table = v
		}
	}
	if ret.table == nil {
		return nil, conv.ErrArgs("APPEND", 0, "table is not specified")
	}
	return ret, nil
}

type appender struct {
	db     spi.Database
	writer spec.OutputStream
	nrows  int

	dbAppender spi.Appender

	table *Table
}

func (app *appender) Open(db spi.Database) (err error) {
	app.db = db
	app.dbAppender, err = app.db.Appender(app.table.Name)
	return
}

func (app *appender) Close() {
	var succ, fail int64
	var err error
	if app.dbAppender != nil {
		succ, fail, err = app.dbAppender.Close()
	}
	if app.writer != nil {
		if err != nil {
			app.writer.Write([]byte(fmt.Sprintf("append fail, %s", err.Error())))
		} else {
			app.writer.Write([]byte(fmt.Sprintf("append %d rows (success %d, fail %d).", app.nrows, succ, fail)))
		}
	}
}

func (app *appender) SetOutputStream(w spec.OutputStream) {
	app.writer = w
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
