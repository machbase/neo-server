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
	case int32:
		return scanInt32(sv, dst)
	case *int32:
		return scanInt32(*sv, dst)
	case int64:
		return scanInt64(sv, dst)
	case *int64:
		return scanInt64(*sv, dst)
	case float64:
		return scanFloat64(sv, dst)
	case *float64:
		return scanFloat64(*sv, dst)
	case float32:
		return scanFloat32(sv, dst)
	case *float32:
		return scanFloat32(*sv, dst)
	case string:
		return scanString(sv, dst)
	case *string:
		return scanString(*sv, dst)
	case time.Time:
		return scanDatetime(sv, dst)
	case *time.Time:
		return scanDatetime(*sv, dst)
	case []byte:
		return scanBytes(sv, dst)
	case *[]byte:
		return scanBytes(*sv, dst)
	case net.IP:
		return scanIP(sv, dst)
	case *net.IP:
		return scanIP(*sv, dst)
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
	case *driver.Value:
		*dst = driver.Value(src)
	case *TableType:
		*dst = TableType(src)
	case *TableFlag:
		*dst = TableFlag(src)
	case *ColumnType:
		*dst = ColumnType(src)
	case *ColumnFlag:
		*dst = ColumnFlag(src)
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
	case *net.IP:
		if src == "" {
			return ErrDatabaseScanNull("STRING")
		}
		*dst = net.ParseIP(src)
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
	default:
		return val
	}
}
