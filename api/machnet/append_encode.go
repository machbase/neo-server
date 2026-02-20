package machnet

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	appendBindingNull    = -1
	appendBindingArrival = -2
)

type appendBindings struct {
	byColumn   []int
	arrivalArg int
}

func normalizeIdentifier(name string) string {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, `"`, "")
	return strings.ToUpper(normalized)
}

func isAppendArrivalColumn(col columnMeta, idx int) bool {
	name := normalizeIdentifier(col.name)
	if idx == 0 && name == "" {
		return true
	}
	return name == "_ARRIVAL_TIME"
}

func buildAppendBindings(columns []columnMeta, names []string) (appendBindings, error) {
	inputByName := make(map[string]int, len(names))
	matched := make([]bool, len(names))
	arrivalArg := -1
	for idx, name := range names {
		key := normalizeIdentifier(name)
		if key == "" {
			continue
		}
		if key == "_ARRIVAL_TIME" || key == "ARRIVAL_TIME" {
			arrivalArg = idx
		}
		if _, exists := inputByName[key]; exists {
			return appendBindings{}, fmt.Errorf("duplicate append column %s", name)
		}
		inputByName[key] = idx
	}
	bindings := appendBindings{
		byColumn:   make([]int, len(columns)),
		arrivalArg: arrivalArg,
	}
	for idx, col := range columns {
		if isAppendArrivalColumn(col, idx) {
			bindings.byColumn[idx] = appendBindingArrival
			continue
		}
		key := normalizeIdentifier(col.name)
		if key == "" {
			bindings.byColumn[idx] = appendBindingNull
			continue
		}
		if argIdx, ok := inputByName[key]; ok {
			bindings.byColumn[idx] = argIdx
			matched[argIdx] = true
		} else {
			bindings.byColumn[idx] = appendBindingNull
		}
	}
	for idx, name := range names {
		key := normalizeIdentifier(name)
		if key == "" || key == "_ARRIVAL_TIME" || key == "ARRIVAL_TIME" {
			continue
		}
		if !matched[idx] {
			return appendBindings{}, fmt.Errorf("append column %s is not accepted by server metadata", name)
		}
	}
	return bindings, nil
}

func encodeAppendRow(columns []columnMeta, bindings appendBindings, args []any, serverEndian uint32) ([]byte, error) {
	nullBytes := 0
	if len(columns) > 0 {
		nullBytes = (len(columns) / 8) + 1
	}
	row := make([]byte, 0, 64+nullBytes)
	row = append(row, 0) // not compressed

	var nullLen [4]byte
	putUint32ByEndian(nullLen[:], uint32(nullBytes), serverEndian)
	row = append(row, nullLen[:]...)

	nullOffset := len(row)
	if nullBytes > 0 {
		row = append(row, make([]byte, nullBytes)...)
	}

	arrival := int64(0)
	if bindings.arrivalArg >= 0 && bindings.arrivalArg < len(args) && args[bindings.arrivalArg] != nil {
		v, err := toDateTimeInt64(args[bindings.arrivalArg])
		if err != nil {
			return nil, fmt.Errorf("invalid arrival time: %w", err)
		}
		arrival = v
	}

	for idx, col := range columns {
		argIdx := appendBindingNull
		if idx < len(bindings.byColumn) {
			argIdx = bindings.byColumn[idx]
		}
		switch argIdx {
		case appendBindingArrival:
			var buf [8]byte
			putUint64ByEndian(buf[:], uint64(arrival), serverEndian)
			row = append(row, buf[:]...)
			continue
		case appendBindingNull:
			setAppendNullBit(row[nullOffset:], idx)
			continue
		}
		if argIdx < 0 || argIdx >= len(args) {
			setAppendNullBit(row[nullOffset:], idx)
			continue
		}
		value := args[argIdx]
		if value == nil {
			setAppendNullBit(row[nullOffset:], idx)
			continue
		}
		field, err := encodeAppendColumnValue(col, value, serverEndian)
		if err != nil {
			return nil, fmt.Errorf("append column %s: %w", col.name, err)
		}
		row = append(row, field...)
	}
	return row, nil
}

func setAppendNullBit(bits []byte, ordinal int) {
	if len(bits) == 0 || ordinal < 0 {
		return
	}
	bytePos := ordinal / 8
	if bytePos >= len(bits) {
		return
	}
	bitPos := ordinal % 8
	bits[bytePos] |= 1 << (7 - bitPos)
}

func putUint16ByEndian(dst []byte, v uint16, serverEndian uint32) {
	if serverEndian == 0 {
		binary.LittleEndian.PutUint16(dst, v)
	} else {
		binary.BigEndian.PutUint16(dst, v)
	}
}

func putUint32ByEndian(dst []byte, v uint32, serverEndian uint32) {
	if serverEndian == 0 {
		binary.LittleEndian.PutUint32(dst, v)
	} else {
		binary.BigEndian.PutUint32(dst, v)
	}
}

func putUint64ByEndian(dst []byte, v uint64, serverEndian uint32) {
	if serverEndian == 0 {
		binary.LittleEndian.PutUint64(dst, v)
	} else {
		binary.BigEndian.PutUint64(dst, v)
	}
}

func encodeAppendVarField(data []byte, serverEndian uint32) []byte {
	ret := make([]byte, 4+len(data))
	putUint32ByEndian(ret[0:4], uint32(len(data)), serverEndian)
	copy(ret[4:], data)
	return ret
}

