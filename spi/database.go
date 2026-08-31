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
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/machbase/neo-client/v2"
	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/util"
)

var defaultDatabaseUser string = "sys"
var defaultDatabaseKey crypto.PrivateKey
var defaultDSN map[string]string

var (
	defaultPoolOnce sync.Once
	defaultPoolDB   *sql.DB
	defaultPoolErr  error
	userPools       map[string]*sql.DB = make(map[string]*sql.DB)
	userPoolsLock   sync.Mutex
)

func SetDefaultDSN(dsn map[string]string) {
	defaultDSN = dsn
}

func DefaultDSN(overrides map[string]string, deletes ...string) string {
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
	for _, k := range deletes {
		delete(result, k)
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

// DefaultPool returns the shared SQL connection pool for the default database.
func DefaultPool() (*sql.DB, error) {
	defaultPoolOnce.Do(func() {
		if len(defaultDSN) == 0 {
			defaultPoolErr = errors.New("default DSN is not configured")
			return
		}
		if defaultDSN["auth_key_pem"] == "" && defaultDSN["auth_key_file"] == "" {
			defaultPoolErr = errors.New("default key is not configured")
			return
		}
		defaultPoolDB, defaultPoolErr = sql.Open("machbase", DefaultDSN(nil))
		if defaultPoolErr == nil {
			defaultPoolErr = defaultPoolDB.Ping()
		}
	})
	if defaultPoolErr != nil {
		return nil, defaultPoolErr
	}
	if defaultPoolDB == nil {
		return nil, errors.New("default pool is not initialized")
	}
	return defaultPoolDB, nil
}

func CleanUp() {
	if pool, _ := DefaultPool(); pool != nil {
		pool.Close()
	}
}

func SetDefaultKey(user string, key crypto.PrivateKey) {
	defaultDatabaseUser = strings.ToLower(user)
	defaultDatabaseKey = key
}

func DefaultKey() crypto.PrivateKey {
	return defaultDatabaseKey
}

func Connect(ctx context.Context, user string) (*sql.Conn, error) {
	user = strings.ToLower(user)
	var connectDB *sql.DB
	// user can be empty, where the caller is from SQL() function in TQL.
	if user == defaultDatabaseUser || user == "" {
		pool, err := DefaultPool()
		if err != nil {
			return nil, err
		}
		connectDB = pool
	} else {
		userPoolsLock.Lock()
		defer userPoolsLock.Unlock()
		if db, ok := userPools[user]; ok {
			connectDB = db
		} else {
			conf := DefaultDSN(map[string]string{"user": fmt.Sprintf("%s as %s", defaultDatabaseUser, user)})
			if db, err := sql.Open("machbase", conf); err != nil {
				return nil, err
			} else {
				if len(userPools) == 0 {
					util.AddShutdownHook(func() {
						for _, p := range userPools {
							p.Close()
						}
					})
				}
				db.SetConnMaxIdleTime(20 * time.Second)
				db.SetConnMaxLifetime(1 * time.Minute)
				userPools[user] = db
				connectDB = db
			}
		}
	}
	return connectDB.Conn(ctx)
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
	// Issue machbase/neo#1408
	// can not use primitive types directly
	buffer := make([]interface{}, len(columnTypes))
	for i, colType := range columnTypes {
		switch colType.ScanType().String() {
		case "int16":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.NullInt16)
			} else {
				buffer[i] = new(int16)
			}
		case "uint16":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.Null[uint16])
			} else {
				buffer[i] = new(uint16)
			}
		case "int32":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.NullInt32)
			} else {
				buffer[i] = new(int32)
			}
		case "uint32":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.Null[uint32])
			} else {
				buffer[i] = new(uint32)
			}
		case "int64":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.NullInt64)
			} else {
				buffer[i] = new(int64)
			}
		case "uint64":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.Null[uint64])
			} else {
				buffer[i] = new(uint64)
			}
		case "float32":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.Null[float32])
			} else {
				buffer[i] = new(float32)
			}
		case "float64":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.NullFloat64)
			} else {
				buffer[i] = new(float64)
			}
		case "time.Time":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.NullTime)
			} else {
				buffer[i] = new(time.Time)
			}
		case "string":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.NullString)
			} else {
				buffer[i] = new(string)
			}
		case "[]uint8":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.Null[[]byte])
			} else {
				buffer[i] = new([]byte)
			}
		case "net.IP":
			if nullable, _ := colType.Nullable(); nullable {
				buffer[i] = new(sql.Null[net.IP])
			} else {
				buffer[i] = new(net.IP)
			}
		case "api.JSONString":
			if nullable, _ := colType.Nullable(); nullable {
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
		case "sql.RawBytes":
			buffer[i] = new(sql.RawBytes)
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
				// fmt.Printf("=================> colName: %s, databaseType: %s, scanType: %s\n",
				// 	colType.Name(), colType.DatabaseTypeName(), colType.ScanType().String())
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

var defaultHttpLocalEndpoint string

func SetDefaultHttpEndpoint(addrs []string) {
	bestAddr := ""
	bestScore := -1
	for _, addr := range addrs {
		normalized, score := normalizeHTTPListenerAddress(addr)
		if normalized == "" {
			continue
		}
		if score > bestScore {
			bestAddr = normalized
			bestScore = score
		}
	}
	defaultHttpLocalEndpoint = bestAddr
}

func normalizeHTTPListenerAddress(addr string) (string, int) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return "", 0
	}

	scheme := ""
	if idx := strings.Index(trimmed, "://"); idx >= 0 {
		scheme = trimmed[:idx]
		trimmed = trimmed[idx+3:]
	}

	switch scheme {
	case "http", "https":
		return addr, 3
	case "unix":
		return addr, 0
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		return addr, 0
	}

	host = strings.Trim(host, "[]")
	if host == "" {
		return "http://127.0.0.1:" + port, 2
	}

	switch host {
	case "0.0.0.0":
		return "http://127.0.0.1:" + port, 2
	case "::", "[::]":
		return "http://[::1]:" + port, 2
	case "127.0.0.1", "::1", "localhost":
		return "http://" + net.JoinHostPort(host, port), 3
	default:
		return "http://" + net.JoinHostPort(host, port), 1
	}
}

