package tql

import (
	"fmt"

	"github.com/pkg/errors"
)

type input struct {
	task *Task
	next Receiver

	chSrc   DataSource
	dataSrc []*Record
}

func (node *Node) compileSource(code string) (*input, error) {
	expr, err := node.Parse(code)
	if err != nil {
		return nil, err
	}
	node.name = expr.String()
	src, err := expr.Eval(node)
	if err != nil {
		return nil, err
	}
	var ret *input
	switch src := src.(type) {
	case DataSource:
		ret = &input{chSrc: src}
	case []*Record:
		ret = &input{dataSrc: src}
	case *Record:
		ret = &input{dataSrc: []*Record{src}}
	default:
		return nil, fmt.Errorf("type (%T) is not applicable for INPUT", src)
	}
	ret.task = node.task
	return ret, nil
}

func (in *input) execute() error {
	shouldStopNow := false
	if in.chSrc != nil {
		for rec := range in.chSrc.Gen() {
			if rec.IsEOF() || rec.IsCircuitBreak() {
				break
			}
			if rec.IsError() {
				rec.Tell(in.next)
				break
			}
			rec.Tell(in.next)
			if shouldStopNow {
				in.chSrc.stop()
				break
			}
		}
		EofRecord.Tell(in.next)
		return nil
	} else if in.dataSrc != nil {
		for _, rec := range in.dataSrc {
			if rec.IsEOF() || rec.IsCircuitBreak() {
				break
			}
			if rec.IsError() {
				rec.Tell(in.next)
				break
			}
			rec.Tell(in.next)
			if shouldStopNow {
				break
			}
		}
		EofRecord.Tell(in.next)
		return nil
	} else {
		return errors.New("no source")
	}
}
