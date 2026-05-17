package edge

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"veer/config"
)

type segmentInfo struct {
	ID         uint64
	Path       string
	CreateTime int64
	EntryCount uint64
	FileSize   int64
}

type DiskReader struct {
	cfg      config.EdgeDiskCacheConfig
	segments map[uint64]*segmentInfo
	activeID uint64
	mu       sync.RWMutex
	dirPath  string
}

func NewDiskReader(cfg config.EdgeDiskCacheConfig) (*DiskReader, error) {
	r := &DiskReader{
		cfg:      cfg,
		segments: make(map[uint64]*segmentInfo),
		activeID: 0,
		dirPath:  filepath.Join(cfg.Path, "segments"),
	}
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	return r, nil
}

func segmentIDFromName(name string) (uint64, bool) {
	base := name
	if strings.HasSuffix(name, ".bak") {
		base = strings.TrimSuffix(name, ".bak")
	}
	if base == "active.dat" {
		return 0, true
	}
	if strings.HasPrefix(base, "seg_") && strings.HasSuffix(base, ".dat") {
		idStr := base[4 : len(base)-4]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err == nil {
			return id, true
		}
	}
	return 0, false
}

func (r *DiskReader) Refresh() error {
	entries, err := os.ReadDir(r.dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.mu.Lock()
			r.segments = make(map[uint64]*segmentInfo)
			r.mu.Unlock()
			return nil
		}
		return fmt.Errorf("read segments directory %s: %w", r.dirPath, err)
	}

	type fileCandidate struct {
		path  string
		isBak bool
	}
	candidates := make(map[uint64]fileCandidate)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		id, ok := segmentIDFromName(name)
		if !ok {
			continue
		}
		fullPath := filepath.Join(r.dirPath, name)
		isBak := strings.HasSuffix(name, ".bak")

		existing, found := candidates[id]
		if !found {
			candidates[id] = fileCandidate{path: fullPath, isBak: isBak}
		} else if !isBak && existing.isBak {
			candidates[id] = fileCandidate{path: fullPath, isBak: false}
		}
	}

	newSegments := make(map[uint64]*segmentInfo, len(candidates))
	for id, cand := range candidates {
		fi, err := os.Stat(cand.path)
		if err != nil {
			continue
		}

		f, err := os.Open(cand.path)
		if err != nil {
			continue
		}

		sh, err := ReadSegmentHeader(f)
		f.Close()
		if err != nil {
			continue
		}

		newSegments[id] = &segmentInfo{
			ID:         id,
			Path:       cand.path,
			CreateTime: sh.CreateTime,
			EntryCount: sh.Entries,
			FileSize:   fi.Size(),
		}
	}

	r.mu.Lock()
	r.segments = newSegments
	r.mu.Unlock()

	return nil
}

func (r *DiskReader) getSegment(segmentID uint64) (*segmentInfo, error) {
	r.mu.RLock()
	seg, ok := r.segments[segmentID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("segment %d not found", segmentID)
	}
	return seg, nil
}

func (r *DiskReader) Read(loc cacheLocation, expectedKey string) (*CacheEntry, error) {
	seg, err := r.getSegment(loc.SegmentID)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(seg.Path)
	if err != nil {
		return nil, fmt.Errorf("open segment %d: %w", loc.SegmentID, err)
	}
	defer f.Close()

	header := make([]byte, EntryHeaderSize)
	if _, err := f.ReadAt(header, int64(loc.BodyOffset)); err != nil {
		return nil, fmt.Errorf("read entry header at segment %d offset %d: %w", loc.SegmentID, loc.BodyOffset, err)
	}

	keyLen := int(binary.BigEndian.Uint16(header[10:12]))
	contentTypeLen := int(binary.BigEndian.Uint16(header[24:26]))
	headersLen := int(binary.BigEndian.Uint16(header[26:28]))
	bodyLen := int(binary.BigEndian.Uint32(header[6:10]))
	totalSize := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen

	data := make([]byte, totalSize)
	if _, err := f.ReadAt(data, int64(loc.BodyOffset)); err != nil {
		return nil, fmt.Errorf("read entry data at segment %d offset %d: %w", loc.SegmentID, loc.BodyOffset, err)
	}

	key, statusCode, contentType, headers, body, expiresAt, cachedAt, err := UnmarshalEntry(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal entry at segment %d offset %d: %w", loc.SegmentID, loc.BodyOffset, err)
	}

	if key != expectedKey {
		return nil, fmt.Errorf("hash collision at segment %d offset %d: expected %q, got %q", loc.SegmentID, loc.BodyOffset, expectedKey, key)
	}

	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)

	return &CacheEntry{
		StatusCode:  statusCode,
		ContentType: contentType,
		Headers:     headers,
		Body:        bodyCopy,
		CachedAt:    cachedAt,
		ExpiresAt:   expiresAt,
	}, nil
}