func DefaultHttpEndpoint() string {
	return defaultHttpLocalEndpoint
}

type Explainer interface {
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

func ColumnTypes(rows *sql.Rows) ([]string, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	types := make([]string, len(columnTypes))
	for i, col := range columnTypes {
		if str := col.DatabaseTypeName(); str != "" {
			types[i] = str
		} else {
			types[i] = col.ScanType().String()
			if types[i] == "*interface {}" {
				// TODO: improve datatype detection,
				// database type name is "", and scan type is "*interface {}",
				// so we can not determine the data type.
				types[i] = "string"
			}
		}
	}
	return types, nil
}

type LicenseInfo struct {
	Id            string `json:"id"`
	Type          string `json:"type"`
	Customer      string `json:"customer"`
	Project       string `json:"project"`
	CountryCode   string `json:"countryCode"`
	InstallDate   string `json:"installDate"`
	IssueDate     string `json:"issueDate"`
	LicenseStatus string `json:"licenseStatus,omitempty"`
}

func GetLicenseInfo(ctx context.Context, conn *sql.Conn) (*LicenseInfo, error) {
	ret := &LicenseInfo{}
	var violateStatus int
	var violateMsg sql.NullString
	row := conn.QueryRowContext(ctx, "select ID, TYPE, CUSTOMER, PROJECT, COUNTRY_CODE, INSTALL_DATE, ISSUE_DATE, VIOLATE_STATUS, VIOLATE_MSG from v$license_info")
	if err := row.Err(); err != nil {
		return nil, err
	}
	if err := row.Scan(&ret.Id, &ret.Type, &ret.Customer, &ret.Project, &ret.CountryCode, &ret.InstallDate, &ret.IssueDate, &violateStatus, &violateMsg); err != nil {
		return nil, err
	}
	if violateStatus == 0 {
		ret.LicenseStatus = "Valid"
	} else if violateMsg.Valid {
		ret.LicenseStatus = violateMsg.String
	}
	return ret, nil
}

func InstallLicenseFile(ctx context.Context, conn *sql.Conn, path string) (*LicenseInfo, error) {
	if strings.ContainsRune(path, ';') {
		return nil, errors.New("invalid license file path")
	}
	_, err := conn.ExecContext(ctx, "alter system install license='"+path+"'")
	if err != nil {
		return nil, err
	}
	return GetLicenseInfo(ctx, conn)
}

func InstallLicenseData(ctx context.Context, conn *sql.Conn, licenseFilePath string, content []byte) (*LicenseInfo, error) {
	_, err := os.Stat(licenseFilePath)
	if err == nil {
		// backup existing file
		os.Rename(licenseFilePath, fmt.Sprintf("%s_%s", licenseFilePath, time.Now().Format("20060102_150405")))
	}
	if err := os.WriteFile(licenseFilePath, content, 0640); err != nil {
		return nil, err
	}
	return InstallLicenseFile(ctx, conn, licenseFilePath)
}

type TableInfo struct {
	Database string           `json:"database"`       // M$SYS_TABLES.DATABASE_ID
	User     string           `json:"user"`           // M$SYS_USERS.NAME
	Name     string           `json:"name"`           // M$SYS_TABLES.NAME
	Id       int64            `json:"id"`             // M$SYS_TABLES.ID
	Type     client.TableType `json:"type"`           // M$SYS_TABLES.TYPE
	Flag     client.TableFlag `json:"flag,omitempty"` // M$SYS_TABLES.FLAG
}

func (ti *TableInfo) Kind() string {
	desc := "undef"
	switch ti.Type {
	case client.TableTypeLog:
		desc = "Log Table"
	case client.TableTypeFixed:
		desc = "Fixed Table"
	case client.TableTypeVolatile:
		desc = "Volatile Table"
	case client.TableTypeLookup:
		desc = "Lookup Table"
	case client.TableTypeKeyValue:
		desc = "KeyValue Table"
	case client.TableTypeTag:
		desc = "Tag Table"
	}
	switch ti.Flag {
	case client.TableFlagData:
		desc += " (data)"
	case client.TableFlagRollup:
		desc += " (rollup)"
	case client.TableFlagMeta:
		desc += " (meta)"
	case client.TableFlagStat:
		desc += " (stat)"
	}
	return desc
}

func ifThenElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func ListTablesWalk(ctx context.Context, conn *sql.Conn, showAll bool, callback func(*TableInfo, error) bool) {
	descriptiveType := false
	sqlText := SqlTidy(
		`SELECT
			j.DATABASE_NAME as DATABASE_NAME,
			u.NAME as USER_NAME,
			j.NAME as TABLE_NAME,
			j.ID as TABLE_ID,`,
		ifThenElse(descriptiveType, `
			case j.TYPE
				when 0 then 'Log'
				when 1 then 'Fixed'
				when 3 then 'Volatile'
				when 4 then 'Lookup'
				when 5 then 'KeyValue'
				when 6 then 'Tag'
				else ''
			end as TABLE_TYPE,
			case j.FLAG
				when 1 then 'Data'
				when 2 then 'Rollup'
				when 4 then 'Meta'
				when 8 then 'Stat'
				else ''
			end as TABLE_FLAG`,
			`
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG`),
		`FROM
			M$SYS_USERS u,
			(
				select
					ID as ID,
					NAME as NAME,
					USER_ID as USER_ID,
					TYPE as TYPE,
					FLAG as FLAG,
					DATABASE_NAME
				from
					M$SYS_TABLES
				where
					DATABASE_NAME = current_database()
			) as j
		WHERE
			u.USER_ID = j.USER_ID`,
		ifThenElse(showAll, "", `AND j.NAME NOT LIKE '\_%'`),
		`ORDER by j.NAME`)

	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		callback(nil, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		ti := &TableInfo{}
		err = rows.Scan(&ti.Database, &ti.User, &ti.Name, &ti.Id, &ti.Type, &ti.Flag)
		if !callback(ti, err) {
			return
		}
	}
	if err := rows.Err(); err != nil {
		callback(nil, err)
	}
}

func QueryTableType(ctx context.Context, conn *sql.Conn, fullTableName string) (client.TableType, error) {
	_, userName, tableName := TableName(fullTableName).Split()
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRowContext(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	if r.Err() != nil {
		return -1, r.Err()
	}
	var ret client.TableType
	if err := r.Scan(&ret); err != nil {
		return -1, err
	}
	return ret, nil
}

func TruncateTableIfExists(ctx context.Context, conn *sql.Conn, fullTableName string, truncate bool) (exists bool, truncated bool, err error) {
	exists, err = ExistsTable(ctx, conn, fullTableName)
	if err != nil {
		return
	}
	if !exists {
		return
	}

	// TRUNCATE TABLE
	if !truncate {
		return
	}
	tableType, err0 := QueryTableType(ctx, conn, fullTableName)
	if err0 != nil {
		err = fmt.Errorf("table '%s' doesn't exist, %s", fullTableName, err0.Error())
		return
	}
	if tableType == client.TableTypeLog {
		_, err0 := conn.ExecContext(ctx, fmt.Sprintf("truncate table %s", fullTableName))
		if err0 != nil {
			err = err0
			return
		}
		truncated = true
	} else {
		_, err0 := conn.ExecContext(ctx, fmt.Sprintf("delete from %s", fullTableName))
		if err0 != nil {
			err = err0
			return
		}
		truncated = true
	}
	return
}

func ExistsTable(ctx context.Context, conn *sql.Conn, fullTableName string) (bool, error) {
	dbName, userName, tableName := TableName(fullTableName).SplitOr("", "SYS")
	_, dbID, err := databaseInfo(ctx, conn, dbName)
	if err != nil {
		return false, err
	}
	sql := "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.DATABASE_ID = ? AND T.NAME = ?"
	r := conn.QueryRowContext(ctx, sql, strings.ToUpper(userName), dbID, strings.ToUpper(tableName))
	if err := r.Err(); err != nil {
		fmt.Println("error", err.Error())
		return false, err
	}
	var count = 0
	if err := r.Scan(&count); err != nil {
		return false, err
	}
	return (count == 1), nil
}

func DatabaseID(ctx context.Context, conn *sql.Conn, dbName string) (int64, error) {
	_, dbID, err := databaseInfo(ctx, conn, dbName)
	return dbID, err
}

func databaseInfo(ctx context.Context, conn *sql.Conn, dbName string) (string, int64, error) {
	var row *sql.Row
	var resolvedName string
	var dbID int64

	if dbName == "" {
		row = conn.QueryRowContext(ctx, "select NAME, DATABASE_ID from V$DATABASES where NAME = CURRENT_DATABASE()")
	} else {
		row = conn.QueryRowContext(ctx, "select NAME, DATABASE_ID from V$DATABASES where NAME = ?", dbName)
	}
	if err := row.Err(); err != nil {
		return legacyDatabaseInfo(ctx, conn, dbName)
	}
	if err := row.Scan(&resolvedName, &dbID); err != nil {
		return "", 0, err
	}
	return resolvedName, dbID, nil
}

func legacyDatabaseInfo(ctx context.Context, conn *sql.Conn, dbName string) (string, int64, error) {
	resolvedName := strings.ToUpper(dbName)
	if resolvedName == "" || resolvedName == "MACHBASEDB" {
		return "MACHBASEDB", -1, nil
	}
	row := conn.QueryRowContext(ctx, "select BACKUP_TBSID from V$STORAGE_MOUNT_DATABASES where MOUNTDB = ?", resolvedName)
	if row.Err() != nil {
		return "", 0, row.Err()
	}
	var dbID int64
	if err := row.Scan(&dbID); err != nil {
		return "", 0, err
	}
	return resolvedName, dbID, nil
}

// TableDescription is represents data that comes as a result of 'desc <table>'
type TableDescription struct {
	Database string              `json:"database"`
	User     string              `json:"user"`
	Name     string              `json:"name"`
	Id       int64               `json:"id"`
	Type     client.TableType    `json:"type"`
	Flag     client.TableFlag    `json:"flag,omitempty"`
	Columns  client.Columns      `json:"columns"`
	Indexes  []*IndexDescription `json:"indexes"`

	Summarized       bool   `json:"summarized"`
	SummarizedColumn string `json:"summarizedColum,omitempty"`
	TagNameColumn    string `json:"tagNameColumn,omitempty"`
}

type IndexDescription struct {
	Id             int64            `json:"id"`
	Name           string           `json:"name"`
	Type           client.IndexType `json:"type"`
	Cols           []string         `json:"columns"`
	KeyCompress    bool             `json:"keyCompress"`
	MaxLevel       int              `json:"maxLevel"`
	PartValueCount int              `json:"partValueCount"`
	BitMapEncode   string           `json:"bitMapEncode"`
}

// String returns string representation of table type.
func (td *TableDescription) String() string {
	desc := "undef"
	switch td.Type {
	case client.TableTypeLog:
		desc = "Log Table"
	case client.TableTypeFixed:
		desc = "Fixed Table"
	case client.TableTypeVolatile:
		desc = "Volatile Table"
	case client.TableTypeLookup:
		desc = "Lookup Table"
	case client.TableTypeKeyValue:
		desc = "KeyValue Table"
	case client.TableTypeTag:
		desc = "Tag Table"
	}
	switch td.Flag {
	case client.TableFlagData:
		desc += " (data)"
	case client.TableFlagRollup:
		desc += " (rollup)"
	case client.TableFlagMeta:
		desc += " (meta)"
	case client.TableFlagStat:
		desc += " (stat)"
	}
	return desc
}

// Describe retrieves the result of 'desc table'.
//
// If includeHiddenColumns is true, the result includes hidden columns those name start with '_'
// such as "_RID" and "_ARRIVAL_TIME".
func DescribeTable(ctx context.Context, conn *sql.Conn, name string, includeHiddenColumns bool) (*TableDescription, error) {
	_, _, tableName := TableName(name).Split()
	if strings.HasPrefix(tableName, "V$") {
		return describe_mv(ctx, conn, TableName(name), includeHiddenColumns)
	} else if strings.HasPrefix(tableName, "M$") {
		return describe_mv(ctx, conn, TableName(name), includeHiddenColumns)
	} else {
		return describe(ctx, conn, TableName(name), includeHiddenColumns)
	}
}

func describe(ctx context.Context, conn *sql.Conn, name TableName, includeHiddenColumns bool) (*TableDescription, error) {
	d := &TableDescription{}
	var colCount int

	dbName, userName, tableName := name.SplitOr("", "SYS")
	resolvedDBName, dbId, err := databaseInfo(ctx, conn, dbName)
	if err != nil {
		return nil, err
	}

	describeSqlText := SqlTidy(
		`SELECT
			j.ID as TABLE_ID,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG,
			j.COLCOUNT as TABLE_COLCOUNT
		from
			M$SYS_USERS u,
			M$SYS_TABLES j
		where
			u.NAME = ?
		and j.USER_ID = u.USER_ID
		and j.DATABASE_ID = ?
		and j.NAME = ?`)

	r := conn.QueryRowContext(ctx, describeSqlText, userName, dbId, tableName)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := r.Scan(&d.Id, &d.Type, &d.Flag, &colCount); err != nil {
		return nil, err
	}
	d.Database = resolvedDBName
	d.User = userName
	d.Name = tableName

	rows, err := conn.QueryContext(ctx, "select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? AND database_id = ? order by id", d.Id, dbId)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()

	for rows.Next() {
		col := &client.Column{}
		err = rows.Scan(&col.Name, &col.Type, &col.Length, &col.Id, &col.Flag)
		if err != nil {
			return nil, err
		}
		if !includeHiddenColumns && strings.HasPrefix(col.Name, "_") {
			continue
		}
		col.DataType = col.Type.DataType()
		d.Columns = append(d.Columns, col)

		if col.IsSummarized() {
			d.Summarized = true
			d.SummarizedColumn = col.Name
		}
		if col.IsTagName() {
			d.TagNameColumn = col.Name
		}
		if col.IsBaseDistance() {
			col.Flag = api.ColumnFlagBaseDistance
		}
	}
	rows.Close()
	rows = nil

	if indexes, err := describe_idx(ctx, conn, d.Id, dbId); err != nil {
		return nil, err
	} else {
		d.Indexes = indexes
	}
	return d, nil
}

func describe_mv(ctx context.Context, conn *sql.Conn, name TableName, includeHiddenColumns bool) (*TableDescription, error) {
	d := &TableDescription{}
	var tableType int
	var colCount int

	d.Database, d.User, d.Name = name.Split()
	tablesTable := "M$SYS_TABLES"
	columnsTable := "M$SYS_COLUMNS"
	if strings.HasPrefix(d.Name, "V$") {
		tablesTable = "V$TABLES"
		columnsTable = "V$COLUMNS"
	} else if strings.HasPrefix(d.Name, "M$") {
		tablesTable = "M$TABLES"
		columnsTable = "M$COLUMNS"
	}
	r := conn.QueryRowContext(ctx, fmt.Sprintf("select name, type, flag, id, colcount from %s where name = ?", tablesTable), d.Name)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := r.Scan(&d.Name, &tableType, &d.Flag, &d.Id, &colCount); err != nil {
		return nil, err
	}
	d.Type = client.TableType(tableType)

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`select name, type, length, id from %s where table_id = ? order by id`, columnsTable), d.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		col := &client.Column{}
		err = rows.Scan(&col.Name, &col.Type, &col.Length, &col.Id)
		if err != nil {
			return nil, err
		}
		if !includeHiddenColumns && strings.HasPrefix(col.Name, "_") {
			continue
		}
		col.DataType = col.Type.DataType()
		d.Columns = append(d.Columns, col)
	}
	return d, nil
}

