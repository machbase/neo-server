package spi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/machbase/neo-client/api"
)

type ResultSet interface {
	Columns() api.Columns
	Err() error
	Iter(func(values []interface{}) bool)
	Message() string
}

var serverInfoProvider func() map[string]any

func SetDefaultServerInfo(provider func() map[string]any) {
	serverInfoProvider = provider
}

type ServerInfoResultSet struct {
	keys []string
	data map[string]any
	err  error
}

var _ ResultSet = (*ServerInfoResultSet)(nil)

func (si *ServerInfoResultSet) Err() error {
	return si.err
}

func (si *ServerInfoResultSet) Message() string {
	if si.err != nil {
		return si.err.Error()
	}
	return ""
}

func (si *ServerInfoResultSet) Columns() api.Columns {
	return api.Columns{
		api.MakeColumnString("NAME"),
		api.MakeColumnAny("VALUE"),
	}
}

func (si *ServerInfoResultSet) Iter(callback func(values []interface{}) bool) {
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

func QueryServerInfo() *ServerInfoResultSet {
	if serverInfoProvider == nil {
		return &ServerInfoResultSet{err: errors.New("server info provider is not set")}
	}
	serverInfo := serverInfoProvider()
	keys := make([]string, 0, len(serverInfo))
	for k := range serverInfo {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return &ServerInfoResultSet{keys: keys, data: serverInfo}
}

type TablesResultSet struct {
	list []*TableInfo
	err  error
}

var _ ResultSet = (*TablesResultSet)(nil)

func (ti *TablesResultSet) Err() error {
	return ti.err
}

func (ti *TablesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "FLAG", DataType: api.DataTypeString},
	}
}

func (ti *TablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Database, t.User, t.Name, t.Id, t.Type.ShortString(), t.Flag.String()}) {
			return
		}
	}
}

func (ti *TablesResultSet) Message() string {
	if ti.err != nil {
		return ti.err.Error()
	}
	return ""
}

func QueryTables(ctx context.Context, conn api.Conn, showAll bool) *TablesResultSet {
	list, err := ListTables(ctx, conn, showAll)
	return &TablesResultSet{list: list, err: err}
}

type TableResultSet struct {
	desc *api.TableDescription
	err  error
}

var _ ResultSet = (*TableResultSet)(nil)

func (tr *TableResultSet) Err() error {
	return tr.err
}

func (tr *TableResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "LENGTH", DataType: api.DataTypeInt32},
		{Name: "FLAG", DataType: api.DataTypeString},
		{Name: "INDEX", DataType: api.DataTypeString},
	}
}

func (tr *TableResultSet) Iter(callback func(values []interface{}) bool) {
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

func (tr *TableResultSet) Message() string {
	if tr.err != nil {
		return tr.err.Error()
	}
	return ""
}

func QueryTable(ctx context.Context, conn api.Conn, tableName string, all bool) *TableResultSet {
	desc, err := api.DescribeTable(ctx, conn, tableName, all)
	return &TableResultSet{desc: desc, err: err}
}

type IndexesResultSet struct {
	list []*IndexInfo
	err  error
}

var _ ResultSet = (*IndexesResultSet)(nil)

func (ii *IndexesResultSet) Err() error {
	return ii.err
}

func (ii *IndexesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "COLUMN_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_TYPE", DataType: api.DataTypeString},
		{Name: "KEY_COMPRESS", DataType: api.DataTypeString},
		{Name: "MAX_LEVEL", DataType: api.DataTypeInt64},
		{Name: "PART_VALUE_COUNT", DataType: api.DataTypeInt64},
		{Name: "BITMAP_ENCODE", DataType: api.DataTypeString},
	}
}

