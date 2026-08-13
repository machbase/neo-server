package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-client/v2/api"
	_ "github.com/microsoft/go-mssqldb"
)

type Bridge struct {
	name string
	path string
	db   *sql.DB
	u    *url.URL
}

func NewBridge(name string, path string) *Bridge {
	return &Bridge{name: name, path: path}
}

func (c *Bridge) BeforeRegister() error {
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
		default:
			q.Add(key, val)
		}
	}
	if !q.Has("dial timeout") {
		q.Add("dial timeout", "3")
	}
	if !q.Has("connection timeout") {
		q.Add("connection timeout", "5")
	}
	if !q.Has("app name") {
		q.Add("app name", "neo-bridge")
	}
	c.u = &url.URL{
		Scheme:   "sqlserver",
		Host:     server,
		RawQuery: q.Encode(),
	}
	db, err := Connect(c.u.String())
	if err != nil {
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
	return fmt.Sprintf("bridge '%s' (mssql)", c.name)
}

func (c *Bridge) Name() string {
	return c.name
}

func (c *Bridge) Type() string {
	return "mssql"
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

func (c *Bridge) SupportLastInsertId() bool { return false }

func (c *Bridge) ParameterMarker(idx int) string {
	return fmt.Sprintf("@p%d", idx+1)
}

func (c *Bridge) NormalizeType(values []any) []any {
	return api.NormalizeTypes(values, time.UTC)
}

func (c *Bridge) NewScanType(reflectType string, databaseTypeName string) any {
	return NewScanType(reflectType, databaseTypeName)
}

func (c *Bridge) ParsedURL() *url.URL {
	return c.u
}

func Connect(dsn string) (*sql.DB, error) {
	return sql.Open("sqlserver", dsn)
}

func NewScanType(reflectType string, databaseTypeName string) any {
	switch databaseTypeName {
	case "INT", "SMALLINT", "TINYINT", "BIGINT":
		return new(sql.NullInt64)
	case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY", "REAL", "FLOAT":
		return new(sql.NullFloat64)
	case "BIT":
		return new(sql.NullBool)
	case "VARCHAR", "TEXT", "NCHAR", "NVARCHAR":
		return new(sql.NullString)
	case "DATETIME":
		return new(sql.NullTime)
	}

	switch reflectType {
	case "sql.RawBytes":
		return new([]byte)
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
