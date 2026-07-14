package spi

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/util"
)

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

func ShowLicense(ctx context.Context, conn api.Conn) *LicenseResultSet {
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

func ShowUsers(ctx context.Context, conn api.Conn) *ShowUsersResultSet {
	rows, err := conn.Query(ctx, "SELECT USER_ID, NAME FROM M$SYS_USERS ORDER BY USER_ID")
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

func ShowTables(ctx context.Context, conn api.Conn, showAll bool) *ShowTablesResultSet {
	var list = []*TableInfo{}
	var err error
	ListTablesWalk(ctx, conn, showAll, func(t *TableInfo) bool {
		if err = t.Err(); err != nil {
			return false
		}
		list = append(list, t)
		return true
	})
	return &ShowTablesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTableResultSet struct {
	ResultSetBase
	desc *api.TableDescription
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
	for _, col := range tr.desc.Columns {
		indexes := []string{}
		for _, idxDesc := range tr.desc.Indexes {
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

func ShowTable(ctx context.Context, conn api.Conn, tableName string, all bool) *ShowTableResultSet {
	desc, err := api.DescribeTable(ctx, conn, tableName, all)
	return &ShowTableResultSet{ResultSetBase: ResultSetBase{err: err}, desc: desc}
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

func ShowMetaTables(ctx context.Context, conn api.Conn) *ShowMetaTablesResultSet {
	var list = []*TableInfo{}
	var err error
	rows, err := conn.Query(ctx, "SELECT ID, NAME, TYPE FROM M$TABLES ORDER BY ID")
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

func ShowVirtualTables(ctx context.Context, conn api.Conn) *ShowVirtualTablesResultSet {
	var list = []*TableInfo{}
	var err error
	rows, err := conn.Query(ctx, "SELECT ID, NAME, TYPE FROM V$TABLES ORDER BY ID")
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

func ShowSessions(ctx context.Context, conn api.Conn) *ShowSessionsResultSet {
	ret := &ShowSessionsResultSet{}
	func() {
		rows, err := conn.Query(ctx, "SELECT ID, USER_ID, LOGIN_TIME, CLIENT_TYPE, USER_NAME, USER_IP, MAX_QPX_MEM FROM V$SESSION")
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
		rows, err := conn.Query(ctx, "SELECT ID, USER_ID, USER_NAME FROM V$NEO_SESSION")
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

func ShowStatements(ctx context.Context, conn api.Conn) *ShowStatementsResultSet {
	stmtRows, err := conn.Query(ctx, "SELECT ID, SESS_ID, STATE, RECORD_SIZE, QUERY FROM V$STMT")
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

func ShowIndexes(ctx context.Context, conn api.Conn) *ShowIndexesResultSet {
	list, err := ListIndexes(ctx, conn)
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

func ShowIndex(ctx context.Context, conn api.Conn, indexName string) *ShowIndexResultSet {
	idx, err := DescribeIndex(ctx, conn, indexName)
	return &ShowIndexResultSet{ResultSetBase: ResultSetBase{err: err}, desc: idx}
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

func ShowStorage(ctx context.Context, conn api.Conn) *ShowStorageResultSet {
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

	rows, err := conn.Query(ctx, sqlText)
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
	TableName    string `json:"table_name"`
	StorageUsage int64  `json:"storage_usage"`
}

var _ ResultSet = (*ShowTableUsageResultSet)(nil)

func (tui *ShowTableUsageResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "STORAGE_USAGE", DataType: api.DataTypeInt64},
	}
}

func (tui *ShowTableUsageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range tui.list {
		if !callback([]interface{}{t.TableName, t.StorageUsage}) {
			return
		}
	}
}

func ShowTableUsage(ctx context.Context, conn api.Conn) *ShowTableUsageResultSet {
	sqlText := SqlTidy(`SELECT
		a.NAME as TABLE_NAME,
		t.STORAGE_USAGE as STORAGE_USAGE
	FROM
		M$SYS_TABLES a,
		M$SYS_USERS u,
		V$STORAGE_TABLES t
	WHERE
		a.user_id = u.user_id
	AND t.ID = a.id
	ORDER BY a.NAME`)

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*TableUsageInfo{}
	for rows.Next() {
		rec := &TableUsageInfo{}
		err = rows.Scan(&rec.TableName, &rec.StorageUsage)
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

func ShowLsm(ctx context.Context, conn api.Conn) *ShowLsmResultSet {
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
	rows, err := conn.Query(ctx, sqlText)
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