func (ii *IndexesResultSet) Iter(callback func(values []interface{}) bool) {
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

func (ii *IndexesResultSet) Message() string {
	if ii.err != nil {
		return ii.err.Error()
	}
	return ""
}

func QueryIndexes(ctx context.Context, conn api.Conn) *IndexesResultSet {
	list, err := ListIndexes(ctx, conn)
	return &IndexesResultSet{list: list, err: err}
}

type QueryIndexResultSet struct {
	desc *IndexInfo
	err  error
}

var _ ResultSet = (*QueryIndexResultSet)(nil)

func (qir *QueryIndexResultSet) Err() error {
	return qir.err
}

func (qir *QueryIndexResultSet) Columns() api.Columns {
	return api.Columns{
		api.MakeColumnString("TABLE_NAME"),
		api.MakeColumnString("COLUMN_NAME"),
		api.MakeColumnString("INDEX_NAME"),
		api.MakeColumnString("INDEX_TYPE"),
		api.MakeColumnString("KEY_COMPRESS"),
		api.MakeColumnInt64("MAX_LEVEL"),
		api.MakeColumnInt64("PART_VALUE_COUNT"),
		api.MakeColumnString("BITMAP_ENCODE"),
	}
}

func (qir *QueryIndexResultSet) Iter(callback func(values []interface{}) bool) {
	if qir.desc == nil {
		return
	}
	cont := callback([]interface{}{
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

func (qir *QueryIndexResultSet) Message() string {
	if qir.err != nil {
		return qir.err.Error()
	}
	return ""
}

func QueryIndex(ctx context.Context, conn api.Conn, indexName string) *QueryIndexResultSet {
	idx, err := DescribeIndex(ctx, conn, indexName)
	return &QueryIndexResultSet{desc: idx, err: err}
}

type LsmIndexesResultSet struct {
	list []*LsmIndexInfo
	err  error
}

var _ ResultSet = (*LsmIndexesResultSet)(nil)

func (li *LsmIndexesResultSet) Err() error {
	return li.err
}

func (li *LsmIndexesResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "LEVEL", DataType: api.DataTypeInt64},
		{Name: "COUNT", DataType: api.DataTypeInt64},
	}
}

func (li *LsmIndexesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range li.list {
		cont := callback([]interface{}{
			idx.TableName, idx.IndexName, idx.Level, idx.Count,
		})
		if !cont {
			return
		}
	}
}

func (li *LsmIndexesResultSet) Message() string {
	if li.err != nil {
		return li.err.Error()
	}
	return ""
}

func QueryLsmIndexes(ctx context.Context, conn api.Conn) *LsmIndexesResultSet {
	list, err := ListLsmIndexesInfo(ctx, conn)
	return &LsmIndexesResultSet{list: list, err: err}
}

type LicenseResultSet struct {
	err error
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

func (li *LicenseResultSet) Err() error {
	return li.err
}

func (li *LicenseResultSet) Message() string {
	return ""
}

func (li *LicenseResultSet) Iter(callback func(values []interface{}) bool) {
	callback([]interface{}{
		li.lic.Id, li.lic.Type, li.lic.Customer, li.lic.Project, li.lic.CountryCode,
		li.lic.InstallDate, li.lic.IssueDate, li.lic.LicenseStatus,
	})
}

func QueryLicense(ctx context.Context, conn api.Conn) *LicenseResultSet {
	licenseInfo, err := GetLicenseInfo(ctx, conn)
	return &LicenseResultSet{lic: licenseInfo, err: err}
}

type TagsResultSet struct {
	conn      api.Conn
	tableName string
	tagNames  []string
	desc      *api.TableDescription
	err       error
}

var _ ResultSet = (*TagsResultSet)(nil)

func (tr *TagsResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "_ID", DataType: api.DataTypeInt64},
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

func (tr *TagsResultSet) Err() error {
	return tr.err
}

func (tr *TagsResultSet) Message() string {
	return ""
}

func (tr *TagsResultSet) Iter(callback func(values []interface{}) bool) {
	ctx := context.Background()
	ListTagsWalk(ctx, tr.conn, tr.tableName, tr.desc.TagNameColumn, func(tagInfo *TagInfo) bool {
		if tagInfo.Err != nil {
			return false
		}
		if len(tr.tagNames) > 0 {
			if !slices.Contains(tr.tagNames, tagInfo.Name) {
				return true // skip this tag
			}
		}
		tagInfo.Summarized = tr.desc.Summarized
		if stat, err := TagStat(ctx, tr.conn, tr.tableName, tagInfo.Name); err != nil {
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

func QueryTags(ctx context.Context, conn api.Conn, tableName string, tagNames ...string) *TagsResultSet {
	tableName = strings.ToUpper(tableName)
	desc, err := api.DescribeTable(ctx, conn, tableName, false)
	if err != nil {
		return &TagsResultSet{err: err}
	}
	if desc.Type != api.TableTypeTag {
		err := fmt.Errorf("f(SQL) table %q is not a tag table", tableName)
		return &TagsResultSet{err: err}
	}
	return &TagsResultSet{conn: conn, tableName: tableName, tagNames: tagNames, desc: desc, err: nil}
}

type IndexGapResultSet struct {
	list []*IndexGapInfo
	err  error
}

var _ ResultSet = (*IndexGapResultSet)(nil)

func (igi *IndexGapResultSet) Err() error {
	return igi.err
}

func (igi *IndexGapResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "INDEX", DataType: api.DataTypeString},
		{Name: "GAP", DataType: api.DataTypeInt64},
	}
}

func (igi *IndexGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range igi.list {
		cont := callback([]interface{}{
			idx.ID, idx.TableName, idx.IndexName, idx.Gap,
		})
		if !cont {
			return
		}
	}
}

func (igi *IndexGapResultSet) Message() string {
	if igi.err != nil {
		return igi.err.Error()
	}
	return ""
}

func QueryIndexGap(ctx context.Context, conn api.Conn) *IndexGapResultSet {
	list, err := ListIndexGap(ctx, conn)
	return &IndexGapResultSet{list: list, err: err}
}

type TagIndexGapResultSet struct {
	list []*IndexGapInfo
	err  error
}

var _ ResultSet = (*TagIndexGapResultSet)(nil)

func (tigi *TagIndexGapResultSet) Err() error {
	return tigi.err
}

func (tigi *TagIndexGapResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "STATUS", DataType: api.DataTypeString},
		{Name: "DISK_GAP", DataType: api.DataTypeInt64},
		{Name: "MEMORY_GAP", DataType: api.DataTypeInt64},
	}
}

func (tigi *TagIndexGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range tigi.list {
		cont := callback([]interface{}{
			idx.ID, idx.Status, idx.DiskGap, idx.MemoryGap,
		})
		if !cont {
			return
		}
	}
}

func (tigi *TagIndexGapResultSet) Message() string {
	if tigi.err != nil {
		return tigi.err.Error()
	}
	return ""
}

func QueryTagIndexGap(ctx context.Context, conn api.Conn) *TagIndexGapResultSet {
	list, err := ListTagIndexGap(ctx, conn)
	return &TagIndexGapResultSet{list: list, err: err}
}

type RollupGapResultSet struct {
	list []*RollupGapInfo
	err  error
}

var _ ResultSet = (*RollupGapResultSet)(nil)

func (rgi *RollupGapResultSet) Err() error {
	return rgi.err
}

func (rgi *RollupGapResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "SRC_TABLE", DataType: api.DataTypeString},
		{Name: "ROLLUP_TABLE", DataType: api.DataTypeString},
		{Name: "SRC_END_RID", DataType: api.DataTypeInt64},
		{Name: "ROLLUP_END_RID", DataType: api.DataTypeInt64},
		{Name: "GAP", DataType: api.DataTypeInt64},
		{Name: "LAST_TIME", DataType: api.DataTypeInt64},
	}
}

func (rgi *RollupGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range rgi.list {
		cont := callback([]interface{}{
			idx.SrcTable, idx.RollupTable, idx.SrcEndRID, idx.RollupEndRID, idx.Gap, idx.LastElapsed,
		})
		if !cont {
			return
		}
	}
}

func (rgi *RollupGapResultSet) Message() string {
	if rgi.err != nil {
		return rgi.err.Error()
	}
	return ""
}

func QueryRollupGap(ctx context.Context, conn api.Conn) *RollupGapResultSet {
	list, err := ListRollupGap(ctx, conn)
	return &RollupGapResultSet{list: list, err: err}
}

type StorageResultSet struct {
	list []*StorageInfo
	err  error
}

var _ ResultSet = (*StorageResultSet)(nil)

func (sui *StorageResultSet) Err() error {
	return sui.err
}

func (sui *StorageResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "DATA_SIZE", DataType: api.DataTypeInt64},
		{Name: "INDEX_SIZE", DataType: api.DataTypeInt64},
		{Name: "TOTAL_SIZE", DataType: api.DataTypeInt64},
	}
}

