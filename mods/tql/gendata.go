package tql

import (
	"context"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/api"
)

type DataGen interface {
	gen(*Node)
}

var _ DataGen = (*databaseSource)(nil)
var _ DataGen = (*DataGenDescTable)(nil)
var _ DataGen = (*DataGenShowTags)(nil)

type databaseSource struct {
	task    *Task
	sqlText string
	params  []any

	resultMsg string
}

func (dc *databaseSource) gen(node *Node) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := dc.task.ConnDatabase(ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()

	query := &api.Query{
		Begin: func(q *api.Query) {
			cols := q.Columns()
			cols = append([]*api.Column{api.MakeColumnRownum()}, cols...)
			dc.task.SetResultColumns(cols)
		},
		Next: func(q *api.Query, nrow int64) bool {
			if dc.task.shouldStop() {
				return false
			}
			values, err := q.Columns().MakeBuffer()
			if err != nil {
				ErrorRecord(err).Tell(node.next)
				return false
			}
			if err = q.Scan(values...); err != nil {
				ErrorRecord(err).Tell(node.next)
				return false
			}
			if len(values) > 0 {
				NewRecord(nrow, values).Tell(node.next)
			}
			return !dc.task.shouldStop()
		},
		End: func(q *api.Query) {
			dc.resultMsg = q.UserMessage()
			if !q.IsFetch() {
				dc.task.SetResultColumns(api.Columns{
					api.MakeColumnRownum(),
					api.MakeColumnString("MESSAGE"),
				})
				NewRecord(1, q.UserMessage()).Tell(node.next)
			}
		},
	}
	if err := query.Execute(ctx, conn, dc.sqlText, dc.params...); err != nil {
		dc.resultMsg = err.Error()
		ErrorRecord(err).Tell(node.next)
	}
}

type DataGenDescTable struct {
	table   string
	showAll bool
}

func (dt *DataGenDescTable) gen(node *Node) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := node.task.ConnDatabase(ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()
	desc, err := api.DescribeTable(ctx, conn, dt.table, dt.showAll)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	node.task.SetResultColumns(api.Columns{
		api.MakeColumnRownum(),
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "LENGTH", DataType: api.DataTypeInt32},
		{Name: "FLAG", DataType: api.DataTypeString},
		{Name: "INDEX", DataType: api.DataTypeString},
	})
	for i, col := range desc.Columns {
		indexes := []string{}
		for _, idxDesc := range desc.Indexes {
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
		NewRecord(i+1, values).Tell(node.next)
	}
}

type DataGenShowTags struct {
	table string
}

func (dt *DataGenShowTags) gen(node *Node) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := node.task.ConnDatabase(ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()

	tableType, err := api.QueryTableType(ctx, conn, dt.table)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	if tableType != api.TableTypeTag {
		ErrorRecord(fmt.Errorf("'%s' is not a tag table", dt.table)).Tell(node.next)
		return
	}

	desc, err := api.DescribeTable(ctx, conn, dt.table, false)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	summarized := false
	for _, c := range desc.Columns {
		if c.Flag&api.ColumnFlagSummarized > 0 {
			summarized = true
			break
		}
	}

	if summarized {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
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
		})
	} else {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "_ID", DataType: api.DataTypeInt64},
			{Name: "NAME", DataType: api.DataTypeString},
			{Name: "ROW_COUNT", DataType: api.DataTypeInt64},
			{Name: "MIN_TIME", DataType: api.DataTypeDatetime},
			{Name: "MAX_TIME", DataType: api.DataTypeDatetime},
			{Name: "RECENT_ROW_TIME", DataType: api.DataTypeDatetime},
		})
	}

	rownum := 0
	api.ListTagsWalk(ctx, conn, dt.table, func(tag *api.TagInfo, err error) bool {
		if err != nil {
			ErrorRecord(err).Tell(node.next)
			return false
		}
		rownum++
		var values []any
		if summarized {
			stat, err := api.TagStat(ctx, conn, dt.table, tag.Name)
			if err != nil {
				ErrorRecord(err).Tell(node.next)
				return false
			}
			values = []any{tag.Id, tag.Name, stat.RowCount,
				stat.MinTime, stat.MaxTime, stat.RecentRowTime,
				stat.MinValue, stat.MinValueTime,
				stat.MaxValue, stat.MaxValueTime}
		} else {
			stat, err := api.TagStat(ctx, conn, dt.table, tag.Name)
			if err != nil {
				// tag exists in _table_meta, but not found in v$table_stat
				values = []any{tag.Id, tag.Name, nil, nil, nil, nil}
			} else {
				values = []any{tag.Id, tag.Name, stat.RowCount,
					stat.MinTime, stat.MaxTime, stat.RecentRowTime}
			}
		}
		NewRecord(rownum, values).Tell(node.next)
		return true
	})
}

func parseDataGenCommands(str string, x *Node, params []any) (DataGen, bool) {
	str = strings.TrimSuffix(strings.TrimSpace(str), ";")
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return nil, false
	}
	var showAll bool
	var args []string
	switch strings.ToLower(fields[0]) {
	case "show":
		args = append(args, "show")
	case "desc":
		args = append(args, "desc")
	default:
		return nil, false
	}
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "-a", "--all":
			showAll = true
		default:
			if strings.HasPrefix(fields[i], "-") {
				continue
			}
			args = append(args, fields[i])
		}
	}
	switch args[0] {
	case "show":
		if len(args) == 2 && args[1] == "tables" {
			return &databaseSource{task: x.task, sqlText: api.ListTablesSql(showAll, true), params: params}, true
		}
		if len(args) == 2 && args[1] == "indexes" {
			return &databaseSource{task: x.task, sqlText: api.ListIndexesSql(), params: params}, true
		}
		if len(args) == 3 && args[1] == "tags" {
			return &DataGenShowTags{table: args[2]}, true
		}
	case "desc":
		if len(args) == 2 {
			return &DataGenDescTable{table: args[1], showAll: showAll}, true
		}
	}
	return nil, false
}
