package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/machbase/neo-server/mods/bridge/internal"
	_ "github.com/microsoft/go-mssqldb"
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
	server := ""

	q := url.Values{}
	fields := strings.Fields(c.path)
	for _, field := range fields {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "user id", "user", "user-id":
			q.Add("user id", val)
		case "password", "pass":
			q.Add("password", val)
		case "database":
			q.Add("database", val)
		case "connection timeout", "connection-timeout":
			q.Add("connection timeout", val)
		case "dial timeout", "dial-timeout":
			q.Add("dial timeout", val)
		case "app name", "app-name":
			q.Add("app name", val)
		case "encrypt":
			q.Add("encrypt", val)
		case "server":
			server = val
		}
	}
	if !q.Has("dial timeout") {
		q.Add("dial timeout", "3")
	}
	if !q.Has("connection timeout") {
		q.Add("connection timeout", "5")
	}
	if q.Has("app name") {
		q.Add("app name", "neo-bridge")
	}
	u := &url.URL{
		Scheme:   "sqlserver",
		Host:     server,
		RawQuery: q.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
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
	return fmt.Sprintf("bridge '%s' (mssql)", c.name)
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

func (c *bridge) SupportLastInsertId() bool { return false }

func (c *bridge) ParameterMarker(idx int) string {
	return fmt.Sprintf("@p%d", idx+1)
}

func (c *bridge) NewScanType(reflectType string, databaseTypeName string) any {
	switch databaseTypeName {
	case "INT", "SMALLINT":
		return new(sql.NullInt64)
	case "DECIMAL", "REAL":
		return new(sql.NullFloat64)
	case "VARCHAR", "TEXT":
		return new(sql.NullString)
	case "DATETIME":
		return new(sql.NullTime)
	}
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}
