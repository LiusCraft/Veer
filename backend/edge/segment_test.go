package edge

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"veer/config"
)

func TestMarshalUnmarshalEntry(t *testing.T) {
	now := time.Now()
	entry := &CacheEntry{
		StatusCode:  200,
		ContentType: "application/json",
		Headers: http.Header{
			"Content-Type":   {"application/json"},
			"Cache-Control":  {"public, max-age=3600"},
			"Content-Length": {"42"},
		},
		Body:      []byte(`{"message":"hello world"}`),
		CachedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}

	data, err := MarshalEntry("test-key", entry.StatusCode, entry.ContentType, entry.Headers, entry.Body, entry.ExpiresAt, entry.CachedAt)
	if err != nil {
		t.Fatalf("MarshalEntry failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalEntry returned empty data")
	}

	key, statusCode, contentType, headers, body, expiresAt, cachedAt, err := UnmarshalEntry(data)
	if err != nil {
		t.Fatalf("UnmarshalEntry failed: %v", err)
	}
	if key != "test-key" {
		t.Fatalf("key mismatch: got %q, want %q", key, "test-key")
	}
	if statusCode != 200 {
		t.Fatalf("statusCode mismatch: got %d, want %d", statusCode, 200)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType mismatch: got %q, want %q", contentType, "application/json")
	}
	if string(body) != `{"message":"hello world"}` {
		t.Fatalf("body mismatch: got %q", string(body))
	}
	if headers.Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("header Cache-Control mismatch: got %q", headers.Get("Cache-Control"))
	}
	if !expiresAt.Equal(entry.ExpiresAt) {
		t.Fatalf("expiresAt mismatch: got %v, want %v", expiresAt, entry.ExpiresAt)
	}
	if !cachedAt.Equal(entry.CachedAt.Truncate(time.Second)) {
		t.Fatalf("cachedAt mismatch: got %v, want %v", cachedAt, entry.CachedAt)
	}
}

func TestMarshalEntryLargeKey(t *testing.T) {
	key := string(make([]byte, 65536))
	_, err := MarshalEntry(key, 200, "", nil, []byte("body"), time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected error for key > 65535")
	}
}

func TestUnmarshalCorruptData(t *testing.T) {
	_, _, _, _, _, _, _, err := UnmarshalEntry([]byte{0, 1, 2})
	if err == nil {
		t.Fatal("expected error for truncated data")
	}
}

func TestMarshalEntryEmptyFields(t *testing.T) {
	now := time.Now()
	data, err := MarshalEntry("k", 204, "", nil, nil, now, now)
	if err != nil {
		t.Fatalf("MarshalEntry failed: %v", err)
	}
	key, statusCode, contentType, headers, body, _, _, err := UnmarshalEntry(data)
	if err != nil {
		t.Fatalf("UnmarshalEntry failed: %v", err)
	}
	if key != "k" || statusCode != 204 || contentType != "" || headers == nil || len(body) != 0 {
		t.Fatalf("unexpected: key=%q status=%d ct=%q body=%v", key, statusCode, contentType, body)
	}
}

func TestSegmentHeader(t *testing.T) {
	var buf bytes.Buffer
	createTime := int64(1715000000000000000)
	err := WriteSegmentHeader(&buf, createTime, 100, SegmentHeaderSize)
	if err != nil {
		t.Fatalf("WriteSegmentHeader failed: %v", err)
	}
	if buf.Len() != SegmentHeaderSize {
		t.Fatalf("header size: got %d, want %d", buf.Len(), SegmentHeaderSize)
	}

	sh, err := ReadSegmentHeader(&buf)
	if err != nil {
		t.Fatalf("ReadSegmentHeader failed: %v", err)
	}
	if sh.Magic != SegmentMagic {
		t.Fatalf("magic mismatch: got %08x, want %08x", sh.Magic, SegmentMagic)
	}
	if sh.Version != SegmentVersion {
		t.Fatalf("version mismatch: got %d, want %d", sh.Version, SegmentVersion)
	}
	if sh.CreateTime != createTime {
		t.Fatalf("createTime mismatch: got %d, want %d", sh.CreateTime, createTime)
	}
	if sh.Entries != 100 {
		t.Fatalf("entries mismatch: got %d, want %d", sh.Entries, 100)
	}
	if sh.DataOffset != SegmentHeaderSize {
		t.Fatalf("dataOffset mismatch: got %d, want %d", sh.DataOffset, SegmentHeaderSize)
	}
}

