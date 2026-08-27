package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
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

// BridgeDefinition is a custom bridge definition persisted in _NEO_BRIDGE_DEF.
//
// Unlike shell definitions, bridge has no reserved definitions, so its Id
// is exposed as int64 directly instead of a string.
type BridgeDefinition struct {
	Id          int64      `json:"id"`
	Type        BridgeType `json:"type"`
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Owner       string     `json:"owner"`
	IsPublic    bool       `json:"isPublic"`
	AllowedUser string     `json:"allowedUser,omitempty"`
}

func (s *Provider) ensureBridgeTable(ctx context.Context, conn *sql.Conn) error {
	s.bridgeTableMu.Lock()
	defer s.bridgeTableMu.Unlock()
	if s.bridgeTableReady {
		return nil
	}
	_, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _NEO_BRIDGE_DEF (
ID LONG PRIMARY KEY AUTO_INCREMENT,
OWNER_NAME VARCHAR(64) NOT NULL,
IS_PUBLIC SHORT NOT NULL,
ALLOWED_USER VARCHAR(64),
NAME VARCHAR(128) NOT NULL,
TYPE VARCHAR(32) NOT NULL,
PATH VARCHAR(4096) NOT NULL
)`)
	if err != nil {
		return err
	}
	s.bridgeTableReady = true
	return nil
}

func (s *Provider) bridgeConn(ctx context.Context, scope UserScope) (*sql.Conn, string, error) {
	if err := s.normalizeContext(ctx); err != nil {
		return nil, "", err
	}
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, "", err
	}
	if s.connect == nil {
		return nil, "", errors.New("database connect function is not configured")
	}
	conn, err := s.connect(ctx, "sys")
	if err != nil {
		return nil, "", err
	}
	if err := s.ensureBridgeTable(ctx, conn); err != nil {
		conn.Close()
		return nil, "", err
	}
	return conn, user, nil
}

// bridgeConnUnscoped opens a SYS connection for the bridge table without
// requiring a UserScope. It is used only for the server bootstrap path
// that registers every user's bridge into the runtime registry.
func (s *Provider) bridgeConnUnscoped(ctx context.Context) (*sql.Conn, error) {
	if err := s.normalizeContext(ctx); err != nil {
		return nil, err
	}
	if s.connect == nil {
		return nil, errors.New("database connect function is not configured")
	}
	conn, err := s.connect(ctx, "sys")
	if err != nil {
		return nil, err
	}
	if err := s.ensureBridgeTable(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

type bridgeScanner interface {
	Scan(dest ...any) error
}

func scanBridgeDefinition(scanner bridgeScanner) (*BridgeDefinition, error) {
	var id int64
	var isPublic int64
	var owner, allowedUser, name, typ, path sql.NullString
	if err := scanner.Scan(&id, &owner, &isPublic, &allowedUser, &name, &typ, &path); err != nil {
		return nil, err
	}
	return &BridgeDefinition{
		Id:          id,
		Owner:       owner.String,
		IsPublic:    isPublic != 0,
		AllowedUser: allowedUser.String,
		Name:        name.String,
		Type:        BridgeType(typ.String),
		Path:        path.String,
	}, nil
}

// LoadAllBridges returns bridge definitions visible to the given user scope:
// bridges owned by the user, public bridges, and bridges explicitly shared
// with the user.
func (s *Provider) LoadAllBridges(ctx context.Context, scope UserScope) ([]*BridgeDefinition, error) {
	conn, user, err := s.bridgeConn(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, OWNER_NAME, IS_PUBLIC, ALLOWED_USER, NAME, TYPE, PATH FROM _NEO_BRIDGE_DEF
WHERE OWNER_NAME = ? OR IS_PUBLIC = 1 OR ALLOWED_USER = ? ORDER BY ID`, user, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ret := []*BridgeDefinition{}
	for rows.Next() {
		def, err := scanBridgeDefinition(rows)
		if err != nil {
			return nil, err
		}
		ret = append(ret, def)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

// LoadAllBridgesForBootstrap returns every bridge definition regardless of
// owner or scope. It is intended only for the runtime registry bootstrap
// at server startup, not for public/API use.
func (s *Provider) LoadAllBridgesForBootstrap(ctx context.Context) ([]*BridgeDefinition, error) {
	conn, err := s.bridgeConnUnscoped(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, OWNER_NAME, IS_PUBLIC, ALLOWED_USER, NAME, TYPE, PATH FROM _NEO_BRIDGE_DEF ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ret := []*BridgeDefinition{}
	for rows.Next() {
		def, err := scanBridgeDefinition(rows)
		if err != nil {
			return nil, err
		}
		ret = append(ret, def)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

// LoadBridge loads a single bridge definition by name, visible to the given
// user scope (owner, public, or explicitly allowed user).
func (s *Provider) LoadBridge(ctx context.Context, scope UserScope, name string) (*BridgeDefinition, error) {
	conn, user, err := s.bridgeConn(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	row := conn.QueryRowContext(ctx, `SELECT ID, OWNER_NAME, IS_PUBLIC, ALLOWED_USER, NAME, TYPE, PATH FROM _NEO_BRIDGE_DEF
WHERE NAME = ? AND (OWNER_NAME = ? OR IS_PUBLIC = 1 OR ALLOWED_USER = ?)`, name, user, user)
	def, err := scanBridgeDefinition(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bridge '%s' not found", name)
	}
	if err != nil {
		return nil, err
	}
	return def, nil
}

// SaveBridge inserts or updates a bridge definition. When def.Id == 0 a new
// row is inserted with Owner forced to scope.User; otherwise the existing
// row is updated, restricted to rows owned by scope.User.
func (s *Provider) SaveBridge(ctx context.Context, scope UserScope, def *BridgeDefinition) error {
	if def == nil {
		return errors.New("bridge definition not specified")
	}
	if len(def.Name) == 0 {
		return errors.New("bridge name not specified")
	}
	if _, err := ParseBridgeType(string(def.Type)); err != nil {
		return err
	}
	if len(def.Path) == 0 {
		return errors.New("bridge path not specified")
	}
	conn, user, err := s.bridgeConn(ctx, scope)
	if err != nil {
		return err
	}
	defer conn.Close()

	if def.Id == 0 {
		row := conn.QueryRowContext(ctx, `SELECT ID FROM _NEO_BRIDGE_DEF WHERE OWNER_NAME = ? AND NAME = ?`, user, def.Name)
		var existingId int64
		if err := row.Scan(&existingId); err != nil && err != sql.ErrNoRows {
			return err
		} else if err == nil {
			return fmt.Errorf("bridge '%s' already exists", def.Name)
		}
		def.Owner = user
		result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_BRIDGE_DEF (OWNER_NAME, IS_PUBLIC, ALLOWED_USER, NAME, TYPE, PATH) VALUES (?, ?, ?, ?, ?, ?)`,
			user, boolToShort(def.IsPublic), nullIfEmpty(def.AllowedUser), def.Name, string(def.Type), def.Path)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		def.Id = id
		return nil
	}

	result, err := conn.ExecContext(ctx, `UPDATE _NEO_BRIDGE_DEF SET IS_PUBLIC = ?, ALLOWED_USER = ?, NAME = ?, TYPE = ?, PATH = ? WHERE OWNER_NAME = ? AND ID = ?`,
		boolToShort(def.IsPublic), nullIfEmpty(def.AllowedUser), def.Name, string(def.Type), def.Path, user, def.Id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return os.ErrNotExist
	}
	def.Owner = user
	return nil
}

// RemoveBridge deletes a bridge definition by name, restricted to rows
// owned by scope.User.
func (s *Provider) RemoveBridge(ctx context.Context, scope UserScope, name string) error {
	conn, user, err := s.bridgeConn(ctx, scope)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_BRIDGE_DEF WHERE OWNER_NAME = ? AND NAME = ?`, user, name)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return os.ErrNotExist
	}
	return nil
}

func boolToShort(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
