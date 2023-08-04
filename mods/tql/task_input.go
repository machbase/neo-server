package tql

import (
	"fmt"

	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type input struct {
	task *Task
	name string
	next Receiver
	db   spi.Database

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
	ret.task = node.task
	ret.name = code
	return ret, nil
}

func (in *input) start() error {
	if in.dbSrc == nil && in.chSrc == nil {
		return errors.New("nil source")
	}
	shouldStopNow := false
	if in.dbSrc != nil {
		fetched := 0
		executed := false
		queryCtx := &do.QueryContext{
			DB: in.db,
			OnFetchStart: func(c spi.Columns) {
				in.task.output.resultColumns = c
			},
			OnFetch: func(nrow int64, values []any) bool {
				fetched++
				if shouldStopNow {
					return false
				} else {
					NewRecord(fetched, values).Tell(in.next)
					return true
				}
			},
			OnFetchEnd: func() {},
			OnExecuted: func(usermsg string, rowsAffected int64) {
				executed = true
			},
		}
		if msg, err := do.Query(queryCtx, in.dbSrc.ToSQL()); err != nil {
			ErrorRecord(err).Tell(in.next)
			return err
		} else {
			if executed {
				in.task.output.resultColumns = spi.Columns{{Name: "message", Type: "string"}}
				NewRecord(msg, "").Tell(in.next)
				EofRecord.Tell(in.next)
			} else {
				EofRecord.Tell(in.next)
			}
			return nil
		}
	} else if in.chSrc != nil {
		in.task.output.resultColumns = in.chSrc.Header()
		for values := range in.chSrc.Gen() {
			if len(values) == 0 {
				continue
			}
			NewRecord(values[0], values[1:]).Tell(in.next)
			if shouldStopNow {
				in.chSrc.Stop()
				break
			}
		}
		EofRecord.Tell(in.next)
		return nil
	} else {
		return errors.New("no source")
	}
}
