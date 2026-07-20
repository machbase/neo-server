package spi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/util"
)

type ResultSet interface {
	Columns() api.Columns
	Err() error
	Iter(func(values []interface{}) bool)
	Message() string
}

type ResultSetBase struct {
	err error
	msg string
}

func (rs *ResultSetBase) Err() error {
	return rs.err
}

func (rs *ResultSetBase) Message() string {
	if rs.err != nil {
		return rs.err.Error()
	}
	return rs.msg
}

var serverInfoProvider func() map[string]any

func SetServerInfoProvider(provider func() map[string]any) {
	serverInfoProvider = provider
}

type ShowInfoResultSet struct {
	ResultSetBase
	keys []string
	data map[string]any
}

var _ ResultSet = (*ShowInfoResultSet)(nil)

func (si *ShowInfoResultSet) Columns() api.Columns {
	return api.Columns{
		api.MakeColumnString("NAME"),
		api.MakeColumnAny("VALUE"),
	}
}

func (si *ShowInfoResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, k := range si.keys {
		v := si.data[k]
		if !callback([]interface{}{k, v}) {
			return
		}
	}
}

func ShowInfo() *ShowInfoResultSet {
	if serverInfoProvider == nil {
		return &ShowInfoResultSet{ResultSetBase: ResultSetBase{err: errors.New("server info provider is not set")}}
	}
	serverInfo := serverInfoProvider()
	keys := make([]string, 0, len(serverInfo))
	for k := range serverInfo {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return &ShowInfoResultSet{keys: keys, data: serverInfo}
}

type LicenseResultSet struct {
	ResultSetBase
	lic *LicenseInfo
}

var _ ResultSet = (*LicenseResultSet)(nil)

func (li *LicenseResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "CUSTOMER", DataType: api.DataTypeString},
		{Name: "PROJECT", DataType: api.DataTypeString},
		{Name: "COUNTRY_CODE", DataType: api.DataTypeString},
		{Name: "INSTALL_DATE", DataType: api.DataTypeString},
		{Name: "ISSUE_DATE", DataType: api.DataTypeString},
		{Name: "STATUS", DataType: api.DataTypeString},
	}
}

func (li *LicenseResultSet) Iter(callback func(values []interface{}) bool) {
	callback([]interface{}{
		li.lic.Id, li.lic.Type, li.lic.Customer, li.lic.Project, li.lic.CountryCode,
		li.lic.InstallDate, li.lic.IssueDate, strings.ToUpper(li.lic.LicenseStatus),
	})
}

func ShowLicense(ctx context.Context, conn *sql.Conn) *LicenseResultSet {
	licenseInfo, err := GetLicenseInfo(ctx, conn)
	return &LicenseResultSet{ResultSetBase: ResultSetBase{err: err}, lic: licenseInfo}
}

var serverPortsProvider func(string) ([]*model.ServicePort, error)

func SetServerPortsProvider(provider func(string) ([]*model.ServicePort, error)) {
	serverPortsProvider = provider
}

type ShowPortsResultSet struct {
	ResultSetBase
	data []*model.ServicePort
}

var _ ResultSet = (*ShowPortsResultSet)(nil)

func (si *ShowPortsResultSet) Columns() api.Columns {
	return api.Columns{
		api.MakeColumnString("PORT"),
		api.MakeColumnString("ADDRESS"),
	}
}

func (si *ShowPortsResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, sp := range si.data {
		if !callback([]interface{}{sp.Service, sp.Address}) {
			return
		}
	}
}

func ShowPorts(portType string) *ShowPortsResultSet {
	if serverPortsProvider == nil {
		return &ShowPortsResultSet{ResultSetBase: ResultSetBase{err: errors.New("server ports provider is not set")}}
	}
	serverInfo, err := serverPortsProvider(portType)
	if err != nil {
		return &ShowPortsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowPortsResultSet{data: serverInfo}
}

type ShowUsersResultSet struct {
	ResultSetBase
	data []*UserInfo
}

type UserInfo struct {
	UserId int64  `json:"user_id"`
	Name   string `json:"name"`
}

var _ ResultSet = (*ShowUsersResultSet)(nil)

func (si *ShowUsersResultSet) Columns() api.Columns {
	return api.Columns{
		api.MakeColumnInt64("USER_ID"),
		api.MakeColumnString("NAME"),
	}
}

func (si *ShowUsersResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, u := range si.data {
		if !callback([]interface{}{u.UserId, u.Name}) {
			return
		}
	}
}

func ShowUsers(ctx context.Context, conn *sql.Conn) *ShowUsersResultSet {
	rows, err := conn.QueryContext(ctx, "SELECT USER_ID, NAME FROM M$SYS_USERS ORDER BY USER_ID")
	if err != nil {
		return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	var users []*UserInfo
	for rows.Next() {
		var u UserInfo
		if err := rows.Scan(&u.UserId, &u.Name); err != nil {
			return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowUsersResultSet{data: users}
}

type ShowTablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

var _ ResultSet = (*ShowTablesResultSet)(nil)

func (ti *ShowTablesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "DATABASE_NAME", DataType: api.DataTypeString},
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "TABLE_ID", DataType: api.DataTypeInt64},
		{Name: "TABLE_TYPE", DataType: api.DataTypeString},
		{Name: "TABLE_FLAG", DataType: api.DataTypeString},
	}
}

func (ti *ShowTablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Database, t.User, t.Name, t.Id, t.Type.ShortString(), t.Flag.String()}) {
			return
		}
	}
}

