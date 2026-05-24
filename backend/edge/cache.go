package edge

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

func (e *CacheEntry) CanRevalidate() bool {
	return e.Headers.Get("ETag") != "" || e.Headers.Get("Last-Modified") != ""
}

func parseCacheControlTTL(headers http.Header, defaultTTL time.Duration) time.Duration {
	if cc := headers.Get("Cache-Control"); cc != "" {
		for _, part := range strings.Split(cc, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "s-maxage=") {
				if secs, err := strconv.Atoi(part[9:]); err == nil && secs >= 0 {
					return time.Duration(secs) * time.Second
				}
			}
		}
		for _, part := range strings.Split(cc, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "max-age=") {
				if secs, err := strconv.Atoi(part[7:]); err == nil && secs >= 0 {
					return time.Duration(secs) * time.Second
				}
			}
		}
		lower := strings.ToLower(cc)
		if strings.Contains(lower, "no-store") {
			return 0
		}
	}

	if expires := headers.Get("Expires"); expires != "" {
		if t, err := http.ParseTime(expires); err == nil {
			remaining := time.Until(t)
			if remaining > 0 {
				return remaining
			}
		}
	}

	return defaultTTL
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
	if entry.ExpiresAt.IsZero() || !entry.ExpiresAt.After(time.Now()) {
		ttl := parseCacheControlTTL(entry.Headers, c.ttl)
		if ttl > 0 {
			entry.ExpiresAt = time.Now().Add(ttl)
		} else {
			entry.ExpiresAt = time.Now()
		}
	}
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

