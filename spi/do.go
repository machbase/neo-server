package spi

import (
	"context"
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

type TablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

var _ ResultSet = (*TablesResultSet)(nil)

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

func QueryTables(ctx context.Context, conn api.Conn, showAll bool) *TablesResultSet {
	var list = []*TableInfo{}
	var err error
	ListTablesWalk(ctx, conn, showAll, func(t *TableInfo) bool {
		if err = t.Err(); err != nil {
			return false
		}
		list = append(list, t)
		return true
	})
	return &TablesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type TableResultSet struct {
	ResultSetBase
	desc *api.TableDescription
}

var _ ResultSet = (*TableResultSet)(nil)

func (tr *TableResultSet) Err() error {
	return tr.err
}

func (tr *TableResultSet) Message() string {
	if tr.err != nil {
		return tr.err.Error()
	}
	return ""
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

func QueryTable(ctx context.Context, conn api.Conn, tableName string, all bool) *TableResultSet {
	desc, err := api.DescribeTable(ctx, conn, tableName, all)
	return &TableResultSet{ResultSetBase: ResultSetBase{err: err}, desc: desc}
}

type IndexesResultSet struct {
	ResultSetBase
	list []*IndexInfo
}

var _ ResultSet = (*IndexesResultSet)(nil)

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

func QueryIndexes(ctx context.Context, conn api.Conn) *IndexesResultSet {
	list, err := ListIndexes(ctx, conn)
	return &IndexesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type QueryIndexResultSet struct {
	ResultSetBase
	desc *IndexInfo
}

var _ ResultSet = (*QueryIndexResultSet)(nil)

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

func QueryIndex(ctx context.Context, conn api.Conn, indexName string) *QueryIndexResultSet {
	idx, err := DescribeIndex(ctx, conn, indexName)
	return &QueryIndexResultSet{ResultSetBase: ResultSetBase{err: err}, desc: idx}
}

type LsmIndexesResultSet struct {
	ResultSetBase
	list []*LsmIndexInfo
}

var _ ResultSet = (*LsmIndexesResultSet)(nil)

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

func QueryLsmIndexes(ctx context.Context, conn api.Conn) *LsmIndexesResultSet {
	list, err := ListLsmIndexesInfo(ctx, conn)
	return &LsmIndexesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type TagsResultSet struct {
	ResultSetBase
	conn      api.Conn
	tableName string
	tagNames  []string
	desc      *api.TableDescription
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
		return &TagsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	if desc.Type != api.TableTypeTag {
		err := fmt.Errorf("f(SQL) table %q is not a tag table", tableName)
		return &TagsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &TagsResultSet{ResultSetBase: ResultSetBase{err: nil}, conn: conn, tableName: tableName, tagNames: tagNames, desc: desc}
}

type IndexGapResultSet struct {
	ResultSetBase
	list []*IndexGapInfo
}

var _ ResultSet = (*IndexGapResultSet)(nil)

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

func QueryIndexGap(ctx context.Context, conn api.Conn) *IndexGapResultSet {
	list, err := ListIndexGap(ctx, conn)
	return &IndexGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type TagIndexGapResultSet struct {
	ResultSetBase
	list []*IndexGapInfo
}

var _ ResultSet = (*TagIndexGapResultSet)(nil)

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

func QueryTagIndexGap(ctx context.Context, conn api.Conn) *TagIndexGapResultSet {
	list, err := ListTagIndexGap(ctx, conn)
	return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type RollupGapResultSet struct {
	ResultSetBase
	list []*RollupGapInfo
}

var _ ResultSet = (*RollupGapResultSet)(nil)

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

func QueryRollupGap(ctx context.Context, conn api.Conn) *RollupGapResultSet {
	list, err := ListRollupGap(ctx, conn)
	return &RollupGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type StorageResultSet struct {
	ResultSetBase
	list []*StorageInfo
}

var _ ResultSet = (*StorageResultSet)(nil)

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

func QueryStorage(ctx context.Context, conn api.Conn) *StorageResultSet {
	list, err := ListStorage(ctx, conn)
	return &StorageResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type TableUsageResultSet struct {
	ResultSetBase
	list []*TableUsageInfo
}

var _ ResultSet = (*TableUsageResultSet)(nil)

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

func QueryTableUsage(ctx context.Context, conn api.Conn) *TableUsageResultSet {
	list, err := ListTableUsage(ctx, conn)
	return &TableUsageResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type StatementsResultSet struct {
	ResultSetBase
	list []*StatementInfo
}

var _ ResultSet = (*StatementsResultSet)(nil)

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

func QueryStatements(ctx context.Context, conn api.Conn) *StatementsResultSet {
	list, err := ListStatements(ctx, conn)
	return &StatementsResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type SessionsResultSet struct {
	ResultSetBase
	list []*SessionInfo
}

var _ ResultSet = (*SessionsResultSet)(nil)

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

func QuerySessions(ctx context.Context, conn api.Conn) *SessionsResultSet {
	list, err := ListSessions(ctx, conn)
	return &SessionsResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}
