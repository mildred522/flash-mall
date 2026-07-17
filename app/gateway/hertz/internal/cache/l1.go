package cache

import (
	"strings"
	"sync"
	"time"
)

type l1Entry struct {
	value      []byte
	expiresAt  time.Time
	lastAccess time.Time
}

type localCache struct {
	mu         sync.Mutex
	entries    map[string]l1Entry
	maxEntries int
}

func newLocalCache(maxEntries int) *localCache {
	if maxEntries <= 0 {
		maxEntries = 512
	}
	return &localCache{entries: make(map[string]l1Entry), maxEntries: maxEntries}
}

func (c *localCache) get(key string, now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.After(now) {
		delete(c.entries, key)
		return nil, false
	}
	entry.lastAccess = now
	c.entries[key] = entry
	return cloneBytes(entry.value), true
}

func (c *localCache) set(key string, value []byte, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
		c.evictOldestLocked()
	}
	now := time.Now()
	c.entries[key] = l1Entry{value: cloneBytes(value), expiresAt: expiresAt, lastAccess: now}
}

func (c *localCache) delete(keys ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.entries, key)
	}
}

func (c *localCache) deletePrefix(prefixes ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				delete(c.entries, key)
				break
			}
		}
	}
}

func (c *localCache) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.lastAccess.Before(oldest) {
			oldestKey = key
			oldest = entry.lastAccess
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	copyValue := make([]byte, len(value))
	copy(copyValue, value)
	return copyValue
}