// ReadBody returns an io.ReadCloser for the body portion of a cached entry,
// avoiding a full []byte allocation in user space.
func (r *DiskReader) ReadBody(loc cacheLocation, expectedKey string) (body io.ReadCloser, bodyLen int64, hdr http.Header, statusCode int, err error) {
	seg, segErr := r.getSegment(loc.SegmentID)
	if segErr != nil {
		err = segErr
		return
	}

	f, openErr := os.Open(seg.Path)
	if openErr != nil {
		err = fmt.Errorf("open segment %d: %w", loc.SegmentID, openErr)
		return
	}

	buf := make([]byte, EntryHeaderSize)
	if _, rErr := f.ReadAt(buf, int64(loc.BodyOffset)); rErr != nil {
		f.Close()
		err = fmt.Errorf("read entry header: %w", rErr)
		return
	}

	keyLen := int(binary.BigEndian.Uint16(buf[10:12]))
	contentTypeLen := int(binary.BigEndian.Uint16(buf[24:26]))
	headersLen := int(binary.BigEndian.Uint16(buf[26:28]))
	bLen := int(binary.BigEndian.Uint32(buf[6:10]))
	totalSize := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bLen

	entry := make([]byte, totalSize)
	if _, rErr := f.ReadAt(entry, int64(loc.BodyOffset)); rErr != nil {
		f.Close()
		err = fmt.Errorf("read entry data: %w", rErr)
		return
	}

	expectedCRC := binary.BigEndian.Uint32(entry[0:4])
	if crc32.ChecksumIEEE(entry[4:]) != expectedCRC {
		f.Close()
		err = fmt.Errorf("crc mismatch at segment %d offset %d", loc.SegmentID, loc.BodyOffset)
		return
	}

	statusCode = int(binary.BigEndian.Uint16(buf[4:6]))
	key := string(entry[EntryHeaderSize : EntryHeaderSize+keyLen])
	if key != expectedKey {
		f.Close()
		err = fmt.Errorf("key mismatch at segment %d offset %d: expected %q, got %q", loc.SegmentID, loc.BodyOffset, expectedKey, key)
		return
	}

	headersOff := EntryHeaderSize + keyLen + contentTypeLen
	contentType := string(entry[EntryHeaderSize+keyLen : headersOff])
	hdrBytes := entry[headersOff : headersOff+headersLen]

	statusCode = int(binary.BigEndian.Uint16(buf[4:6]))
	hdr = make(http.Header)
	dec := gob.NewDecoder(bytes.NewReader(hdrBytes))
	if dErr := dec.Decode(&hdr); dErr != nil {
		f.Close()
		err = fmt.Errorf("gob decode headers: %w", dErr)
		return
	}
	hdr.Set("Content-Type", contentType)

	bodyStart := int64(loc.BodyOffset) + int64(EntryHeaderSize+keyLen+contentTypeLen+headersLen)
	body = &fileSection{
		f:      f,
		reader: io.NewSectionReader(f, bodyStart, int64(bLen)),
	}
	bodyLen = int64(bLen)
	return
}

// fileSection wraps *os.File + io.SectionReader to provide a ReadCloser
// for a specific byte range of a file.
type fileSection struct {
	f      *os.File
	reader *io.SectionReader
}

func (fs *fileSection) Read(p []byte) (int, error) {
	return fs.reader.Read(p)
}

func (fs *fileSection) Close() error {
	return fs.f.Close()
}

