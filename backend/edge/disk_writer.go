package edge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"veer/config"
)

type pendingEntryMeta struct {
	expiresAt int64
	cachedAt  uint32
}

type DiskWriter struct {
	cfg            config.EdgeDiskCacheConfig
	index          *CacheIndex
	dirPath        string
	activeFile     *os.File
	activePath     string
	activeID       uint64
	seqNum         uint64
	createTime     int64
	entries        []IndexEntry
	pendingMeta    []pendingEntryMeta
	segmentEntries []IndexEntry
	buf            bytes.Buffer
	flushAtBytes   int64
	flushInterval  time.Duration
	mu             sync.Mutex
	isFlushing     bool
	closed         bool
	stopChan       chan struct{}
	flushWg        sync.WaitGroup
	writeErr       error
}

func NewDiskWriter(cfg config.EdgeDiskCacheConfig, idx *CacheIndex) (*DiskWriter, error) {
	segmentsDir := filepath.Join(cfg.Path, "segments")
	tmpDir := filepath.Join(cfg.Path, "tmp")

	for _, dir := range []string{segmentsDir, tmpDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	var maxSeq uint64
	matches, err := filepath.Glob(filepath.Join(segmentsDir, "seg_*.dat"))
	if err == nil {
		for _, m := range matches {
			base := filepath.Base(m)
			var seq uint64
			if n, _ := fmt.Sscanf(base, "seg_%d.dat", &seq); n == 1 && seq > maxSeq {
				maxSeq = seq
			}
		}
	}

	activePath := filepath.Join(segmentsDir, "active.dat")
	var activeFile *os.File
	var createTime int64

	fi, err := os.Stat(activePath)
	if err == nil && fi.Size() > 0 {
		activeFile, err = os.OpenFile(activePath, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open existing active.dat: %w", err)
		}

		var headerBuf [SegmentHeaderSize]byte
		if _, err := activeFile.ReadAt(headerBuf[:], 0); err != nil {
			activeFile.Close()
			return nil, fmt.Errorf("read active.dat header: %w", err)
		}
		createTime = int64(binary.BigEndian.Uint64(headerBuf[6:14]))
	} else {
		activeFile, err = os.Create(activePath)
		if err != nil {
			return nil, fmt.Errorf("create active.dat: %w", err)
		}
		createTime = time.Now().UnixNano()
		if err := WriteSegmentHeader(activeFile, createTime, 0, SegmentHeaderSize); err != nil {
			activeFile.Close()
			return nil, fmt.Errorf("write segment header: %w", err)
		}
	}

	nextSeq := maxSeq + 1
	if nextSeq == 0 {
		nextSeq = 1
	}

	flushAtBytes := int64(cfg.WriteBufferKB) * 1024
	if flushAtBytes <= 0 {
		flushAtBytes = 4096 * 1024
	}
	flushInterval := time.Duration(cfg.FlushIntervalMS) * time.Millisecond
	if flushInterval <= 0 {
		flushInterval = 100 * time.Millisecond
	}

	w := &DiskWriter{
		cfg:           cfg,
		index:         idx,
		dirPath:       segmentsDir,
		activeFile:    activeFile,
		activePath:    activePath,
		activeID:      0,
		seqNum:        nextSeq,
		createTime:    createTime,
		flushAtBytes:  flushAtBytes,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}

	w.flushWg.Add(1)
	go w.flushLoop()

	return w, nil
}

func (w *DiskWriter) logf(format string, args ...interface{}) {
	if w.cfg.Debug {
		log.Printf("[edge:disk] "+format, args...)
	}
}

func (w *DiskWriter) Write(key string, entry *CacheEntry) error {
	data, err := MarshalEntry(key, entry.StatusCode, entry.ContentType, entry.Headers, entry.Body, entry.ExpiresAt, entry.CachedAt)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	keyHash := ComputeKeyHash(key)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		if w.writeErr != nil {
			return w.writeErr
		}
		return errors.New("disk writer is closed")
	}
	if w.writeErr != nil {
		return w.writeErr
	}

	w.entries = append(w.entries, IndexEntry{
		KeyHash: keyHash,
		BodyLen: uint32(len(data)),
	})
	w.pendingMeta = append(w.pendingMeta, pendingEntryMeta{
		expiresAt: entry.ExpiresAt.UnixNano(),
		cachedAt:  uint32(entry.CachedAt.Unix()),
	})
	w.buf.Write(data)

	w.logf("write queued key=%q buf=%d", key, w.buf.Len())

	if w.buf.Len() >= int(w.flushAtBytes) && !w.isFlushing {
		w.isFlushing = true
		w.logf("buffer full (%d >= %d), triggering async flush", w.buf.Len(), w.flushAtBytes)
		go w.flush()
	}

	return nil
}

func (w *DiskWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.isFlushing = false
	if w.closed || len(w.entries) == 0 {
		return
	}
	w.flushLocked()
}

