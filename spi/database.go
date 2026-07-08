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
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machbase"
)

var defaultDatabase api.Database
var defaultDatabaseKey crypto.PrivateKey
var defaultDSN map[string]string

func SetDefaultDSN(dsn map[string]string) {
	defaultDSN = dsn
}

func DefaultDSN(overrides map[string]string) string {
	result := make(map[string]string)
	for k, v := range defaultDSN {
		result[k] = v
	}
	for k, v := range overrides {
		result[k] = v
	}
	if _, ok := result["auth_key_pem"]; ok {
		delete(result, "auth_key_file")
	}
	parts := make([]string, 0, len(result))
	for k, v := range result {
		if strings.ContainsAny(v, " ;\n\r\t") {
			v = fmt.Sprintf("\"%s\"", v)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ";")
}

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
	SQLStatementTypeCommonTableExpression
	SQLStatementTypeExplain
	SQLStatementTypeShow
)

func (st SQLStatementType) String() string {
	switch st {
	case SQLStatementTypeSelect:
		return "SELECT"
	case SQLStatementTypeInsert:
		return "INSERT"
	case SQLStatementTypeUpdate:
		return "UPDATE"
	case SQLStatementTypeDelete:
		return "DELETE"
	case SQLStatementTypeCreate:
		return "CREATE"
	case SQLStatementTypeDrop:
		return "DROP"
	case SQLStatementTypeAlter:
		return "ALTER"
	case SQLStatementTypeDescribe:
		return "DESCRIBE"
	case SQLStatementTypeCommonTableExpression:
		return "CTE"
	case SQLStatementTypeExplain:
		return "EXPLAIN"
	case SQLStatementTypeShow:
		return "SHOW"
	default:
		return "OTHER"
	}
}

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
	case "DESCRIBE", "DESC":
		return SQLStatementTypeDescribe
	case "WITH":
		return SQLStatementTypeCommonTableExpression
	case "SHOW":
		return SQLStatementTypeShow
	case "EXPLAIN":
		return SQLStatementTypeExplain
	default:
		return SQLStatementTypeOther
	}
}

func (st SQLStatementType) IsFetch() bool {
	return st == SQLStatementTypeSelect || st == SQLStatementTypeDescribe || st == SQLStatementTypeCommonTableExpression
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
	maxOpenConn     = 20
	maxIdleConn     = 2
	connMaxLifetime = 10 * time.Minute
	connMaxIdleTime = 1 * time.Minute
)

func SetDefaultPoolConfig(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) {
	maxOpenConn = maxOpen
	maxIdleConn = maxIdle
	connMaxLifetime = maxLifetime
	connMaxIdleTime = maxIdleTime
}

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
		defaultPoolDB.SetMaxOpenConns(maxOpenConn)
		defaultPoolDB.SetMaxIdleConns(maxIdleConn)
		defaultPoolDB.SetConnMaxLifetime(connMaxLifetime)
		defaultPoolDB.SetConnMaxIdleTime(connMaxIdleTime)
		defaultPoolErr = defaultPoolDB.Ping()
	})
	if defaultPoolErr != nil {
		return nil, defaultPoolErr
	}
	if defaultPoolDB == nil {
		return nil, errors.New("default pool is not initialized")
	}
	return defaultPoolDB, nil
}

func ColumnTypesToDataTypes(columnTypes []*sql.ColumnType) []api.DataType {
	var dataTypes = make([]api.DataType, len(columnTypes))
	for i, colType := range columnTypes {
		switch dbType := colType.DatabaseTypeName(); dbType {
		case "SHORT", "INT16":
			dataTypes[i] = api.DataTypeInt16
		case "USHORT", "UINT16":
			dataTypes[i] = api.DataTypeUInt16
		case "INT", "INTEGER", "INT32":
			dataTypes[i] = api.DataTypeInt32
		case "UINT", "UINTEGER", "UINT32":
			dataTypes[i] = api.DataTypeUInt32
		case "LONG", "INT64":
			dataTypes[i] = api.DataTypeInt64
		case "ULONG", "UINT64":
			dataTypes[i] = api.DataTypeUInt64
		case "FLOAT":
			dataTypes[i] = api.DataTypeFloat32
		case "DOUBLE":
			dataTypes[i] = api.DataTypeFloat64
		case "VARCHAR":
			dataTypes[i] = api.DataTypeString
		case "DATETIME":
			dataTypes[i] = api.DataTypeDatetime
		case "BINARY":
			dataTypes[i] = api.DataTypeBinary
		case "JSON":
			dataTypes[i] = api.DataTypeJSON
		case "IPV4":
			dataTypes[i] = api.DataTypeIPv4
		case "IPV6":
			dataTypes[i] = api.DataTypeIPv6
		default:
			dataTypes[i] = api.DataType(dbType)
		}
	}
	return dataTypes
}

