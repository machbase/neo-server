package bridge

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type sqlite3Bridge struct {
	define *Define
	db     *sql.DB
}

func NewSqlite3Bridge(def *Define) Bridge {
	return &sqlite3Bridge{define: def}
}

var _ SqlBridge = &sqlite3Bridge{}

func (c *sqlite3Bridge) BeforeRegister() error {
	db, err := sql.Open("sqlite3", c.define.Path)
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *sqlite3Bridge) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *sqlite3Bridge) Type() Type {
	return c.define.Type
}

func (c *sqlite3Bridge) Name() string {
	return c.define.Name
}

func (c *sqlite3Bridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.define.Name)
	}
	return c.db.Conn(ctx)
}

func (c *sqlite3Bridge) SupportLastInsertId() bool { return true }