func describe_idx(ctx context.Context, conn *sql.Conn, tableId int64, dbId int64) ([]*IndexDescription, error) {
	rows, err := conn.QueryContext(ctx,
		`select
			b.name,
			b.type,
			b.id,
			b.key_compress,
			b.max_level,
			b.part_value_count,
			case b.bitmap_encode
				when 0 then 'EQUAL'
				else 'RANGE' end
			as bitmap_encode
		from
			M$SYS_TABLES  a,
			M$SYS_INDEXES b
		where
			a.id = ?
		AND a.database_id = ?
		AND a.id = b.table_id
		AND a.database_id = b.database_id
		AND b.database_id = ?
		`, tableId, dbId, dbId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := []*IndexDescription{}
	for rows.Next() {
		d := &IndexDescription{}
		var indexType int
		var keyCompress int
		if err = rows.Scan(&d.Name, &indexType, &d.Id, &keyCompress, &d.MaxLevel, &d.PartValueCount, &d.BitMapEncode); err != nil {
			return nil, err
		}
		d.Type = client.IndexType(indexType)
		d.KeyCompress = (keyCompress != 0)
		idxCols, err := conn.QueryContext(ctx, `select name from M$SYS_INDEX_COLUMNS where index_id = ? AND database_id = ? order by col_id`, d.Id, dbId)
		if err != nil {
			return nil, err
		}
		for idxCols.Next() {
			var col string
			if err = idxCols.Scan(&col); err != nil {
				idxCols.Close()
				return nil, err
			}
			d.Cols = append(d.Cols, col)
		}
		idxCols.Close()
		indexes = append(indexes, d)
	}
	return indexes, nil
}

/* Interpreting Influx line protocol

   | Machbase            | influxdb                                    |
   | ------------------- | ------------------------------------------- |
   | table name          | db                                          |
   | tag name            | measurement + '.' + field name              |
   | time                | timestamp                                   |
   | value               | value of the field (if it is not a number type, will be ignored and not inserted) |
*/

func WriteLineProtocol(ctx context.Context, conn *sql.Conn, dbName string, descColumns client.Columns, measurement string, fields map[string]any, tags map[string]string, ts time.Time) *InsertResult {
	columns := descColumns.Names()
	columns = columns[:3]

	/*
		Machbase : name, time, value, host
		influxdb : tags key[DC, HOST, NAME, SYSTEM]
		=> HOST append / DC, NAME, SYSTEM not append
	*/
	compareNames := descColumns.Names()
	compareTypes := descColumns.DataTypes()
	compareNames = compareNames[3:]
	compareTypes = compareTypes[3:]
	for idx, val := range compareNames {
		if _, ok := tags[val]; ok {
			if compareTypes[idx] == api.DataTypeString {
				columns = append(columns, val)
			}
		}
	}

	rows := make([][]any, 0)

	for k, v := range fields {
		values := make([]any, 0)
		values = append(values, fmt.Sprintf("%s.%s", measurement, k))
		values = append(values, ts)

		switch val := v.(type) {
		case float32:
			values = append(values, float64(val))
		case float64:
			values = append(values, val)
		case int:
			values = append(values, float64(val))
		case int32:
			values = append(values, float64(val))
		case int64:
			values = append(values, float64(val))
		default:
			// unsupported value type
			continue
		}

		for i := 3; i < len(columns); i++ {
			values = append(values, tags[columns[i]])
		}

		rows = append(rows, values)
	}

	if len(rows) == 0 {
		return &InsertResult{
			rowsAffected: 0,
			message:      "no rows inserted",
		}
	}

	vf := make([]string, len(columns))
	for i := range vf {
		vf[i] = "?"
	}
	tableName := dbName
	valuesPlaces := strings.Join(vf, ",")
	columnsPhrase := strings.Join(columns, ",")

	sqlText := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, columnsPhrase, valuesPlaces)
	var numRows int
	for _, rec := range rows {
		_, err := conn.ExecContext(ctx, sqlText, rec...)
		if err != nil {
			return &InsertResult{
				err:          err,
				rowsAffected: numRows,
				message:      "batch inserts aborted - " + sqlText,
			}
		}
		numRows++
	}

	ret := &InsertResult{
		rowsAffected: numRows,
	}
	ret.message = MakeUserMessage(SQLStatementTypeInsert, int64(numRows))
	return ret
}

