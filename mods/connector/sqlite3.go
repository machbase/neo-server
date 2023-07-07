package connector

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type sqlite3Conn struct {
	define *Define
	db     *sql.DB
}

func NewSqlite3Connector(def *Define) Connector {
	return &sqlite3Conn{define: def}
}

var _ Connector = &sqlite3Conn{}
var _ SqlConnector = &sqlite3Conn{}

func (c *sqlite3Conn) BeforeRegister() error {
	db, err := sql.Open("sqlite3", c.define.Path)
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *sqlite3Conn) AfterUnregister() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *sqlite3Conn) Type() Type {
	return c.define.Type
}

func (c *sqlite3Conn) Name() string {
	return c.define.Name
}

func (c *sqlite3Conn) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("connector '%s' is not initialized", c.define.Name)
	}
	return c.db.Conn(ctx)
}