func (c *MemoryCache) GetStale(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !entry.CanRevalidate() {
		return nil, false
	}
	return entry, true
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

type RamTier = MemoryCache

func NewRamTier(cfg config.EdgeCacheConfig) *RamTier {
	l1MB := cfg.MaxSizeMB * 5 / 100
	if l1MB < 64 {
		l1MB = 64
	}
	if cfg.MaxL1MB > 0 && l1MB > cfg.MaxL1MB {
		l1MB = cfg.MaxL1MB
	}
	l1Cfg := config.EdgeCacheConfig{
		TTLSeconds: cfg.TTLSeconds,
		MaxSizeMB:  l1MB,
	}
	return NewMemoryCache(l1Cfg)
}

type TieredCache struct {
	cfg       config.EdgeCacheConfig
	ramTier   *RamTier
	index     *CacheIndex
	diskR     *DiskReader
	diskW     *DiskWriter
	compactor *Compactor
	cleaner   *Cleaner
	enabled   bool
	startedAt time.Time
}

func NewTieredCache(cfg config.EdgeCacheConfig) (*TieredCache, error) {
	tc := &TieredCache{
		cfg:       cfg,
		ramTier:   NewRamTier(cfg),
		enabled:   cfg.Disk.Enabled,
		startedAt: time.Now(),
	}

	if cfg.Disk.Enabled {
		tc.index = NewCacheIndex(cfg.Disk.Index, cfg.Disk.Index.SparseMaxEntries)

		reader, err := NewDiskReader(cfg.Disk)
		if err != nil {
			return nil, fmt.Errorf("create disk reader: %w", err)
		}
		tc.diskR = reader

		writer, err := NewDiskWriter(cfg.Disk, tc.index)
		if err != nil {
			reader.Stop()
			return nil, fmt.Errorf("create disk writer: %w", err)
		}
		tc.diskW = writer

		tc.cleaner = NewCleaner(tc.index, time.Minute)
		tc.cleaner.Start()

		if cfg.Disk.Compaction.Enabled {
			tc.compactor = NewCompactor(cfg.Disk, tc.diskR, tc.diskW, tc.index)
			tc.compactor.Start()
		}

		if err := tc.rebuildIndex(); err != nil {
			reader.Stop()
			writer.Stop()
			return nil, fmt.Errorf("rebuild index: %w", err)
		}
	}

	return tc, nil
}

func (tc *TieredCache) Get(key string) (*CacheEntry, bool) {
	if entry, ok := tc.ramTier.Get(key); ok {
		return entry, true
	}

	if !tc.enabled {
		return nil, false
	}

	keyHash := ComputeKeyHash(key)
	if !tc.index.BloomCheck(keyHash) {
		return nil, false
	}

	loc, ok := tc.index.Lookup(keyHash)
	if !ok {
		return nil, false
	}

	if loc.ExpiresAt > 0 && time.Now().UnixNano() > loc.ExpiresAt {
		tc.index.Remove(keyHash)
		return nil, false
	}

	entry, err := tc.diskR.Read(loc, key)
	if err != nil {
		tc.index.Remove(keyHash)
		return nil, false
	}

	tc.ramTier.Set(key, entry)

	return entry, true
}

type bodyReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (b *bodyReadCloser) Read(p []byte) (int, error) { return b.reader.Read(p) }
func (b *bodyReadCloser) Close() error               { return b.closer.Close() }

// GetBodyReader returns a body reader for a cached entry.
// For RAM hits, wraps the existing []body (no copy).
// For disk hits, reads the full entry into RAM (with body as an independent
// allocation, see DiskReader.Read), promotes to the RAM tier so subsequent
// requests are served from memory, and returns a reader over the body.
// The caller MUST close the returned io.ReadCloser.
func (tc *TieredCache) GetBodyReader(key string) (body io.ReadCloser, bodyLen int64, headers http.Header, statusCode int, ok bool) {
	if entry, entryOk := tc.ramTier.Get(key); entryOk {
		body = &bodyReadCloser{
			reader: bytes.NewReader(entry.Body),
			closer: nopCloser{},
		}
		return body, int64(len(entry.Body)), entry.Headers, entry.StatusCode, true
	}

	if !tc.enabled {
		return nil, 0, nil, 0, false
	}

	keyHash := ComputeKeyHash(key)
	if !tc.index.BloomCheck(keyHash) {
		return nil, 0, nil, 0, false
	}

	loc, locOk := tc.index.Lookup(keyHash)
	if !locOk {
		return nil, 0, nil, 0, false
	}

	if loc.ExpiresAt > 0 && time.Now().UnixNano() > loc.ExpiresAt {
		tc.index.Remove(keyHash)
		return nil, 0, nil, 0, false
	}

	entry, err := tc.diskR.Read(loc, key)
	if err != nil {
		tc.index.Remove(keyHash)
		return nil, 0, nil, 0, false
	}

	tc.ramTier.Set(key, entry)

	body = &bodyReadCloser{
		reader: bytes.NewReader(entry.Body),
		closer: nopCloser{},
	}
	return body, int64(len(entry.Body)), entry.Headers, entry.StatusCode, true
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

func (tc *TieredCache) Set(key string, entry *CacheEntry) {
	tc.ramTier.Set(key, entry)

	if !tc.enabled {
		return
	}

	keyHash := ComputeKeyHash(key)
	tc.index.Add(keyHash, cacheLocation{
		ExpiresAt: entry.ExpiresAt.UnixNano(),
		CachedAt:  uint32(entry.CachedAt.Unix()),
	})

	if err := tc.diskW.Write(key, entry); err != nil {
	}
}

func (tc *TieredCache) Delete(key string) {
	tc.ramTier.Delete(key)
	if tc.enabled {
		tc.index.Remove(ComputeKeyHash(key))
	}
}

func (tc *TieredCache) Stats() map[string]interface{} {
	stats := tc.ramTier.Stats()
	if tc.enabled {
		stats["disk_enabled"] = true
		stats["disk_segments"] = tc.diskR.SegmentCount()
		stats["disk_writer_closed"] = tc.diskW.IsClosed()
		indexStats := tc.index.Stats()
		for k, v := range indexStats {
			stats[k] = v
		}
	} else {
		stats["disk_enabled"] = false
	}
	return stats
}

func (tc *TieredCache) Stop() {
	if tc.enabled {
		if tc.compactor != nil {
			tc.compactor.Stop()
		}
		if tc.cleaner != nil {
			tc.cleaner.Stop()
		}
		if tc.diskW != nil {
			tc.diskW.Stop()
		}
		if tc.diskR != nil {
			tc.diskR.Stop()
		}
	}
	tc.ramTier.Stop()
}

func (tc *TieredCache) Len() int {
	return tc.ramTier.Len()
}

func (tc *TieredCache) GetStale(key string) (*CacheEntry, bool) {
	return tc.ramTier.GetStale(key)
}

func (tc *TieredCache) rebuildIndex() error {
	for _, seg := range tc.diskR.Segments() {
		f, err := os.Open(seg.Path)
		if err != nil {
			continue
		}

		if seg.ID == 0 {
			if tc.cfg.Disk.Debug {
				log.Printf("[edge:cache] rebuild: scanning active.dat (size=%d)", seg.FileSize)
			}
			tc.rebuildActiveIndex(f)
			f.Close()
			continue
		}

		sh, err := ReadSegmentHeader(f)
		if err != nil {
			f.Close()
			continue
		}

		fi, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}

		indexes, err := ReadEntryIndexTable(f, fi.Size(), sh.Entries)
		f.Close()
		if err != nil {
			continue
		}

		for _, idx := range indexes {
			tc.index.Add(idx.KeyHash, cacheLocation{
				SegmentID:  seg.ID,
				BodyOffset: idx.BodyOffset,
				BodyLen:    idx.BodyLen,
			})
		}
	}

	if tc.cfg.Disk.Debug {
		stats := tc.index.Stats()
		log.Printf("[edge:cache] rebuild done: bloom=%d sparse=%d/%d",
			stats["bloom_entries"], stats["sparse_entries"], stats["sparse_max"])
	}
	return nil
}

func (tc *TieredCache) rebuildActiveIndex(f *os.File) {
	sh, err := ReadSegmentHeader(f)
	if err != nil {
		if tc.cfg.Disk.Debug {
			log.Printf("[edge:cache] rebuild: read header failed: %v", err)
		}
		return
	}

	var count int
	offset := int64(sh.DataOffset)
	for {
		data := make([]byte, EntryHeaderSize)
		if _, err := f.ReadAt(data, offset); err != nil {
			break
		}

		bodyLen := int(binary.BigEndian.Uint32(data[6:10]))
		keyLen := int(binary.BigEndian.Uint16(data[10:12]))
		contentTypeLen := int(binary.BigEndian.Uint16(data[24:26]))
		headersLen := int(binary.BigEndian.Uint16(data[26:28]))
		totalSize := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen

		if totalSize <= EntryHeaderSize {
			break
		}
		if totalSize > 100*1024*1024 { // 100MB per-entry safety limit
			break
		}

		entry := make([]byte, totalSize)
		if _, err := f.ReadAt(entry, offset); err != nil {
			break
		}

		expectedCRC := binary.BigEndian.Uint32(entry[0:4])
		if crc32.ChecksumIEEE(entry[4:]) != expectedCRC {
			break
		}

		expiresAt := int64(binary.BigEndian.Uint64(data[12:20]))
		cachedAt := binary.BigEndian.Uint32(data[20:24])
		key := string(entry[EntryHeaderSize : EntryHeaderSize+keyLen])
		keyHash := ComputeKeyHash(key)

		tc.index.Add(keyHash, cacheLocation{
			SegmentID:  0,
			BodyOffset: uint64(offset),
			BodyLen:    uint32(totalSize),
			ExpiresAt:  expiresAt,
			CachedAt:   cachedAt,
		})
		count++

		offset += int64(totalSize)
	}

	if tc.cfg.Disk.Debug {
		log.Printf("[edge:cache] rebuild: active.dat scanned %d entries from offset %d to %d",
			count, sh.DataOffset, offset)
	}
}