type InsertResult struct {
	err          error
	rowsAffected int
	message      string
}

func (ir *InsertResult) Err() error {
	return ir.err
}

func (ir *InsertResult) RowsAffected() int64 {
	return int64(ir.rowsAffected)
}

func (ir *InsertResult) Message() string {
	return ir.message
}

// ParseUserName parses the given loginName into a UserName struct.
// It returns the parsed UserName and a boolean indicating whether proxy authentication is allowed.
// The expected format for proxy authentication is "sys as proxy_user" (case-insensitive for "as").
// If the format is correct and the login username is "sys", it allows proxy authentication and returns true.
// If the format is correct but the login username is not "sys", it does not allow proxy authentication and returns false.
// If the format is incorrect, it treats the entire loginName as the Login with no Proxy and returns false.
func ParseUserName(loginName string) (UserName, bool) {
	username := UserName{}
	allowed := username.Parse(loginName)
	return username, allowed
}

type UserName struct {
	// Login is the login username token (the left side of "as") or the raw input when parsing fails.
	Login string
	// Proxy is the proxy username token (the right side of "as") when present.
	Proxy string
}

var usernameProxyPattern = regexp.MustCompile(`(?i)^\s*(\S+)\s+as\s+(\S+)\s*$`)

// Parse attempts to parse the given loginName into Login and Proxy components.
// The expected format is "login as proxy_user" (case-insensitive for "as").
// It returns true if parsing is successful and the login username is "sys" (case-insensitive), allowing proxy authentication.
// On parse failure, it sets Login to the original loginName, Proxy to an empty string, and returns false.
// Example:
//   - Input: "sys as alice" -> Login: "sys", Proxy: "alice", returns true (proxy auth allowed)
//   - Input: "bob as alice" -> Login: "bob", Proxy: "alice", returns false (proxy auth not allowed)
func (u *UserName) Parse(loginName string) bool {
	matches := usernameProxyPattern.FindStringSubmatch(loginName)
	if len(matches) == 3 {
		u.Login = matches[1]
		u.Proxy = matches[2]
		if strings.EqualFold(u.Login, "sys") {
			if strings.EqualFold(u.Proxy, "sys") {
				// treat "sys as sys" as normal "sys" login without proxy
				u.Proxy = ""
				return false
			} else {
				// "sys as proxy_user" format is only allowed when login is "sys""
				return true
			}
		} else {
			// proxy auth not allowed when login is not "sys", even if the format is correct
			return false
		}
	}
	u.Login = loginName
	u.Proxy = ""
	return false
}

