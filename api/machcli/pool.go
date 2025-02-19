package machcli

import (
	"context"
	"sync"
)

type PoolItem interface {
	ShouldEvict() bool
}

type Pool[T any] struct {
	PoolConfig[T]
	mu     sync.Mutex
	items  []T
	openCh chan struct{}
}

type PoolConfig[T any] struct {
	Capacity   int                              // default 1
	Creator    func(context.Context) (T, error) // mandatory creator
	Destructor func(T) error                    // mandatory destructor
	OnGet      func(T)                          // optional hook
	OnPut      func(T)                          // optional hook
}

func NewPool[T any](conf PoolConfig[T]) *Pool[T] {
	if conf.Capacity <= 0 {
		conf.Capacity = 1
	}
	openChan := make(chan struct{}, conf.Capacity)
	for i := 0; i < conf.Capacity; i++ {
		openChan <- struct{}{}
	}
	return &Pool[T]{
		PoolConfig: conf,
		items:      make([]T, 0, conf.Capacity),
		openCh:     openChan,
	}
}

func (p *Pool[T]) Remains() int {
	return len(p.openCh)
}

func (p *Pool[T]) Get(ctx context.Context) (T, error) {
	var ret T
	select {
	case <-ctx.Done():
		return ret, ctx.Err()
	case <-p.openCh:
	}

	p.mu.Lock()
	defer func() {
		p.mu.Unlock()
		if p.OnGet != nil {
			p.OnGet(ret)
		}
	}()
	if len(p.items) == 0 {
		if obj, err := p.Creator(ctx); err != nil {
			return obj, err
		} else {
			ret = obj
		}
	} else {
		ret = p.items[len(p.items)-1]
		p.items = p.items[:len(p.items)-1]
	}
	return ret, nil
}

func (p *Pool[T]) Put(item T) error {
	if p.OnPut != nil {
		p.OnPut(item)
	}

	p.mu.Lock()
	defer func() {
		p.openCh <- struct{}{}
		p.mu.Unlock()
	}()

	var anyItem any
	anyItem = item

	if len(p.items) >= p.Capacity {
		if err := p.Destructor(item); err != nil {
			return err
		}
	} else if evicted, ok := anyItem.(PoolItem); ok && evicted.ShouldEvict() {
		if err := p.Destructor(item); err != nil {
			return err
		}
	} else {
		p.items = append(p.items, item)
	}
	return nil
}

func (p *Pool[T]) Drain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, item := range p.items {
		p.Destructor(item)
	}
	p.items = nil
}
