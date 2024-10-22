package machsvr

import (
	"net"
	"time"
	"unsafe"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api/types"
)

func bind(stmt unsafe.Pointer, idx int, c any) error {
	if c == nil {
		if err := mach.EngBindNull(stmt, idx); err != nil {
			return types.ErrDatabaseBindNull(idx, err)
		}
		return nil
	}
	switch cv := c.(type) {
	case int:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *int:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case uint:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *uint:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case int16:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *int16:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case uint16:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *uint16:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case int32:
		if err := mach.EngBindInt32(stmt, idx, cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *int32:
		if err := mach.EngBindInt32(stmt, idx, *cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case uint32:
		if err := mach.EngBindInt32(stmt, idx, int32(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *uint32:
		if err := mach.EngBindInt32(stmt, idx, int32(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case int64:
		if err := mach.EngBindInt64(stmt, idx, cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *int64:
		if err := mach.EngBindInt64(stmt, idx, *cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case uint64:
		if err := mach.EngBindInt64(stmt, idx, int64(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *uint64:
		if err := mach.EngBindInt64(stmt, idx, int64(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case float32:
		if err := mach.EngBindFloat64(stmt, idx, float64(cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *float32:
		if err := mach.EngBindFloat64(stmt, idx, float64(*cv)); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case float64:
		if err := mach.EngBindFloat64(stmt, idx, cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *float64:
		if err := mach.EngBindFloat64(stmt, idx, *cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case string:
		if err := mach.EngBindString(stmt, idx, cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *string:
		if err := mach.EngBindString(stmt, idx, *cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case []byte:
		if err := mach.EngBindBinary(stmt, idx, cv); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case net.IP:
		if err := mach.EngBindString(stmt, idx, cv.String()); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case time.Time:
		if err := mach.EngBindInt64(stmt, idx, cv.UnixNano()); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	case *time.Time:
		if err := mach.EngBindInt64(stmt, idx, cv.UnixNano()); err != nil {
			return types.ErrDatabaseBind(idx, c, err)
		}
	default:
		return types.ErrDatabaseBindType(idx, c)
	}
	return nil
}
