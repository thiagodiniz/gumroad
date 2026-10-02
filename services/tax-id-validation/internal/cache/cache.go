// Package cache is a process-local TTL cache standing in for Rails.cache in the ported
// validators. Each replica keeps its own copy; the 10-minute TTL only exists to absorb
// duplicate lookups within a single checkout, so sharing it across pods is not needed.
package cache

import (
	"sync"
	"time"
)

type entry struct {
	value     bool
	expiresAt time.Time
}

type Cache struct {
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	entries map[string]entry
}

func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, now: time.Now, entries: make(map[string]entry)}
}

func (c *Cache) Get(key string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return false, false
	}
	if c.now().After(e.expiresAt) {
		delete(c.entries, key)
		return false, false
	}
	return e.value, true
}

func (c *Cache) Set(key string, value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry{value: value, expiresAt: c.now().Add(c.ttl)}
}

// Len is for tests and the readiness handler.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Sweep drops expired entries so a long-running pod does not grow without bound; the
// server runs it on a ticker.
func (c *Cache) Sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
}
