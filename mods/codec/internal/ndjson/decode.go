package ndjson

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/machbase/neo-server/api/types"
	"github.com/machbase/neo-server/mods/stream/spec"
)

type Decoder struct {
	reader       *json.Decoder
	columnTypes  []types.DataType
	columnNames  []string
	nrow         int64
	input        spec.InputStream
	timeformat   string
	timeLocation *time.Location
	tableName    string
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (dec *Decoder) SetInputStream(in spec.InputStream) {
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

func (dec *Decoder) SetColumnTypes(types ...types.DataType) {
	dec.columnTypes = types
}

func (dec *Decoder) Open() {
}

func (dec *Decoder) NextRow() ([]any, []string, error) {
	if dec.reader == nil {
		dec.reader = json.NewDecoder(dec.input)
		dec.reader.UseNumber()
	}

	var obj = map[string]any{}
	err := dec.reader.Decode(&obj)
	if err != nil {
		return nil, nil, err
	}

	dec.nrow++

	values := make([]any, 0, len(obj))
	columns := make([]string, 0, len(obj))

	for idx, colName := range dec.columnNames {
		field, ok := obj[colName]
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
