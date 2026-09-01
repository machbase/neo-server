package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type ApiTokenDefinition struct {
	Id         int64
	UserName   string
	Name       string
	TokenHash  string
	TokenHint  string
	CreatedAt  time.Time
	NotBefore  time.Time
	NotAfter   time.Time
	LastUsedAt time.Time
	Attributes json.RawMessage
}

func (p *Provider) apiTokenConn(ctx context.Context) (*sql.Conn, error) {
	if err := p.normalizeContext(ctx); err != nil {
		return nil, err
	}
	if p.connect == nil {
		return nil, errors.New("database connect function is not configured")
	}
	conn, err := p.connect(ctx, "sys")
	if err != nil {
		return nil, err
	}
	if err := p.ensureApiTokenTable(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (p *Provider) ensureApiTokenTable(ctx context.Context, conn *sql.Conn) error {
	p.apiTokenTableMu.Lock()
	defer p.apiTokenTableMu.Unlock()
	if p.apiTokenTableReady {
		return nil
	}
	_, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _NEO_API_TOKEN (
ID LONG PRIMARY KEY AUTO_INCREMENT,
USER_NAME VARCHAR(64) NOT NULL,
NAME VARCHAR(256) NOT NULL,
TOKEN_HASH VARCHAR(64) NOT NULL,
TOKEN_HINT VARCHAR(64) NOT NULL,
CREATED_AT DATETIME,
NOT_BEFORE DATETIME,
NOT_AFTER DATETIME,
LAST_USED_AT DATETIME,
ATTRIBUTES JSON
)`)
	if err != nil {
		return err
	}
	p.apiTokenTableReady = true
	return nil
}

func (p *Provider) GetApiToken(ctx context.Context, id int64) (*ApiTokenDefinition, error) {
	conn, err := p.apiTokenConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return scanApiToken(conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, TOKEN_HASH, TOKEN_HINT, CREATED_AT, NOT_BEFORE, NOT_AFTER, LAST_USED_AT, ATTRIBUTES FROM _NEO_API_TOKEN WHERE ID = ?`, id))
}

func (p *Provider) GetAllApiTokens(ctx context.Context, scope UserScope) ([]*ApiTokenDefinition, error) {
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := p.apiTokenConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, USER_NAME, NAME, TOKEN_HASH, TOKEN_HINT, CREATED_AT, NOT_BEFORE, NOT_AFTER, LAST_USED_AT, ATTRIBUTES FROM _NEO_API_TOKEN WHERE USER_NAME = ? ORDER BY ID`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*ApiTokenDefinition{}
	for rows.Next() {
		def, err := scanApiToken(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, def)
	}
	return result, rows.Err()
}

func (p *Provider) SaveApiToken(ctx context.Context, scope UserScope, def *ApiTokenDefinition) error {
	if def == nil {
		return errors.New("api token definition is not specified")
	}
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	if strings.TrimSpace(def.Name) == "" || strings.TrimSpace(def.TokenHash) == "" || strings.TrimSpace(def.TokenHint) == "" {
		return errors.New("api token definition is invalid")
	}
	conn, err := p.apiTokenConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	def.UserName = user
	if def.CreatedAt.IsZero() {
		def.CreatedAt = time.Now()
	}
	result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_API_TOKEN (USER_NAME, NAME, TOKEN_HASH, TOKEN_HINT, CREATED_AT, NOT_BEFORE, NOT_AFTER, LAST_USED_AT, ATTRIBUTES) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, user, def.Name, def.TokenHash, def.TokenHint, def.CreatedAt, def.NotBefore, def.NotAfter, nullableTime(def.LastUsedAt), nullableJSON(def.Attributes))
	if err != nil {
		return err
	}
	def.Id, err = result.LastInsertId()
	return err
}

func (p *Provider) RemoveApiToken(ctx context.Context, scope UserScope, id int64) error {
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	conn, err := p.apiTokenConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_API_TOKEN WHERE ID = ? AND USER_NAME = ?`, id, user)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (p *Provider) TouchApiToken(ctx context.Context, id int64, at time.Time) error {
	conn, err := p.apiTokenConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `UPDATE _NEO_API_TOKEN SET LAST_USED_AT = ? WHERE ID = ?`, at, id)
	return err
}

func (p *Provider) UpdateApiTokenHint(ctx context.Context, scope UserScope, id int64, hint string) error {
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	conn, err := p.apiTokenConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `UPDATE _NEO_API_TOKEN SET TOKEN_HINT = ? WHERE ID = ? AND USER_NAME = ?`, hint, id, user)
	return err
}

type apiTokenScanner interface{ Scan(...any) error }

func scanApiToken(scanner apiTokenScanner) (*ApiTokenDefinition, error) {
	def := &ApiTokenDefinition{}
	var lastUsedAt sql.NullTime
	var attributes sql.NullString
	err := scanner.Scan(&def.Id, &def.UserName, &def.Name, &def.TokenHash, &def.TokenHint, &def.CreatedAt, &def.NotBefore, &def.NotAfter, &lastUsedAt, &attributes)
	if err != nil {
		return nil, err
	}
	if lastUsedAt.Valid {
		def.LastUsedAt = lastUsedAt.Time
	}
	if attributes.Valid {
		def.Attributes = json.RawMessage(attributes.String)
	}
	return def, nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
