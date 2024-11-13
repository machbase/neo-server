package mssql

import (
	"context"
	"database/sql"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/connector"
	"github.com/machbase/neo-server/mods/util"
	_ "github.com/microsoft/go-mssqldb"
)

type Database struct {
	db *sql.DB
}

var _ api.Database = (*Database)(nil)

var databases = map[string]*sql.DB{}

func init() {
	util.AddShutdownHook(func() {
		for _, d := range databases {
			d.Close()
		}
	})
}

func New(db *sql.DB) *Database {
	return &Database{db: db}
}

func NewWithDSN(dsn string) (*Database, error) {
	db := databases[dsn]
	if db == nil {
		if d, err := sql.Open("sqlserver", dsn); err != nil {
			return nil, err
		} else {
			databases[dsn] = d
			return New(d), nil
		}
	}
	return New(db), nil
}

func (d *Database) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	if c, err := d.db.Conn(ctx); err != nil {
		return nil, err
	} else {
		return connector.NewConn(c), nil
	}
}

func (d *Database) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	return true, "", nil
}

func (d *Database) Ping(ctx context.Context) (time.Duration, error) {
	tick := time.Now()
	if err := d.db.Ping(); err != nil {
		return 0, err
	}
	return time.Since(tick), nil
}
