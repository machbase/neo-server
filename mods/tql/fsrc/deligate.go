package fsrc

import spi "github.com/machbase/neo-spi"

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