func ShowTables(ctx context.Context, conn *sql.Conn, showAll bool) *ShowTablesResultSet {
	var list = []*TableInfo{}
	var err error
	ListTablesWalk(ctx, conn, showAll, func(t *TableInfo, err error) bool {
		if err != nil {
			return false
		}
		list = append(list, t)
		return true
	})
	return &ShowTablesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTableResultSet struct {
	ResultSetBase
	Description *api.TableDescription
}

var _ ResultSet = (*ShowTableResultSet)(nil)

func (tr *ShowTableResultSet) Err() error {
	return tr.err
}

func (tr *ShowTableResultSet) Message() string {
	if tr.err != nil {
		return tr.err.Error()
	}
	return ""
}

func (tr *ShowTableResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "LENGTH", DataType: api.DataTypeInt32},
		{Name: "FLAG", DataType: api.DataTypeString},
		{Name: "INDEX", DataType: api.DataTypeString},
	}
}

func (tr *ShowTableResultSet) Iter(callback func(values []interface{}) bool) {
	for _, col := range tr.Description.Columns {
		indexes := []string{}
		for _, idxDesc := range tr.Description.Indexes {
			for _, colName := range idxDesc.Cols {
				if colName == col.Name {
					indexes = append(indexes, idxDesc.Name)
					break
				}
			}
		}
		values := []any{
			col.Name, col.Type.String(), col.Width(), col.Flag.String(), strings.Join(indexes, ","),
		}
		if !callback(values) {
			return
		}
	}
}

func ShowTable(ctx context.Context, sqlConn *sql.Conn, tableName string, all bool) *ShowTableResultSet {
	conn := WrapSqlConn(sqlConn)
	desc, err := api.DescribeTable(ctx, conn, tableName, all)
	return &ShowTableResultSet{ResultSetBase: ResultSetBase{err: err}, Description: desc}
}

type ShowMetaTablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

type MetaTableInfo struct {
	Id   int64         `json:"id"`
	Name string        `json:"name"`
	Type api.TableType `json:"type"`
}

var _ ResultSet = (*ShowMetaTablesResultSet)(nil)

func (ti *ShowMetaTablesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
	}
}

func (ti *ShowMetaTablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Id, t.Name, t.Type.ShortString()}) {
			return
		}
	}
}

func ShowMetaTables(ctx context.Context, conn *sql.Conn) *ShowMetaTablesResultSet {
	var list = []*TableInfo{}
	var err error
	rows, err := conn.QueryContext(ctx, "SELECT ID, NAME, TYPE FROM M$TABLES ORDER BY ID")
	if err != nil {
		return &ShowMetaTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	for rows.Next() {
		var t TableInfo
		if err = rows.Scan(&t.Id, &t.Name, &t.Type); err != nil {
			return &ShowMetaTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, &t)
	}
	return &ShowMetaTablesResultSet{list: list}
}

type ShowVirtualTablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

type VirtualTableInfo struct {
	Id   int64         `json:"id"`
	Name string        `json:"name"`
	Type api.TableType `json:"type"`
}

var _ ResultSet = (*ShowVirtualTablesResultSet)(nil)

func (ti *ShowVirtualTablesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
	}
}

func (ti *ShowVirtualTablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Id, t.Name, t.Type.ShortString()}) {
			return
		}
	}
}

func ShowVirtualTables(ctx context.Context, conn *sql.Conn) *ShowVirtualTablesResultSet {
	var list = []*TableInfo{}
	var err error
	rows, err := conn.QueryContext(ctx, "SELECT ID, NAME, TYPE FROM V$TABLES ORDER BY ID")
	if err != nil {
		return &ShowVirtualTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	for rows.Next() {
		var t TableInfo
		if err = rows.Scan(&t.Id, &t.Name, &t.Type); err != nil {
			return &ShowVirtualTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, &t)
	}
	return &ShowVirtualTablesResultSet{list: list}
}

type ShowSessionsResultSet struct {
	ResultSetBase
	rows [][]any
}

var _ ResultSet = (*ShowSessionsResultSet)(nil)

func (sri *ShowSessionsResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "USER_ID", DataType: api.DataTypeInt64},
		{Name: "LOGIN_TIME", DataType: api.DataTypeDatetime},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "USER_IP", DataType: api.DataTypeString},
		{Name: "MAX_QPX_MEM", DataType: api.DataTypeInt64},
	}
}

func (sri *ShowSessionsResultSet) Iter(callback func(values []interface{}) bool) {
	for _, row := range sri.rows {
		if !callback(row) {
			return
		}
	}
}

