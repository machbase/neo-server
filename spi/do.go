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
