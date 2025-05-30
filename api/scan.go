package api

import (
	"database/sql"
	"database/sql/driver"
	"math"
	"net"
	"strconv"
	"time"
)

func Scan(src any, dst any) error {
	switch sv := src.(type) {
	case int:
		return scanInt32(int32(sv), dst)
	case *int:
		return scanInt32(int32(*sv), dst)
	case int16:
		return scanInt16(sv, dst)
	case *int16:
		return scanInt16(*sv, dst)
	case *sql.NullInt16:
		if sv.Valid {
			return scanInt16(sv.Int16, dst)
		}
	case int32:
		return scanInt32(sv, dst)
	case *int32:
		return scanInt32(*sv, dst)
	case *sql.NullInt32:
		if sv.Valid {
			return scanInt32(sv.Int32, dst)
		}
	case int64:
		return scanInt64(sv, dst)
	case *int64:
		return scanInt64(*sv, dst)
	case *sql.NullInt64:
		if sv.Valid {
			return scanInt64(sv.Int64, dst)
		}
	case float64:
		return scanFloat64(sv, dst)
	case *float64:
		return scanFloat64(*sv, dst)
	case *sql.NullFloat64:
		if sv.Valid {
			return scanFloat64(sv.Float64, dst)
		}
	case float32:
		return scanFloat32(sv, dst)
	case *float32:
		return scanFloat32(*sv, dst)
	case *sql.Null[float32]:
		if sv.Valid {
			return scanFloat32(sv.V, dst)
		}
	case string:
		return scanString(sv, dst)
	case *string:
		return scanString(*sv, dst)
	case *sql.NullString:
		if sv.Valid {
			return scanString(sv.String, dst)
		}
	case time.Time:
		return scanDatetime(sv, dst)
	case *time.Time:
		return scanDatetime(*sv, dst)
	case *sql.NullTime:
		if sv.Valid {
			return scanDatetime(sv.Time, dst)
		}
	case []byte:
		return scanBytes(sv, dst)
	case *[]byte:
		return scanBytes(*sv, dst)
	case net.IP:
		return scanIP(sv, dst)
	case *net.IP:
		return scanIP(*sv, dst)
	case *sql.Null[net.IP]:
		if sv.Valid {
			return scanIP(sv.V, dst)
		}
	}
	return ErrCannotConvertValue(src, dst)
}

func ScanNull(dst any) bool {
	switch d := dst.(type) {
	case *sql.NullBool:
		d.Valid = false
	case *sql.Null[int]:
		d.Valid = false
	case *sql.NullInt16:
		d.Valid = false
	case *sql.Null[int16]:
		d.Valid = false
	case *sql.NullInt32:
		d.Valid = false
	case *sql.Null[int32]:
		d.Valid = false
	case *sql.NullInt64:
		d.Valid = false
	case *sql.Null[int64]:
		d.Valid = false
	case *sql.Null[float32]:
		d.Valid = false
	case *sql.NullFloat64:
		d.Valid = false
	case *sql.Null[float64]:
		d.Valid = false
	case *sql.NullString:
		d.Valid = false
	case *sql.Null[string]:
		d.Valid = false
	case *sql.NullTime:
		d.Valid = false
	case *sql.Null[time.Time]:
		d.Valid = false
	case *sql.Null[net.IP]:
		d.Valid = false
	case *sql.Null[[]byte]:
		d.Valid = false
	default:
		return false
	}
	return true
}

func scanInt16(src int16, pDst any) error {
	if src == math.MinInt16 {
		return ErrDatabaseScanNull("INT16")
	}
	switch dst := pDst.(type) {
	case *int:
		*dst = int(src)
	case *uint:
		*dst = uint(src)
	case *int16:
		*dst = int16(src)
	case *uint16:
		*dst = uint16(src)
	case *int32:
		*dst = int32(src)
	case *uint32:
		*dst = uint32(src)
	case *int64:
		*dst = int64(src)
	case *uint64:
		*dst = uint64(src)
	case *string:
		*dst = strconv.Itoa(int(src))
	case *sql.NullInt16:
		dst.Valid = true
		dst.Int16 = src
	case *sql.NullInt32:
		dst.Valid = true
		dst.Int32 = int32(src)
	case *sql.NullInt64:
		dst.Valid = true
		dst.Int64 = int64(src)
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("INT16", pDst)
	}
	return nil
}

func scanInt32(src int32, pDst any) error {
	if src == math.MinInt32 {
		return ErrDatabaseScanNull("INT32")
	}
	switch dst := pDst.(type) {
	case *int:
		*dst = int(src)
	case *uint:
		*dst = uint(src)
	case *int16:
		*dst = int16(src)
	case *uint16:
		*dst = uint16(src)
	case *int32:
		*dst = int32(src)
	case *uint32:
		*dst = uint32(src)
	case *int64:
		*dst = int64(src)
	case *uint64:
		*dst = uint64(src)
	case *string:
		*dst = strconv.FormatInt(int64(src), 10)
	case *TableType:
		*dst = TableType(src)
	case *TableFlag:
		*dst = TableFlag(src)
	case *ColumnType:
		*dst = ColumnType(src)
	case *ColumnFlag:
		*dst = ColumnFlag(src)
	case *sql.NullInt32:
		dst.Valid = true
		dst.Int32 = src
	case *sql.NullInt64:
		dst.Valid = true
		dst.Int64 = int64(src)
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("INT32", pDst)
	}
	return nil
}