func ShowSessions(ctx context.Context, conn *sql.Conn) *ShowSessionsResultSet {
	ret := &ShowSessionsResultSet{}
	func() {
		rows, err := conn.QueryContext(ctx, "SELECT ID, USER_ID, LOGIN_TIME, CLIENT_TYPE, USER_NAME, USER_IP, MAX_QPX_MEM FROM V$SESSION")
		if err != nil {
			ret.err = err
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var userId int64
			var loginTime time.Time
			var clientType string
			var userName string
			var userIp string
			var maxQpxMem int64
			if err := rows.Scan(&id, &userId, &loginTime, &clientType, &userName, &userIp, &maxQpxMem); err != nil {
				ret.err = err
				return
			}
			row := []any{id, userName, userId, loginTime, clientType, userIp, util.HumanizeByteCount(maxQpxMem)}
			ret.rows = append(ret.rows, row)
		}
		if err := rows.Err(); err != nil {
			ret.err = err
			return
		}
	}()
	if ret.err != nil {
		return ret
	}
	func() {
		rows, err := conn.QueryContext(ctx, "SELECT ID, USER_ID, USER_NAME FROM V$NEO_SESSION")
		if err != nil {
			ret.err = err
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id int64
			var userId int64
			var userName string
			if err := rows.Scan(&id, &userId, &userName); err != nil {
				ret.err = err
				return
			}
			row := []any{id, userName, userId, nil, "neo", nil, nil}
			ret.rows = append(ret.rows, row)
		}
		if err := rows.Err(); err != nil {
			ret.err = err
			return
		}
	}()
	return ret
}

type ShowStatementsResultSet struct {
	ResultSetBase
	list []*StatementInfo
}

type StatementInfo struct {
	ID         int64  `json:"id"`
	SessionID  int64  `json:"session_id"`
	State      string `json:"state"`
	Query      string `json:"query"`
	RecordSize int64  `json:"record_size"`
}

func (si *StatementInfo) Values() []interface{} {
	var recordSize any
	recordSize = si.RecordSize
	return []interface{}{
		si.ID, si.SessionID, si.State, recordSize, si.Query,
	}
}

var _ ResultSet = (*ShowStatementsResultSet)(nil)

func (sri *ShowStatementsResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "SESSION_ID", DataType: api.DataTypeInt64},
		{Name: "STATE", DataType: api.DataTypeString},
		{Name: "RECORD_SIZE", DataType: api.DataTypeInt64},
		{Name: "QUERY", DataType: api.DataTypeString},
	}
}

func (sri *ShowStatementsResultSet) Iter(callback func(values []interface{}) bool) {
	for _, s := range sri.list {
		if !callback(s.Values()) {
			return
		}
	}
}

