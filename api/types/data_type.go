package types

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/util"
)

type DataType string

const (
	DataTypeInt16    DataType = "int16"
	DataTypeInt32    DataType = "int32"
	DataTypeInt64    DataType = "int64"
	DataTypeDatetime DataType = "datetime"
	DataTypeFloat32  DataType = "float"
	DataTypeFloat64  DataType = "double"
	DataTypeIPv4     DataType = "ipv4"
	DataTypeIPv6     DataType = "ipv6"
	DataTypeString   DataType = "string"
	DataTypeBinary   DataType = "binary"
	// exceptional case
	DataTypeBoolean DataType = "bool"
	DataTypeByte    DataType = "int8"
	DataTypeAny     DataType = "any"
	DataTypeList    DataType = "list"
	DataTypeDict    DataType = "dict"
)

func DataTypeOf(v any) DataType {
	switch v.(type) {
	default:
		return DataTypeAny
	case *string, string:
		return DataTypeString
	case *time.Time, time.Time:
		return DataTypeDatetime
	case *float32, float32:
		return DataTypeFloat32
	case *float64, float64:
		return DataTypeFloat64
	}
}

func (typ DataType) Apply(value any, timeformat string, tz *time.Location) (any, error) {
	if timeformat == "" {
		timeformat = "ns"
	}
	if tz == nil {
		tz = time.UTC
	}
	switch typ {
	case DataTypeString, COLUMN_TYPE_VARCHAR, COLUMN_TYPE_TEXT, COLUMN_TYPE_JSON:
		switch v := value.(type) {
		case string:
			return v, nil
		default:
			return nil, fmt.Errorf("%T is not convertible to %s", v, typ)
		}
	case DataTypeDatetime:
		switch v := value.(type) {
		case string:
			return util.ParseTime(v, timeformat, tz)
		default:
			ts, err := util.ToInt64(v)
			if err != nil {
				return nil, fmt.Errorf("%T is not datetime convertible, %s", v, err)
			}
			switch timeformat {
			case "s":
				return time.Unix(ts, 0), nil
			case "ms":
				return time.Unix(0, ts*int64(time.Millisecond)), nil
			case "us":
				return time.Unix(0, ts*int64(time.Microsecond)), nil
			default: // "ns"
				return time.Unix(0, ts), nil
			}
		}
	case DataTypeInt16, COLUMN_TYPE_SHORT:
		return util.ToInt16(value)
	case COLUMN_TYPE_USHORT, "unsigned short":
		return util.ToUint16(value)
	case DataTypeInt32, COLUMN_TYPE_INTEGER, "int":
		return util.ToInt32(value)
	case COLUMN_TYPE_UINTEGER, "unsigned integer":
		return util.ToUint32(value)
	case DataTypeInt64, COLUMN_TYPE_LONG:
		return util.ToInt64(value)
	case COLUMN_TYPE_ULONG, "unsigned long":
		return util.ToUint64(value)
	case DataTypeFloat32: //, DB_COLUMN_TYPE_FLOAT:
		return util.ToFloat32(value)
	case DataTypeFloat64: //, DB_COLUMN_TYPE_DOUBLE:
		return util.ToFloat64(value)
	case DataTypeIPv4, DataTypeIPv6:
		switch v := value.(type) {
		case string:
			return util.ParseIP(v)
		default:
			return nil, fmt.Errorf("%T is not %s convertible", v, typ)
		}
	case DataTypeBoolean:
		switch v := value.(type) {
		case string:
			return util.ParseBoolean(v)
		default:
			return nil, fmt.Errorf("%T is not %s convertible", v, typ)
		}
	case DataTypeByte:
		return util.ToInt8(value)
	// case DataTypeBinary:
	// 	return util.ParseBinary(v)
	// case DB_COLUMN_TYPE_CLOB:
	// 	return util.ParseString(v)
	// case DB_COLUMN_TYPE_BLOB:
	// 	return util.ParseBinary(v)
	// case DB_COLUMN_TYPE_BINARY:
	// 	return util.ParseBinary(v)
	default:
		return nil, fmt.Errorf("unsupported column type; %s", typ)
	}
}