func (sui *StorageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range sui.list {
		if !callback([]interface{}{t.TableName, t.DataSize, t.IndexSize, t.TotalSize}) {
			return
		}
	}
}

func (sui *StorageResultSet) Message() string {
	if sui.err != nil {
		return sui.err.Error()
	}
	return ""
}

func QueryStorage(ctx context.Context, conn api.Conn) *StorageResultSet {
	list, err := ListStorage(ctx, conn)
	return &StorageResultSet{list: list, err: err}
}

type TableUsageResultSet struct {
	list []*TableUsageInfo
	err  error
}

var _ ResultSet = (*TableUsageResultSet)(nil)

func (tui *TableUsageResultSet) Err() error {
	return tui.err
}

func (tui *TableUsageResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "STORAGE_USAGE", DataType: api.DataTypeInt64},
	}
}

func (tui *TableUsageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range tui.list {
		if !callback([]interface{}{t.TableName, t.StorageUsage}) {
			return
		}
	}
}

func (tui *TableUsageResultSet) Message() string {
	if tui.err != nil {
		return tui.err.Error()
	}
	return ""
}

func QueryTableUsage(ctx context.Context, conn api.Conn) *TableUsageResultSet {
	list, err := ListTableUsage(ctx, conn)
	return &TableUsageResultSet{list: list, err: err}
}