func (w *DiskWriter) flushLocked() {
	if len(w.entries) == 0 {
		return
	}

	w.logf("flush starting: %d entries, %d bytes", len(w.entries), w.buf.Len())

	pos, err := w.activeFile.Seek(0, io.SeekEnd)
	if err != nil {
		w.writeErr = fmt.Errorf("seek active file: %w", err)
		return
	}

	for i := range w.entries {
		w.entries[i].BodyOffset = uint64(pos)
		pos += int64(w.entries[i].BodyLen)
	}

	data := w.buf.Bytes()
	if _, err := w.activeFile.Write(data); err != nil {
		w.writeErr = fmt.Errorf("write segment data: %w", err)
		return
	}

	w.segmentEntries = append(w.segmentEntries, w.entries...)

	for i, entry := range w.entries {
		loc := cacheLocation{
			SegmentID:  w.activeID,
			BodyOffset: entry.BodyOffset,
			BodyLen:    entry.BodyLen,
			ExpiresAt:  w.pendingMeta[i].expiresAt,
			CachedAt:   w.pendingMeta[i].cachedAt,
		}
		w.index.AddSparse(entry.KeyHash, loc)
	}

	w.entries = w.entries[:0]
	w.pendingMeta = w.pendingMeta[:0]
	w.buf.Reset()

	w.logf("flush done: wrote %d entries at offset %d", len(w.segmentEntries), pos)

	fi, err := w.activeFile.Stat()
	if err != nil {
		w.writeErr = fmt.Errorf("stat active file: %w", err)
		return
	}

	segmentSizeLimit := int64(w.cfg.SegmentSizeMB) * 1024 * 1024
	if segmentSizeLimit > 0 && fi.Size() >= segmentSizeLimit {
		w.logf("segment full (size=%d >= limit=%d), rotating", fi.Size(), segmentSizeLimit)
		if err := w.rotateSegment(); err != nil {
			w.writeErr = err
		}
	}
}

func (w *DiskWriter) rotateSegment() error {
	w.logf("rotate: finalizing seg_%d.dat with %d entries", w.seqNum, len(w.segmentEntries))

	headerBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(headerBuf, uint64(len(w.segmentEntries)))
	if _, err := w.activeFile.WriteAt(headerBuf, 14); err != nil {
		return fmt.Errorf("update header entries: %w", err)
	}

	if err := WriteEntryIndexTable(w.activeFile, w.segmentEntries); err != nil {
		return fmt.Errorf("write index table: %w", err)
	}

	if err := w.activeFile.Sync(); err != nil {
		return fmt.Errorf("fsync: %w", err)
	}

	if err := w.activeFile.Close(); err != nil {
		return fmt.Errorf("close active file: %w", err)
	}

	newPath := filepath.Join(w.dirPath, fmt.Sprintf("seg_%d.dat", w.seqNum))
	if err := os.Rename(w.activePath, newPath); err != nil {
		return fmt.Errorf("rename active.dat to seg_%d.dat: %w", w.seqNum, err)
	}

	w.logf("rotate: created %s", filepath.Base(newPath))

	w.seqNum++
	w.activeID = 0
	w.createTime = time.Now().UnixNano()
	w.segmentEntries = w.segmentEntries[:0]
	w.activePath = filepath.Join(w.dirPath, "active.dat")

	f, err := os.Create(w.activePath)
	if err != nil {
		return fmt.Errorf("create new active.dat: %w", err)
	}
	if err := WriteSegmentHeader(f, w.createTime, 0, SegmentHeaderSize); err != nil {
		f.Close()
		return fmt.Errorf("write new segment header: %w", err)
	}
	w.activeFile = f

	w.logf("rotate: new active.dat created")
	return nil
}

func (w *DiskWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		if w.writeErr != nil {
			return w.writeErr
		}
		return nil
	}
	w.flushLocked()
	return w.writeErr
}

func (w *DiskWriter) Stop() error {
	w.logf("stop: shutting down disk writer")
	close(w.stopChan)
	w.flushWg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return w.writeErr
	}
	w.closed = true

	w.logf("stop: flushing remaining %d entries", len(w.entries))
	w.flushLocked()

	if len(w.segmentEntries) > 0 {
		headerBuf := make([]byte, 8)
		binary.BigEndian.PutUint64(headerBuf, uint64(len(w.segmentEntries)))
		if _, err := w.activeFile.WriteAt(headerBuf, 14); err != nil {
			w.writeErr = err
			w.activeFile.Close()
			return w.writeErr
		}

		if err := WriteEntryIndexTable(w.activeFile, w.segmentEntries); err != nil {
			w.writeErr = err
			w.activeFile.Close()
			return w.writeErr
		}

		if err := w.activeFile.Sync(); err != nil {
			w.writeErr = err
			w.activeFile.Close()
			return w.writeErr
		}
	}

	if err := w.activeFile.Close(); err != nil {
		w.writeErr = err
		return w.writeErr
	}

	return w.writeErr
}

func (w *DiskWriter) flushLoop() {
	defer w.flushWg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.mu.Lock()
			if w.writeErr != nil {
				w.mu.Unlock()
				return
			}
			needsFlush := len(w.entries) > 0 && !w.isFlushing && !w.closed
			if needsFlush {
				w.isFlushing = true
			}
			w.mu.Unlock()
			if needsFlush {
				w.flush()
			}
		}
	}
}

func (w *DiskWriter) ActiveSegmentID() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.activeID
}

func (w *DiskWriter) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}