func (typ DataType) ColumnType() ColumnType {
	switch typ {
	case DataTypeInt16:
		return ColumnTypeShort
	case DataTypeInt32:
		return ColumnTypeInteger
	case DataTypeInt64:
		return ColumnTypeLong
	case DataTypeDatetime:
		return ColumnTypeDatetime
	case DataTypeFloat32:
		return ColumnTypeFloat
	case DataTypeFloat64:
		return ColumnTypeDouble
	case DataTypeIPv4:
		return ColumnTypeIPv4
	case DataTypeIPv6:
		return ColumnTypeIPv6
	case DataTypeString:
		return ColumnTypeVarchar
	case DataTypeBinary:
		return ColumnTypeBlob
	case DataTypeBoolean:
		return ColumnTypeInteger
	case DataTypeByte:
		return ColumnTypeInteger
	default:
		switch strings.ToLower(string(typ)) {
		case COLUMN_TYPE_SHORT:
			return ColumnTypeShort
		case COLUMN_TYPE_USHORT, "unsigned short":
			return ColumnTypeUshort
		case COLUMN_TYPE_INTEGER, "int":
			return ColumnTypeInteger
		case COLUMN_TYPE_UINTEGER, "unsigned integer":
			return ColumnTypeUinteger
		case COLUMN_TYPE_LONG, "int64":
			return ColumnTypeLong
		case COLUMN_TYPE_ULONG, "unsigned long":
			return ColumnTypeUlong
		case COLUMN_TYPE_FLOAT:
			return ColumnTypeFloat
		case COLUMN_TYPE_DOUBLE:
			return ColumnTypeDouble
		case COLUMN_TYPE_VARCHAR:
			return ColumnTypeVarchar
		case COLUMN_TYPE_TEXT:
			return ColumnTypeText
		case COLUMN_TYPE_CLOB:
			return ColumnTypeClob
		case COLUMN_TYPE_BLOB:
			return ColumnTypeBlob
		case COLUMN_TYPE_BINARY:
			return ColumnTypeBinary
		case COLUMN_TYPE_DATETIME:
			return ColumnTypeDatetime
		case COLUMN_TYPE_IPV4:
			return ColumnTypeIPv4
		case COLUMN_TYPE_IPV6:
			return ColumnTypeIPv6
		case COLUMN_TYPE_JSON:
			return ColumnTypeJson
		default:
			return ColumnTypeVarchar
		}
	}
}

func ParseDataType(typ string) DataType {
	switch strings.ToLower(typ) {
	case "int16":
		return DataTypeInt16
	case "int32":
		return DataTypeInt32
	case "int64":
		return DataTypeInt64
	case "datetime":
		return DataTypeDatetime
	case "float":
		return DataTypeFloat32
	case "double":
		return DataTypeFloat64
	case "ipv4":
		return DataTypeIPv4
	case "ipv6":
		return DataTypeIPv6
	case "string":
		return DataTypeString
	case "binary":
		return DataTypeBinary
	case "bool":
		return DataTypeBoolean
	case "int8":
		return DataTypeByte
	default:
		switch typ {
		default:
			return DataType(fmt.Sprintf("Unsupported DataType: %s", typ))
		case "sql.NullString":
			return DataTypeString
		case "time.Time", "sql.NullTime":
			return DataTypeDatetime
		case "sql.NullInt16":
			return DataTypeInt16
		case "sql.NullInt32":
			return DataTypeInt32
		case "sql.NullInt64":
			return DataTypeInt64
		case "sql.NullByte":
			return DataTypeByte
		case "sql.NullFloat32":
			return DataTypeFloat32
		case "sql.NullFloat64":
			return DataTypeFloat64
		case "sql.NullBool":
			return DataTypeBoolean
		}
	}
}

func (typ DataType) makeBuffer() (any, error) {
	switch typ {
	case DataTypeInt16:
		return new(int16), nil
	case DataTypeInt32:
		return new(int32), nil
	case DataTypeInt64:
		return new(int64), nil
	case DataTypeDatetime:
		return new(time.Time), nil
	case DataTypeFloat32:
		return new(float32), nil
	case DataTypeFloat64:
		return new(float64), nil
	case DataTypeIPv4:
		return new(net.IP), nil
	case DataTypeIPv6:
		return new(net.IP), nil
	case DataTypeString:
		return new(string), nil
	case DataTypeBinary:
		return new([]byte), nil
	case DataTypeBoolean:
		return new(bool), nil
	case DataTypeByte:
		return new(byte), nil
	case DataTypeAny:
		return new(string), nil
	default:
		return nil, ErrDatabaseUnsupportedTypeName("makeBuffer", string(typ))
	}
}
