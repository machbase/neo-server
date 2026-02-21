package machnet

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

type BoundParam struct {
	sqlType SqlType
	value   any
	isNull  bool
}

func encodeParams(params []BoundParam) ([]byte, error) {
	if len(params) == 0 {
		return nil, nil
	}
	offset := 0
	buf := make([]byte, 0, 128)
	h := make([]byte, 2)
	binary.BigEndian.PutUint16(h, uint16(len(params)))
	buf = append(buf, h...)
	offset += 2

	for idx, p := range params {
		typ, data, err := encodeBoundParam(p)
		if err != nil {
			return nil, fmt.Errorf("bind %d: %w", idx, err)
		}
		entry := make([]byte, 11)
		entry[0] = byte(idx + 1)
		entry[1] = sqlParamInput
		entry[2] = byte(typ)
		binary.BigEndian.PutUint32(entry[3:7], uint32(len(data)))
		binary.BigEndian.PutUint32(entry[7:11], uint32(len(data)))
		buf = append(buf, entry...)
		offset += len(entry)
		buf = append(buf, data...)
		offset += len(data)
		if offset&1 == 1 {
			buf = append(buf, 0)
			offset++
		}
	}
	return buf, nil
}

func encodeBoundParam(p BoundParam) (int, []byte, error) {
	cmdType := sqlTypeToCmdType(p.sqlType)
	if p.isNull || p.value == nil {
		switch p.sqlType {
		case MACHCLI_SQL_TYPE_INT16:
			b := make([]byte, 2)
			binary.BigEndian.PutUint16(b, 0x8000)
			return cmdType, b, nil
		case MACHCLI_SQL_TYPE_INT32:
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, 0x80000000)
			return cmdType, b, nil
		case MACHCLI_SQL_TYPE_INT64:
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, 0x8000000000000000)
			return cmdType, b, nil
		case MACHCLI_SQL_TYPE_DATETIME:
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, datetimeNull)
			return cmdType, b, nil
		case MACHCLI_SQL_TYPE_FLOAT:
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, math.Float32bits(floatNull))
			return cmdType, b, nil
		case MACHCLI_SQL_TYPE_DOUBLE:
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, math.Float64bits(doubleNull))
			return cmdType, b, nil
		case MACHCLI_SQL_TYPE_IPV4:
			return cmdType, make([]byte, 5), nil
		case MACHCLI_SQL_TYPE_IPV6:
			return cmdType, make([]byte, 17), nil
		default:
			return cmdType, nil, nil
		}
	}

	switch p.sqlType {
	case MACHCLI_SQL_TYPE_INT16:
		v, err := toInt64(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(int16(v)))
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_INT32:
		v, err := toInt64(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(int32(v)))
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_INT64:
		v, err := toInt64(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(v))
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_DATETIME:
		v, err := toDateTimeInt64(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(v))
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_FLOAT:
		v, err := toFloat64(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, math.Float32bits(float32(v)))
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_DOUBLE:
		v, err := toFloat64(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, math.Float64bits(v))
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_IPV4:
		ip, err := toIP(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 5)
		if ip4 := ip.To4(); ip4 != nil {
			b[0] = 4
			copy(b[1:], ip4)
		}
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_IPV6:
		ip, err := toIP(p.value)
		if err != nil {
			return 0, nil, err
		}
		b := make([]byte, 17)
		if ip16 := ip.To16(); ip16 != nil {
			b[0] = 6
			copy(b[1:], ip16)
		}
		return cmdType, b, nil
	case MACHCLI_SQL_TYPE_BINARY:
		switch v := p.value.(type) {
		case []byte:
			return cmdType, append([]byte(nil), v...), nil
		case string:
			return cmdType, []byte(v), nil
		default:
			return 0, nil, fmt.Errorf("unsupported binary type %T", p.value)
		}
	default:
		switch v := p.value.(type) {
		case string:
			return cmdType, []byte(v), nil
		case []byte:
			return cmdType, append([]byte(nil), v...), nil
		default:
			return cmdType, []byte(fmt.Sprint(v)), nil
		}
	}
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case uint:
		return int64(x), nil
	case uint16:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	case float32:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported integer type %T", v)
	}
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int16:
		return float64(x), nil
	case int32:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case uint16:
		return float64(x), nil
	case uint32:
		return float64(x), nil
	case uint64:
		return float64(x), nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(x), 64)
	default:
		return 0, fmt.Errorf("unsupported float type %T", v)
	}
}

func toDateTimeInt64(v any) (int64, error) {
	switch x := v.(type) {
	case time.Time:
		return x.UnixNano(), nil
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
			return n, nil
		}
		t, err := time.Parse(time.RFC3339Nano, x)
		if err != nil {
			return 0, err
		}
		return t.UnixNano(), nil
	default:
		return 0, fmt.Errorf("unsupported datetime type %T", v)
	}
}

func toIP(v any) (net.IP, error) {
	switch x := v.(type) {
	case net.IP:
		return x, nil
	case string:
		ip := net.ParseIP(strings.TrimSpace(x))
		if ip == nil {
			return nil, fmt.Errorf("invalid ip %q", x)
		}
		return ip, nil
	case []byte:
		if len(x) == 4 || len(x) == 16 {
			ip := net.IP(x)
			if ip != nil {
				return ip, nil
			}
		}
		ip := net.ParseIP(strings.TrimSpace(string(x)))
		if ip == nil {
			return nil, fmt.Errorf("invalid ip bytes")
		}
		return ip, nil
	default:
		return nil, fmt.Errorf("unsupported ip type %T", v)
	}
}
