package machcli

import (
	"context"
	"crypto"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
)

//go:embed machcli.js
var machcli_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"machcli.js": machcli_js,
	}
}

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	exports := module.Get("exports").(*goja.Object)
	exports.Set("NewDatabase", func(data string) (*Database, error) {
		return newDatabase(ctx, data)
	})
	exports.Set("Unbox", api.Unbox)
	exports.Set("RowsScan", RowsScan)
}

type Config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password"`
	IdentityFile    string `json:"identityFile,omitempty"`
	AlternativeHost string `json:"alternativeHost,omitempty"`
	AlternativePort int    `json:"alternativePort,omitempty"`
}

type Database struct {
	Ctx        context.Context
	Cancel     context.CancelFunc
	cli        *machgo.Database
	user       string
	password   string
	privateKey crypto.PrivateKey
}

func NewDatabase(data string) (*Database, error) {
	return newDatabase(context.Background(), data)
}

func newDatabase(ctx context.Context, data string) (*Database, error) {
	obj := Config{
		Host:     "127.0.0.1",
		Port:     5656,
		User:     "sys",
		Password: "manager",
	} // default values
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, err
	}
	conf := &machgo.Config{
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
	db, err := machgo.NewDatabase(conf)
	if err != nil {
		return nil, err
	}
	var privateKey crypto.PrivateKey
	if obj.IdentityFile != "" {
		if strings.HasPrefix(obj.IdentityFile, "@") {
			// path is os filesystem
			path := strings.TrimPrefix(obj.IdentityFile, "@")
			if key, err := machgo.LoadPrivateKeyFromFile(path); err == nil {
				privateKey = key
			} else {
				return nil, err
			}
		} else {
			// path is virtual filesystem
			// TODO: load private key from virtual filesystem
			return nil, fmt.Errorf("loading private key from virtual filesystem is not supported yet")
		}
	}
	derivedCtx, cancel := context.WithCancel(ctx)
	return &Database{
		Ctx:        derivedCtx,
		Cancel:     cancel,
		cli:        db,
		user:       strings.ToUpper(obj.User),
		password:   obj.Password,
		privateKey: privateKey,
	}, nil
}

func (db *Database) Close() error {
	return db.cli.Close()
}

func (db *Database) User() string {
	return db.user
}

func (db *Database) Connect() (*machgo.Conn, error) {
	ctx, cancel := context.WithCancel(db.Ctx)
	defer cancel()
	var conn api.Conn
	var err error
	if db.privateKey != nil {
		conn, err = db.cli.Connect(ctx, api.WithAuthKey("sys", db.privateKey), api.WithProxyUser(db.user))
	} else {
		conn, err = db.cli.Connect(ctx, api.WithPassword(db.user, db.password))
	}
	if err != nil {
		return nil, err
	}
	return conn.(*machgo.Conn), nil
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

// This helper function is used to fetch rows that includes null values,
// which are not properly-handled by goja's variadic arguments in rows.Scan(...buffer).
func RowsScan(rows *machgo.Rows) ([]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	buffer, err := cols.MakeBuffer()
	if err != nil {
		return nil, err
	}
	err = rows.Scan(buffer...)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}
