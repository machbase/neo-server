package tql

import (
	"fmt"

	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type input struct {
	selfNode *Node
	next     *Node

	dbSrc DatabaseSource
	chSrc ChannelSource
}

func (node *Node) compileSource(code string) (*input, error) {
	expr, err := node.Parse(code)
	if err != nil {
		return nil, err
	}
	src, err := expr.Eval(node)
	if err != nil {
		return nil, err
	}
	var ret *input
	switch src := src.(type) {
	case DatabaseSource:
		ret = &input{dbSrc: src}
	case ChannelSource:
		ret = &input{chSrc: src}
	default:
		return nil, fmt.Errorf("%T is not applicable for INPUT", src)
	}
	ret.selfNode = node
	return ret, nil
}

func (in *input) run() error {
	if in.dbSrc == nil && in.chSrc == nil {
		return errors.New("nil source")
	}
	if in.dbSrc != nil {
		fetched := 0
		executed := false
		queryCtx := &do.QueryContext{
			DB: in.next.task.db,
			OnFetchStart: func(c spi.Columns) {
				in.selfNode.task.resultColumns = c
			},
			OnFetch: func(nrow int64, values []any) bool {
				fetched++
				if in.selfNode.task.shouldStopNodes() {
					return false
				} else {
					in.selfNode.task.feedNodes(values)
					return true
				}
			},
			OnFetchEnd: func() {},
			OnExecuted: func(usermsg string, rowsAffected int64) {
				executed = true
			},
		}
		if msg, err := do.Query(queryCtx, in.dbSrc.ToSQL()); err != nil {
			in.selfNode.task.feedNodes(nil)
			return err
		} else {
			if executed {
				in.selfNode.task.resultColumns = spi.Columns{{Name: "message", Type: "string"}}
				in.selfNode.task.feedNodes([]any{msg})
				in.selfNode.task.feedNodes(nil)
			} else if fetched == 0 {
				in.selfNode.task.feedNodes([]any{EofRecord})
				in.selfNode.task.feedNodes(nil)
			} else {
				in.selfNode.task.feedNodes(nil)
			}
			return nil
		}
	} else if in.chSrc != nil {
		in.selfNode.task.resultColumns = in.chSrc.Header()
		for values := range in.chSrc.Gen() {
			in.selfNode.task.feedNodes(values)
			if in.selfNode.task.shouldStopNodes() {
				in.chSrc.Stop()
				break
			}
		}
		in.selfNode.task.feedNodes(nil)
		return nil
	} else {
		return errors.New("no source")
	}
}
