package tql

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"sync"
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

	alive     bool
	closeWait sync.WaitGroup

	task *Task
}

// TODO not registered yet
// BRIDGE_QUERY('my-sqlite', 'select * from table where id=?', 123)
func (x *Node) fmBridgeQuery(name string, command string, params ...any) *bridgeNode {
	ret := &bridgeNode{name: name, command: command, params: params}
	ret.execType = "query"
	ret.task = x.task
	return ret
}

func (bn *bridgeNode) Header() spi.Columns {
	return nil
}

func (bn *bridgeNode) Gen() <-chan *Record {
	switch bn.execType {
	case "query":
		return bn.genQuery()
	default: // never happen for now
		return nil
	}
}

func (bn *bridgeNode) genQuery() <-chan *Record {
	ch := make(chan *Record)
	bn.alive = true
	bn.closeWait.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				bn.task.LogErrorf("panic bridge '%s' %v\n%s", bn.name, r, w.String())
			}
		}()

		if br, err := bridge.GetBridge(bn.name); err != nil {
			ch <- ErrorRecord(err)
		} else {
			sqlC, ok := br.(bridge.SqlBridge)
			if !ok {
				ch <- ErrorRecord(fmt.Errorf("bridge '%s' is not a sql type", bn.name))
				goto done
			}
			conn, err := sqlC.Connect(bn.task.ctx)
			if err != nil {
				ch <- ErrorRecord(err)
				goto done
			}
			defer conn.Close()
			rows, err := conn.QueryContext(bn.task.ctx, bn.command, bn.params...)
			if err != nil {
				ch <- ErrorRecord(err)
				goto done
			}
			defer rows.Close()

			columns, err := rows.ColumnTypes()
			if err != nil {
				ch <- ErrorRecord(err)
				goto done
			}

			var cols spi.Columns = make([]*spi.Column, len(columns))
			for i, c := range columns {
				cols[i] = &spi.Column{Name: c.Name(), Type: c.ScanType().String()}
			}
			bn.task.SetResultColumns(cols)

			for rows.Next() && bn.alive {
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
					default:
						// fmt.Println("---", i, c.Name(), c.ScanType().String())
						values[i] = new(string)
					}
				}
				rows.Scan(values...)
				ch <- NewRecord(values[0], values[1:])
			}
		}
	done:
		close(ch)
		bn.closeWait.Done()
	}()

	return ch
}

func (bn *bridgeNode) Stop() {
	bn.alive = false
	bn.closeWait.Wait()
}
