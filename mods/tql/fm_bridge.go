package tql

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/machbase/neo-server/mods/bridge"
	spi "github.com/machbase/neo-spi"
)

type bridgeName struct {
	name string
}

// bridge('name')
func (x *Node) fmBridge(name string) *bridgeName {
	return &bridgeName{name: name}
}

type bridgeNode struct {
	name     string
	execType string
	command  string
	params   []any
}

// TODO not registered yet
// BRIDGE_QUERY('my-sqlite', 'select * from table where id=?', 123)
func (x *Node) fmBridgeQuery(name string, command string, params ...any) (any, error) {
	ret := &bridgeNode{name: name, command: command, params: params}
	ret.execType = "query"
	ret.gen(x)
	return nil, nil
}

func (bn *bridgeNode) gen(node *Node) {
	switch bn.execType {
	case "query":
		br, err := bridge.GetBridge(bn.name)
		if err != nil {
			ErrorRecord(err).Tell(node.next)
			return
		}
		sqlBridge, ok := br.(bridge.SqlBridge)
		if !ok {
			ErrorRecord(fmt.Errorf("bridge '%s' is not a sql type", bn.name)).Tell(node.next)
			return
		}
		bn.genBridgeQuery(node, sqlBridge)
	default: // never happen for now
	}
}

func (bn *bridgeNode) genBridgeQuery(node *Node, br bridge.SqlBridge) {
	defer func() {
		if r := recover(); r != nil {
			w := &bytes.Buffer{}
			w.Write(debug.Stack())
			node.task.LogErrorf("panic bridge '%s' %v\n%s", bn.name, r, w.String())
		}
	}()

	conn, err := br.Connect(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()
	rows, err := conn.QueryContext(node.task.ctx, bn.command, bn.params...)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer rows.Close()
	columns, err := rows.ColumnTypes()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	var cols spi.Columns = make([]*spi.Column, 0)
	cols = append(cols, &spi.Column{Name: "ROWNUM", Type: "int"})
	for _, c := range columns {
		cols = append(cols, &spi.Column{Name: c.Name(), Type: c.ScanType().String()})
	}
	node.task.SetResultColumns(cols)

	rownum := 0
	for rows.Next() && !node.task.shouldStop() {
		values := make([]any, len(columns))
		for i, c := range columns {
			scanType := c.ScanType().String()
			typeName := strings.ToUpper(c.DatabaseTypeName())
			values[i] = br.NewScanType(scanType, typeName)
			if values[i] == nil {
				node.task.LogWarnf("genBridgeQuery can not handle column '%s' type '%s/%s'", c.Name(), scanType, typeName)
				values[i] = new(string)
			}
		}
		if err := rows.Scan(values...); err == nil {
			rownum++
			values = br.NormalizeType(values)
			NewRecord(rownum, values).Tell(node.next)
		} else {
			ErrorRecord(err).Tell(node.next)
		}
	}
}
