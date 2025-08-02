package cache

import (
	"net/http"
	"sync"
	"time"
)

type CacheEntry struct {
	StatusCode  int
	Header      http.Header
	Body        []byte
	StoredAt    time.Time
	StallPeriod time.Duration // Optional stall period for the entry
}

type Cache struct {
	rw      sync.RWMutex
	entries map[string]*CacheEntry
}

func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
	}
}

func (c *Cache) Get(key string) (*CacheEntry, bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	entry, found := c.entries[key]
	return entry, found
}
func (c *Cache) Set(key string, entry *CacheEntry) {
	c.rw.Lock()
	defer c.rw.Unlock()
	c.entries[key] = entry
}
func (c *Cache) Delete(key string) {
	c.rw.Lock()
	defer c.rw.Unlock()
	delete(c.entries, key)
}
