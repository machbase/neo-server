package tql

import (
	"crypto/sha1"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OutOfBedlam/metric"
	"github.com/jellydator/ttlcache/v3"
	"github.com/machbase/neo-server/v8/api"
)

var tqlResultCache *Cache

type CacheOption struct {
	MaxCapacity uint64 // max number of items
	// TODO: ttlcache.WithMaxCost has a bug that introduces deadlock
	// MaxCost     uint64 // max cost (memory consumption in Bytes) of items
}

func StartCache(cap CacheOption) {
	tqlResultCache = newCache(cap)
	tqlResultCache.closeWg.Add(1)
	go func() {
		defer tqlResultCache.closeWg.Done()
		tqlResultCache.cache.Start()
	}()

	api.AddMetricsFunc(func() (metric.Measurement, error) {
		if tqlResultCache == nil || tqlResultCache.cache == nil {
			return metric.Measurement{}, fmt.Errorf("tql cache not started")
		}
		stat := tqlResultCache.cache.Metrics()
		m := metric.Measurement{Name: "tql:cache"}
		m.AddField(
			metric.Field{Name: "evictions", Value: float64(stat.Evictions), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "insertions", Value: float64(stat.Insertions), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "hits", Value: float64(stat.Hits), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "misses", Value: float64(stat.Misses), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "items", Value: float64(tqlResultCache.cache.Len()), Type: metric.GaugeType(metric.UnitShort)},
		)
		return m, nil
	})
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
