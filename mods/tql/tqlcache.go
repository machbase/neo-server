package tql

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

var tqlResultCache *Cache

var (
	tqlResultCacheDataSize = metric.NewCounter()
	metricCacheDataSize    = metric.NewExpVarIntGauge("machbase:tql:cache:data_size", api.MetricTimeFrames...)
	tqlResultCacheCount    = metric.NewCounter()
	metricCacheCount       = metric.NewExpVarIntGauge("machbase:tql:cache:count", api.MetricTimeFrames...)
	metricInsertions       = metric.NewExpVarIntCounter("machbase:tql:cache:insertions", api.MetricTimeFrames...)
	metricEvictions        = metric.NewExpVarIntCounter("machbase:tql:cache:evictions", api.MetricTimeFrames...)
	metricHits             = metric.NewExpVarIntCounter("machbase:tql:cache:hits", api.MetricTimeFrames...)
	metricMisses           = metric.NewExpVarIntCounter("machbase:tql:cache:misses", api.MetricTimeFrames...)
)

type CacheOption struct {
	MaxCapacity uint64 // max number of items
	// TODO: ttlcache.WithMaxCost has a bug that introduces deadlock
	// MaxCost     uint64 // max cost (memory consumptio in Bytes) of items
}

func StartCache(cap CacheOption) {
	tqlResultCache = newCache(cap)
	tqlResultCache.closeWg.Add(1)
	go func() {
		defer tqlResultCache.closeWg.Done()
		tqlResultCache.cache.Start()
	}()

	tqlResultCache.closeWg.Add(1)
	go func() {
		defer tqlResultCache.closeWg.Done()
		var prevMet = tqlResultCache.cache.Metrics()
		for {
			select {
			case <-tqlResultCache.closeCh:
				return
			case <-time.Tick(api.MetricMeasurePeriod):
				value := tqlResultCacheDataSize.(*metric.Counter).Value()
				metricCacheDataSize.Add(value)
				value = int64(tqlResultCache.cache.Len())
				metricCacheCount.Add(value)

				met := tqlResultCache.cache.Metrics()
				metricInsertions.Add(int64(met.Insertions - prevMet.Insertions))
				metricEvictions.Add(int64(met.Evictions - prevMet.Evictions))
				metricHits.Add(int64(met.Hits - prevMet.Hits))
				metricMisses.Add(int64(met.Misses - prevMet.Misses))
				prevMet = met
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
	Data      []byte
	ExpiresAt time.Time
	TTL       time.Duration
	updates   atomic.Int32
}

func newCache(cap CacheOption) *Cache {
	cache := ttlcache.New(
		ttlcache.WithCapacity[string, *CacheData](cap.MaxCapacity),
		//
		// TODO: ttlcache.WithMaxCost has a bug that introduces deadlock
		//
		// ttlcache.WithMaxCost[string, *CacheData](cap.MaxCost, func(item *ttlcache.Item[string, *CacheData]) uint64 {
		// 	return uint64(len(item.Value().Data))
		// }),
	)
	cache.OnInsertion(func(ctx context.Context, i *ttlcache.Item[string, *CacheData]) {
		data := i.Value()
		tqlResultCacheDataSize.Add(float64(len(data.Data)))
		tqlResultCacheCount.Add(1)
	})
	cache.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *CacheData]) {
		data := i.Value()
		tqlResultCacheDataSize.Add(float64(len(data.Data)) * -1)
		tqlResultCacheCount.Add(-1)
	})
	return &Cache{
		cache:   cache,
		closeCh: make(chan struct{}),
	}
}

func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	data := &CacheData{
		Data: value,
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

type CacheParam struct {
	key              string
	ttl              time.Duration
	preemptiveUpdate float64
}

func (co *CacheParam) String() string {
	return fmt.Sprintf("key: %s, ttl: %s, preemptiveUpdate: %f", co.key, co.ttl, co.preemptiveUpdate)
}

func (node *Node) fmCache(key string, ttlStr string, extra ...float64) (*CacheParam, error) {
	preemptiveUpdateRatio := 0.0
	if len(extra) > 0 {
		preemptiveUpdateRatio = extra[0]
	}
	return node.fmCachePreUpdate(key, ttlStr, preemptiveUpdateRatio)
}

func (node *Node) fmCachePreUpdate(key string, ttlStr string, preemptiveUpdate float64) (*CacheParam, error) {
	ttl := time.Minute
	if len(key) > 40 {
		// make a long key to shorten one via sha1 hash
		h := sha1.New()
		h.Write([]byte(key))
		key = fmt.Sprintf("%x", h.Sum(nil))
	}
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
	return &CacheParam{
		key:              key,
		ttl:              ttl,
		preemptiveUpdate: preemptiveUpdate,
	}, nil
}