func (r *DiskReader) ScanSegment(segmentID uint64) (map[string]*CacheEntry, error) {
	seg, err := r.getSegment(segmentID)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(seg.Path)
	if err != nil {
		return nil, fmt.Errorf("open segment %d: %w", segmentID, err)
	}
	defer f.Close()

	sh, err := ReadSegmentHeader(f)
	if err != nil {
		return nil, fmt.Errorf("read segment %d header: %w", segmentID, err)
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat segment %d: %w", segmentID, err)
	}

	indexes, err := ReadEntryIndexTable(f, fi.Size(), sh.Entries)
	if err != nil {
		return nil, fmt.Errorf("read segment %d index table: %w", segmentID, err)
	}

	result := make(map[string]*CacheEntry, len(indexes))
	for _, idx := range indexes {
		header := make([]byte, EntryHeaderSize)
		if _, err := f.ReadAt(header, int64(idx.BodyOffset)); err != nil {
			return nil, fmt.Errorf("read entry header at segment %d offset %d: %w", segmentID, idx.BodyOffset, err)
		}

		keyLen := int(binary.BigEndian.Uint16(header[10:12]))
		contentTypeLen := int(binary.BigEndian.Uint16(header[24:26]))
		headersLen := int(binary.BigEndian.Uint16(header[26:28]))
		bodyLen := int(binary.BigEndian.Uint32(header[6:10]))
		totalSize := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen

		data := make([]byte, totalSize)
		if _, err := f.ReadAt(data, int64(idx.BodyOffset)); err != nil {
			return nil, fmt.Errorf("read entry data at segment %d offset %d: %w", segmentID, idx.BodyOffset, err)
		}

		key, statusCode, contentType, headers, body, expiresAt, cachedAt, err := UnmarshalEntry(data)
		if err != nil {
			return nil, fmt.Errorf("unmarshal entry at segment %d offset %d: %w", segmentID, idx.BodyOffset, err)
		}

		result[key] = &CacheEntry{
			StatusCode:  statusCode,
			ContentType: contentType,
			Headers:     headers,
			Body:        body,
			CachedAt:    cachedAt,
			ExpiresAt:   expiresAt,
		}
	}

	return result, nil
}

func (r *DiskReader) ReadEntryData(loc cacheLocation) ([]byte, error) {
	seg, err := r.getSegment(loc.SegmentID)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(seg.Path)
	if err != nil {
		return nil, fmt.Errorf("open segment %d: %w", loc.SegmentID, err)
	}
	defer f.Close()

	header := make([]byte, EntryHeaderSize)
	if _, err := f.ReadAt(header, int64(loc.BodyOffset)); err != nil {
		return nil, fmt.Errorf("read entry header at segment %d offset %d: %w", loc.SegmentID, loc.BodyOffset, err)
	}

	keyLen := int(binary.BigEndian.Uint16(header[10:12]))
	contentTypeLen := int(binary.BigEndian.Uint16(header[24:26]))
	headersLen := int(binary.BigEndian.Uint16(header[26:28]))
	bodyLen := int(binary.BigEndian.Uint32(header[6:10]))
	totalSize := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen

	data := make([]byte, totalSize)
	if _, err := f.ReadAt(data, int64(loc.BodyOffset)); err != nil {
		return nil, fmt.Errorf("read entry data at segment %d offset %d: %w", loc.SegmentID, loc.BodyOffset, err)
	}

	return data, nil
}

func (r *DiskReader) SegmentCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.segments)
}

func (r *DiskReader) Segments() []segmentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]segmentInfo, 0, len(r.segments))
	for _, seg := range r.segments {
		result = append(result, *seg)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreateTime != result[j].CreateTime {
			return result[i].CreateTime < result[j].CreateTime
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (r *DiskReader) SegmentIDs() []uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]uint64, 0, len(r.segments))
	for id := range r.segments {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool {
		a := r.segments[result[i]]
		b := r.segments[result[j]]
		if a.CreateTime != b.CreateTime {
			return a.CreateTime < b.CreateTime
		}
		return a.ID < b.ID
	})
	return result
}

func (r *DiskReader) Stop() {
}
