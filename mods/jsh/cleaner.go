package jsh

import (
	"io"
	"slices"
	"sync"
)

type Cleaner struct {
	cleanups     []int64
	cleanupTable map[int64]func(io.Writer)
	cleanupSeq   int64
	cleanupMutex sync.Mutex
}

func (c *Cleaner) AddCleanup(f func(io.Writer)) int64 {
	c.cleanupMutex.Lock()
	defer c.cleanupMutex.Unlock()

	c.cleanupSeq++
	c.cleanups = append(c.cleanups, c.cleanupSeq)
	if c.cleanupTable == nil {
		c.cleanupTable = make(map[int64]func(io.Writer))
	}
	c.cleanupTable[c.cleanupSeq] = f
	return c.cleanupSeq
}

func (c *Cleaner) RemoveCleanup(tok int64) {
	c.cleanupMutex.Lock()
	defer c.cleanupMutex.Unlock()

	for i, t := range c.cleanups {
		if t == tok {
			c.cleanups = append(c.cleanups[:i], c.cleanups[i+1:]...)
			delete(c.cleanupTable, tok)
			break
		}
	}
}

func (c *Cleaner) RunCleanup(w io.Writer) {
	c.cleanupMutex.Lock()
	defer c.cleanupMutex.Unlock()

	slices.Reverse(c.cleanups)
	for _, tok := range c.cleanups {
		if f, ok := c.cleanupTable[tok]; ok {
			f(w)
		}
	}
	c.cleanups = nil
	c.cleanupTable = make(map[int64]func(io.Writer))
}
