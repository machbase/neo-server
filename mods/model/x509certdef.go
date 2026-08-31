package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// X509CertDefinition. Id is the auto-increment management key used by the
// key.* CRUD RPCs; Name is the certificate CommonName and is not unique,
// since the same name may be reused across owners or reissued certificates.
type X509CertDefinition struct {
	Id         int64
	Name       string
	UserName   string
	CertPEM    string
	CertHash   string
	KeyType    string
	NotBefore  time.Time
	NotAfter   time.Time
	CreatedAt  time.Time
	LastUsedAt time.Time
	Comment    string
}

func (p *Provider) x509CertConn(ctx context.Context) (*sql.Conn, error) {
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
	if err := p.ensureX509CertTable(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (p *Provider) ensureX509CertTable(ctx context.Context, conn *sql.Conn) error {
	p.x509CertTableMu.Lock()
	defer p.x509CertTableMu.Unlock()
	if p.x509CertTableReady {
		return nil
	}
	_, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _NEO_X509_CERT (
ID LONG PRIMARY KEY AUTO_INCREMENT,
NAME VARCHAR(40) NOT NULL,
USER_NAME VARCHAR(64) NOT NULL,
CERT_PEM VARCHAR(4000) NOT NULL,
CERT_HASH VARCHAR(64) NOT NULL,
KEY_TYPE VARCHAR(10),
NOT_BEFORE DATETIME,
NOT_AFTER DATETIME,
CREATED_AT DATETIME,
LAST_USED_AT DATETIME,
COMMENT VARCHAR(256)
)`)
	if err != nil {
		return err
	}
	p.x509CertTableReady = true
	return nil
}

// GetX509CertByHash is used by the mTLS auth path; it looks up by CERT_HASH
// (unique per issued certificate) and is not user-scoped because the caller
// identity is not yet established at that point in the connection lifecycle.
func (p *Provider) GetX509CertByHash(ctx context.Context, certHash string) (*X509CertDefinition, error) {
	conn, err := p.x509CertConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return scanX509Cert(conn.QueryRowContext(ctx, `SELECT ID, NAME, USER_NAME, CERT_PEM, CERT_HASH, KEY_TYPE, NOT_BEFORE, NOT_AFTER, CREATED_AT, LAST_USED_AT, COMMENT FROM _NEO_X509_CERT WHERE CERT_HASH = ?`, certHash))
}

func (p *Provider) GetAllX509Certs(ctx context.Context, scope UserScope) ([]*X509CertDefinition, error) {
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := p.x509CertConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, NAME, USER_NAME, CERT_PEM, CERT_HASH, KEY_TYPE, NOT_BEFORE, NOT_AFTER, CREATED_AT, LAST_USED_AT, COMMENT FROM _NEO_X509_CERT WHERE USER_NAME = ? ORDER BY ID`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*X509CertDefinition{}
	for rows.Next() {
		def, err := scanX509Cert(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, def)
	}
	return result, rows.Err()
}

func (p *Provider) SaveX509Cert(ctx context.Context, scope UserScope, def *X509CertDefinition) error {
	if def == nil || strings.TrimSpace(def.Name) == "" || strings.TrimSpace(def.CertPEM) == "" || strings.TrimSpace(def.CertHash) == "" {
		return errors.New("x509 certificate definition is invalid")
	}
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	conn, err := p.x509CertConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	def.UserName = user
	if def.CreatedAt.IsZero() {
		def.CreatedAt = time.Now()
	}
	result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_X509_CERT (NAME, USER_NAME, CERT_PEM, CERT_HASH, KEY_TYPE, NOT_BEFORE, NOT_AFTER, CREATED_AT, LAST_USED_AT, COMMENT) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		def.Name, user, def.CertPEM, def.CertHash, def.KeyType, def.NotBefore, def.NotAfter, def.CreatedAt, nullableTime(def.LastUsedAt), def.Comment)
	if err != nil {
		return err
	}
	def.Id, err = result.LastInsertId()
	return err
}

// RemoveX509Cert deletes the caller's own certificate row and returns its
// CERT_HASH so the caller can invalidate the auth verifier's cache.
func (p *Provider) RemoveX509Cert(ctx context.Context, scope UserScope, id int64) (string, error) {
	user, err := p.normalizeUserScope(scope)
	if err != nil {
		return "", err
	}
	conn, err := p.x509CertConn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	var certHash string
	err = conn.QueryRowContext(ctx, `SELECT CERT_HASH FROM _NEO_X509_CERT WHERE ID = ? AND USER_NAME = ?`, id, user).Scan(&certHash)
	if err != nil {
		return "", err
	}
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_X509_CERT WHERE ID = ? AND USER_NAME = ?`, id, user)
	if err != nil {
		return "", err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", sql.ErrNoRows
	}
	return certHash, nil
}

func (p *Provider) TouchX509Cert(ctx context.Context, id int64, at time.Time) error {
	conn, err := p.x509CertConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `UPDATE _NEO_X509_CERT SET LAST_USED_AT = ? WHERE ID = ?`, at, id)
	return err
}

type x509CertScanner interface{ Scan(...any) error }

func scanX509Cert(scanner x509CertScanner) (*X509CertDefinition, error) {
	def := &X509CertDefinition{}
	var lastUsedAt sql.NullTime
	var keyType, comment sql.NullString
	err := scanner.Scan(&def.Id, &def.Name, &def.UserName, &def.CertPEM, &def.CertHash, &keyType, &def.NotBefore, &def.NotAfter, &def.CreatedAt, &lastUsedAt, &comment)
	if err != nil {
		return nil, err
	}
	def.KeyType = keyType.String
	def.Comment = comment.String
	if lastUsedAt.Valid {
		def.LastUsedAt = lastUsedAt.Time
	}
	return def, nil
}
