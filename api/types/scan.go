package types

import (
	"net"
	"strconv"
	"time"
)

func ScanInt16(src int16, pDst any) error {
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
	default:
		return ErrDatabaseScanType("INT16", pDst)
	}
	return nil
}

func ScanInt32(src int32, pDst any) error {
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
	default:
		return ErrDatabaseScanType("INT32", pDst)
	}
	return nil
}

func ScanInt64(src int64, pDst any) error {
	switch cv := pDst.(type) {
	case *int:
		*cv = int(src)
	case *uint:
		*cv = uint(src)
	case *int16:
		*cv = int16(src)
	case *uint16:
		*cv = uint16(src)
	case *int32:
		*cv = int32(src)
	case *uint32:
		*cv = uint32(src)
	case *int64:
		*cv = int64(src)
	case *uint64:
		*cv = uint64(src)
	case *string:
		*cv = strconv.FormatInt(src, 10)
	case *time.Time:
		*cv = time.Unix(0, src)
	default:
		return ErrDatabaseScanType("INT64", pDst)
	}
	return nil
}

func ScanDatetime(src time.Time, pDst any) error {
	switch dst := pDst.(type) {
	case *int64:
		*dst = src.UnixNano()
	case *time.Time:
		*dst = src
	case *string:
		*dst = src.In(time.UTC).Format(time.RFC3339)
	default:
		return ErrDatabaseScanType("DATETIME", pDst)
	}
	return nil
}

func ScanFloat32(src float32, pDst any) error {
	switch dst := pDst.(type) {
	case *float32:
		*dst = src
	case *float64:
		*dst = float64(src)
	case *string:
		*dst = strconv.FormatFloat(float64(src), 'f', -1, 32)
	default:
		return ErrDatabaseScanType("FLOAT32", pDst)
	}
	return nil
}

func ScanFloat64(src float64, pDst any) error {
	switch cv := pDst.(type) {
	case *float32:
		*cv = float32(src)
	case *float64:
		*cv = src
	case *string:
		*cv = strconv.FormatFloat(src, 'f', -1, 64)
	default:
		return ErrDatabaseScanType("FLOAT64", pDst)
	}
	return nil
}

func ScanString(src string, pDst any) error {
	switch dst := pDst.(type) {
	case *string:
		*dst = src
	case *[]uint8:
		*dst = []uint8(src)
	case *net.IP:
		*dst = net.ParseIP(src)
	default:
		return ErrDatabaseScanType("STRING", pDst)
	}
	return nil
}

func ScanBytes(src []byte, pDst any) error {
	switch dst := pDst.(type) {
	case *[]uint8:
		*dst = src
	case *string:
		*dst = string(src)
	default:
		return ErrDatabaseScanType("BYTES", pDst)
	}
	return nil
}

func ScanIP(src net.IP, pDst any) error {
	switch dst := pDst.(type) {
	case *net.IP:
		*dst = src
	case *string:
		*dst = src.String()
	default:
		return ErrDatabaseScanType("IPv4", pDst)
	}
	return nil
}
