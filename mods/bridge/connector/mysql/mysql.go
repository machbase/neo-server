package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/machbase/neo-client/api"
)

type Bridge struct {
	name string
	path string
	db   *sql.DB
}

func NewBridge(name string, path string) *Bridge {
	return &Bridge{name: name, path: path}
}

func (c *Bridge) BeforeRegister() error {
	db, err := Connect(c.path)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *Bridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *Bridge) String() string {
	return fmt.Sprintf("bridge '%s' (mysql)", c.name)
}

func (c *Bridge) Name() string {
	return c.name
}

func (c *Bridge) Type() string {
	return "mysql"
}

func (c *Bridge) DB() *sql.DB {
	return c.db
}

func (c *Bridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *Bridge) SupportLastInsertId() bool      { return true }
func (c *Bridge) ParameterMarker(idx int) string { return "?" }
func (c *Bridge) NormalizeType(values []any) []any {
	return api.NormalizeTypes(values, time.UTC)
}
func (c *Bridge) NewScanType(reflectType string, databaseTypeName string) any {
	return NewScanType(reflectType, databaseTypeName)
}

func Connect(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}

func NewScanType(reflectType string, databaseTypeName string) any {
	switch reflectType {
	case "sql.RawBytes":
		switch databaseTypeName {
		case "VARCHAR", "CHAR", "TEXT":
			return new(sql.NullString)
		default:
			return new([]byte)
		}
	case "[]uint8":
		return new([]byte)
	case "sql.NullBool":
		return new(sql.NullBool)
	case "sql.NullByte":
		return new(sql.NullByte)
	case "sql.NullFloat64":
		return new(sql.NullFloat64)
	case "sql.NullInt16":
		return new(sql.NullInt16)
	case "sql.NullInt32":
		return new(sql.NullInt32)
	case "sql.NullInt64":
		return new(sql.NullInt64)
	case "sql.NullString":
		return new(sql.NullString)
	case "sql.NullTime":
		if databaseTypeName == "DATE" {
			return new(sql.NullString)
		}
		return new(sql.NullTime)
	case "bool":
		return new(bool)
	case "int16":
		return new(int16)
	case "int32":
		return new(int32)
	case "int64":
		return new(int64)
	case "float32":
		return new(float32)
	case "float64":
		return new(float64)
	case "string":
		return new(string)
	case "time.Time":
		return new(sql.NullTime)
	}
	return nil
}
