package api

import (
	"fmt"
	"net"
	"time"
)

type Column struct {
	Id       uint64     `json:"id,omitempty"`     // if the column came from database table
	Name     string     `json:"name"`             //
	Type     ColumnType `json:"type"`             //
	Length   int        `json:"length,omitempty"` //
	DataType DataType   `json:"data_type"`        //
	Flag     ColumnFlag `json:"flag,omitempty"`   // database column flag
}

func (col *Column) IsBaseTime() bool {
	return col.Flag&ColumnFlagBasetime > 0
}

func (col *Column) IsTagName() bool {
	return col.Flag&ColumnFlagTagName > 0
}

func (col *Column) IsSummarized() bool {
	return col.Flag&ColumnFlagSummarized > 0
}

func (col *Column) IsMetaColumn() bool {
	return col.Flag&ColumnFlagMetaColumn > 0
}

func (col *Column) makeBuffer() (any, error) {
	if col.Type != 0 {
		return col.Type.makeBuffer()
	} else if col.DataType != "" {
		return col.DataType.makeBuffer()
	} else {
		return nil, fmt.Errorf("Column type is not defined")
	}
}

// Width returns the size of the database column size.
// ,database column only
func (col *Column) Width() int {
	switch col.Type {
	case ColumnTypeShort:
		return 6
	case ColumnTypeUShort:
		return 5
	case ColumnTypeInteger:
		return 11
	case ColumnTypeUInteger:
		return 10
	case ColumnTypeLong:
		return 20
	case ColumnTypeULong:
		return 20
	case ColumnTypeFloat:
		return 17
	case ColumnTypeDouble:
		return 17
	case ColumnTypeIPv4:
		return 15
	case ColumnTypeIPv6:
		return 45
	case ColumnTypeDatetime:
		return 31
	}
	return col.Length
}

type Columns []*Column

func (cols Columns) Names() []string {
	names := make([]string, len(cols))
	for i := range cols {
		names[i] = cols[i].Name
	}
	return names
}

func (cols Columns) NamesWithTimeLocation(tz *time.Location) []string {
	names := make([]string, len(cols))
	for i := range cols {
		if cols[i].DataType == DataTypeDatetime {
			names[i] = fmt.Sprintf("%s(%s)", cols[i].Name, tz.String())
		} else {
			names[i] = cols[i].Name
		}
	}
	return names
}

func (cols Columns) DataTypes() []DataType {
	types := make([]DataType, len(cols))
	for i := range cols {
		if cols[i].DataType == "" {
			types[i] = cols[i].Type.DataType()
		} else {
			types[i] = cols[i].DataType
		}
	}
	return types
}

func (cols Columns) MakeBuffer() ([]any, error) {
	rec := make([]any, len(cols))
	for i, c := range cols {
		if v, err := c.makeBuffer(); err != nil {
			return nil, err
		} else {
			rec[i] = v
		}
	}
	return rec, nil
}

type ColumnType int

const (
	ColumnTypeShort    ColumnType = iota + 4
	ColumnTypeUShort   ColumnType = 104
	ColumnTypeInteger  ColumnType = 8
	ColumnTypeUInteger ColumnType = 108
	ColumnTypeLong     ColumnType = 12
	ColumnTypeULong    ColumnType = 112
	ColumnTypeFloat    ColumnType = 16
	ColumnTypeDouble   ColumnType = 20
	ColumnTypeVarchar  ColumnType = 5
	ColumnTypeText     ColumnType = 49
	ColumnTypeClob     ColumnType = 53
	ColumnTypeBlob     ColumnType = 57
	ColumnTypeBinary   ColumnType = 97
	ColumnTypeDatetime ColumnType = 6
	ColumnTypeIPv4     ColumnType = 32
	ColumnTypeIPv6     ColumnType = 36
	ColumnTypeJSON     ColumnType = 61
	ColumnTypeUnknown  ColumnType = 0
)

