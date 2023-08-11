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
	appName := "neo-bridge"

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
			appName = val
		case "encrypt":
			q.Add("encrypt", val)
		case "server":
			server = val
		}
	}
	q.Add("app name", appName)
	u := &url.URL{
		Scheme:   "sqlserver",
		Host:     server,
		RawQuery: q.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return err
	}
	conn, err := db.Conn(context.TODO())
	if err != nil {
		return err
	}
	conn.Close()
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
	// question mark bind parameter is deprecated
	return "?"
	// TODO
	// instead, should use "@ID" with sql.Named("ID", 100)
}

func (c *bridge) NewScanType(reflectType string, databaseTypeName string) any {
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}
