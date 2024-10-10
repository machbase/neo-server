package ndjson

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

type Decoder struct {
	reader       *json.Decoder
	columnTypes  []string
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

func (dec *Decoder) SetColumnTypes(types ...string) {
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
		switch dec.columnTypes[idx] {
		case mach.DB_COLUMN_TYPE_VARCHAR, mach.DB_COLUMN_TYPE_JSON, mach.DB_COLUMN_TYPE_TEXT:
			switch v := field.(type) {
			case string:
				value = v
			default:
				return nil, nil, fmt.Errorf("rows[%d] field[%s] is not a string, but %T", dec.nrow, colName, v)
			}
		case mach.DB_COLUMN_TYPE_DATETIME:
			if v, ok := field.(string); ok && dec.timeformat != "" {
				if value, err = util.ParseTime(v, dec.timeformat, dec.timeLocation); err != nil {
					return nil, nil, fmt.Errorf("rows[%d] field[%s] is not a datetime convertible, %s", dec.nrow, colName, err.Error())
				}
			} else {
				ts, err := util.ToInt64(field)
				if err != nil {
					return nil, nil, fmt.Errorf("rows[%d] field[%s] is not datetime convertible, %s", dec.nrow, colName, err.Error())
				}
				switch dec.timeformat {
				case "s":
					value = time.Unix(ts, 0)
				case "ms":
					value = time.Unix(0, ts*int64(time.Millisecond))
				case "us":
					value = time.Unix(0, ts*int64(time.Microsecond))
				default: // "ns"
					value = time.Unix(0, ts)
				}
			}
		case mach.DB_COLUMN_TYPE_FLOAT:
			value, err = util.ToFloat32(field)
			if err != nil {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_DOUBLE:
			value, err = util.ToFloat64(field)
			if err != nil {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_LONG:
			value, err = util.ToInt64(field)
			if err != nil {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_ULONG:
			if v, err := util.ToInt64(field); err == nil {
				value = uint64(v)
			} else {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_SHORT:
			if v, err := util.ToInt64(field); err == nil {
				value = int16(v)
			} else {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_USHORT:
			if v, err := util.ToInt64(field); err == nil {
				value = uint16(v)
			} else {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_INTEGER:
			if v, err := util.ToInt64(field); err == nil {
				value = int(v)
			} else {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_UINTEGER:
			if v, err := util.ToInt64(field); err == nil {
				value = uint(v)
			} else {
				return nil, nil, fmt.Errorf("rows[%d] field[%s], %s", dec.nrow, colName, err.Error())
			}
		case mach.DB_COLUMN_TYPE_IPV4, mach.DB_COLUMN_TYPE_IPV6:
			switch v := field.(type) {
			case string:
				addr := net.ParseIP(v)
				value = addr
			default:
				return nil, nil, fmt.Errorf("rows[%d] field[%s] is not compatible with %s", dec.nrow, colName, dec.columnTypes[idx])
			}
		default:
			return nil, nil, fmt.Errorf("rows[%d] field[%s] unsupported column type; %s", dec.nrow, colName, dec.columnTypes[idx])
		}
		values = append(values, value)
	}
	return values, columns, nil
}