func TestReadSegmentHeaderInvalidMagic(t *testing.T) {
	buf := make([]byte, SegmentHeaderSize)
	_, err := ReadSegmentHeader(bytes.NewReader(buf))
	if err == nil {
		t.Fatal("expected error for invalid magic")
	}
}

func TestEntryIndexTable(t *testing.T) {
	entries := []IndexEntry{
		{KeyHash: 100, BodyOffset: 64, BodyLen: 128},
		{KeyHash: 200, BodyOffset: 192, BodyLen: 256},
		{KeyHash: 300, BodyOffset: 448, BodyLen: 64},
	}

	var buf bytes.Buffer
	err := WriteEntryIndexTable(&buf, entries)
	if err != nil {
		t.Fatalf("WriteEntryIndexTable failed: %v", err)
	}

	fileEnd := int64(SegmentHeaderSize + buf.Len())
	written := buf.Bytes()
	r := bytes.NewReader(written)
	readEntries, err := ReadEntryIndexTable(r, fileEnd-int64(SegmentHeaderSize), uint64(len(entries)))
	if err != nil {
		t.Fatalf("ReadEntryIndexTable failed: %v", err)
	}
	if len(readEntries) != len(entries) {
		t.Fatalf("entry count: got %d, want %d", len(readEntries), len(entries))
	}
	for i, e := range entries {
		if readEntries[i].KeyHash != e.KeyHash {
			t.Fatalf("entry[%d] KeyHash: got %d, want %d", i, readEntries[i].KeyHash, e.KeyHash)
		}
		if readEntries[i].BodyOffset != e.BodyOffset {
			t.Fatalf("entry[%d] BodyOffset: got %d, want %d", i, readEntries[i].BodyOffset, e.BodyOffset)
		}
		if readEntries[i].BodyLen != e.BodyLen {
			t.Fatalf("entry[%d] BodyLen: got %d, want %d", i, readEntries[i].BodyLen, e.BodyLen)
		}
	}
}

func TestComputeKeyHash(t *testing.T) {
	h1 := ComputeKeyHash("hello")
	h2 := ComputeKeyHash("hello")
	h3 := ComputeKeyHash("world")
	if h1 != h2 {
		t.Fatal("same key should produce same hash")
	}
	if h1 == h3 {
		t.Fatal("different keys should produce different hashes (collision unlikely)")
	}
}

func TestEntryWireSize(t *testing.T) {
	size := EntryWireSize(10, 15, 20, 100)
	expected := EntryHeaderSize + 10 + 15 + 20 + 100
	if size != expected {
		t.Fatalf("EntryWireSize: got %d, want %d", size, expected)
	}
}

func TestBloomFilter(t *testing.T) {
	bf := NewBloomFilter(1000, 16)
	if bf == nil {
		t.Fatal("NewBloomFilter returned nil")
	}
	if bf.Len() != 0 {
		t.Fatal("fresh bloom filter should have 0 entries")
	}

	bf.Add(ComputeKeyHash("key1"))
	bf.Add(ComputeKeyHash("key2"))
	bf.Add(ComputeKeyHash("key3"))

	if !bf.MaybeContains(ComputeKeyHash("key1")) {
		t.Fatal("bloom should contain key1")
	}
	if !bf.MaybeContains(ComputeKeyHash("key2")) {
		t.Fatal("bloom should contain key2")
	}
	if bf.MaybeContains(ComputeKeyHash("nonexistent")) {
		t.Log("note: bloom false positive (expected ~0.1%)")
	}

	bf.Reset()
	if bf.Len() != 0 {
		t.Fatal("after reset, Len should be 0")
	}
}

func TestBloomFilterZeroCapacity(t *testing.T) {
	bf := NewBloomFilter(0, 16)
	bf.Add(ComputeKeyHash("test"))
	if bf.MaybeContains(ComputeKeyHash("test")) != true {
		t.Fatal("bloom with min size should still work")
	}
}

