package tql

import (
	"fmt"

	"github.com/pkg/errors"
)

type input struct {
	task *Task
	name string
	next Receiver

	chSrc DataSource
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
	case DataSource:
		ret = &input{chSrc: src}
	default:
		return nil, fmt.Errorf("%T is not applicable for INPUT", src)
	}
	ret.task = node.task
	ret.name = code
	return ret, nil
}

func (in *input) execute() error {
	if in.chSrc == nil {
		return errors.New("nil source")
	}
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
	} else {
		return errors.New("no source")
	}
}
