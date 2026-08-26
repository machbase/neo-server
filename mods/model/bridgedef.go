package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BridgeType string

const (
	BRIDGE_SQLITE   BridgeType = "sqlite"
	BRIDGE_POSTGRES BridgeType = "postgres"
	BRIDGE_MYSQL    BridgeType = "mysql"
	BRIDGE_MSSQL    BridgeType = "mssql"
	BRIDGE_MQTT     BridgeType = "mqtt"
	BRIDGE_NATS     BridgeType = "nats"
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
	default:
		return "", fmt.Errorf("unsupported bridge type: %s", typ)
	}
}

type BridgeDefinition struct {
	Type BridgeType `json:"type"`
	Name string     `json:"name"`
	Path string     `json:"path"`
}

func (s *Provider) LoadAllBridges() ([]*BridgeDefinition, error) {
	ret := []*BridgeDefinition{}
	err := s.iterateBridgeDefs(func(define *BridgeDefinition) bool {
		ret = append(ret, define)
		return true
	})
	return ret, err
}

func (s *Provider) LoadBridge(name string) (*BridgeDefinition, error) {
	path := filepath.Join(s.bridgeDir, fmt.Sprintf("%s.json", name))
	content, err := os.ReadFile(path)
	if err != nil {
		s.log.Warn("bridge load def file", err.Error())
		return nil, err
	}
	def := &BridgeDefinition{}
	if err := json.Unmarshal(content, def); err != nil {
		s.log.Warn("bridge load def format", err.Error())
		return nil, err
	}
	return def, nil
}

func (s *Provider) SaveBridge(def *BridgeDefinition) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warn("bridge save def file", err.Error())
		return err
	}

	path := filepath.Join(s.bridgeDir, fmt.Sprintf("%s.json", def.Name))
	return os.WriteFile(path, buf, 00600)
}

func (s *Provider) RemoveBridge(name string) error {
	path := filepath.Join(s.bridgeDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}

func (s *Provider) iterateBridgeDefs(cb func(*BridgeDefinition) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.bridgeDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.bridgeDir, entry.Name()))
		if err != nil {
			s.log.Warn(" %s", err.Error())
			continue
		}
		def := &BridgeDefinition{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warn("bridge def format", err.Error())
			continue
		}
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}