func MakeBuffer(columnTypes []*sql.ColumnType) []interface{} {
	buffer := make([]interface{}, len(columnTypes))
	for i, colType := range columnTypes {
		switch colType.ScanType().String() {
		case "int16":
			if nullable, ok := colType.Nullable(); ok && nullable {
				buffer[i] = new(sql.NullInt16)
			} else {
				buffer[i] = new(int16)
			}
		case "uint16":
			buffer[i] = new(uint16)
		case "int32":
			if nullable, ok := colType.Nullable(); ok && nullable {
				buffer[i] = new(sql.NullInt32)
			} else {
				buffer[i] = new(int32)
			}
		case "uint32":
			buffer[i] = new(uint32)
		case "int64":
			buffer[i] = new(int64)
		case "float32":
			buffer[i] = new(float32)
		case "float64":
			buffer[i] = new(float64)
		case "time.Time":
			buffer[i] = new(time.Time)
		case "string":
			// Issue machbase/neo#1408
			// can not use string type directly
			if nullable, ok := colType.Nullable(); ok && nullable {
				buffer[i] = new(sql.NullString)
			} else {
				buffer[i] = new(string)
			}
		case "[]uint8":
			buffer[i] = new([]byte)
		case "net.IP":
			buffer[i] = new(net.IP)
		case "api.JSONString":
			if nullable, ok := colType.Nullable(); ok && nullable {
				buffer[i] = new(sql.Null[api.JSONString])
			} else {
				buffer[i] = new(api.JSONString)
			}
		case "sql.NullInt16":
			buffer[i] = new(sql.NullInt16)
		case "sql.NullInt32":
			buffer[i] = new(sql.NullInt32)
		case "sql.NullInt64":
			buffer[i] = new(sql.NullInt64)
		case "sql.NullFloat64":
			buffer[i] = new(sql.NullFloat64)
		case "sql.NullString":
			buffer[i] = new(sql.NullString)
		case "sql.NullBool":
			buffer[i] = new(sql.NullBool)
		case "sql.NullTime":
			buffer[i] = new(sql.NullTime)
		default:
			switch colType.DatabaseTypeName() {
			case "INT", "BIGINT", "SMALLINT", "TINYINT":
				buffer[i] = new(sql.NullInt64)
			case "FLOAT", "DOUBLE", "REAL":
				buffer[i] = new(sql.NullFloat64)
			case "VARCHAR", "TEXT", "CHAR":
				buffer[i] = new(sql.NullString)
			case "BOOLEAN":
				buffer[i] = new(sql.NullBool)
			case "DATE", "DATETIME", "TIMESTAMP":
				buffer[i] = new(sql.NullTime)
			default:
				buffer[i] = new(interface{})
			}
		}
	}
	return buffer
}

func MakeUserMessage(smtType SQLStatementType, rowsCount int64) string {
	rowsObj := ""
	switch rowsCount {
	case 0:
		rowsObj = "no rows"
	case 1:
		rowsObj = "a row"
	default:
		rowsObj = fmt.Sprintf("%d rows", rowsCount)
	}
	switch smtType {
	case SQLStatementTypeSelect, SQLStatementTypeDescribe, SQLStatementTypeCommonTableExpression:
		return fmt.Sprintf("%s selected.", rowsObj)
	case SQLStatementTypeInsert:
		return fmt.Sprintf("%s inserted.", rowsObj)
	case SQLStatementTypeUpdate:
		return fmt.Sprintf("%s updated.", rowsObj)
	case SQLStatementTypeDelete:
		return fmt.Sprintf("%s deleted.", rowsObj)
	case SQLStatementTypeCreate:
		return "Created successfully."
	case SQLStatementTypeDrop:
		return "Dropped successfully."
	case SQLStatementTypeAlter:
		return "Altered successfully."
	default:
		return "executed."
	}
}
