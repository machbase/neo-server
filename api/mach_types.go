package api

import (
	"net"
	"time"

	mach "github.com/machbase/neo-engine"
)

// 0: Log Table, 1: Fixed Table, 3: Volatile Table,
// 4: Lookup Table, 5: KeyValue Table, 6: Tag Table
type TableType int

const (
	LogTableType      TableType = iota + 0
	FixedTableType              = 1
	VolatileTableType           = 3
	LookupTableType             = 4
	KeyValueTableType           = 5
	TagTableType                = 6
)

func (t TableType) String() string {
	return mach.TableType(t).String()
}

type IndexType int

func IndexTypeString(typ IndexType) string {
	return mach.IndexTypeString(mach.IndexType(typ))
}

type ColumnType int

const (
	Int16ColumnType    ColumnType = iota + 4
	Uint16ColumnType              = 104
	Int32ColumnType               = 8
	Uint32ColumnType              = 108
	Int64ColumnType               = 12
	Uint64ColumnType              = 112
	Float32ColumnType             = 16
	Float64ColumnType             = 20
	VarcharColumnType             = 5
	TextColumnType                = 49
	ClobColumnType                = 53
	BlobColumnType                = 57
	BinaryColumnType              = 97
	DatetimeColumnType            = 6
	IpV4ColumnType                = 32
	IpV6ColumnType                = 36
	JsonColumnType                = 61
)

// ColumnTypeString converts ColumnType into string.
func ColumnTypeString(typ ColumnType) string {
	return mach.ColumnTypeString(mach.ColumnType(typ))
}

func ColumnTypeStringNative(typ ColumnType) string {
	return mach.ColumnTypeStringNative(mach.ColumnType(typ))
}

const (
	ColumnFlagTagName    = mach.ColumnFlagTagName
	ColumnFlagBasetime   = mach.ColumnFlagBasetime
	ColumnFlagSummarized = mach.ColumnFlagSummarized
	ColumnFlagMetaColumn = mach.ColumnFlagMetaColumn
)

func ColumnFlagString(flag int) string {
	return mach.ColumnFlagString(flag)
}

const (
	ColumnBufferTypeInt16    = mach.ColumnBufferTypeInt16
	ColumnBufferTypeInt32    = mach.ColumnBufferTypeInt32
	ColumnBufferTypeInt64    = mach.ColumnBufferTypeInt64
	ColumnBufferTypeDatetime = mach.ColumnBufferTypeDatetime
	ColumnBufferTypeFloat    = mach.ColumnBufferTypeFloat
	ColumnBufferTypeDouble   = mach.ColumnBufferTypeDouble
	ColumnBufferTypeIPv4     = mach.ColumnBufferTypeIPv4
	ColumnBufferTypeIPv6     = mach.ColumnBufferTypeIPv6
	ColumnBufferTypeString   = mach.ColumnBufferTypeString
	ColumnBufferTypeBinary   = mach.ColumnBufferTypeBinary
	ColumnBufferTypeBoolean  = mach.ColumnBufferTypeBoolean
	ColumnBufferTypeByte     = mach.ColumnBufferTypeByte
)

func ColumnBufferType(typ ColumnType) string {
	return mach.ColumnBufferType(mach.ColumnType(typ))
}

func MakeBuffer(cols Columns) []any {
	rec := make([]any, len(cols))
	for i := range cols {
		switch cols[i].Type {
		case "int16":
			rec[i] = new(int16)
		case "int32":
			rec[i] = new(int32)
		case "int64":
			rec[i] = new(int64)
		case "datetime":
			rec[i] = new(time.Time)
		case "float":
			rec[i] = new(float32)
		case "double":
			rec[i] = new(float64)
		case "ipv4":
			rec[i] = new(net.IP)
		case "ipv6":
			rec[i] = new(net.IP)
		case "string":
			rec[i] = new(string)
		case "binary":
			rec[i] = new([]byte)
		case "bool":
			rec[i] = new(bool)
		case "int8":
			rec[i] = new(byte)
		}
	}
	return rec
}

func ColumnTypeOf(value any) *Column {
	newName := "key"
	switch v := value.(type) {
	case string, *string:
		return &Column{Name: newName, Type: ColumnBufferTypeString}
	case bool, *bool:
		return &Column{Name: newName, Type: ColumnBufferTypeBoolean}
	case int, int32, *int, *int32:
		return &Column{Name: newName, Type: ColumnBufferTypeInt32}
	case int8, *int8:
		return &Column{Name: newName, Type: ColumnBufferTypeByte}
	case int16, *int16:
		return &Column{Name: newName, Type: ColumnBufferTypeInt16}
	case int64, *int64:
		return &Column{Name: newName, Type: ColumnBufferTypeInt64}
	case time.Time, *time.Time:
		return &Column{Name: newName, Type: ColumnBufferTypeDatetime}
	case float32, *float32:
		return &Column{Name: newName, Type: ColumnBufferTypeFloat}
	case float64, *float64:
		return &Column{Name: newName, Type: ColumnBufferTypeDouble}
	case net.IP:
		if len(v) == net.IPv6len {
			return &Column{Name: newName, Type: ColumnBufferTypeIPv6}
		} else {
			return &Column{Name: newName, Type: ColumnBufferTypeIPv4}
		}
	case []byte:
		return &Column{Name: newName, Type: ColumnBufferTypeBinary}
	default:
		return &Column{Name: newName, Type: "any"}
	}
}
