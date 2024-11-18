package tql

import (
	"bytes"
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/connector/mssql"
	"github.com/machbase/neo-server/api/connector/mysql"
	"github.com/machbase/neo-server/api/connector/postgres"
	"github.com/machbase/neo-server/api/connector/sqlite"
	"github.com/machbase/neo-server/mods/bridge"
)

type DataGen interface {
	gen(*Node)
}

var _ DataGen = (*DataGenMachbase)(nil)
var _ DataGen = (*DataGenDescTable)(nil)
var _ DataGen = (*DataGenShowTags)(nil)
var _ DataGen = (*DataGenExplain)(nil)
var _ DataGen = (*DataGenBridge)(nil)

type DataGenMachbase struct {
	task    *Task
	sqlText string
	params  []any

	resultMsg string
}

func (dc *DataGenMachbase) gen(node *Node) {
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

type DataGenExplain struct {
	sqlText string
	full    bool
}

func (dt *DataGenExplain) gen(node *Node) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := node.task.ConnDatabase(ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}

	plan, err := conn.Explain(ctx, dt.sqlText, dt.full)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	node.task.SetResultColumns(api.Columns{
		api.MakeColumnString("PLAN"),
	})
	for _, line := range strings.Split(plan, "\n") {
		NewRecord(1, []any{line}).Tell(node.next)
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

func parseDataGenCommands(x *Node, str string, params []any) (DataGen, bool) {
	str = strings.TrimSuffix(strings.TrimSpace(str), ";")
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return nil, false
	}
	var showAll bool
	var args []string
	var sqlText string
	var explainFull bool
	switch strings.ToLower(fields[0]) {
	case "show":
		args = append(args, "show")
		if len(fields) > 2 && (fields[1] == "-a" || fields[1] == "--all") {
			showAll = true
			args = append(args, fields[2:]...)
		} else {
			args = append(args, fields[1:]...)
		}
	case "desc":
		args = append(args, "desc")
		if len(fields) > 2 && (fields[1] == "-a" || fields[1] == "--all") {
			showAll = true
			args = append(args, fields[2:]...)
		} else {
			args = append(args, fields[1:]...)
		}
	case "explain":
		args = append(args, "explain")
		if len(fields) > 2 && (fields[1] == "-f" || fields[1] == "--full") {
			explainFull = true
			sqlText = strings.Join(fields[2:], " ")
		} else if len(fields) > 1 {
			sqlText = strings.Join(fields[1:], " ")
		}
	default:
		return nil, false
	}
	switch args[0] {
	case "show":
		if len(args) == 2 && strings.ToLower(args[1]) == "tables" {
			return &DataGenMachbase{task: x.task, sqlText: api.ListTablesSql(showAll, true), params: params}, true
		}
		if len(args) == 2 && strings.ToLower(args[1]) == "indexes" {
			return &DataGenMachbase{task: x.task, sqlText: api.ListIndexesSql(), params: params}, true
		}
		if len(args) == 3 && strings.ToLower(args[1]) == "tags" {
			return &DataGenShowTags{table: args[2]}, true
		}
	case "desc":
		if len(args) == 2 {
			return &DataGenDescTable{table: args[1], showAll: showAll}, true
		}
	case "explain":
		if len(sqlText) > 0 {
			return &DataGenExplain{sqlText: sqlText, full: explainFull}, true
		}
	}
	return nil, false
}

type DataGenBridge struct {
	task      *Task
	name      string
	sqlText   string
	params    []any
	resultMsg string
}

func (dc *DataGenBridge) gen(node *Node) {
	defer func() {
		if r := recover(); r != nil {
			w := &bytes.Buffer{}
			w.Write(debug.Stack())
			node.task.LogErrorf("panic bridge '%s' %v\n%s", dc.name, r, w.String())
		}
	}()
	db, err := NewConnector(dc.name)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	conn, err := db.Connect(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()

	if api.DetectSQLStatementType(dc.sqlText).IsFetch() {
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
		if err := query.Execute(node.task.ctx, conn, dc.sqlText, dc.params...); err != nil {
			dc.resultMsg = err.Error()
			ErrorRecord(err).Tell(node.next)
		}
	} else {
		result := conn.Exec(node.task.ctx, dc.sqlText, dc.params...)
		if err := result.Err(); err != nil {
			dc.resultMsg = err.Error()
			ErrorRecord(err).Tell(node.next)
			return
		}
		dc.resultMsg = result.Message()
		dc.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			api.MakeColumnString("MESSAGE"),
		})
		NewRecord(1, dc.resultMsg).Tell(node.next)
	}
}

func NewConnector(name string) (api.Database, error) {
	var db api.Database
	var bridgeName string
	var bridgeType string
	var path string
	if toks := strings.SplitN(name, ",", 2); len(toks) == 2 {
		bridgeType = toks[0]
		path = toks[1]
	} else {
		bridgeName = name
	}

	if bridgeName != "" {
		br, err := bridge.GetBridge(bridgeName)
		if err != nil {
			return nil, err
		}
		sqlBridge, ok := br.(bridge.SqlBridge)
		if !ok {
			return nil, fmt.Errorf("bridge '%s' is not a sql type", bridgeName)
		}
		switch sqlBridge.Type() {
		case "sqlite":
			db = sqlite.New(sqlBridge.DB())
			bridgeType = "sqlite"
		case "mssql":
			db = mssql.New(sqlBridge.DB())
			bridgeType = "mssql"
		case "postgres":
			db = postgres.New(sqlBridge.DB())
			bridgeType = "postgres"
		case "mysql":
			db = mysql.New(sqlBridge.DB())
			bridgeType = "mysql"
		default:
			return nil, fmt.Errorf("bridge '%s' is not supported", sqlBridge.Type())
		}
	} else {
		switch bridgeType {
		case "sqlite", "sqlite3":
			if d, err := sqlite.NewWithDSN(path); err != nil {
				return nil, fmt.Errorf("fail to create sqlite, %w", err)
			} else {
				db = d
			}
		case "mssql":
			if d, err := mssql.NewWithDSN(path); err != nil {
				return nil, fmt.Errorf("fail to create mssql, %w", err)
			} else {
				db = d
			}
		case "postgres", "pgsql", "postgresql":
			if d, err := postgres.NewWithDSN(path); err != nil {
				return nil, fmt.Errorf("fail to create postgres, %w", err)
			} else {
				db = d
			}
		case "mysql":
			if d, err := mysql.NewWithDSN(path); err != nil {
				return nil, fmt.Errorf("fail to create mysql, %w", err)
			} else {
				db = d
			}
		default:
			return nil, fmt.Errorf("unsupported bridge type '%s'", bridgeType)
		}
	}
	return db, nil
}
