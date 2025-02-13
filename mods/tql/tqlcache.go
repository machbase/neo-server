package tql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

var tqlResultCache *Cache

var (
	tqlResultCacheDataSize = metric.NewCounter()
	metricCacheDataSize    = metric.NewExpVarIntGauge("machbase:tql:cache:data_size", api.MetricTimeFrames...)
	metricCacheCount       = metric.NewExpVarIntGauge("machbase:tql:cache:count", api.MetricTimeFrames...)
)

func StartCache() {
	tqlResultCache = newCache()
	tqlResultCache.closeWg.Add(1)
	go func() {
		defer tqlResultCache.closeWg.Done()
		tqlResultCache.cache.Start()
	}()

	tqlResultCache.closeWg.Add(1)
	go func() {
		defer tqlResultCache.closeWg.Done()
		for {
			select {
			case <-tqlResultCache.closeCh:
				return
			case <-time.Tick(100 * time.Millisecond):
				value := tqlResultCacheDataSize.(*metric.Counter).Value()
				metricCacheDataSize.Add(value)
				metricCacheCount.Add(int64(tqlResultCache.cache.Len()))
			}
		}
	}()
}

func StopCache() {
	if tqlResultCache != nil {
		tqlResultCache.cache.Stop()
		close(tqlResultCache.closeCh)
		tqlResultCache.closeWg.Wait()
		tqlResultCache = nil
	}
}

type Cache struct {
	cache   *ttlcache.Cache[string, *CacheData]
	closeWg sync.WaitGroup
	closeCh chan struct{}
}

type CacheData struct {
	CachedAt  time.Time
	Data      []byte
	ExpiresAt time.Time
	TTL       time.Duration
}

func newCache() *Cache {
	capacity := uint64(500)
	cache := ttlcache.New(
		ttlcache.WithCapacity[string, *CacheData](capacity),
	)
	cache.OnInsertion(func(ctx context.Context, i *ttlcache.Item[string, *CacheData]) {
		data := i.Value()
		tqlResultCacheDataSize.Add(float64(len(data.Data)))
	})
	cache.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *CacheData]) {
		data := i.Value()
		tqlResultCacheDataSize.Add(float64(len(data.Data)) * -1)
	})
	return &Cache{
		cache:   cache,
		closeCh: make(chan struct{}),
	}
}

func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	data := &CacheData{
		Data:     value,
		CachedAt: time.Now(),
	}
	c.cache.Set(key, data, ttl)
}

// Get returns cached content and its expiration time
// If the key is empty, it will return nil
func (c *Cache) Get(key string) *CacheData {
	if key == "" {
		return nil
	}
	item := c.cache.Get(key, ttlcache.WithDisableTouchOnHit[string, *CacheData]())
	if item == nil {
		// cache miss
		return nil
	}

	ret := item.Value()
	ret.ExpiresAt = item.ExpiresAt()
	ret.TTL = item.TTL()
	return ret
}

type CacheOption struct {
	key              string
	ttl              time.Duration
	preemptiveUpdate float64
}

func (co *CacheOption) String() string {
	return fmt.Sprintf("key: %s, ttl: %s, preemptiveUpdate: %f", co.key, co.ttl, co.preemptiveUpdate)
}

func (node *Node) fmCache(key string, ttlStr string) (*CacheOption, error) {
	return node.fmCache2(key, ttlStr, 0)
}

func (node *Node) fmCache2(key string, ttlStr string, preemptiveUpdate float64) (*CacheOption, error) {
	ttl := time.Minute
	key = fmt.Sprintf("%s:%s:%s", node.task.sourcePath, node.task.sourceHash, key)
	if ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err != nil || d <= time.Second {
			return nil, fmt.Errorf("invalid cache ttl %q", ttlStr)
		} else {
			ttl = d
		}
	}
	if preemptiveUpdate < 0 || preemptiveUpdate >= 1 {
		return nil, fmt.Errorf("invalid preemptive update ratio %f", preemptiveUpdate)
	}
	return &CacheOption{
		key:              key,
		ttl:              ttl,
		preemptiveUpdate: preemptiveUpdate,
	}, nil
}
