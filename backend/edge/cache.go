package edge

import (
	"net/http"
	"sync"
	"time"
	"veer/config"
)

type CacheEntry struct {
	StatusCode  int
	ContentType string
	Headers     http.Header
	Body        []byte
	CachedAt    time.Time
	ExpiresAt   time.Time
	HitCount    int64
}

func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

type MemoryCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	maxBytes int64
	curBytes int64
	ttl      time.Duration
	stopChan chan struct{}
}

func NewMemoryCache(cfg config.EdgeCacheConfig) *MemoryCache {
	ttl := time.Duration(cfg.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 300 * time.Second
	}
	maxMB := cfg.MaxSizeMB
	if maxMB <= 0 {
		maxMB = 512
	}

	c := &MemoryCache{
		entries:  make(map[string]*CacheEntry),
		maxBytes: int64(maxMB) * 1024 * 1024,
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}

	go c.cleanupLoop()
	return c
}

func (c *MemoryCache) Stop() {
	close(c.stopChan)
}

func (c *MemoryCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if entry.IsExpired() {
		c.Delete(key)
		return nil, false
	}

	c.mu.Lock()
	entry.HitCount++
	c.mu.Unlock()

	return entry, true
}

func (c *MemoryCache) Set(key string, entry *CacheEntry) {
	entry.ExpiresAt = time.Now().Add(c.ttl)
	size := int64(len(entry.Body))

	c.mu.Lock()
	defer c.mu.Unlock()

	if old, exists := c.entries[key]; exists {
		c.curBytes -= int64(len(old.Body))
	}

	for c.curBytes+size > c.maxBytes && len(c.entries) > 0 {
		c.evictLocked()
	}

	c.entries[key] = entry
	c.curBytes += size
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[key]; ok {
		c.curBytes -= int64(len(entry.Body))
		delete(c.entries, key)
	}
}

func (c *MemoryCache) evictLocked() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.entries {
		if first || v.CachedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.CachedAt
			first = false
		}
	}
	if oldestKey != "" {
		entry := c.entries[oldestKey]
		c.curBytes -= int64(len(entry.Body))
		delete(c.entries, oldestKey)
	}
}

func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopChan:
			return
		}
	}
}

func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.entries {
		if now.After(v.ExpiresAt) {
			c.curBytes -= int64(len(v.Body))
			delete(c.entries, k)
		}
	}
}

func (c *MemoryCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]interface{}{
		"entries":     len(c.entries),
		"used_bytes":  c.curBytes,
		"max_bytes":   c.maxBytes,
		"used_mb":     float64(c.curBytes) / 1024 / 1024,
		"max_mb":      float64(c.maxBytes) / 1024 / 1024,
		"ttl_seconds": int(c.ttl.Seconds()),
	}
}

func (c *MemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
