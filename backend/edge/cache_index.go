package edge

import (
	"container/list"
	"math"
	"sync"
	"veer/config"
)

type cacheLocation struct {
	SegmentID  uint64
	BodyOffset uint64
	BodyLen    uint32
	ExpiresAt  int64
	CachedAt   uint32
}

type BloomFilter struct {
	bits []uint64
	k    uint
	m    uint
	n    uint
}

func NewBloomFilter(capacity int, bitsPerEntry int) *BloomFilter {
	m := uint(capacity * bitsPerEntry)
	if m%64 != 0 {
		m = (m/64 + 1) * 64
	}
	if m < 64 {
		m = 64
	}
	k := uint(math.Round(float64(bitsPerEntry) * math.Ln2))
	if k < 1 {
		k = 1
	}
	if k > 16 {
		k = 16
	}
	return &BloomFilter{
		bits: make([]uint64, m/64),
		k:    k,
		m:    m,
	}
}

func (bf *BloomFilter) Add(hash uint64) {
	h1 := uint32(hash >> 32)
	h2 := uint32(hash)
	for i := uint(0); i < bf.k; i++ {
		pos := (uint(h1) + uint(i)*uint(h2)) % bf.m
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
	bf.n++
}

func (bf *BloomFilter) MaybeContains(hash uint64) bool {
	h1 := uint32(hash >> 32)
	h2 := uint32(hash)
	for i := uint(0); i < bf.k; i++ {
		pos := (uint(h1) + uint(i)*uint(h2)) % bf.m
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) Reset() {
	for i := range bf.bits {
		bf.bits[i] = 0
	}
	bf.n = 0
}

func (bf *BloomFilter) Len() uint {
	return bf.n
}

type sparseEntry struct {
	keyHash  uint64
	location cacheLocation
}

type sparseIndex struct {
	mu      sync.Mutex
	maxSize int
	entries map[uint64]*list.Element
	lru     *list.List
}

func newSparseIndex(maxSize int) *sparseIndex {
	if maxSize <= 0 {
		maxSize = 10000000
	}
	return &sparseIndex{
		maxSize: maxSize,
		entries: make(map[uint64]*list.Element),
		lru:     list.New(),
	}
}

func (si *sparseIndex) Get(keyHash uint64) (cacheLocation, bool) {
	si.mu.Lock()
	defer si.mu.Unlock()
	if elem, ok := si.entries[keyHash]; ok {
		si.lru.MoveToFront(elem)
		return elem.Value.(*sparseEntry).location, true
	}
	return cacheLocation{}, false
}

func (si *sparseIndex) Set(keyHash uint64, loc cacheLocation) {
	si.mu.Lock()
	defer si.mu.Unlock()

	if elem, ok := si.entries[keyHash]; ok {
		elem.Value.(*sparseEntry).location = loc
		si.lru.MoveToFront(elem)
		return
	}

	if si.lru.Len() >= si.maxSize {
		back := si.lru.Back()
		if back != nil {
			e := back.Value.(*sparseEntry)
			delete(si.entries, e.keyHash)
			si.lru.Remove(back)
		}
	}

	elem := si.lru.PushFront(&sparseEntry{
		keyHash:  keyHash,
		location: loc,
	})
	si.entries[keyHash] = elem
}

func (si *sparseIndex) Remove(keyHash uint64) {
	si.mu.Lock()
	defer si.mu.Unlock()
	if elem, ok := si.entries[keyHash]; ok {
		delete(si.entries, keyHash)
		si.lru.Remove(elem)
	}
}

func (si *sparseIndex) Len() int {
	si.mu.Lock()
	defer si.mu.Unlock()
	return si.lru.Len()
}

func (si *sparseIndex) Reset() {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.entries = make(map[uint64]*list.Element)
	si.lru = list.New()
}

type CacheIndex struct {
	bloom     *BloomFilter
	sparse    *sparseIndex
	maxSparse int
	hits      uint64
	misses    uint64
	bloomMiss uint64
}

func NewCacheIndex(cfg config.EdgeDiskIndexConfig, estimatedEntries int) *CacheIndex {
	capacity := estimatedEntries
	if capacity < cfg.SparseMaxEntries {
		capacity = cfg.SparseMaxEntries
	}
	return &CacheIndex{
		bloom:     NewBloomFilter(capacity, cfg.BloomBitsPerEntry),
		sparse:    newSparseIndex(cfg.SparseMaxEntries),
		maxSparse: cfg.SparseMaxEntries,
	}
}

func (ci *CacheIndex) Add(keyHash uint64, loc cacheLocation) {
	ci.bloom.Add(keyHash)
	if ci.sparse.Len() < ci.maxSparse {
		ci.sparse.Set(keyHash, loc)
	}
}

func (ci *CacheIndex) AddSparse(keyHash uint64, loc cacheLocation) {
	ci.sparse.Set(keyHash, loc)
}

func (ci *CacheIndex) BloomCheck(keyHash uint64) bool {
	return ci.bloom.MaybeContains(keyHash)
}

func (ci *CacheIndex) Lookup(keyHash uint64) (cacheLocation, bool) {
	if !ci.bloom.MaybeContains(keyHash) {
		ci.bloomMiss++
		return cacheLocation{}, false
	}
	loc, ok := ci.sparse.Get(keyHash)
	if ok {
		ci.hits++
	} else {
		ci.misses++
	}
	return loc, ok
}

func (ci *CacheIndex) Remove(keyHash uint64) {
	ci.sparse.Remove(keyHash)
}

func (ci *CacheIndex) Reset() {
	ci.bloom.Reset()
	ci.sparse.Reset()
	ci.hits = 0
	ci.misses = 0
	ci.bloomMiss = 0
}

func (ci *CacheIndex) Stats() map[string]interface{} {
	return map[string]interface{}{
		"bloom_entries":  ci.bloom.Len(),
		"sparse_entries": ci.sparse.Len(),
		"sparse_max":     ci.maxSparse,
		"bloom_check":    ci.hits + ci.misses + ci.bloomMiss,
		"bloom_miss":     ci.bloomMiss,
		"sparse_hit":     ci.hits,
		"sparse_miss":    ci.misses,
	}
}