// String returns the string representation of the Username.
// It formats as "login as proxy" if Proxy is present, otherwise just "login".
// The returned string is in lowercase for consistency, as login names are typically case-insensitive.
func (u UserName) String() string {
	if u.Proxy != "" {
		return fmt.Sprintf("%s as %s", strings.ToLower(u.Login), strings.ToLower(u.Proxy))
	}
	return strings.ToLower(u.Login)
}

type TableName string

func (tn TableName) String() string {
	return strings.ToUpper(string(tn))
}

// Split splits the full table name that consists of database, user, and table name.
func (tn TableName) SplitOr(dbName string, userName string) (string, string, string) {
	tableName := strings.ToUpper(string(tn))
	parts := strings.SplitN(tableName, ".", 3)
	if len(parts) == 2 {
		userName = parts[0]
		tableName = parts[1]
	} else if len(parts) == 3 {
		dbName = parts[0]
		userName = parts[1]
		tableName = parts[2]
	}
	return dbName, userName, tableName
}

// Split splits the full table name that consists of database, user, and table name.
func (tn TableName) Split() (string, string, string) {
	dbName := "MACHBASEDB"
	userName := "SYS"
	tableName := strings.ToUpper(string(tn))
	parts := strings.SplitN(tableName, ".", 3)
	if len(parts) == 2 {
		userName = parts[0]
		tableName = parts[1]
	} else if len(parts) == 3 {
		dbName = parts[0]
		userName = parts[1]
		tableName = parts[2]
	}
	return dbName, userName, tableName
}

