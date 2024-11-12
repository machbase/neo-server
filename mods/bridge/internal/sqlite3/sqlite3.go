package sqlite3

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/machbase/neo-server/mods/bridge/internal"
	_ "github.com/mattn/go-sqlite3"
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
	db, err := sql.Open("sqlite3", c.path)
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
	return fmt.Sprintf("bridge '%s' (sqlite3)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) Type() string {
	return "sqlite"
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
	case "sql.RawBytes": // BLOB
		switch databaseTypeName {
		case "BLOB":
			return new([]byte)
		default:
			return new([]byte)
		}
	case "sql.NullBool": // BOOLEAN
		return new(sql.NullBool)
	case "sql.NullByte":
		return new(sql.NullByte)
	case "sql.NullFloat64": // REAL
		return new(sql.NullFloat64)
	case "sql.NullInt16":
		return new(sql.NullInt16)
	case "sql.NullInt32":
		return new(sql.NullInt32)
	case "sql.NullInt64": // INTEGER
		return new(sql.NullInt64)
	case "sql.NullString": // TEXT
		return new(sql.NullString)
	case "sql.NullTime": // DATETIME
		return new(sql.NullTime)
	case "*interface {}":
		if databaseTypeName == "" {
			// Case: When query like "select count(*) from ...."
			// SQLite bind count(*) fields on this case
			// so, just bind it into string
			return new(string)
		}
	}
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}
