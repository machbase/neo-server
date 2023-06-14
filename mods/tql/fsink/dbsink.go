package fsink

import (
	"errors"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

type dbSink interface {
	Open(db spi.Database) error
	Close()
	AddRow([]any) error

	SetOutputStream(spec.OutputStream)
}

var _ dbSink = &insert{}

type insert struct {
	db     spi.Database
	writer spec.OutputStream
	nrows  int

	columns      []string
	placeHolders []string

	table *table
	tag   *tag
}

func INSERT(args ...any) (any, error) {
	ret := &insert{}
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			ret.columns = append(ret.columns, v)
		case *table:
			ret.table = v
		case *tag:
			ret.tag = v
		}
	}
	if ret.table == nil {
		return nil, errors.New("f(INSERT) table is not specified")
	}
	if ret.tag != nil {
		ret.columns = append([]string{ret.tag.column}, ret.columns...)
	}
	for range ret.columns {
		ret.placeHolders = append(ret.placeHolders, "?")
	}
	return ret, nil
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
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)", ins.table.name, strings.Join(ins.columns, ","), strings.Join(ins.placeHolders, ","))
	var err error
	if ins.tag == nil {
		if result := ins.db.Exec(sqlText, values...); result.Err() != nil {
			err = result.Err()
		}
	} else {
		if result := ins.db.Exec(sqlText, append([]any{ins.tag.name}, values...)...); result.Err() != nil {
			err = result.Err()
		}
	}
	if err == nil {
		ins.nrows++
	}
	return err
}

type table struct {
	name string
}

func to_table(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, errInvalidNumOfArgs("table", 1, len(args))
	}
	if str, ok := args[0].(string); !ok {
		return nil, errWrongTypeOfArgs("table", 0, "string", str)
	} else {
		return &table{name: str}, nil
	}
}

type tag struct {
	column string
	name   string
}

// tag('sensor_1' [, 'column_name'])
func to_tag(args ...any) (any, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, errInvalidNumOfArgs("tag", 1, len(args))
	}
	ret := &tag{}
	if str, ok := args[0].(string); !ok {
		return nil, errWrongTypeOfArgs("tag", 0, "string", str)
	} else {
		ret.name = str
		ret.column = "name"
	}
	if len(args) == 2 {
		if str, ok := args[1].(string); !ok {
			return nil, errWrongTypeOfArgs("tag", 1, "string", str)
		} else {
			ret.column = str
		}
	}
	return ret, nil
}