func encodeAppendColumnValue(col columnMeta, value any, serverEndian uint32) ([]byte, error) {
	switch col.spinerType {
	case cmdBoolType:
		b, err := toAppendBool(value)
		if err != nil {
			return nil, err
		}
		var ret [2]byte
		if b {
			putUint16ByEndian(ret[:], 1, serverEndian)
		} else {
			putUint16ByEndian(ret[:], 0, serverEndian)
		}
		return ret[:], nil
	case cmdInt16Type:
		v, err := toInt64(value)
		if err != nil {
			return nil, err
		}
		if v < math.MinInt16 || v > math.MaxInt16 {
			return nil, fmt.Errorf("out of int16 range: %v", value)
		}
		var ret [2]byte
		putUint16ByEndian(ret[:], uint16(int16(v)), serverEndian)
		return ret[:], nil
	case cmdUInt16Type:
		v, err := toInt64(value)
		if err != nil {
			return nil, err
		}
		if v < 0 || v > math.MaxUint16 {
			return nil, fmt.Errorf("out of uint16 range: %v", value)
		}
		var ret [2]byte
		putUint16ByEndian(ret[:], uint16(v), serverEndian)
		return ret[:], nil
	case cmdInt32Type:
		v, err := toInt64(value)
		if err != nil {
			return nil, err
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return nil, fmt.Errorf("out of int32 range: %v", value)
		}
		var ret [4]byte
		putUint32ByEndian(ret[:], uint32(int32(v)), serverEndian)
		return ret[:], nil
	case cmdUInt32Type:
		v, err := toInt64(value)
		if err != nil {
			return nil, err
		}
		if v < 0 || v > math.MaxUint32 {
			return nil, fmt.Errorf("out of uint32 range: %v", value)
		}
		var ret [4]byte
		putUint32ByEndian(ret[:], uint32(v), serverEndian)
		return ret[:], nil
	case cmdInt64Type:
		v, err := toInt64(value)
		if err != nil {
			return nil, err
		}
		var ret [8]byte
		putUint64ByEndian(ret[:], uint64(v), serverEndian)
		return ret[:], nil
	case cmdUInt64Type:
		v, err := toInt64(value)
		if err != nil {
			return nil, err
		}
		if v < 0 {
			return nil, fmt.Errorf("out of uint64 range: %v", value)
		}
		var ret [8]byte
		putUint64ByEndian(ret[:], uint64(v), serverEndian)
		return ret[:], nil
	case cmdFlt32Type:
		v, err := toFloat64(value)
		if err != nil {
			return nil, err
		}
		var ret [4]byte
		putUint32ByEndian(ret[:], math.Float32bits(float32(v)), serverEndian)
		return ret[:], nil
	case cmdFlt64Type:
		v, err := toFloat64(value)
		if err != nil {
			return nil, err
		}
		var ret [8]byte
		putUint64ByEndian(ret[:], math.Float64bits(v), serverEndian)
		return ret[:], nil
	case cmdDateType:
		v, err := toDateTimeInt64(value)
		if err != nil {
			return nil, err
		}
		var ret [8]byte
		putUint64ByEndian(ret[:], uint64(v), serverEndian)
		return ret[:], nil
	case cmdIpv4Type:
		ip, err := toIP(value)
		if err != nil {
			return nil, err
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return nil, fmt.Errorf("invalid ipv4 value %v", value)
		}
		ret := make([]byte, 5)
		ret[0] = 4
		copy(ret[1:], ip4)
		return ret, nil
	case cmdIpv6Type:
		ip, err := toIP(value)
		if err != nil {
			return nil, err
		}
		ip16 := ip.To16()
		if ip16 == nil {
			return nil, fmt.Errorf("invalid ipv6 value %v", value)
		}
		ret := make([]byte, 17)
		ret[0] = 6
		copy(ret[1:], ip16)
		return ret, nil
	case cmdTextType, cmdVarcharType, cmdJSONType, cmdCharType, cmdClobType, cmdIPNetType:
		switch x := value.(type) {
		case []byte:
			return encodeAppendVarField(x, serverEndian), nil
		case string:
			return encodeAppendVarField([]byte(x), serverEndian), nil
		default:
			return encodeAppendVarField([]byte(fmt.Sprint(value)), serverEndian), nil
		}
	case cmdBinaryType, cmdBlobType:
		switch x := value.(type) {
		case []byte:
			return encodeAppendVarField(x, serverEndian), nil
		case string:
			return encodeAppendVarField([]byte(x), serverEndian), nil
		default:
			return nil, fmt.Errorf("unsupported binary type %T", value)
		}
	case cmdNulType:
		return nil, nil
	default:
		if col.isVariable {
			return encodeAppendVarField([]byte(fmt.Sprint(value)), serverEndian), nil
		}
		return nil, fmt.Errorf("unsupported append spiner type %d", col.spinerType)
	}
}

func toAppendBool(value any) (bool, error) {
	switch x := value.(type) {
	case bool:
		return x, nil
	case int:
		return x != 0, nil
	case int8:
		return x != 0, nil
	case int16:
		return x != 0, nil
	case int32:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case uint:
		return x != 0, nil
	case uint8:
		return x != 0, nil
	case uint16:
		return x != 0, nil
	case uint32:
		return x != 0, nil
	case uint64:
		return x != 0, nil
	case float32:
		return x != 0, nil
	case float64:
		return x != 0, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "t", "y", "yes":
			return true, nil
		case "0", "false", "f", "n", "no":
			return false, nil
		default:
			return false, fmt.Errorf("invalid boolean string %q", x)
		}
	default:
		return false, fmt.Errorf("unsupported boolean type %T", value)
	}
}
