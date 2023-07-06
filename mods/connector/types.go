package connector

import (
	"context"
	"database/sql"
)

type Type string

const (
	SQLITE3 Type = "sqlite3"
)

type Define struct {
	Type Type   `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Registry struct {
	define    *Define
	factoryFn FactoryFn
}

type FactoryFn func(*Define) (Connector, error)

type Connector interface {
	Close() error
	Type() Type
	Name() string
}

type SqlConnector interface {
	Connect(ctx context.Context) (*sql.Conn, error)
}