func ShowStatements(ctx context.Context, conn *sql.Conn) *ShowStatementsResultSet {
	stmtRows, err := conn.QueryContext(ctx, "SELECT ID, SESS_ID, STATE, RECORD_SIZE, QUERY FROM V$STMT")
	if err != nil {
		return &ShowStatementsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer stmtRows.Close()

	list := []*StatementInfo{}
	for stmtRows.Next() {
		rec := &StatementInfo{}
		err = stmtRows.Scan(&rec.ID, &rec.SessionID, &rec.State, &rec.RecordSize, &rec.Query)
		if err != nil {
			return &ShowStatementsResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	return &ShowStatementsResultSet{list: list}
}

type ShowIndexesResultSet struct {
	ResultSetBase
	list []*IndexInfo
}

type IndexInfo struct {
	Id             int64  `json:"id"`
	Database       string `json:"database"`
	DatabaseId     int64  `json:"database_id,omitempty"`
	User           string `json:"user"`
	TableName      string `json:"table_name"`
	ColumnName     string `json:"column_name"`
	IndexName      string `json:"index_name"`
	IndexType      string `json:"index_type"`
	KeyCompress    string `json:"key_compress"`
	MaxLevel       int64  `json:"max_level"`
	PartValueCount int64  `json:"part_value_count"`
	BitMapEncode   string `json:"bitmap_encode"`
}

var _ ResultSet = (*ShowIndexesResultSet)(nil)

func (ii *ShowIndexesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_TYPE", DataType: api.DataTypeString},
		{Name: "KEY_COMPRESS", DataType: api.DataTypeString},
		{Name: "MAX_LEVEL", DataType: api.DataTypeInt64},
		{Name: "PART_VALUE_COUNT", DataType: api.DataTypeInt64},
		{Name: "BITMAP_ENCODE", DataType: api.DataTypeString},
	}
}

func (ii *ShowIndexesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range ii.list {
		cont := callback([]interface{}{
			idx.Id, idx.Database, idx.User, idx.TableName, idx.ColumnName, idx.IndexName,
			idx.IndexType, idx.KeyCompress, idx.MaxLevel, idx.PartValueCount, idx.BitMapEncode,
		})
		if !cont {
			return
		}
	}
}

func ShowIndexes(ctx context.Context, conn *sql.Conn) *ShowIndexesResultSet {
	var listIndexesSql = SqlTidy(`
		SELECT
			u.name as USER_NAME,
			j.DB_NAME as DATABASE_NAME,
			j.TABLE_NAME as TABLE_NAME,
			c.name as COLUMN_NAME,
			b.name as INDEX_NAME,
			b.id as INDEX_ID,
			case b.type
				when 1 then 'BITMAP'
				when 2 then 'KEYWORD'
				when 5 then 'REDBLACK'
				when 6 then 'LSM'
				when 8 then 'REDBLACK'
				when 9 then 'KEYWORD_LSM'
				when 11 then 'TAG'
				else 'LSM' 
			end as INDEX_TYPE,
			case b.key_compress
				when 0 then 'UNCOMPRESS'
				else 'COMPRESSED'
			end as KEY_COMPRESS,
			b.max_level as MAX_LEVEL,
			b.part_value_count as PART_VALUE_COUNT,
			case b.bitmap_encode
				when 0 then 'EQUAL'
				else 'RANGE'
			end as BITMAP_ENCODE
		FROM
			m$sys_indexes b, 
			m$sys_index_columns c, 
			m$sys_users u,
			(
				select
					case a.DATABASE_ID
						when -1 then 'MACHBASEDB'
						else d.MOUNTDB
					end as DB_NAME,
					a.name as TABLE_NAME,
					a.id as TABLE_ID,
					a.USER_ID as USER_ID
				from
					M$SYS_TABLES a
				left join
					V$STORAGE_MOUNT_DATABASES d
				on
					a.DATABASE_ID = d.BACKUP_TBSID
			) as j
		WHERE
			j.TABLE_ID = b.TABLE_ID
		AND b.ID = c.INDEX_ID
		AND j.USER_ID = u.USER_ID
		ORDER BY
			j.DB_NAME, j.TABLE_NAME, b.ID
	`)
	rows, err := conn.QueryContext(ctx, listIndexesSql)
	if err != nil {
		return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*IndexInfo{}
	for rows.Next() {
		nfo := &IndexInfo{}
		err = rows.Scan(
			&nfo.User, &nfo.Database, &nfo.TableName, &nfo.ColumnName,
			&nfo.IndexName, &nfo.Id, &nfo.IndexType, &nfo.KeyCompress,
			&nfo.MaxLevel, &nfo.PartValueCount, &nfo.BitMapEncode)
		if err != nil {
			return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, nfo)
	}
	err = rows.Err()
	return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowIndexResultSet struct {
	ResultSetBase
	desc *IndexInfo
}

var _ ResultSet = (*ShowIndexResultSet)(nil)

func (qir *ShowIndexResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_TYPE", DataType: api.DataTypeString},
		{Name: "KEY_COMPRESS", DataType: api.DataTypeString},
		{Name: "MAX_LEVEL", DataType: api.DataTypeInt64},
		{Name: "PART_VALUE_COUNT", DataType: api.DataTypeInt64},
		{Name: "BITMAP_ENCODE", DataType: api.DataTypeString},
	}
}

func (qir *ShowIndexResultSet) Iter(callback func(values []interface{}) bool) {
	if qir.desc == nil {
		return
	}
	cont := callback([]interface{}{
		qir.desc.Id,
		qir.desc.TableName,
		qir.desc.ColumnName,
		qir.desc.IndexName,
		qir.desc.IndexType,
		qir.desc.KeyCompress,
		qir.desc.MaxLevel,
		qir.desc.PartValueCount,
		qir.desc.BitMapEncode,
	})
	if !cont {
		return
	}
}

func ShowIndex(ctx context.Context, conn *sql.Conn, indexName string) *ShowIndexResultSet {
	sqlText := `select 
			a.name as TABLE_NAME,
			c.name as COLUMN_NAME,
			b.name as INDEX_NAME,
			case b.type
				when 1 then 'BITMAP'
				when 2 then 'KEYWORD'
				when 5 then 'REDBLACK'
				when 6 then 'LSM'
				when 8 then 'REDBLACK'
				when 9 then 'KEYWORD_LSM'
				else 'LSM' end 
			as INDEX_TYPE,
			case b.key_compress
				when 0 then 'UNCOMPRESSED'
				else 'COMPRESSED' end 
			as KEY_COMPRESS,
			b.max_level as MAX_LEVEL,
			b.part_value_count as PART_VALUE_COUNT,
			case b.bitmap_encode
				when 0 then 'EQUAL'
				else 'RANGE' end 
			as BITMAP_ENCODE
		from
			m$sys_tables a,
			m$sys_indexes b,
			m$sys_index_columns c
		where
			a.id = b.table_id 
		and b.id = c.index_id
		and b.name = ?`
	row := conn.QueryRowContext(ctx, sqlText, strings.ToUpper(indexName))
	if err := row.Err(); err != nil {
		return &ShowIndexResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	nfo := &IndexInfo{}
	err := row.Scan(
		&nfo.TableName, &nfo.ColumnName, &nfo.IndexName, &nfo.IndexType,
		&nfo.KeyCompress, &nfo.MaxLevel, &nfo.PartValueCount, &nfo.BitMapEncode)
	return &ShowIndexResultSet{ResultSetBase: ResultSetBase{err: err}, desc: nfo}
}

type ShowStorageResultSet struct {
	ResultSetBase
	list []*StorageInfo
}

type StorageInfo struct {
	TableName string `json:"table_name"`
	DataSize  int64  `json:"data_size"`
	IndexSize int64  `json:"index_size"`
	TotalSize int64  `json:"total_size"`
}

var _ ResultSet = (*ShowStorageResultSet)(nil)

func (sui *ShowStorageResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "DATA_SIZE", DataType: api.DataTypeInt64},
		{Name: "INDEX_SIZE", DataType: api.DataTypeInt64},
		{Name: "TOTAL_SIZE", DataType: api.DataTypeInt64},
	}
}

func (sui *ShowStorageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range sui.list {
		if !callback([]interface{}{t.TableName, t.DataSize, t.IndexSize, t.TotalSize}) {
			return
		}
	}
}

func ShowStorage(ctx context.Context, conn *sql.Conn) *ShowStorageResultSet {
	sqlText := SqlTidy(`select
		a.table_name as TABLE_NAME,
		a.data_size as DATA_SIZE,
		case b.index_size 
			when b.index_size then b.index_size 
			else 0 end 
		as INDEX_SIZE,
		case a.data_size + b.index_size 
			when a.data_size + b.index_size then a.data_size + b.index_size 
			else a.data_size end 
		as TOTAL_SIZE
	from
		(select
			a.name as table_name,
			sum(b.storage_usage) as data_size
		from
			m$sys_tables a,
			v$storage_tables b
		where a.id = b.id
		group by a.name
		) as a LEFT OUTER JOIN
		(select
			a.name,
			sum(b.disk_file_size) as index_size
		from
			m$sys_tables a,
			v$storage_dc_table_indexes b
		where a.id = b.table_id
		group by a.name) as b
	on a.table_name = b.name
	order by a.table_name`)

	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*StorageInfo{}
	for rows.Next() {
		rec := &StorageInfo{}
		err = rows.Scan(&rec.TableName, &rec.DataSize, &rec.IndexSize, &rec.TotalSize)
		if err != nil {
			return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTableUsageResultSet struct {
	ResultSetBase
	list []*TableUsageInfo
}

type TableUsageInfo struct {
	DatabaseName string `json:"database_name"`
	UserName     string `json:"user_name"`
	TableName    string `json:"table_name"`
	StorageUsage int64  `json:"storage_usage"`
}

var _ ResultSet = (*ShowTableUsageResultSet)(nil)

func (tui *ShowTableUsageResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "STORAGE_USAGE", DataType: api.DataTypeInt64},
	}
}

func (tui *ShowTableUsageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range tui.list {
		if !callback([]interface{}{t.DatabaseName, t.UserName, t.TableName, t.StorageUsage}) {
			return
		}
	}
}

func ShowTableUsage(ctx context.Context, conn *sql.Conn) *ShowTableUsageResultSet {
	sqlText := SqlTidy(`
		SELECT
			j.DATABASE_NAME as DATABASE_NAME,
			u.NAME as USER_NAME,
			j.NAME as TABLE_NAME,
			s.STORAGE_USAGE
		FROM
			M$SYS_USERS u,
			V$STORAGE_TABLES s,
			(
				SELECT
					a.ID as ID,
					a.NAME as NAME,
					a.USER_ID as USER_ID,
					a.DATABASE_ID,
					case a.DATABASE_ID
						when -1 then 'MACHBASEDB'
						else d.MOUNTDB
					end as DATABASE_NAME,
					case a.DATABASE_ID
						when -1 then 'Normal'
						else 'Mounted'
					end as STATUS
				FROM
					M$SYS_TABLES a
				LEFT JOIN
					V$STORAGE_MOUNT_DATABASES d
				ON a.DATABASE_ID = d.BACKUP_TBSID
			) j
		WHERE
			u.USER_ID = j.USER_ID
		AND s.ID = j.ID
		AND s.STATUS = j.STATUS
		ORDER BY j.DATABASE_ID, u.USER_ID, s.ID
	`)

	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*TableUsageInfo{}
	for rows.Next() {
		rec := &TableUsageInfo{}
		err = rows.Scan(&rec.DatabaseName, &rec.UserName, &rec.TableName, &rec.StorageUsage)
		if err != nil {
			return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowLsmResultSet struct {
	ResultSetBase
	list []*LsmIndexInfo
}

type LsmIndexInfo struct {
	TableName string `json:"table_name"`
	IndexName string `json:"index_name"`
	Level     int64  `json:"level"`
	Count     int64  `json:"count"`
}

var _ ResultSet = (*ShowLsmResultSet)(nil)

func (li *ShowLsmResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "LEVEL", DataType: api.DataTypeInt64},
		{Name: "COUNT", DataType: api.DataTypeInt64},
	}
}

func (li *ShowLsmResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range li.list {
		cont := callback([]interface{}{
			idx.TableName, idx.IndexName, idx.Level, idx.Count,
		})
		if !cont {
			return
		}
	}
}

func ShowLsm(ctx context.Context, conn *sql.Conn) *ShowLsmResultSet {
	sqlText := `select 
		b.name as TABLE_NAME,
		c.name as INDEX_NAME,
		a.level as LEVEL,
		a.end_rid - a.begin_rid as COUNT
	from
		v$storage_dc_lsmindex_levels a,
		m$sys_tables b, m$sys_indexes c
	where
		c.id = a.index_id 
	and b.id = a.table_id
	order by 1, 2, 3`
	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()
	var list []*LsmIndexInfo
	for rows.Next() {
		rec := &LsmIndexInfo{}
		err = rows.Scan(&rec.TableName, &rec.IndexName, &rec.Level, &rec.Count)
		if err != nil {
			return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowIndexGapResultSet struct {
	ResultSetBase
	list []*IndexGapInfo
}

type IndexGapInfo struct {
	ID        int64  `json:"id"`
	TableName string `json:"table_name"`
	IndexName string `json:"index_name"`
	Gap       int64  `json:"gap"`
}

var _ ResultSet = (*ShowIndexGapResultSet)(nil)

func (igi *ShowIndexGapResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "INDEX_ID", DataType: api.DataTypeInt64},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "GAP", DataType: api.DataTypeInt64},
	}
}

func (igi *ShowIndexGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range igi.list {
		cont := callback([]interface{}{
			idx.ID, idx.TableName, idx.IndexName, idx.Gap,
		})
		if !cont {
			return
		}
	}
}

func ShowIndexGap(ctx context.Context, conn *sql.Conn) *ShowIndexGapResultSet {
	sqlText := SqlTidy(`select
		c.id,
		b.name as TABLE_NAME, 
		c.name as INDEX_NAME, 
		a.table_end_rid - a.end_rid as GAP
	from
		v$storage_dc_table_indexes a,
		m$sys_tables b,
		m$sys_indexes c
	where
		a.id = c.id 
	and c.table_id = b.id 
	order by 3 desc`)

	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*IndexGapInfo{}
	for rows.Next() {
		rec := &IndexGapInfo{}
		err = rows.Scan(&rec.ID, &rec.TableName, &rec.IndexName, &rec.Gap)
		if err != nil {
			return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type TagIndexGapResultSet struct {
	ResultSetBase
	list []*TagIndexGapInfo
}

type TagIndexGapInfo struct {
	TableId   int64  `json:"id"`
	TableName string `json:"table_name"`
	Status    string `json:"status"`
	DiskGap   int64  `json:"disk_gap"`
	MemoryGap int64  `json:"memory_gap"`
}

var _ ResultSet = (*TagIndexGapResultSet)(nil)

func (tigi *TagIndexGapResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_ID", DataType: api.DataTypeInt64},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "STATUS", DataType: api.DataTypeString},
		{Name: "DISK_GAP", DataType: api.DataTypeInt64},
		{Name: "MEMORY_GAP", DataType: api.DataTypeInt64},
	}
}

func (tigi *TagIndexGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range tigi.list {
		cont := callback([]interface{}{
			idx.TableId, idx.TableName, idx.Status, idx.DiskGap, idx.MemoryGap,
		})
		if !cont {
			return
		}
	}
}

func ShowTagIndexGap(ctx context.Context, conn *sql.Conn) *TagIndexGapResultSet {
	sqlText := SqlTidy(`SELECT
			t.ID AS ID,
            t.NAME AS TABLE_NAME,
            i.INDEX_STATE AS STATUS,
            i.TABLE_END_RID - i.DISK_INDEX_END_RID AS DISK_GAP,
            i.TABLE_END_RID - i.MEMORY_INDEX_END_RID AS MEMORY_GAP
        from
            M$SYS_TABLES t,
            V$STORAGE_TAG_INDEX i
        where
            t.ID = i.TABLE_ID
        order by id`)

	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*TagIndexGapInfo{}
	for rows.Next() {
		rec := &TagIndexGapInfo{}
		err := rows.Scan(&rec.TableId, &rec.TableName, &rec.Status, &rec.DiskGap, &rec.MemoryGap)
		if err != nil {
			return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowRollupGapResultSet struct {
	ResultSetBase
	list []*RollupGapInfo
}

type RollupGapInfo struct {
	UserName        string    `json:"user_name"`
	RollupName      string    `json:"rollup_name"`
	SrcTable        string    `json:"src_table"`
	RollupTable     string    `json:"rollup_table"`
	SrcEndRID       int64     `json:"src_end_rid"`
	RollupEndRID    int64     `json:"rollup_end_rid"`
	Gap             int64     `json:"gap"`
	RunState        string    `json:"run_state"`
	LastElapsedMsec float64   `json:"last_elapsed_msec"`
	LastWakeupTime  time.Time `json:"last_wakeup_time"`
	NextWakeupTime  time.Time `json:"next_wakeup_time"`
}

var _ ResultSet = (*ShowRollupGapResultSet)(nil)

func (rgi *ShowRollupGapResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "ROLLUP_NAME", DataType: api.DataTypeString},
		{Name: "SRC_TABLE", DataType: api.DataTypeString},
		{Name: "ROLLUP_TABLE", DataType: api.DataTypeString},
		{Name: "SRC_END_RID", DataType: api.DataTypeInt64},
		{Name: "ROLLUP_END_RID", DataType: api.DataTypeInt64},
		{Name: "GAP", DataType: api.DataTypeInt64},
		{Name: "RUN_STATE", DataType: api.DataTypeString},
		{Name: "LAST_ELAPSED_MSEC", DataType: api.DataTypeInt64},
		{Name: "LAST_WAKEUP_TIME", DataType: api.DataTypeDatetime},
		{Name: "NEXT_WAKEUP_TIME", DataType: api.DataTypeDatetime},
	}
}

func (rgi *ShowRollupGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range rgi.list {
		cont := callback([]interface{}{
			idx.UserName, idx.RollupName, idx.SrcTable, idx.RollupTable,
			idx.SrcEndRID, idx.RollupEndRID, idx.Gap, idx.RunState,
			idx.LastElapsedMsec, idx.LastWakeupTime, idx.NextWakeupTime,
		})
		if !cont {
			return
		}
	}
}

func ShowRollupGap(ctx context.Context, conn *sql.Conn) *ShowRollupGapResultSet {
	sqlText := SqlTidy(`SELECT
            U.NAME AS USER_NAME,
            C.ROLLUP_NAME AS ROLLUP_NAME,
            C.SOURCE_TABLE AS SRC_TABLE,
            C.ROLLUP_TABLE,
            B.TABLE_END_RID AS SRC_END_RID,
            C.END_RID AS ROLLUP_END_RID,
            B.TABLE_END_RID - C.END_RID AS GAP,
            CASE C.RUN_STATE WHEN 'I' THEN 'INIT' WHEN 'S' THEN 'SLEEPING' WHEN 'R' THEN 'RUNNING' ELSE 'UNKNOWN' END AS RUN_STATE,
            C.LAST_ELAPSED_MSEC AS LAST_ELAPSED_MSEC,
            C.LAST_WAKEUP_TIME AS LAST_WAKEUP_TIME,
            C.NEXT_WAKEUP_TIME AS NEXT_WAKEUP_TIME
        FROM
            M$SYS_TABLES A,
            V$STORAGE_TAG_TABLES B,
            V$ROLLUP C,
            M$SYS_USERS U
        WHERE 
            A.ID=B.ID
        AND A.DATABASE_ID=C.DATABASE_ID
        AND A.DATABASE_ID=-1
        AND A.NAME=C.SOURCE_TABLE
        AND A.USER_ID=C.USER_ID
        AND U.USER_ID=C.USER_ID
        AND B.TABLE_END_RID <> 0
        ORDER BY U.USER_ID, SRC_TABLE`)

	rows, err := conn.QueryContext(ctx, sqlText)
	if err != nil {
		return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*RollupGapInfo{}
	for rows.Next() {
		rec := &RollupGapInfo{}
		err := rows.Scan(&rec.UserName, &rec.RollupName, &rec.SrcTable, &rec.RollupTable,
			&rec.SrcEndRID, &rec.RollupEndRID, &rec.Gap, &rec.RunState,
			&rec.LastElapsedMsec, &rec.LastWakeupTime, &rec.NextWakeupTime)
		if err != nil {
			return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTagsResultSet struct {
	ResultSetBase
	conn      *sql.Conn
	tableName string
	tagNames  []string
	desc      *api.TableDescription
}

var _ ResultSet = (*ShowTagsResultSet)(nil)

func (tr *ShowTagsResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "ROW_COUNT", DataType: api.DataTypeInt64},
		{Name: "MIN_TIME", DataType: api.DataTypeDatetime},
		{Name: "MAX_TIME", DataType: api.DataTypeDatetime},
		{Name: "RECENT_ROW_TIME", DataType: api.DataTypeDatetime},
		{Name: "MIN_VALUE", DataType: api.DataTypeFloat64},
		{Name: "MIN_VALUE_TIME", DataType: api.DataTypeDatetime},
		{Name: "MAX_VALUE", DataType: api.DataTypeFloat64},
		{Name: "MAX_VALUE_TIME", DataType: api.DataTypeDatetime},
	}
}

func (tr *ShowTagsResultSet) Iter(callback func(values []interface{}) bool) {
	ctx := context.Background()
	ListTagsWalk(ctx, tr.conn, tr.tableName, tr.desc.TagNameColumn, func(tagInfo *TagInfo, err error) bool {
		if err != nil {
			return false
		}
		if len(tr.tagNames) > 0 {
			if !slices.Contains(tr.tagNames, tagInfo.Name) {
				return true // skip this tag
			}
		}
		tagInfo.Summarized = tr.desc.Summarized
		if stat, err := QueryTagStat(ctx, tr.conn, tr.tableName, tagInfo.Name); err != nil {
			// some tags may not have stat
			// the err may be 'no rows in result set'
			// ignore the error, for processing the next tag
		} else {
			tagInfo.Stat = stat
		}

		var values []any
		if tagInfo.Stat != nil {
			if tagInfo.Summarized {
				values = []any{tagInfo.Id, tagInfo.Name, tagInfo.Stat.RowCount,
					tagInfo.Stat.MinTime, tagInfo.Stat.MaxTime, tagInfo.Stat.RecentRowTime,
					tagInfo.Stat.MinValue, tagInfo.Stat.MinValueTime,
					tagInfo.Stat.MaxValue, tagInfo.Stat.MaxValueTime}
			} else {
				values = []any{tagInfo.Id, tagInfo.Name, tagInfo.Stat.RowCount,
					tagInfo.Stat.MinTime, tagInfo.Stat.MaxTime, tagInfo.Stat.RecentRowTime,
					nil, nil, nil, nil}
			}
		} else {
			values = []any{tagInfo.Id, tagInfo.Name, nil,
				nil, nil, nil,
				nil, nil, nil, nil}
		}
		if !callback(values) {
			return false
		}
		return true
	})
}

func ShowTags(ctx context.Context, conn *sql.Conn, tableName string, tagNames ...string) *ShowTagsResultSet {
	tableName = strings.ToUpper(tableName)
	rs := ShowTable(ctx, conn, tableName, false)
	if rs.err != nil {
		return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: rs.err}}
	}
	if rs.Description.Type != api.TableTypeTag {
		err := fmt.Errorf("table '%s' is not a tag table", tableName)
		return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowTagsResultSet{conn: conn, tableName: tableName, tagNames: tagNames, desc: rs.Description}
}

type TagInfo struct {
	Database   string       `json:"database"`
	User       string       `json:"user"`
	Table      string       `json:"table"`
	Name       string       `json:"name"`
	Id         int64        `json:"id"`
	Summarized bool         `json:"summarized"`
	Stat       *TagStatInfo `json:"stat,omitempty"`
}

func (ti *TagInfo) Values() []interface{} {
	return []interface{}{
		ti.Database, ti.User, ti.Table, ti.Name, ti.Id, ti.Summarized,
	}
}

type TagStatInfo struct {
	Database      string    `json:"database"`
	User          string    `json:"user"`
	Table         string    `json:"table"`
	Name          string    `json:"name"`
	RowCount      int64     `json:"row_count"`
	MinTime       time.Time `json:"min_time"`
	MaxTime       time.Time `json:"max_time"`
	MinValue      float64   `json:"min_value"`
	MinValueTime  time.Time `json:"min_value_time"`
	MaxValue      float64   `json:"max_value"`
	MaxValueTime  time.Time `json:"max_value_time"`
	RecentRowTime time.Time `json:"recent_row_time"`
}

func (tsi *TagStatInfo) Values() []interface{} {
	return []interface{}{
		tsi.Database, tsi.User, tsi.Table, tsi.Name, tsi.RowCount,
		tsi.MinTime, tsi.MaxTime, tsi.MinValue, tsi.MinValueTime,
		tsi.MaxValue, tsi.MaxValueTime, tsi.RecentRowTime,
	}
}

func ListTagsWalk(ctx context.Context, conn *sql.Conn, table string, tagNameColumn string, callback func(*TagInfo, error) bool) {
	database, userName, tableName := api.TableName(table).Split()
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT _ID, %s FROM %s.%s._%s_META`, tagNameColumn, database, userName, tableName))
	if err != nil {
		callback(nil, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		nfo := &TagInfo{Database: database, User: userName, Table: tableName}
		err = rows.Scan(&nfo.Id, &nfo.Name)
		if !callback(nfo, err) {
			return
		}
	}
}

func QueryTagStat(ctx context.Context, conn *sql.Conn, table string, tag string) (*TagStatInfo, error) {
	database, user, table := api.TableName(table).Split()
	sqlText := SqlTidy(`SELECT`,
		`NAME, ROW_COUNT,`,
		`MIN_TIME, MAX_TIME,`,
		`MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME,`,
		`RECENT_ROW_TIME`,
		`FROM`,
		fmt.Sprintf("%s.%s.V$%s_STAT", database, user, table),
		`WHERE NAME = ?`)
	nfo := &TagStatInfo{
		Database: database,
		User:     user,
		Table:    table,
	}
	row := conn.QueryRowContext(ctx, sqlText, tag)
	if err := row.Err(); err != nil {
		return nil, row.Err()
	}
	var name sql.NullString
	var rowCount sql.NullInt64
	var minTime sql.NullTime
	var maxTime sql.NullTime
	var minValue sql.NullFloat64
	var minValueTime sql.NullTime
	var maxValue sql.NullFloat64
	var maxValueTime sql.NullTime
	var recentRowTime sql.NullTime
	err := row.Scan(
		&name, &rowCount,
		&minTime, &maxTime,
		&minValue, &minValueTime, &maxValue, &maxValueTime,
		&recentRowTime)

	if name.Valid {
		nfo.Name = name.String
	}
	if rowCount.Valid {
		nfo.RowCount = rowCount.Int64
	}
	if minTime.Valid {
		nfo.MinTime = minTime.Time
	}
	if maxTime.Valid {
		nfo.MaxTime = maxTime.Time
	}
	if minValue.Valid {
		nfo.MinValue = minValue.Float64
	}
	if minValueTime.Valid {
		nfo.MinValueTime = minValueTime.Time
	}
	if maxValue.Valid {
		nfo.MaxValue = maxValue.Float64
	}
	if maxValueTime.Valid {
		nfo.MaxValueTime = maxValueTime.Time
	}
	if recentRowTime.Valid {
		nfo.RecentRowTime = recentRowTime.Time
	}
	return nfo, err
}
