package machnet

import (
	"io"
	"sync/atomic"
)

type ioByteCounter struct {
	enabled      atomic.Bool
	readBytes    atomic.Uint64
	writtenBytes atomic.Uint64
}

func newIOByteCounter(enabled bool) *ioByteCounter {
	c := &ioByteCounter{}
	c.enabled.Store(enabled)
	return c
}

func (c *ioByteCounter) setEnabled(enabled bool) {
	if c == nil {
		return
	}
	c.enabled.Store(enabled)
}

func (c *ioByteCounter) isEnabled() bool {
	if c == nil {
		return false
	}
	return c.enabled.Load()
}

func (c *ioByteCounter) snapshot() (readBytes uint64, writtenBytes uint64) {
	if c == nil {
		return 0, 0
	}
	return c.readBytes.Load(), c.writtenBytes.Load()
}

func (c *ioByteCounter) reset() {
	if c == nil {
		return
	}
	c.readBytes.Store(0)
	c.writtenBytes.Store(0)
}

type countingReader struct {
	r       io.Reader
	counter *ioByteCounter
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 && r.counter != nil && r.counter.isEnabled() {
		r.counter.readBytes.Add(uint64(n))
	}
	return n, err
}

type countingWriter struct {
	w       io.Writer
	counter *ioByteCounter
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 && w.counter != nil && w.counter.isEnabled() {
		w.counter.writtenBytes.Add(uint64(n))
	}
	return n, err
}
