package types

import (
	"database/sql/driver"
	"net"
	"time"
)

func Unbox(val any) any {
	switch v := val.(type) {
	case *int:
		return *v
	case *uint:
		return *v
	case *int8:
		return *v
	case *uint8:
		return *v
	case *int16:
		return *v
	case *uint16:
		return *v
	case *int32:
		return *v
	case *uint32:
		return *v
	case *int64:
		return *v
	case *uint64:
		return *v
	case *float32:
		return *v
	case *float64:
		return *v
	case *string:
		return *v
	case *time.Time:
		return *v
	case *bool:
		return *v
	case *[]byte:
		return *v
	case *net.IP:
		return *v
	case *driver.Value:
		return *v
	default:
		return val
	}
}
