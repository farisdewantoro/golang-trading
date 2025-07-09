package cache

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	once          sync.Once
	inmemoryCache Cache
)

type Cache interface {
	Set(key string, value interface{}, duration time.Duration)
	Get(key string) (interface{}, bool)
	Delete(key string)
	Flush()
}

type goCache struct {
	internal *cache.Cache
}

// NewCache returns a new Cache instance with default expiration and cleanup interval
func NewCache(defaultExpiration, cleanupInterval time.Duration) Cache {
	once.Do(func() {
		inmemoryCache = &goCache{
			internal: cache.New(defaultExpiration, cleanupInterval),
		}
	})
	return inmemoryCache
}

func GetInMemoryCache() Cache {
	return inmemoryCache
}

func (c *goCache) Set(key string, value interface{}, duration time.Duration) {
	c.internal.Set(key, value, duration)
}

func (c *goCache) Get(key string) (interface{}, bool) {
	return c.internal.Get(key)
}

func (c *goCache) Delete(key string) {
	c.internal.Delete(key)
}

func (c *goCache) Flush() {
	c.internal.Flush()
}

func GetFromCache[T any](key string) (T, bool) {
	val, found := inmemoryCache.Get(key)
	if !found {
		var zero T
		return zero, false
	}
	typedVal, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return typedVal, true
}
