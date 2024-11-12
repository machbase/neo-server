package mysql

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/machbase/neo-server/mods/bridge/internal"
)

type bridge struct {
	internal.SqlBridgeBase

	name string
	path string
	db   *sql.DB
}

func New(name string, path string) *bridge {
	return &bridge{name: name, path: path}
}

func (c *bridge) BeforeRegister() error {
	db, err := sql.Open("mysql", c.path)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *bridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *bridge) String() string {
	return fmt.Sprintf("bridge '%s' (mysql)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) Type() string {
	return "mysql"
}

func (c *bridge) DB() *sql.DB {
	return c.db
}

func (c *bridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *bridge) SupportLastInsertId() bool      { return true }
func (c *bridge) ParameterMarker(idx int) string { return "?" }

func (c *bridge) NewScanType(reflectType string, databaseTypeName string) any {
	switch reflectType {
	case "sql.RawBytes":
		switch databaseTypeName {
		case "VARCHAR", "CHAR", "TEXT":
			return new(sql.NullString)
		default:
			return new([]byte)
		}
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
		return new(sql.NullTime)
	}
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}
