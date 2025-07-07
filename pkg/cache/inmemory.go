package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
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
	return &goCache{
		internal: cache.New(defaultExpiration, cleanupInterval),
	}
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
