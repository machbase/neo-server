package machcli

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	exports := module.Get("exports").(*goja.Object)
	exports.Set("NewDatabase", NewDatabase)
	exports.Set("Unbox", api.Unbox)
}

type Config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password"`
	AlternativeHost string `json:"alternativeHost,omitempty"`
	AlternativePort int    `json:"alternativePort,omitempty"`
}

type Database struct {
	Ctx      context.Context
	Cancel   context.CancelFunc
	cli      *machcli.Database
	user     string
	password string
}

func NewDatabase(data string) (*Database, error) {
	obj := Config{
		Host:     "127.0.0.1",
		Port:     5656,
		User:     "sys",
		Password: "manager",
	} // default values
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, err
	}
	conf := &machcli.Config{
		Host:         obj.Host,
		Port:         obj.Port,
		MaxOpenConn:  -1,
		MaxOpenQuery: -1,
	}
	if obj.AlternativeHost != "" {
		conf.AlternativeHost = obj.AlternativeHost
	}
	if obj.AlternativePort != 0 {
		conf.AlternativePort = obj.AlternativePort
	}
	db, err := machcli.NewDatabase(conf)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Database{
		Ctx:      ctx,
		Cancel:   cancel,
		cli:      db,
		user:     strings.ToUpper(obj.User),
		password: obj.Password,
	}, nil
}

func (db *Database) Close() error {
	return db.cli.Close()
}

func (db *Database) User() string {
	return db.user
}

func (db *Database) Connect() (*machcli.Conn, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db.cli.Connect(ctx, api.WithPassword(db.user, db.password))
	if err != nil {
		return nil, err
	}
	return conn.(*machcli.Conn), nil
}

func (db *Database) NormalizeTableName(tableName string) [3]string {
	tableName = strings.ToUpper(tableName)
	toks := strings.Split(tableName, ".")
	if len(toks) == 1 {
		return [3]string{"MACHBASEDB", db.user, toks[0]}
	} else if len(toks) == 2 {
		return [3]string{"MACHBASEDB", toks[0], toks[1]}
	} else if len(toks) == 3 {
		return [3]string{toks[0], toks[1], toks[2]}
	}
	return [3]string{"", "", tableName}
}
