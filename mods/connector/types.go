package connector

import (
	"context"
	"database/sql"
	"fmt"
)

type Type string

const (
	SQLITE Type = "sqlite"
)

func ParseType(typ string) (Type, error) {
	switch typ {
	case "sqlite":
		return SQLITE, nil
	default:
		return "", fmt.Errorf("unsupported type of connector: %s", typ)
	}
}

type Define struct {
	Type Type   `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type FactoryFn func(*Define) (Connector, error)

type Connector interface {
	Type() Type
	Name() string

	BeforeRegister() error
	AfterUnregister() error
}

type SqlConnector interface {
	Connect(ctx context.Context) (*sql.Conn, error)
}
