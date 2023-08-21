package internal

import (
	"database/sql"
	"time"
)

type SqlBridgeBase struct {
}

func (b *SqlBridgeBase) NewScanType(reflectType string, databaseTypeName string) any {
	switch reflectType {
	case "sql.NullBool":
		return new(bool)
	case "sql.NullByte":
		return new(uint8)
	case "sql.NullFloat64":
		return new(float64)
	case "sql.NullInt16":
		return new(int16)
	case "sql.NullInt32":
		return new(int32)
	case "sql.NullInt64":
		return new(int64)
	case "sql.NullString":
		return new(string)
	case "sql.NullTime":
		return new(time.Time)
	case "sql.RawBytes":
		return new([]byte)
	case "[]uint8":
		return new([]byte)
	case "bool":
		return new(bool)
	case "int32":
		return new(int32)
	case "int64":
		return new(int64)
	case "string":
		return new(string)
	case "time.Time":
		return new(time.Time)
	}
	return nil
}

func (c *SqlBridgeBase) NormalizeType(values []any) []any {
	for i, val := range values {
		switch v := val.(type) {
		case sql.RawBytes:
			values[i] = []byte(v)
		case *sql.NullBool:
			if v.Valid {
				values[i] = v.Bool
			} else {
				values[i] = nil
			}
		case *sql.NullByte:
			if v.Valid {
				values[i] = v.Byte
			} else {
				values[i] = nil
			}
		case *sql.NullFloat64:
			if v.Valid {
				values[i] = v.Float64
			} else {
				values[i] = nil
			}
		case *sql.NullInt16:
			if v.Valid {
				values[i] = v.Int16
			} else {
				values[i] = nil
			}
		case *sql.NullInt32:
			if v.Valid {
				values[i] = v.Int32
			} else {
				values[i] = nil
			}
		case *sql.NullInt64:
			if v.Valid {
				values[i] = v.Int64
			} else {
				values[i] = nil
			}
		case *sql.NullString:
			if v.Valid {
				values[i] = v.String
			} else {
				values[i] = nil
			}
		case *sql.NullTime:
			if v.Valid {
				values[i] = v.Time
			} else {
				values[i] = nil
			}
		}
	}
	return values
}
