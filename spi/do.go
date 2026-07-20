package spi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
)

/* Interpreting Influx line protocol

   | Machbase            | influxdb                                    |
   | ------------------- | ------------------------------------------- |
   | table name          | db                                          |
   | tag name            | measurement + '.' + field name              |
   | time                | timestamp                                   |
   | value               | value of the field (if it is not a number type, will be ignored and not inserted) |
*/

func WriteLineProtocol(ctx context.Context, conn api.Conn, dbName string, descColumns api.Columns, measurement string, fields map[string]any, tags map[string]string, ts time.Time) api.Result {
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
		result := conn.Exec(ctx, sqlText, rec...)
		if result.Err() != nil {
			return &InsertResult{
				err:          result.Err(),
				rowsAffected: numRows,
				message:      "batch inserts aborted - " + sqlText,
			}
		}
		numRows++
	}

	ret := &InsertResult{
		rowsAffected: numRows,
	}
	switch numRows {
	case 0:
		ret.message = "no rows inserted"
	case 1:
		ret.message = "a row inserted"
	default:
		ret.message = fmt.Sprintf("%d rows inserted", numRows)
	}
	return ret
}

var _ api.Result = &InsertResult{}

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
	Database string        `json:"database"`       // M$SYS_TABLES.DATABASE_ID
	User     string        `json:"user"`           // M$SYS_USERS.NAME
	Name     string        `json:"name"`           // M$SYS_TABLES.NAME
	Id       int64         `json:"id"`             // M$SYS_TABLES.ID
	Type     api.TableType `json:"type"`           // M$SYS_TABLES.TYPE
	Flag     api.TableFlag `json:"flag,omitempty"` // M$SYS_TABLES.FLAG
}

func (ti *TableInfo) Kind() string {
	desc := "undef"
	switch ti.Type {
	case api.TableTypeLog:
		desc = "Log Table"
	case api.TableTypeFixed:
		desc = "Fixed Table"
	case api.TableTypeVolatile:
		desc = "Volatile Table"
	case api.TableTypeLookup:
		desc = "Lookup Table"
	case api.TableTypeKeyValue:
		desc = "KeyValue Table"
	case api.TableTypeTag:
		desc = "Tag Table"
	}
	switch ti.Flag {
	case api.TableFlagData:
		desc += " (data)"
	case api.TableFlagRollup:
		desc += " (rollup)"
	case api.TableFlagMeta:
		desc += " (meta)"
	case api.TableFlagStat:
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
			j.DB_NAME as DATABASE_NAME,
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
					a.ID as ID,
					a.NAME as NAME,
					a.USER_ID as USER_ID,
					a.TYPE as TYPE,
					a.FLAG as FLAG,
					case a.DATABASE_ID
						when -1 then 'MACHBASEDB'
						else d.MOUNTDB
					end as DB_NAME
				from
					M$SYS_TABLES a
				left join
					V$STORAGE_MOUNT_DATABASES d
				on
					a.DATABASE_ID = d.BACKUP_TBSID
			) as j
		WHERE
			u.USER_ID = j.USER_ID`,
		ifThenElse(showAll, "", "AND SUBSTR(j.NAME, 1, 1) <> '_'"),
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

func QueryTableType(ctx context.Context, conn *sql.Conn, fullTableName string) (api.TableType, error) {
	_, userName, tableName := api.TableName(fullTableName).Split()
	sql := "select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRowContext(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
	if r.Err() != nil {
		return -1, r.Err()
	}
	var ret api.TableType
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
	if tableType == api.TableTypeLog {
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
	_, userName, tableName := api.TableName(fullTableName).Split()
	sql := "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?"
	r := conn.QueryRowContext(ctx, sql, strings.ToUpper(userName), strings.ToUpper(tableName))
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
