package machcli

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type Item struct {
	Name  string
	count int
}

var itemSerial atomic.Int32
var itemSerialLock sync.Mutex

var itemMaxUse int = 50

func NewItem(ctx context.Context) (*Item, error) {
	itemSerialLock.Lock()
	defer itemSerialLock.Unlock()
	n := itemSerial.Add(1)
	ret := &Item{Name: fmt.Sprintf("item_%d", n-1)}
	return ret, nil
}

func (i *Item) ShouldEvict() bool {
	return i.count > itemMaxUse
}

func TestPool(t *testing.T) {
	runCount := 1000
	cap := 1
	p := NewPool(PoolConfig[*Item]{
		Capacity: cap,
		Creator:  NewItem,
		Destructor: func(i *Item) error {
			i.Name = ""
			return nil
		},
		OnGet: func(i *Item) {
			i.count++
		},
		OnPut: func(i *Item) {
		},
	})

	wg := sync.WaitGroup{}
	for i := 0; i < runCount; i++ {
		wg.Add(1)
		go func(run int) {
			defer wg.Done()
			ctx := context.TODO()
			item, _ := p.Get(ctx)
			if item == nil || item.Name == "" {
				t.Logf("[%03d] %+v", run, item)
				t.Fail()
				return
			}
			//t.Logf("[%03d] item.Name: %s", run, item.Name)
			time.Sleep(2 * time.Millisecond)
			p.Put(item)
		}(i)
	}
	wg.Wait()

	p.Drain()

	require.Equal(t, int(itemSerial.Load()), (runCount/itemMaxUse)/cap*cap)
}

func TestPoolTimeout(t *testing.T) {
	cap := 2
	p := NewPool(PoolConfig[*Item]{
		Capacity: cap,
		Creator:  NewItem,
		Destructor: func(i *Item) error {
			i.Name = ""
			return nil
		},
		OnGet: func(i *Item) {
			i.count++
		},
		OnPut: func(i *Item) {
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	o1, _ := p.Get(ctx)
	require.NotNil(t, o1)
	o2, _ := p.Get(ctx)
	require.NotNil(t, o2)

	cancel()
	o3, err := p.Get(ctx)
	require.Nil(t, o3)
	require.Error(t, err)
}