type Appender interface {
	// TableName returns the name of the table to which the Appender is appending data.
	TableName() string

	// Append adds a new row with the specified values to the table.
	// The number of values must match the number of columns in the table.
	//
	// Example:
	//	appender.Append("name", time.Now(), 3.14)
	Append(values ...any) error

	// AppendLogTime adds a new row with the specified timestamp and values to the table.
	// This is applicable only for log tables, where the timestamp is applied to _ARRIVAL_TIME instead of the current system time.
	//
	// Example:
	//	appender.AppendLogTime(time.Now(), "name", 3.14)
	AppendLogTime(ts time.Time, values ...any) error

	// Close finalizes the appending process and releases any resources associated with the Appender.
	// It returns the number of rows successfully appended and the number of rows that failed to append.
	//
	// Example:
	//	rowsAppended, rowsFailed, err := appender.Close()
	Close() (int64, int64, error)

	// Columns returns a list of column information for the table.
	//
	// Example:
	//	columns, err := appender.Columns()
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	for _, col := range columns {
	//		fmt.Println(col.Name)
	//	}
	Columns() client.Columns

	// TableType returns the type of the table to which the Appender is appending data.
	TableType() client.TableType

	// WithInputColumns sets the input column names for the Appender.
	WithInputColumns(columns ...string) Appender

	// WithInputFormats sets the input formats for the Appender.
	WithInputFormats(formats ...string) Appender

	// WithBatchMaxRows sets the maximum batch size in rows for batch append. If the batch size exceeds the limit, it will be flushed immediately.
	// The default value is 512 rows. The minimum value is 1 row.
	WithBatchMaxRows(rows int) Appender

	// WithBatchMaxBytes sets the maximum batch size in bytes for batch append. If the batch size exceeds the limit, it will be flushed immediately.
	// The default value is 512KB. The minimum value is 4KB.
	WithBatchMaxBytes(bytes int) Appender

	// WithBatchMaxDelay sets the maximum delay for batch append. If the batch is not full, it will be flushed when the delay is reached.
	// The default value is 5 milliseconds. The minimum value is 1 millisecond.
	WithBatchMaxDelay(duration time.Duration) Appender
}

type Flusher interface {
	Flush() error
}