const (
	COLUMN_TYPE_SHORT    = "short"
	COLUMN_TYPE_USHORT   = "ushort"
	COLUMN_TYPE_INTEGER  = "integer"
	COLUMN_TYPE_UINTEGER = "uinteger"
	COLUMN_TYPE_LONG     = "long"
	COLUMN_TYPE_ULONG    = "ulong"
	COLUMN_TYPE_FLOAT    = "float"
	COLUMN_TYPE_DOUBLE   = "double"
	COLUMN_TYPE_DATETIME = "datetime"
	COLUMN_TYPE_VARCHAR  = "varchar"
	COLUMN_TYPE_IPV4     = "ipv4"
	COLUMN_TYPE_IPV6     = "ipv6"
	COLUMN_TYPE_TEXT     = "text"
	COLUMN_TYPE_CLOB     = "clob"
	COLUMN_TYPE_BLOB     = "blob"
	COLUMN_TYPE_BINARY   = "binary"
	COLUMN_TYPE_JSON     = "json"
)

func (typ ColumnType) String() string {
	switch typ {
	case ColumnTypeShort:
		return COLUMN_TYPE_SHORT
	case ColumnTypeUShort:
		return COLUMN_TYPE_USHORT
	case ColumnTypeInteger:
		return COLUMN_TYPE_INTEGER
	case ColumnTypeUInteger:
		return COLUMN_TYPE_UINTEGER
	case ColumnTypeLong:
		return COLUMN_TYPE_LONG
	case ColumnTypeULong:
		return COLUMN_TYPE_ULONG
	case ColumnTypeFloat:
		return COLUMN_TYPE_FLOAT
	case ColumnTypeDouble:
		return COLUMN_TYPE_DOUBLE
	case ColumnTypeVarchar:
		return COLUMN_TYPE_VARCHAR
	case ColumnTypeText:
		return COLUMN_TYPE_TEXT
	case ColumnTypeClob:
		return COLUMN_TYPE_CLOB
	case ColumnTypeBlob:
		return COLUMN_TYPE_BLOB
	case ColumnTypeBinary:
		return COLUMN_TYPE_BINARY
	case ColumnTypeDatetime:
		return COLUMN_TYPE_DATETIME
	case ColumnTypeIPv4:
		return COLUMN_TYPE_IPV4
	case ColumnTypeIPv6:
		return COLUMN_TYPE_IPV6
	case ColumnTypeJSON:
		return COLUMN_TYPE_JSON
	default:
		return fmt.Sprintf("UndefinedColumnType-%d", typ)
	}
}

func (typ ColumnType) makeBuffer() (any, error) {
	switch typ {
	case ColumnTypeShort:
		return new(int16), nil
	case ColumnTypeUShort:
		return new(uint16), nil
	case ColumnTypeInteger:
		return new(int32), nil
	case ColumnTypeUInteger:
		return new(uint32), nil
	case ColumnTypeLong:
		return new(int64), nil
	case ColumnTypeULong:
		return new(uint64), nil
	case ColumnTypeFloat:
		return new(float32), nil
	case ColumnTypeDouble:
		return new(float64), nil
	case ColumnTypeVarchar:
		return new(string), nil
	case ColumnTypeText:
		return new(string), nil
	case ColumnTypeIPv4:
		return new(net.IP), nil
	case ColumnTypeIPv6:
		return new(net.IP), nil
	case ColumnTypeJSON:
		return new(string), nil
	case ColumnTypeDatetime:
		return new(time.Time), nil
	case ColumnTypeBinary:
		return new([]byte), nil
	default:
		return nil, fmt.Errorf("unsupported column type: %d", typ)
	}
}

func (typ ColumnType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, typ.String())), nil
}

func (typ ColumnType) DataType() DataType {
	switch typ {
	case ColumnTypeShort:
		return DataTypeInt16
	case ColumnTypeUShort:
		return DataTypeInt16
	case ColumnTypeInteger:
		return DataTypeInt32
	case ColumnTypeUInteger:
		return DataTypeInt32
	case ColumnTypeLong:
		return DataTypeInt64
	case ColumnTypeULong:
		return DataTypeInt64
	case ColumnTypeFloat:
		return DataTypeFloat32
	case ColumnTypeDouble:
		return DataTypeFloat64
	case ColumnTypeVarchar:
		return DataTypeString
	case ColumnTypeText:
		return DataTypeString
	case ColumnTypeClob:
		return DataTypeBinary
	case ColumnTypeBlob:
		return DataTypeBinary
	case ColumnTypeBinary:
		return DataTypeBinary
	case ColumnTypeDatetime:
		return DataTypeDatetime
	case ColumnTypeIPv4:
		return DataTypeIPv4
	case ColumnTypeIPv6:
		return DataTypeIPv6
	case ColumnTypeJSON:
		return DataTypeString
	default:
		return DataType(fmt.Sprintf("UndefinedColumnType-%d", typ))
	}
}