func TestSparseIndex(t *testing.T) {
	si := newSparseIndex(3)
	if si.Len() != 0 {
		t.Fatal("fresh sparse index should be empty")
	}

	loc1 := cacheLocation{SegmentID: 1, BodyOffset: 64, BodyLen: 128, ExpiresAt: 0, CachedAt: 100}
	loc2 := cacheLocation{SegmentID: 1, BodyOffset: 192, BodyLen: 256, ExpiresAt: 0, CachedAt: 200}
	loc3 := cacheLocation{SegmentID: 2, BodyOffset: 64, BodyLen: 64, ExpiresAt: 0, CachedAt: 300}

	si.Set(100, loc1)
	si.Set(200, loc2)
	si.Set(300, loc3)

	if si.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", si.Len())
	}

	got, ok := si.Get(100)
	if !ok || got.BodyOffset != 64 {
		t.Fatal("should find key 100")
	}

	got, ok = si.Get(999)
	if ok {
		t.Fatal("should not find nonexistent key")
	}

	si.Set(400, cacheLocation{SegmentID: 3, BodyOffset: 0, BodyLen: 0})
	if si.Len() != 3 {
		t.Fatalf("after evict, expected 3 entries, got %d", si.Len())
	}
	_, ok = si.Get(200)
	if ok {
		t.Fatal("200 should have been evicted (LRU tail)")
	}

	si.Remove(100)
	if si.Len() != 2 {
		t.Fatalf("after remove, expected 2 entries, got %d", si.Len())
	}
	_, ok = si.Get(100)
	if ok {
		t.Fatal("100 should have been removed")
	}

	si.Reset()
	if si.Len() != 0 {
		t.Fatal("after reset, should be empty")
	}
}

func TestSparseIndexUpdateExisting(t *testing.T) {
	si := newSparseIndex(10)
	loc1 := cacheLocation{SegmentID: 1, BodyOffset: 64, BodyLen: 128}
	loc2 := cacheLocation{SegmentID: 1, BodyOffset: 999, BodyLen: 256}

	si.Set(100, loc1)
	si.Set(100, loc2)

	got, ok := si.Get(100)
	if !ok {
		t.Fatal("should find key 100")
	}
	if got.BodyOffset != 999 {
		t.Fatalf("expected updated BodyOffset 999, got %d", got.BodyOffset)
	}
}

func TestCacheIndex(t *testing.T) {
	cfg := config.EdgeDiskIndexConfig{
		BloomBitsPerEntry: 16,
		SparseMaxEntries:  100,
	}
	ci := NewCacheIndex(cfg, 1000)

	loc := cacheLocation{SegmentID: 1, BodyOffset: 64, BodyLen: 128, ExpiresAt: 0, CachedAt: 100}
	ci.Add(ComputeKeyHash("key1"), loc)

	if !ci.BloomCheck(ComputeKeyHash("key1")) {
		t.Fatal("bloom should contain key1")
	}

	got, ok := ci.Lookup(ComputeKeyHash("key1"))
	if !ok {
		t.Fatal("should find key1 in lookup")
	}
	if got.BodyOffset != 64 {
		t.Fatalf("body offset mismatch: got %d, want %d", got.BodyOffset, 64)
	}

	ci.Remove(ComputeKeyHash("key1"))
	_, ok = ci.Lookup(ComputeKeyHash("key1"))
	if ok {
		t.Fatal("after remove, lookup should miss")
	}
	if ci.BloomCheck(ComputeKeyHash("key1")) {
		t.Log("note: bloom still has key1 after remove (bloom is append-only)")
	}

	stats := ci.Stats()
	if stats["bloom_miss"].(uint64) > 0 {
		t.Fatal("expected 0 bloom misses initially")
	}
}

func TestDiskWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EdgeDiskCacheConfig{
		Path:            dir,
		Enabled:         true,
		SegmentSizeMB:   1,
		WriteBufferKB:   1,
		FlushIntervalMS: 50,
		Index: config.EdgeDiskIndexConfig{
			BloomBitsPerEntry: 16,
			SparseMaxEntries:  1000,
		},
	}

	idx := NewCacheIndex(cfg.Index, 1000)
	w, err := NewDiskWriter(cfg, idx)
	if err != nil {
		t.Fatalf("NewDiskWriter failed: %v", err)
	}
	defer w.Stop()

	r, err := NewDiskReader(cfg)
	if err != nil {
		t.Fatalf("NewDiskReader failed: %v", err)
	}
	defer r.Stop()

	now := time.Now()
	entry := &CacheEntry{
		StatusCode:  200,
		ContentType: "text/plain",
		Headers:     http.Header{"Content-Type": {"text/plain"}},
		Body:        []byte("hello world"),
		CachedAt:    now,
		ExpiresAt:   now.Add(time.Hour),
	}

	keyHash := ComputeKeyHash("test-key")
	idx.Add(keyHash, cacheLocation{
		BodyOffset: 0,
		BodyLen:    0,
		ExpiresAt:  entry.ExpiresAt.UnixNano(),
		CachedAt:   uint32(entry.CachedAt.Unix()),
	})

	err = w.Write("test-key", entry)
	if err != nil {
		t.Fatalf("DiskWriter.Write failed: %v", err)
	}
	err = w.Flush()
	if err != nil {
		t.Fatalf("DiskWriter.Flush failed: %v", err)
	}

	r.Refresh()

	loc, ok := idx.Lookup(keyHash)
	if !ok {
		t.Fatal("index should have test-key after write+flush")
	}

	readEntry, err := r.Read(loc, "test-key")
	if err != nil {
		t.Fatalf("DiskReader.Read failed: %v", err)
	}
	if string(readEntry.Body) != "hello world" {
		t.Fatalf("body mismatch: got %q, want %q", string(readEntry.Body), "hello world")
	}
	if readEntry.StatusCode != 200 {
		t.Fatalf("statusCode mismatch: got %d, want %d", readEntry.StatusCode, 200)
	}
}

func TestDiskReaderHashCollision(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EdgeDiskCacheConfig{
		Path:            dir,
		Enabled:         true,
		SegmentSizeMB:   1,
		WriteBufferKB:   1,
		FlushIntervalMS: 50,
		Index: config.EdgeDiskIndexConfig{
			BloomBitsPerEntry: 16,
			SparseMaxEntries:  1000,
		},
	}

	idx := NewCacheIndex(cfg.Index, 1000)
	w, err := NewDiskWriter(cfg, idx)
	if err != nil {
		t.Fatalf("NewDiskWriter failed: %v", err)
	}
	defer w.Stop()

	r, err := NewDiskReader(cfg)
	if err != nil {
		t.Fatalf("NewDiskReader failed: %v", err)
	}
	defer r.Stop()

	now := time.Now()
	entry := &CacheEntry{
		StatusCode: 200,
		Body:       []byte("data"),
		CachedAt:   now,
		ExpiresAt:  now.Add(time.Hour),
	}
	keyHash := ComputeKeyHash("real-key")
	idx.Add(keyHash, cacheLocation{
		ExpiresAt: entry.ExpiresAt.UnixNano(),
		CachedAt:  uint32(entry.CachedAt.Unix()),
	})

	err = w.Write("real-key", entry)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	err = w.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	r.Refresh()

	loc, ok := idx.Lookup(keyHash)
	if !ok {
		t.Fatal("index should have real-key")
	}

	_, err = r.Read(loc, "wrong-key")
	if err == nil {
		t.Fatal("expected hash collision error")
	}
}

func TestDiskSegmentRotation(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EdgeDiskCacheConfig{
		Path:          dir,
		Enabled:       true,
		SegmentSizeMB: 1,
		WriteBufferKB: 4096,
		Index: config.EdgeDiskIndexConfig{
			BloomBitsPerEntry: 16,
			SparseMaxEntries:  1000,
		},
	}
	cfg.SegmentSizeMB = 0

	idx := NewCacheIndex(cfg.Index, 1000)
	w, err := NewDiskWriter(cfg, idx)
	if err != nil {
		t.Fatalf("NewDiskWriter failed: %v", err)
	}
	defer w.Stop()

	r, err := NewDiskReader(cfg)
	if err != nil {
		t.Fatalf("NewDiskReader failed: %v", err)
	}
	defer r.Stop()

	now := time.Now()
	data := make([]byte, 100)
	for i := 0; i < 200; i++ {
		entry := &CacheEntry{
			StatusCode: 200,
			Body:       data,
			CachedAt:   now,
			ExpiresAt:  now.Add(time.Hour),
		}
		key := string(rune('a' + i%26))
		w.Write(key, entry)
	}
	w.Flush()

	r.Refresh()
	if r.SegmentCount() < 1 {
		t.Fatal("expected at least 1 segment")
	}
}

