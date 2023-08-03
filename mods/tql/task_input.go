package tql

import (
	"fmt"

	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type input struct {
	dbSrc DatabaseSource
	chSrc ChannelSource
}

func (x *Task) compileSource(code string) (*input, error) {
	expr, err := x.inputNode.Parse(code)
	if err != nil {
		return nil, err
	}
	src, err := expr.Eval(x)
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
	return ret, nil
}

func (in *input) run(deligate InputDeligate) error {
	if in.dbSrc == nil && in.chSrc == nil {
		return errors.New("nil source")
	}
	if deligate == nil {
		return errors.New("nil deligate")
	}

	fetched := 0
	executed := false
	if in.dbSrc != nil {
		queryCtx := &do.QueryContext{
			DB: deligate.Database(),
			OnFetchStart: func(c spi.Columns) {
				deligate.FeedHeader(c)
			},
			OnFetch: func(nrow int64, values []any) bool {
				fetched++
				if deligate.ShouldStop() {
					return false
				} else {
					deligate.Feed(values)
					return true
				}
			},
			OnFetchEnd: func() {},
			OnExecuted: func(usermsg string, rowsAffected int64) {
				executed = true
			},
		}
		if msg, err := do.Query(queryCtx, in.dbSrc.ToSQL()); err != nil {
			deligate.Feed(nil)
			return err
		} else {
			if executed {
				deligate.FeedHeader(spi.Columns{{Name: "message", Type: "string"}})
				deligate.Feed([]any{msg})
				deligate.Feed(nil)
			} else if fetched == 0 {
				deligate.Feed([]any{EofRecord})
				deligate.Feed(nil)
			} else {
				deligate.Feed(nil)
			}
			return nil
		}
	} else if in.chSrc != nil {
		deligate.FeedHeader(in.chSrc.Header())
		for values := range in.chSrc.Gen() {
			deligate.Feed(values)
			if deligate.ShouldStop() {
				in.chSrc.Stop()
				break
			}
		}
		deligate.Feed(nil)
		return nil
	} else {
		return errors.New("no source")
	}
}

type InputDeligate interface {
	Database() spi.Database
	ShouldStop() bool
	FeedHeader(spi.Columns)
	Feed([]any)
}

type InputDelegateWrapper struct {
	DatabaseFunc   func() spi.Database
	ShouldStopFunc func() bool
	FeedHeaderFunc func(spi.Columns)
	FeedFunc       func([]any)
}

func (w *InputDelegateWrapper) Database() spi.Database {
	if w.DatabaseFunc == nil {
		return nil
	}
	return w.DatabaseFunc()
}

func (w *InputDelegateWrapper) ShouldStop() bool {
	if w.ShouldStopFunc == nil {
		return false
	}
	return w.ShouldStopFunc()
}

func (w *InputDelegateWrapper) FeedHeader(c spi.Columns) {
	if w.FeedHeaderFunc != nil {
		w.FeedHeaderFunc(c)
	}
}

func (w *InputDelegateWrapper) Feed(v []any) {
	if w.FeedFunc != nil {
		w.FeedFunc(v)
	}
}
