package tql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

var tqlResultCache *Cache

func StartCache() {
	tqlResultCache = newEncoderCache()
	tqlResultCache.wg.Add(1)
	go func() {
		defer tqlResultCache.wg.Done()
		tqlResultCache.cache.Start()
	}()
}

func StopCache() {
	if tqlResultCache != nil {
		tqlResultCache.cache.Stop()
		tqlResultCache.wg.Wait()
	}
}

type Cache struct {
	cache   *ttlcache.Cache[string, []byte]
	wg      sync.WaitGroup
	closeCh chan struct{}
}

func newEncoderCache() *Cache {
	cache := ttlcache.New[string, []byte](
		ttlcache.WithCapacity[string, []byte](10),
	)
	cache.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, []byte]) {
		fmt.Println("cache evict", i.Key())
	})
	cache.OnInsertion(func(ctx context.Context, i *ttlcache.Item[string, []byte]) {
		fmt.Println("cache insert", i.Key())
	})
	return &Cache{
		cache:   cache,
		closeCh: make(chan struct{}),
	}
}

func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	c.cache.Set(key, value, ttl)
}

func (c *Cache) Get(key string) ([]byte, bool) {
	if key == "" {
		// when tql sink has `cache` option, but is is not call from .tql file
		// e.g. it called from tql-editor of web ui
		return nil, false
	}
	item := c.cache.Get(key, ttlcache.WithDisableTouchOnHit[string, []byte]())
	if item == nil {
		fmt.Println("cache miss", key)
		return nil, false
	}
	fmt.Println("cache hit", key)
	return item.Value(), true
}

type CacheOption struct {
	key string
	ttl time.Duration
}

func (co *CacheOption) String() string {
	return fmt.Sprintf("key: %s, ttl: %s", co.key, co.ttl)
}

func (node *Node) fmCache(key string, ttlStr string) (*CacheOption, error) {
	ttl := time.Minute
	if node.task.sourcePath == "" {
		// only .tql file source can cache the result output
		// if the sourcePath is empty, it means the task is not from .tql file
		key = ""
	} else {
		if key == "" {
			key = node.task.sourcePath
		} else {
			key = node.task.sourcePath + "#" + key
		}
		if ttlStr != "" {
			if d, err := time.ParseDuration(ttlStr); err != nil || d <= time.Second {
				return nil, fmt.Errorf("invalid cache ttl %q", ttlStr)
			} else {
				ttl = d
			}
		}
	}
	return &CacheOption{
		key: key,
		ttl: ttl,
	}, nil
}