func TestTieredCacheDiskMode(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EdgeCacheConfig{
		TTLSeconds: 3600,
		MaxSizeMB:  512,
		MaxL1MB:    64,
		Disk: config.EdgeDiskCacheConfig{
			Enabled:         true,
			Path:            dir,
			SegmentSizeMB:   1,
			WriteBufferKB:   1,
			FlushIntervalMS: 50,
			Index: config.EdgeDiskIndexConfig{
				BloomBitsPerEntry: 16,
				SparseMaxEntries:  1000,
			},
		},
	}

	tc, err := NewTieredCache(cfg)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Stop()

	now := time.Now()
	entry := &CacheEntry{
		StatusCode:  200,
		ContentType: "text/plain",
		Headers:     http.Header{"Content-Type": {"text/plain"}},
		Body:        []byte("disk-cache-test"),
		CachedAt:    now,
		ExpiresAt:   now.Add(time.Hour),
	}

	tc.Set("test-key", entry)

	got, ok := tc.Get("test-key")
	if !ok {
		t.Fatal("should find entry in cache (L1)")
	}
	if string(got.Body) != "disk-cache-test" {
		t.Fatalf("body mismatch: got %q, want %q", string(got.Body), "disk-cache-test")
	}

	stats := tc.Stats()
	if stats["disk_enabled"] != true {
		t.Fatal("disk should be enabled")
	}
}

func TestTieredCacheDiskRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EdgeCacheConfig{
		TTLSeconds: 3600,
		MaxSizeMB:  512,
		MaxL1MB:    64,
		Disk: config.EdgeDiskCacheConfig{
			Enabled:         true,
			Path:            dir,
			SegmentSizeMB:   1,
			WriteBufferKB:   1,
			FlushIntervalMS: 50,
			Index: config.EdgeDiskIndexConfig{
				BloomBitsPerEntry: 16,
				SparseMaxEntries:  1000,
			},
		},
	}

	tc1, err := NewTieredCache(cfg)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	now := time.Now()
	entry := &CacheEntry{
		StatusCode: 200,
		Body:       []byte("survive-restart"),
		CachedAt:   now,
		ExpiresAt:  now.Add(time.Hour),
	}

	tc1.Set("persist-key", entry)
	tc1.Stop()

	entriesPath := filepath.Join(dir, "segments")
	files, _ := os.ReadDir(entriesPath)
	t.Logf("segments after stop: %v", func() []string {
		var names []string
		for _, f := range files {
			names = append(names, f.Name())
		}
		return names
	}())

	tc2, err := NewTieredCache(cfg)
	if err != nil {
		t.Fatalf("NewTieredCache(restart) failed: %v", err)
	}
	defer tc2.Stop()

	t.Log("attempting Get after simulated restart...")
	got, ok := tc2.Get("persist-key")
	if !ok {
		stats := tc2.Stats()
		t.Logf("post-restart stats: %v", stats)
		t.Fatal("entry should survive restart via disk recovery")
	}
	if string(got.Body) != "survive-restart" {
		t.Fatalf("body mismatch: got %q, want %q", string(got.Body), "survive-restart")
	}
	t.Log("restart recovery successful")
}

