package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
)

type Type string

const (
	SQLITE   Type = "sqlite"
	POSTGRES Type = "postgres"
)

func ParseType(typ string) (Type, error) {
	switch typ {
	case "sqlite":
		return SQLITE, nil
	case "postgresql":
		fallthrough
	case "postgres":
		return POSTGRES, nil
	default:
		return "", fmt.Errorf("unsupported bridge type: %s", typ)
	}
}

type Define struct {
	Type Type   `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Bridge interface {
	Type() Type
	Name() string

	BeforeRegister() error
	AfterUnregister() error
}

type SqlBridge interface {
	Bridge
	Connect(ctx context.Context) (*sql.Conn, error)
	SupportLastInsertId() bool
}

func ConvertToDatum(arr ...any) ([]*bridgerpc.Datum, error) {
	ret := make([]*bridgerpc.Datum, len(arr))
	for i := range arr {
		switch v := arr[i].(type) {
		case *sql.NullInt16:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: int32(v.Int16)}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case int32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: v}}
		case *int32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: *v}}
		case *sql.NullInt32:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt32{VInt32: v.Int32}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case uint32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VUint32{VUint32: v}}
		case int64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt64{VInt64: v}}
		case *sql.NullInt64:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VInt64{VInt64: v.Int64}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case uint64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VUint64{VUint64: v}}
		case float32:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VFloat{VFloat: v}}
		case float64:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VDouble{VDouble: v}}
		case *sql.NullFloat64:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VDouble{VDouble: v.Float64}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case string:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: v}}
		case *string:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: *v}}
		case *sql.NullString:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VString{VString: v.String}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case bool:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBool{VBool: v}}
		case *sql.NullBool:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBool{VBool: v.Bool}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
			}
		case []byte:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VBytes{VBytes: v}}
		case net.IP:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VIp{VIp: v.String()}}
		case time.Time:
			ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VTime{VTime: v.UnixNano()}}
		case *sql.NullTime:
			if v.Valid {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VTime{VTime: v.Time.UnixNano()}}
			} else {
				ret[i] = &bridgerpc.Datum{Value: &bridgerpc.Datum_VNull{VNull: true}}
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
