package model

import (
	"fmt"
)

type BridgeType string

const (
	BRIDGE_SQLITE   BridgeType = "sqlite"
	BRIDGE_POSTGRES BridgeType = "postgres"
	BRIDGE_MYSQL    BridgeType = "mysql"
	BRIDGE_MSSQL    BridgeType = "mssql"
	BRIDGE_MQTT     BridgeType = "mqtt"
	BRIDGE_NATS     BridgeType = "nats"
	BRIDGE_PYTHON   BridgeType = "python"
)

func ParseBridgeType(typ string) (BridgeType, error) {
	switch typ {
	case "sqlite", "sqlite3":
		return BRIDGE_SQLITE, nil
	case "postgres", "postgresql":
		return BRIDGE_POSTGRES, nil
	case "mysql":
		return BRIDGE_MYSQL, nil
	case "mssql":
		return BRIDGE_MSSQL, nil
	case "mqtt":
		return BRIDGE_MQTT, nil
	case "nats":
		return BRIDGE_NATS, nil
	case "python":
		return BRIDGE_PYTHON, nil
	default:
		return "", fmt.Errorf("unsupported bridge type: %s", typ)
	}
}

type BridgeDefinition struct {
	Type BridgeType `json:"type"`
	Name string     `json:"name"`
	Path string     `json:"path"`
}

type BridgeProvider interface {
	LoadAllBridges() ([]*BridgeDefinition, error)
	LoadBridge(name string) (*BridgeDefinition, error)
	SaveBridge(def *BridgeDefinition) error
	RemoveBridge(name string) error
}
