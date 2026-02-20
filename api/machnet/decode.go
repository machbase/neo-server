package machnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

type columnMeta struct {
	name       string
	cmType     uint64
	precision  int
	scale      int
	spinerType int
	length     int
	isVariable bool
	sqlType    SqlType
	nullable   bool
}

func buildColumns(units map[uint32][]marshalUnit) []columnMeta {
	names := units[cmiPColNameID]
	types := units[cmiPColTypeID]
	count := len(names)
	if len(types) < count {
		count = len(types)
	}
	ret := make([]columnMeta, 0, count)
	for i := 0; i < count; i++ {
		cmType := uint64(0)
		if len(types[i].data) >= 8 {
			cmType = binary.LittleEndian.Uint64(types[i].data)
		}
		spiner := extractSpinerType(cmType)
		precision := extractPrecision(cmType)
		meta := columnMeta{
			name:       string(names[i].data),
			cmType:     cmType,
			precision:  precision,
			scale:      extractScale(cmType),
			spinerType: spiner,
			length:     computeColumnLength(spiner, precision),
			isVariable: isVariableSpinerType(spiner),
			sqlType:    spinerTypeToSQLType(spiner),
			nullable:   true,
		}
		ret = append(ret, meta)
	}
	return ret
}

func buildParamDescs(units map[uint32][]marshalUnit, count int) []CliParamDesc {
	typUnits := units[cmiPParamTypeID]
	if count <= 0 {
		count = len(typUnits)
	}
	ret := make([]CliParamDesc, count)
	for i := 0; i < count; i++ {
		d := CliParamDesc{Type: MACHCLI_SQL_TYPE_STRING, Nullable: true}
		if i < len(typUnits) && len(typUnits[i].data) >= 8 {
			cmType := binary.LittleEndian.Uint64(typUnits[i].data)
			d.Type = spinerTypeToSQLType(extractSpinerType(cmType))
			d.Precision = extractPrecision(cmType)
			d.Scale = extractScale(cmType)
		}
		ret[i] = d
	}
	return ret
}

func decodeRowsFromUnits(units []marshalUnit, columns []columnMeta) ([][]any, error) {
	rows := make([][]any, len(units))
	if len(units) == 0 {
		return rows, nil
	}
	if len(columns) == 0 {
		return rows, nil
	}
	flat := make([]any, len(units)*len(columns))
	for i, unit := range units {
		row := flat[i*len(columns) : (i+1)*len(columns)]
		if err := decodeRowInto(row, unit.data, columns); err != nil {
			return nil, err
		}
		rows[i] = row
	}
	return rows, nil
}

func decodeRowInto(ret []any, data []byte, columns []columnMeta) error {
	if len(ret) < len(columns) {
		return fmt.Errorf("invalid row buffer size: have=%d need=%d", len(ret), len(columns))
	}
	off := 0
	for i, col := range columns {
		if col.isVariable {
			if off+4 > len(data) {
				return fmt.Errorf("malformed row variable length")
			}
			l := int(binary.BigEndian.Uint32(data[off : off+4]))
			off += 4
			if l == 0 {
				ret[i] = nil
				continue
			}
			if off+l > len(data) {
				return fmt.Errorf("malformed row variable overrun")
			}
			field := data[off : off+l]
			off += l
			ret[i] = decodeVariableField(col, field)
			continue
		}
		length := col.length
		if length == 0 {
			ret[i] = nil
			continue
		}
		if off+length > len(data) {
			return fmt.Errorf("malformed row fixed overrun")
		}
		field := data[off : off+length]
		off += length
		ret[i] = decodeFixedField(col, field)
	}
	return nil
}

func decodeVariableField(col columnMeta, field []byte) any {
	switch col.spinerType {
	case cmdVarcharType, cmdTextType, cmdCharType, cmdJSONType, cmdClobType:
		return string(field)
	case cmdIPNetType:
		return string(field)
	case cmdBinaryType, cmdBlobType:
		b := make([]byte, len(field))
		copy(b, field)
		return b
	default:
		b := make([]byte, len(field))
		copy(b, field)
		return b
	}
}

func decodeFixedField(col columnMeta, field []byte) any {
	switch col.spinerType {
	case cmdBoolType, cmdInt16Type:
		if len(field) < 2 {
			return nil
		}
		v := int16(binary.BigEndian.Uint16(field))
		if v == shortNull {
			return nil
		}
		return v
	case cmdUInt16Type:
		if len(field) < 2 {
			return nil
		}
		v := binary.BigEndian.Uint16(field)
		if v == ushortNull {
			return nil
		}
		return int32(v)
	case cmdInt32Type:
		if len(field) < 4 {
			return nil
		}
		v := int32(binary.BigEndian.Uint32(field))
		if v == intNull {
			return nil
		}
		return v
	case cmdUInt32Type:
		if len(field) < 4 {
			return nil
		}
		v := binary.BigEndian.Uint32(field)
		if v == uintNull {
			return nil
		}
		return int64(v)
	case cmdInt64Type:
		if len(field) < 8 {
			return nil
		}
		v := int64(binary.BigEndian.Uint64(field))
		if v == longNull {
			return nil
		}
		return v
	case cmdUInt64Type:
		if len(field) < 8 {
			return nil
		}
		v := binary.BigEndian.Uint64(field)
		if v == ulongNull {
			return nil
		}
		return int64(v)
	case cmdFlt32Type:
		if len(field) < 4 {
			return nil
		}
		v := math.Float32frombits(binary.BigEndian.Uint32(field))
		if v == floatNull {
			return nil
		}
		return v
	case cmdFlt64Type:
		if len(field) < 8 {
			return nil
		}
		v := math.Float64frombits(binary.BigEndian.Uint64(field))
		if v == doubleNull {
			return nil
		}
		return v
	case cmdDateType:
		if len(field) < 8 {
			return nil
		}
		raw := binary.BigEndian.Uint64(field)
		if raw == datetimeNull {
			return nil
		}
		return time.Unix(0, int64(raw))
	case cmdIpv4Type:
		if len(field) < 5 || field[0] == 0 {
			return nil
		}
		return net.IP(append([]byte(nil), field[1:5]...))
	case cmdIpv6Type:
		if len(field) < 17 || field[0] == 0 {
			return nil
		}
		return net.IP(append([]byte(nil), field[1:17]...))
	case cmdNulType:
		return nil
	default:
		b := make([]byte, len(field))
		copy(b, field)
		if len(bytes.Trim(b, "\x00")) == 0 {
			return nil
		}
		return b
	}
}
