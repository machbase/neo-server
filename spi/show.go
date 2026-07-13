package spi

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/model"
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
