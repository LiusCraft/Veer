package edge

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
	"hash/fnv"
	"io"
	"net/http"
	"time"
)

const (
	SegmentHeaderSize = 64
	EntryHeaderSize   = 28
	IndexEntrySize    = 20
	SegmentMagic      = uint32(0x56454552)
	SegmentVersion    = uint16(1)
)

type IndexEntry struct {
	KeyHash    uint64
	BodyOffset uint64
	BodyLen    uint32
}

type SegmentHeader struct {
	Magic      uint32
	Version    uint16
	CreateTime int64
	Entries    uint64
	DataOffset uint64
	CRC        uint64
	Reserved   [26]byte
}

func MarshalEntry(key string, statusCode int, contentType string, headers http.Header, body []byte, expiresAt time.Time, cachedAt time.Time) ([]byte, error) {
	keyBytes := []byte(key)
	contentTypeBytes := []byte(contentType)

	var headersBuf bytes.Buffer
	enc := gob.NewEncoder(&headersBuf)
	if err := enc.Encode(headers); err != nil {
		return nil, fmt.Errorf("gob encode headers: %w", err)
	}
	headersData := headersBuf.Bytes()

	keyLen := len(keyBytes)
	contentTypeLen := len(contentTypeBytes)
	headersLen := len(headersData)
	bodyLen := len(body)

	if keyLen > 65535 {
		return nil, errors.New("key length exceeds maximum")
	}
	if contentTypeLen > 65535 {
		return nil, errors.New("content type length exceeds maximum")
	}
	if headersLen > 65535 {
		return nil, errors.New("headers length exceeds maximum")
	}

	totalSize := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen
	buf := make([]byte, totalSize)

	off := 4
	binary.BigEndian.PutUint16(buf[off:], uint16(statusCode))
	off += 2

	binary.BigEndian.PutUint32(buf[off:], uint32(bodyLen))
	off += 4

	binary.BigEndian.PutUint16(buf[off:], uint16(keyLen))
	off += 2

	binary.BigEndian.PutUint64(buf[off:], uint64(expiresAt.UnixNano()))
	off += 8

	binary.BigEndian.PutUint32(buf[off:], uint32(cachedAt.Unix()))
	off += 4

	binary.BigEndian.PutUint16(buf[off:], uint16(contentTypeLen))
	off += 2

	binary.BigEndian.PutUint16(buf[off:], uint16(headersLen))
	off += 2

	copy(buf[off:], keyBytes)
	off += keyLen

	copy(buf[off:], contentTypeBytes)
	off += contentTypeLen

	copy(buf[off:], headersData)
	off += headersLen

	copy(buf[off:], body)

	crc := crc32.ChecksumIEEE(buf[4:])
	binary.BigEndian.PutUint32(buf[0:4], crc)

	return buf, nil
}

