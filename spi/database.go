package spi

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machbase"
)

var defaultDatabase api.Database
var defaultDatabaseKey crypto.PrivateKey

func SetDefault(db api.Database, key crypto.PrivateKey) {
	defaultDatabase = db
	defaultDatabaseKey = key
}

func Default() api.Database {
	return defaultDatabase
}

func DefaultKey() crypto.PrivateKey {
	return defaultDatabaseKey
}

// IssueToken returns signed current timestamp.
// neo-shell uses it as a password for the session.
func IssueToken() string {
	skey := DefaultKey()
	signer, ok := skey.(crypto.Signer)
	if !ok {
		return ""
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	hash := sha256.Sum256([]byte(ts))
	sig, err := signer.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		return ""
	}

	return ts + ":" + base64.RawURLEncoding.EncodeToString(sig)
}

// VerifyToken verifies the token is valid and not expired.
func VerifyToken(token string, ttl time.Duration) bool {
	skey := DefaultKey()
	signer, ok := skey.(crypto.Signer)
	if !ok {
		return false
	}

	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return false
	}

	tsMillis, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.UnixMilli(tsMillis)
	now := time.Now()
	if ttl > 0 && now.Sub(issuedAt) > ttl {
		return false
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	hash := sha256.Sum256([]byte(parts[0]))
	pub := signer.Public()
	switch key := pub.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], sig) == nil
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(key, hash[:], sig)
	default:
		return false
	}
}

type SQLStatementType int

const (
	SQLStatementTypeOther SQLStatementType = iota
	SQLStatementTypeSelect
	SQLStatementTypeInsert
	SQLStatementTypeUpdate
	SQLStatementTypeDelete
	SQLStatementTypeCreate
	SQLStatementTypeDrop
	SQLStatementTypeAlter
	SQLStatementTypeDescribe
)

func DetectSQLStatementType(sqlText string) SQLStatementType {
	toks := strings.Fields(sqlText)
	if len(toks) == 0 {
		return SQLStatementTypeOther
	}
	verb := strings.ToUpper(toks[0])
	switch verb {
	case "SELECT":
		return SQLStatementTypeSelect
	case "INSERT":
		return SQLStatementTypeInsert
	case "UPDATE":
		return SQLStatementTypeUpdate
	case "DELETE":
		return SQLStatementTypeDelete
	case "CREATE":
		return SQLStatementTypeCreate
	case "DROP":
		return SQLStatementTypeDrop
	case "ALTER":
		return SQLStatementTypeAlter
	case "DESCRIBE":
		return SQLStatementTypeDescribe
	default:
		return SQLStatementTypeOther
	}
}

func (st SQLStatementType) IsFetch() bool {
	return st == SQLStatementTypeSelect || st == SQLStatementTypeDescribe
}

func SqlTidy(sqlTextLines ...string) string {
	sqlText := strings.Join(sqlTextLines, "\n")
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.Join(lines, " ")
}

var (
	defaultPoolOnce sync.Once
	defaultPoolDB   *sql.DB
	defaultPoolErr  error
)

// DefaultPool returns the shared SQL connection pool for the default database.
func DefaultPool() (*sql.DB, error) {
	defaultPoolOnce.Do(func() {
		db := Default()
		if db == nil {
			defaultPoolErr = errors.New("default database is not configured")
			return
		}
		defaultPoolDB, defaultPoolErr = machbase.OpenDBWithConnector(db, func(context.Context) ([]api.ConnectOption, error) {
			key := DefaultKey()
			if key == nil {
				return nil, errors.New("default key is not configured")
			}
			return []api.ConnectOption{api.WithAuthKey("sys", key)}, nil
		})
	})
	if defaultPoolErr != nil {
		return nil, defaultPoolErr
	}
	if defaultPoolDB == nil {
		return nil, errors.New("default pool is not initialized")
	}
	return defaultPoolDB, nil
}
