package postgres

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/connector"
)

type Database struct {
	db *sql.DB
}

var _ api.Database = (*Database)(nil)

func New(db *sql.DB) *Database {
	return &Database{db: db}
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
