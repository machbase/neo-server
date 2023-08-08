package tql

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"time"

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
	var cols spi.Columns = make([]*spi.Column, len(columns))
	for i, c := range columns {
		cols[i] = &spi.Column{Name: c.Name(), Type: c.ScanType().String()}
	}
	node.task.SetResultColumns(cols)

	for rows.Next() && !node.task.shouldStop() {
		values := make([]any, len(columns))
		for i, c := range columns {
			switch c.ScanType().String() {
			case "sql.NullBool":
				values[i] = new(bool)
			case "sql.NullByte":
				values[i] = new(uint8)
			case "sql.NullFloat64":
				values[i] = new(float64)
			case "sql.NullInt16":
				values[i] = new(int16)
			case "sql.NullInt32":
				values[i] = new(int32)
			case "sql.NullInt64":
				values[i] = new(int64)
			case "sql.NullString":
				values[i] = new(string)
			case "sql.NullTime":
				values[i] = new(time.Time)
			case "sql.RawBytes":
				values[i] = new([]byte)
			default:
				node.task.LogWarnf("genBridgeQuery can not handle column '%s' type '%s'", c.Name(), c.ScanType().String())
				values[i] = new(string)
			}
		}
		rows.Scan(values...)
		NewRecord(values[0], values[1:]).Tell(node.next)
	}
}