func TestTieredCacheMemoryMode(t *testing.T) {
	cfg := config.EdgeCacheConfig{
		TTLSeconds: 3600,
		MaxSizeMB:  512,
		MaxL1MB:    64,
		Disk: config.EdgeDiskCacheConfig{
			Enabled: false,
		},
	}

	tc, err := NewTieredCache(cfg)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Stop()

	entry := &CacheEntry{
		StatusCode: 200,
		Body:       []byte("hello"),
		CachedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	tc.Set("mem-key", entry)

	got, ok := tc.Get("mem-key")
	if !ok {
		t.Fatal("should find entry in memory-only mode")
	}
	if string(got.Body) != "hello" {
		t.Fatalf("body mismatch: got %q", string(got.Body))
	}

	stats := tc.Stats()
	if stats["disk_enabled"] != false {
		t.Fatal("disk should be disabled")
	}
}

func TestNewRamTier(t *testing.T) {
	cfg := config.EdgeCacheConfig{
		TTLSeconds: 300,
		MaxSizeMB:  512,
		MaxL1MB:    64,
	}
	rt := NewRamTier(cfg)
	if rt == nil {
		t.Fatal("NewRamTier returned nil")
	}
	if rt.Len() != 0 {
		t.Fatal("fresh RamTier should be empty")
	}
}

func TestNewRamTierSizing(t *testing.T) {
	tests := []struct {
		name    string
		maxMB   int
		maxL1MB int
		wantMB  int
	}{
		{"5% of 512 = 25, but min 64", 512, 4096, 64},
		{"5% of 2000 = 100", 2000, 4096, 100},
		{"5% of 512 = 25, capped at max_l1_mb=16", 512, 16, 16},
		{"5% of 2000 = 100, capped at max_l1_mb=50", 2000, 50, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.EdgeCacheConfig{
				MaxSizeMB: tt.maxMB,
				MaxL1MB:   tt.maxL1MB,
			}
			rt := NewRamTier(cfg)
			expectedBytes := int64(tt.wantMB) * 1024 * 1024
			if rt.maxBytes != expectedBytes {
				t.Fatalf("maxBytes: got %d, want %d", rt.maxBytes, expectedBytes)
			}
		})
	}
}

func TestCompactorTrigger(t *testing.T) {
	dir := t.TempDir()
	segmentsDir := filepath.Join(dir, "segments")
	os.MkdirAll(segmentsDir, 0755)

	now := time.Now()
	for i := 0; i < 3; i++ {
		path := filepath.Join(segmentsDir, fmt.Sprintf("seg_%d.dat", i+1))
		f, _ := os.Create(path)
		WriteSegmentHeader(f, now.UnixNano(), 1, SegmentHeaderSize)
		entry, _ := MarshalEntry("k", 200, "", nil, []byte("data"), now.Add(time.Hour), now)
		f.Write(entry)
		WriteEntryIndexTable(f, []IndexEntry{{KeyHash: ComputeKeyHash("k"), BodyOffset: SegmentHeaderSize, BodyLen: uint32(len(entry))}})
		f.Close()
	}

	cfg := config.EdgeDiskCacheConfig{
		Path:            dir,
		Enabled:         true,
		WriteBufferKB:   4096,
		FlushIntervalMS: 1000,
		Compaction: config.EdgeDiskCompactionConfig{
			Enabled:         true,
			Watermark:       0.99,
			IntervalMinutes: 1000,
			MaxSegments:     1,
		},
		Index: config.EdgeDiskIndexConfig{
			BloomBitsPerEntry: 16,
			SparseMaxEntries:  1000,
		},
	}

	idx := NewCacheIndex(cfg.Index, 1000)
	w, err := NewDiskWriter(cfg, idx)
	if err != nil {
		t.Fatalf("NewDiskWriter failed: %v", err)
	}
	defer w.Stop()

	r, err := NewDiskReader(cfg)
	if err != nil {
		t.Fatalf("NewDiskReader failed: %v", err)
	}
	defer r.Stop()

	compactor := NewCompactor(cfg, r, w, idx)
	defer compactor.Stop()

	if !compactor.shouldCompact() {
		t.Fatal("shouldCompact should be true when sealed count (3) > MaxSegments (1)")
	}
}

func TestDiskWriterStopFlushes(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EdgeDiskCacheConfig{
		Path:          dir,
		SegmentSizeMB: 0,
		WriteBufferKB: 4096,
		Index: config.EdgeDiskIndexConfig{
			BloomBitsPerEntry: 16,
			SparseMaxEntries:  1000,
		},
	}

	idx := NewCacheIndex(cfg.Index, 1000)
	w, err := NewDiskWriter(cfg, idx)
	if err != nil {
		t.Fatalf("NewDiskWriter failed: %v", err)
	}

	now := time.Now()
	entry := &CacheEntry{
		StatusCode: 200,
		Body:       []byte("flush-on-stop"),
		CachedAt:   now,
		ExpiresAt:  now.Add(time.Hour),
	}
	err = w.Write("stop-key", entry)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	err = w.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	fi, err := os.Stat(filepath.Join(dir, "segments", "active.dat"))
	if err != nil {
		t.Fatalf("active.dat should exist after Stop: %v", err)
	}
	if fi.Size() <= int64(SegmentHeaderSize) {
		t.Fatalf("active.dat should have data beyond header: %d bytes", fi.Size())
	}
}
