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

var _ Connector = &sqlite3Conn{}
var _ SqlConnector = &sqlite3Conn{}

func sqlite3Connector(def *Define) (Connector, error) {
	db, err := sql.Open("sqlite3", def.Path)
	if err != nil {
		return nil, err
	}
	conn := &sqlite3Conn{db: db, define: def}
	return conn, nil
}

func (c *sqlite3Conn) Type() Type {
	return c.define.Type
}

func (c *sqlite3Conn) Name() string {
	return c.define.Name
}

func (c *sqlite3Conn) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *sqlite3Conn) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("connector '%s' is not initialized", c.define.Name)
	}
	return c.db.Conn(ctx)
}
