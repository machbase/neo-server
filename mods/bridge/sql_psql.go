package bridge

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type postgresBridge struct {
	define *Define
	db     *sql.DB
}

func NewPostgresBridge(def *Define) Bridge {
	return &postgresBridge{define: def}
}

var _ SqlBridge = &postgresBridge{}

func (c *postgresBridge) BeforeRegister() error {
	db, err := sql.Open("postgres", c.define.Path)
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

func (c *postgresBridge) Type() Type {
	return c.define.Type
}

func (c *postgresBridge) Name() string {
	return c.define.Name
}

func (c *postgresBridge) Connect(ctx context.Context) (*sql.Conn, error) {
	if c.db == nil {
		return nil, fmt.Errorf("bridge '%s' is not initialized", c.define.Name)
	}
	return c.db.Conn(ctx)
}

func (c *postgresBridge) SupportLastInsertId() bool { return false }