type ColumnFlag int

const (
	ColumnFlagTagName    = 0x08000000
	ColumnFlagBasetime   = 0x01000000
	ColumnFlagSummarized = 0x02000000
	ColumnFlagMetaColumn = 0x04000000
)

func (flag ColumnFlag) String() string {
	switch flag {
	case ColumnFlagTagName:
		return "tag name"
	case ColumnFlagBasetime:
		return "basetime"
	case ColumnFlagSummarized:
		return "summarized"
	case ColumnFlagMetaColumn:
		return "meta"
	default:
		return ""
	}
}

func MakeColumnRownum() *Column {
	return &Column{Name: "ROWNUM", Type: ColumnTypeInteger, DataType: DataTypeInt64}
}

func MakeColumnInt64(name string) *Column {
	return &Column{Name: name, Type: ColumnTypeLong, DataType: DataTypeInt64}
}

func MakeColumnInt32(name string) *Column {
	return &Column{Name: name, Type: ColumnTypeLong, DataType: DataTypeInt32}
}

func MakeColumnDouble(name string) *Column {
	return &Column{Name: name, Type: ColumnTypeDouble, DataType: DataTypeFloat64}
}

func MakeColumnDatetime(name string) *Column {
	return &Column{Name: name, Type: ColumnTypeDatetime, DataType: DataTypeDatetime}
}

func MakeColumnString(name string) *Column {
	return &Column{Name: name, Type: ColumnTypeVarchar, DataType: DataTypeString}
}

func MakeColumnBoolean(name string) *Column {
	return &Column{Name: name, DataType: DataTypeString}
}

func MakeColumnAny(name string) *Column {
	return &Column{Name: name, DataType: DataTypeAny}
}

func MakeColumnList(name string) *Column {
	return &Column{Name: name, DataType: DataTypeList}
}

func MakeColumnDict(name string) *Column {
	return &Column{Name: name, DataType: DataTypeDict}
}

func MakeColumnOf(name string, value any) *Column {
	switch v := value.(type) {
	case string, *string:
		return &Column{Name: name, Type: ColumnTypeVarchar, DataType: DataTypeString}
	case bool, *bool:
		return &Column{Name: name, Type: ColumnTypeUnknown, DataType: DataTypeBoolean}
	case int, int32, *int, *int32:
		return &Column{Name: name, Type: ColumnTypeInteger, DataType: DataTypeInt32}
	case int8, *int8:
		return &Column{Name: name, Type: ColumnTypeShort, DataType: DataTypeByte}
	case int16, *int16:
		return &Column{Name: name, Type: ColumnTypeShort, DataType: DataTypeInt16}
	case int64, *int64:
		return &Column{Name: name, Type: ColumnTypeLong, DataType: DataTypeInt64}
	case time.Time, *time.Time:
		return &Column{Name: name, Type: ColumnTypeDatetime, DataType: DataTypeDatetime}
	case float32, *float32:
		return &Column{Name: name, Type: ColumnTypeFloat, DataType: DataTypeFloat32}
	case float64, *float64:
		return &Column{Name: name, Type: ColumnTypeDouble, DataType: DataTypeFloat64}
	case net.IP:
		if len(v) == net.IPv6len {
			return &Column{Name: name, Type: ColumnTypeIPv6, DataType: DataTypeIPv6}
		} else {
			return &Column{Name: name, Type: ColumnTypeIPv4, DataType: DataTypeIPv4}
		}
	case []byte:
		return &Column{Name: name, Type: ColumnTypeBinary, DataType: DataTypeBinary}
	default:
		return &Column{Name: name, Type: ColumnTypeUnknown, DataType: DataTypeAny}
	}
}
