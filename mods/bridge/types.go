package bridge

import (
	"context"
	"database/sql"
	"fmt"
)

type Type string

const (
	SQLITE   Type = "sqlite"
	POSTGRES Type = "postgres"
	MYSQL    Type = "mysql"
	MQTT     Type = "mqtt"
)

func ParseType(typ string) (Type, error) {
	switch typ {
	case "sqlite":
		return SQLITE, nil
	case "postgresql":
		fallthrough
	case "postgres":
		return POSTGRES, nil
	case "mysql":
		return MYSQL, nil
	case "mqtt":
		return MQTT, nil
	default:
		return "", fmt.Errorf("unsupported bridge type: %s", typ)
	}
}

type Define struct {
	Type Type   `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Bridge interface {
	Name() string
	String() string

	BeforeRegister() error
	AfterUnregister() error
}

type SqlBridge interface {
	Bridge
	Connect(ctx context.Context) (*sql.Conn, error)
	SupportLastInsertId() bool
}

type MqttBridge interface {
	Bridge
}