func scanInt64(src int64, pDst any) error {
	if src == math.MinInt64 {
		return ErrDatabaseScanNull("INT64")
	}
	switch dst := pDst.(type) {
	case *int:
		*dst = int(src)
	case *uint:
		*dst = uint(src)
	case *int16:
		*dst = int16(src)
	case *uint16:
		*dst = uint16(src)
	case *int32:
		*dst = int32(src)
	case *uint32:
		*dst = uint32(src)
	case *int64:
		*dst = int64(src)
	case *uint64:
		*dst = uint64(src)
	case *string:
		*dst = strconv.FormatInt(src, 10)
	case *time.Time:
		*dst = time.Unix(0, src)
	case *sql.NullInt64:
		dst.Valid = true
		dst.Int64 = src
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("INT64", pDst)
	}
	return nil
}

func scanDatetime(src time.Time, pDst any) error {
	switch dst := pDst.(type) {
	case *int64:
		*dst = src.UnixNano()
	case *time.Time:
		*dst = src.In(time.UTC)
	case *string:
		*dst = src.In(time.UTC).Format(time.RFC3339)
	case *sql.NullTime:
		dst.Valid = true
		dst.Time = src
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("DATETIME", pDst)
	}
	return nil
}

func scanFloat32(src float32, pDst any) error {
	switch dst := pDst.(type) {
	case *float32:
		*dst = src
	case *float64:
		*dst = float64(src)
	case *string:
		*dst = strconv.FormatFloat(float64(src), 'f', -1, 32)
	case *sql.NullFloat64:
		dst.Valid = true
		dst.Float64 = float64(src)
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("FLOAT32", pDst)
	}
	return nil
}

func scanFloat64(src float64, pDst any) error {
	switch dst := pDst.(type) {
	case *float32:
		*dst = float32(src)
	case *float64:
		*dst = src
	case *string:
		*dst = strconv.FormatFloat(src, 'f', -1, 64)
	case *sql.NullFloat64:
		dst.Valid = true
		dst.Float64 = src
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("FLOAT64", pDst)
	}
	return nil
}

func scanString(src string, pDst any) error {
	switch dst := pDst.(type) {
	case *string:
		*dst = src
	case *[]uint8:
		*dst = []uint8(src)
	case *int:
		if i, err := strconv.ParseInt(src, 10, 32); err != nil {
			return err
		} else {
			*dst = int(i)
		}
	case *int32:
		if i, err := strconv.ParseInt(src, 10, 32); err != nil {
			return err
		} else {
			*dst = int32(i)
		}
	case *int64:
		if i, err := strconv.ParseInt(src, 10, 64); err != nil {
			return err
		} else {
			*dst = i
		}
	case *net.IP:
		if src == "" {
			return ErrDatabaseScanNull("STRING")
		}
		*dst = net.ParseIP(src)
	case *sql.NullString:
		dst.Valid = true
		dst.String = src
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("STRING", pDst)
	}
	return nil
}

func scanBytes(src []byte, pDst any) error {
	switch dst := pDst.(type) {
	case *[]uint8:
		*dst = src
	case *string:
		*dst = string(src)
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("BYTES", pDst)
	}
	return nil
}

func scanIP(src net.IP, pDst any) error {
	switch dst := pDst.(type) {
	case *net.IP:
		*dst = src
	case *string:
		*dst = src.String()
	case *driver.Value:
		*dst = driver.Value(src)
	default:
		return ErrDatabaseScanType("IPv4", pDst)
	}
	return nil
}

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
	case *sql.NullBool:
		if v.Valid {
			return v.Bool
		} else {
			return nil
		}
	case *sql.NullInt16:
		if v.Valid {
			return v.Int16
		} else {
			return nil
		}
	case *sql.NullInt32:
		if v.Valid {
			return v.Int32
		} else {
			return nil
		}
	case *sql.NullInt64:
		if v.Valid {
			return v.Int64
		} else {
			return nil
		}
	case *sql.NullFloat64:
		if v.Valid {
			return v.Float64
		} else {
			return nil
		}
	case *sql.NullString:
		if v.Valid {
			return v.String
		} else {
			return nil
		}
	case *sql.NullTime:
		if v.Valid {
			return v.Time
		} else {
			return nil
		}
	case *sql.Null[net.IP]:
		if v.Valid {
			return v.V
		} else {
			return nil
		}
	case *sql.Null[[]byte]:
		if v.Valid {
			return v.V
		} else {
			return nil
		}
	case *sql.Null[float32]:
		if v.Valid {
			return v.V
		} else {
			return nil
		}
	case *sql.Null[float64]:
		if v.Valid {
			return v.V
		} else {
			return nil
		}
	case *sql.Null[any]:
		if v.Valid {
			return v.V
		} else {
			return nil
		}
	default:
		return val
	}
}
