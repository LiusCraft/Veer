package edge

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"veer/config"
)

type Compactor struct {
	cfg      config.EdgeDiskCacheConfig
	reader   *DiskReader
	writer   *DiskWriter
	index    *CacheIndex
	diskPath string
	tmpDir   string

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewCompactor(cfg config.EdgeDiskCacheConfig, reader *DiskReader, writer *DiskWriter, index *CacheIndex) *Compactor {
	return &Compactor{
		cfg:      cfg,
		reader:   reader,
		writer:   writer,
		index:    index,
		diskPath: filepath.Join(cfg.Path, "segments"),
		tmpDir:   filepath.Join(cfg.Path, "tmp"),
		stopChan: make(chan struct{}),
	}
}

func (c *Compactor) Start() {
	if !c.cfg.Compaction.Enabled {
		return
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	interval := time.Duration(c.cfg.Compaction.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 30 * time.Minute
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-c.stopChan:
				return
			case <-ticker.C:
				c.compact()
			}
		}
	}()
}

func (c *Compactor) shouldCompact() bool {
	segments := c.reader.Segments()

	var sealedCount int
	var totalSize int64
	for _, seg := range segments {
		totalSize += seg.FileSize
		if seg.ID != 0 {
			sealedCount++
		}
	}

	maxSegments := c.cfg.Compaction.MaxSegments
	if maxSegments <= 0 {
		maxSegments = 200
	}
	if sealedCount > maxSegments {
		return true
	}

	if c.cfg.MaxSizeGB > 0 {
		maxBytes := int64(c.cfg.MaxSizeGB) * (1024 * 1024 * 1024)
		usage := float64(totalSize) / float64(maxBytes)
		watermark := c.cfg.Compaction.Watermark
		if watermark <= 0 {
			watermark = 0.85
		}
		if usage > watermark {
			return true
		}
	}

	return false
}

func (c *Compactor) compact() {
	if !c.shouldCompact() {
		return
	}

	segments := c.reader.Segments()

	for _, seg := range segments {
		if seg.ID == 0 {
			continue
		}
		if err := c.compactSegment(seg); err != nil {
			log.Printf("compactor: segment %d: %v", seg.ID, err)
		}
	}

	if err := c.reader.Refresh(); err != nil {
		log.Printf("compactor: refresh: %v", err)
	}
}

func (c *Compactor) compactSegment(seg segmentInfo) error {
	entries, err := c.reader.ScanSegment(seg.ID)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	for key, entry := range entries {
		if entry.IsExpired() {
			c.index.Remove(ComputeKeyHash(key))
			continue
		}
		if err := c.writer.Write(key, entry); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}

	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	bakPath := seg.Path + ".bak"
	if err := os.Rename(seg.Path, bakPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	if err := os.Remove(bakPath); err != nil {
		return fmt.Errorf("remove bak: %w", err)
	}

	return nil
}

func (c *Compactor) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopChan)
	c.wg.Wait()
}
