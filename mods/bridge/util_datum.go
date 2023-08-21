package bridge

import (
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
)

func ConvertToDatum(arr ...any) ([]*bridgerpc.Datum, error) {
	ret := make([]*bridgerpc.Datum, len(arr))
	for i := range arr {
		switch v := arr[i].(type) {
		case int32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: v}}
		case *int32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: *v}}
		case uint32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VUint32{VUint32: v}}
		case int64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt64{VInt64: v}}
		case *int64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt64{VInt64: *v}}
		case uint64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VUint64{VUint64: v}}
		case *uint64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VUint64{VUint64: *v}}
		case float32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VFloat{VFloat: v}}
		case *float32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VFloat{VFloat: *v}}
		case float64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VDouble{VDouble: v}}
		case *float64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VDouble{VDouble: *v}}
		case string:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: v}}
		case *string:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: *v}}
		case bool:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBool{VBool: v}}
		case *bool:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBool{VBool: *v}}
		case []byte:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBytes{VBytes: v}}
		case net.IP:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VIp{VIp: v.String()}}
		case time.Time:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VTime{VTime: v.UnixNano()}}
		case *time.Time:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VTime{VTime: v.UnixNano()}}
		case *sql.NullBool:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBool{VBool: v.Bool}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullByte:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: strconv.Itoa(int(v.Byte))}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullFloat64:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VDouble{VDouble: v.Float64}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullInt16:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: int32(v.Int16)}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullInt32:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: v.Int32}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullInt64:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt64{VInt64: v.Int64}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullString:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: v.String}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *sql.NullTime:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VTime{VTime: v.Time.UnixNano()}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case *[]uint8:
			if v == nil {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBytes{}}
			} else {
				dst := make([]byte, len(*v))
				copy(dst, *v)
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBytes{VBytes: dst}}
			}
		case *sql.RawBytes:
			if v == nil {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBytes{}}
			} else {
				dst := make([]byte, len(*v))
				copy(dst, *v)
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBytes{VBytes: dst}}
			}
		default:
			if v == nil {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			} else {
				return nil, fmt.Errorf("ConvertToDatum() does not support %T", v)
			}
		}
	}
	return ret, nil
}

func ConvertFromDatum(arr ...*bridgerpc.Datum) ([]any, error) {
	ret := make([]any, len(arr))
	for i := range arr {
		switch v := arr[i].Value.(type) {
		case *bridgerpc.Datum_VInt32:
			ret[i] = v.VInt32
		case *bridgerpc.Datum_VUint32:
			ret[i] = v.VUint32
		case *bridgerpc.Datum_VInt64:
			ret[i] = v.VInt64
		case *bridgerpc.Datum_VUint64:
			ret[i] = v.VUint64
		case *bridgerpc.Datum_VFloat:
			ret[i] = v.VFloat
		case *bridgerpc.Datum_VDouble:
			ret[i] = v.VDouble
		case *bridgerpc.Datum_VString:
			ret[i] = v.VString
		case *bridgerpc.Datum_VBool:
			ret[i] = v.VBool
		case *bridgerpc.Datum_VBytes:
			ret[i] = v.VBytes
		case *bridgerpc.Datum_VIp:
			ret[i] = net.ParseIP(v.VIp)
		case *bridgerpc.Datum_VTime:
			ret[i] = time.Unix(0, v.VTime)
		case *bridgerpc.Datum_VNull:
			ret[i] = nil
		default:
			return nil, fmt.Errorf("ConvertFromDatum() does not support %T", v)
		}
	}
	return ret, nil
}