func UnmarshalEntry(data []byte) (key string, statusCode int, contentType string, headers http.Header, body []byte, expiresAt time.Time, cachedAt time.Time, err error) {
	if len(data) < EntryHeaderSize {
		err = errors.New("entry data too short for header")
		return
	}

	expectedCRC := binary.BigEndian.Uint32(data[0:4])
	actualCRC := crc32.ChecksumIEEE(data[4:])
	if actualCRC != expectedCRC {
		err = fmt.Errorf("crc32 mismatch: expected %08x, actual %08x", expectedCRC, actualCRC)
		return
	}

	off := 4
	statusCode = int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2

	bodyLen := int(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4

	keyLen := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2

	expiresAtUnix := binary.BigEndian.Uint64(data[off : off+8])
	expiresAt = time.Unix(0, int64(expiresAtUnix))
	off += 8

	cachedAtUnix := binary.BigEndian.Uint32(data[off : off+4])
	cachedAt = time.Unix(int64(cachedAtUnix), 0)
	off += 4

	contentTypeLen := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2

	headersLen := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2

	expectedLen := EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen
	if len(data) < expectedLen {
		err = errors.New("entry data truncated")
		return
	}

	key = string(data[off : off+keyLen])
	off += keyLen

	contentType = string(data[off : off+contentTypeLen])
	off += contentTypeLen

	headersData := data[off : off+headersLen]
	off += headersLen

	body = data[off : off+bodyLen]

	headers = make(http.Header)
	dec := gob.NewDecoder(bytes.NewReader(headersData))
	if derr := dec.Decode(&headers); derr != nil {
		err = fmt.Errorf("gob decode headers: %w", derr)
		return
	}

	return
}

func WriteSegmentHeader(w io.Writer, createTime int64, entries uint64, dataOffset uint64) error {
	b := make([]byte, SegmentHeaderSize)

	binary.BigEndian.PutUint32(b[0:4], SegmentMagic)
	binary.BigEndian.PutUint16(b[4:6], SegmentVersion)
	binary.BigEndian.PutUint64(b[6:14], uint64(createTime))
	binary.BigEndian.PutUint64(b[14:22], entries)
	binary.BigEndian.PutUint64(b[22:30], dataOffset)
	binary.BigEndian.PutUint64(b[30:38], 0)

	_, err := w.Write(b)
	return err
}

func ReadSegmentHeader(r io.Reader) (*SegmentHeader, error) {
	b := make([]byte, SegmentHeaderSize)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read segment header: %w", err)
	}

	h := &SegmentHeader{
		Magic:      binary.BigEndian.Uint32(b[0:4]),
		Version:    binary.BigEndian.Uint16(b[4:6]),
		CreateTime: int64(binary.BigEndian.Uint64(b[6:14])),
		Entries:    binary.BigEndian.Uint64(b[14:22]),
		DataOffset: binary.BigEndian.Uint64(b[22:30]),
		CRC:        binary.BigEndian.Uint64(b[30:38]),
	}
	copy(h.Reserved[:], b[38:64])

	if h.Magic != SegmentMagic {
		return nil, fmt.Errorf("invalid segment magic: expected %08x, got %08x", SegmentMagic, h.Magic)
	}
	if h.Version != SegmentVersion {
		return nil, fmt.Errorf("unsupported segment version: expected %d, got %d", SegmentVersion, h.Version)
	}

	return h, nil
}

func ReadEntryIndexTable(r io.ReaderAt, fileEnd int64, entryCount uint64) ([]IndexEntry, error) {
	if entryCount == 0 {
		return nil, nil
	}

	tableSize := int64(entryCount) * IndexEntrySize
	if tableSize > fileEnd {
		return nil, fmt.Errorf("index table size %d exceeds file size %d", tableSize, fileEnd)
	}

	offset := fileEnd - tableSize
	b := make([]byte, tableSize)
	if _, err := r.ReadAt(b, offset); err != nil {
		return nil, fmt.Errorf("read index table at offset %d: %w", offset, err)
	}

	entries := make([]IndexEntry, entryCount)
	for i := uint64(0); i < entryCount; i++ {
		base := i * IndexEntrySize
		entries[i] = IndexEntry{
			KeyHash:    binary.BigEndian.Uint64(b[base : base+8]),
			BodyOffset: binary.BigEndian.Uint64(b[base+8 : base+16]),
			BodyLen:    binary.BigEndian.Uint32(b[base+16 : base+20]),
		}
	}

	return entries, nil
}

func WriteEntryIndexTable(w io.Writer, entries []IndexEntry) error {
	b := make([]byte, len(entries)*IndexEntrySize)
	for i, e := range entries {
		base := i * IndexEntrySize
		binary.BigEndian.PutUint64(b[base:base+8], e.KeyHash)
		binary.BigEndian.PutUint64(b[base+8:base+16], e.BodyOffset)
		binary.BigEndian.PutUint32(b[base+16:base+20], e.BodyLen)
	}
	_, err := w.Write(b)
	return err
}

func ComputeKeyHash(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}

func EntryWireSize(keyLen int, contentTypeLen int, headersLen int, bodyLen int) int {
	return EntryHeaderSize + keyLen + contentTypeLen + headersLen + bodyLen
}
