package ndjson

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
)

type Decoder struct {
	reader       *json.Decoder
	columnTypes  []api.DataType
	columnNames  []string
	nrow         int64
	input        io.Reader
	timeformat   string
	timeLocation *time.Location
	tableName    string
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (dec *Decoder) SetInputStream(in io.Reader) {
	dec.input = in
}

func (dec *Decoder) SetTimeformat(format string) {
	dec.timeformat = format
}

func (dec *Decoder) SetTimeLocation(tz *time.Location) {
	dec.timeLocation = tz
}

func (dec *Decoder) SetTableName(tableName string) {
	dec.tableName = tableName
}

func (dec *Decoder) SetColumns(names ...string) {
	dec.columnNames = names
}

func (dec *Decoder) SetColumnTypes(types ...api.DataType) {
	dec.columnTypes = types
}

func (dec *Decoder) Open() {
}

func (dec *Decoder) NextRow() ([]any, []string, error) {
	if dec.reader == nil {
		dec.reader = json.NewDecoder(dec.input)
		dec.reader.UseNumber()
	}

	// decode json into a map
	var jsonObj = map[string]any{}
	err := dec.reader.Decode(&jsonObj)
	if err != nil {
		return nil, nil, err
	}
	// make lower cased names
	var obj = map[string]any{}
	for k, v := range jsonObj {
		obj[strings.ToLower(k)] = v
	}
	dec.nrow++

	values := make([]any, 0, len(obj))
	columns := make([]string, 0, len(obj))

	for idx, colName := range dec.columnNames {
		field, ok := obj[strings.ToLower(colName)]
		if !ok {
			continue
		}
		columns = append(columns, colName)
		var value any
		if field == nil {
			values = append(values, nil)
			continue
		}
		value, err = dec.columnTypes[idx].Apply(field, dec.timeformat, dec.timeLocation)
		if err != nil {
			return nil, nil, fmt.Errorf("rows[%d] field[%s] is not a %s, but %T", dec.nrow, colName, dec.columnTypes[idx], field)
		}
		values = append(values, value)
	}
	return values, columns, nil
}
