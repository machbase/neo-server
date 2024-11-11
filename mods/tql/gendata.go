package tql

import (
	"context"
	"strings"

	"github.com/machbase/neo-server/api"
)

type DataGen interface {
	gen(*Node)
}

var _ DataGen = (*databaseSource)(nil)
var _ DataGen = (*DataGenDescTable)(nil)

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
			return &databaseSource{task: x.task, sqlText: api.ListTagsSql(args[2]), params: params}, true
		}
	case "desc":
		if len(args) == 2 {
			return &DataGenDescTable{table: args[1], showAll: showAll}, true
		}
	}
	return nil, false
}