type StatementsResultSet struct {
	list []*StatementInfo
	err  error
}

var _ ResultSet = (*StatementsResultSet)(nil)

func (sri *StatementsResultSet) Err() error {
	return sri.err
}

func (sri *StatementsResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "SESSION_ID", DataType: api.DataTypeInt64},
		{Name: "STATE", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "RECORD_SIZE", DataType: api.DataTypeInt64},
		{Name: "APPEND_SUCCESS_CNT", DataType: api.DataTypeInt64},
		{Name: "APPEND_FAILURE_CNT", DataType: api.DataTypeInt64},
		{Name: "QUERY", DataType: api.DataTypeString},
	}
}

func (sri *StatementsResultSet) Iter(callback func(values []interface{}) bool) {
	for _, s := range sri.list {
		if !callback(s.Values()) {
			return
		}
	}
}

func (sri *StatementsResultSet) Message() string {
	if sri.err != nil {
		return sri.err.Error()
	}
	return ""
}

func QueryStatements(ctx context.Context, conn api.Conn) *StatementsResultSet {
	list, err := ListStatements(ctx, conn)
	return &StatementsResultSet{list: list, err: err}
}

type SessionsResultSet struct {
	list []*SessionInfo
	err  error
}

var _ ResultSet = (*SessionsResultSet)(nil)

func (sri *SessionsResultSet) Err() error {
	return sri.err
}

func (sri *SessionsResultSet) Columns() api.Columns {
	return api.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "USER_ID", DataType: api.DataTypeInt64},
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "LOGIN_TIME", DataType: api.DataTypeDatetime},
		{Name: "MAX_QPX_MEM", DataType: api.DataTypeInt64},
		{Name: "STMT_COUNT", DataType: api.DataTypeInt64},
	}
}

func (sri *SessionsResultSet) Iter(callback func(values []interface{}) bool) {
	for _, s := range sri.list {
		if !callback(s.Values()) {
			return
		}
	}
}

func (sri *SessionsResultSet) Message() string {
	if sri.err != nil {
		return sri.err.Error()
	}
	return ""
}

func QuerySessions(ctx context.Context, conn api.Conn) *SessionsResultSet {
	list, err := ListSessions(ctx, conn)
	return &SessionsResultSet{list: list, err: err}
}
