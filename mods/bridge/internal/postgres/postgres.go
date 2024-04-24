package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"

	_ "github.com/lib/pq"
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
	db, err := sql.Open("postgres", c.path)
	if err != nil {
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
	return fmt.Sprintf("bridge '%s' (postgres)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *bridge) SupportLastInsertId() bool      { return false }
func (c *bridge) ParameterMarker(idx int) string { return "$" + strconv.Itoa(idx+1) }

func (c *bridge) NewScanType(reflectType string, databaseTypeName string) any {
	log.Println("reflectType,databaseTypeName:  ", reflectType, databaseTypeName)
	switch reflectType {
	case "interface {}":
		switch databaseTypeName {
		case "FLOAT4":
			return new(float32)
		case "UUID":
			return new(sql.NullString)
		}
	case "bool":
		return new(sql.NullBool)
	case "int32":
		return new(sql.NullInt32)
	case "int64":
		return new(sql.NullInt64)
	case "string":
		return new(sql.NullString)
	case "time.Time":
		return new(sql.NullTime)
	}
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}
