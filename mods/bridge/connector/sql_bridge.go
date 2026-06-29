package connector

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/machbase/neo-server/v8/mods/bridge/connector/mssql"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/mysql"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/postgres"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/sqlite"
)

type sqliteBridge struct {
	SqlBridgeBase
	name string
	path string
	db   *sql.DB
}

func NewSqliteBridge(name string, path string) *sqliteBridge {
	return &sqliteBridge{name: name, path: path}
}

func (c *sqliteBridge) BeforeRegister() error {
	db, err := sqlite.Connect(c.path)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *sqliteBridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *sqliteBridge) String() string {
	return fmt.Sprintf("bridge '%s' (sqlite3)", c.name)
}

func (c *sqliteBridge) Name() string {
	return c.name
}

func (c *sqliteBridge) Type() string {
	return "sqlite"
}

func (c *sqliteBridge) DB() *sql.DB {
	return c.db
}

func (c *sqliteBridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *sqliteBridge) SupportLastInsertId() bool      { return true }
func (c *sqliteBridge) ParameterMarker(idx int) string { return "?" }

func (c *sqliteBridge) NewScanType(reflectType string, databaseTypeName string) any {
	switch reflectType {
	case "sql.RawBytes":
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
	case "*interface {}":
		if databaseTypeName == "" {
			return new(string)
		}
	}
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}

type postgresBridge struct {
	SqlBridgeBase
	name string
	path string
	db   *sql.DB
}

func NewPostgresBridge(name string, path string) *postgresBridge {
	return &postgresBridge{name: name, path: path}
}

func (c *postgresBridge) BeforeRegister() error {
	db, err := postgres.Connect(c.path)
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *postgresBridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *postgresBridge) String() string {
	return fmt.Sprintf("bridge '%s' (postgres)", c.name)
}

func (c *postgresBridge) Name() string {
	return c.name
}

func (c *postgresBridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *postgresBridge) Type() string {
	return "postgres"
}

func (c *postgresBridge) DB() *sql.DB {
	return c.db
}

func (c *postgresBridge) SupportLastInsertId() bool      { return false }
func (c *postgresBridge) ParameterMarker(idx int) string { return "$" + strconv.Itoa(idx+1) }

func (c *postgresBridge) NewScanType(reflectType string, databaseTypeName string) any {
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

type mysqlBridge struct {
	SqlBridgeBase
	name string
	path string
	db   *sql.DB
}

func NewMySQLBridge(name string, path string) *mysqlBridge {
	return &mysqlBridge{name: name, path: path}
}

func (c *mysqlBridge) BeforeRegister() error {
	db, err := mysql.Connect(c.path)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *mysqlBridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *mysqlBridge) String() string {
	return fmt.Sprintf("bridge '%s' (mysql)", c.name)
}

func (c *mysqlBridge) Name() string {
	return c.name
}

func (c *mysqlBridge) Type() string {
	return "mysql"
}

func (c *mysqlBridge) DB() *sql.DB {
	return c.db
}

func (c *mysqlBridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *mysqlBridge) SupportLastInsertId() bool      { return true }
func (c *mysqlBridge) ParameterMarker(idx int) string { return "?" }

func (c *mysqlBridge) NewScanType(reflectType string, databaseTypeName string) any {
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
		if databaseTypeName == "DATE" {
			return new(sql.NullString)
		}
		return new(sql.NullTime)
	}
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}

type mssqlBridge struct {
	SqlBridgeBase
	name string
	path string
	db   *sql.DB
	u    *url.URL
}

func NewMSSQLBridge(name string, path string) *mssqlBridge {
	return &mssqlBridge{name: name, path: path}
}

func (c *mssqlBridge) BeforeRegister() error {
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
	db, err := mssql.Connect(c.u.String())
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *mssqlBridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *mssqlBridge) String() string {
	return fmt.Sprintf("bridge '%s' (mssql)", c.name)
}

func (c *mssqlBridge) Name() string {
	return c.name
}

func (c *mssqlBridge) Type() string {
	return "mssql"
}

func (c *mssqlBridge) DB() *sql.DB {
	return c.db
}

func (c *mssqlBridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.name)
	}
	return c.db.Conn(ctx)
}

func (c *mssqlBridge) SupportLastInsertId() bool { return false }

func (c *mssqlBridge) ParameterMarker(idx int) string {
	return fmt.Sprintf("@p%d", idx+1)
}

func (c *mssqlBridge) NewScanType(reflectType string, databaseTypeName string) any {
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
	return c.SqlBridgeBase.NewScanType(reflectType, databaseTypeName)
}
