package cache

import (
	"encoding/binary"
	"io"
	"sync"
)

type chunkMeta struct {
	offset uint64
	size   uint64
}

type containerSections struct {
	manifest []byte
	scene    []byte
	blobs    *blobStore
}

type blobStore struct {
	reader       io.ReaderAt
	baseOffset   int64
	size         uint64
	closer       io.Closer
	closerNeeded bool

	mu     sync.Mutex
	closed bool
}

func writeContainer(w io.Writer, chunks []chunk) error {
	header := make([]byte, 0, headerSize+len(chunks)*tableEntrySize)
	header = append(header, cacheMagic[:]...)
	header = binary.LittleEndian.AppendUint16(header, formatVersion)
	header = binary.LittleEndian.AppendUint16(header, 0)
	chunkCount, err := toUint32(len(chunks))
	if err != nil {
		return err
	}
	header = binary.LittleEndian.AppendUint32(header, chunkCount)

	offset := uint64(headerSize)
	chunksLen, err := toUint64(len(chunks))
	if err != nil {
		return err
	}
	offset += chunksLen * tableEntrySize
	for _, chunk := range chunks {
		header = append(header, chunk.id[:]...)
		header = binary.LittleEndian.AppendUint64(header, offset)
		chunkSize, err := toUint64(len(chunk.data))
		if err != nil {
			return err
		}
		header = binary.LittleEndian.AppendUint64(header, chunkSize)
		offset += chunkSize
	}

	if _, err := w.Write(header); err != nil {
		return err
	}
	for _, chunk := range chunks {
		if _, err := w.Write(chunk.data); err != nil {
			return err
		}
	}
	return nil
}

func readContainer(r io.ReaderAt, size int64, cfg readConfig) (containerSections, error) {
	if size < headerSize {
		return containerSections{}, fmtErrorf("%w: truncated header", errInvalidCache)
	}

	header := make([]byte, headerSize)
	if err := readAtFull(r, header, 0); err != nil {
		return containerSections{}, err
	}
	var magic [8]byte
	copy(magic[:], header[:8])
	if magic != cacheMagic {
		return containerSections{}, fmtErrorf("%w: bad magic", errInvalidCache)
	}
	version := binary.LittleEndian.Uint16(header[8:10])
	if version != formatVersion {
		return containerSections{}, fmtErrorf("%w: unsupported version %d", errInvalidCache, version)
	}

	chunkCount := binary.LittleEndian.Uint32(header[12:16])
	tableSize := int64(chunkCount) * tableEntrySize
	if chunkCount == 0 || size < headerSize+tableSize {
		return containerSections{}, fmtErrorf("%w: truncated chunk table", errInvalidCache)
	}

	table := make([]byte, tableSize)
	if err := readAtFull(r, table, headerSize); err != nil {
		return containerSections{}, err
	}

	metas := make(map[[4]byte]chunkMeta, chunkCount)
	for i := uint32(0); i < chunkCount; i++ {
		base := i * tableEntrySize
		var id [4]byte
		copy(id[:], table[base:base+4])
		offset := binary.LittleEndian.Uint64(table[base+4 : base+12])
		chunkSize := binary.LittleEndian.Uint64(table[base+12 : base+20])
		if offset > uint64(size) || chunkSize > uint64(size) || offset+chunkSize > uint64(size) {
			return containerSections{}, fmtErrorf("%w: chunk %q out of bounds", errInvalidCache, string(id[:]))
		}
		metas[id] = chunkMeta{offset: offset, size: chunkSize}
	}

	required := [][4]byte{chunkManifest, chunkScene, chunkBlob}
	for _, id := range required {
		if _, ok := metas[id]; !ok {
			return containerSections{}, fmtErrorf("%w: missing chunk %q", errInvalidCache, string(id[:]))
		}
	}

	manifest, err := readChunkData(r, metas[chunkManifest])
	if err != nil {
		return containerSections{}, err
	}
	scene, err := readChunkData(r, metas[chunkScene])
	if err != nil {
		return containerSections{}, err
	}
	blobMeta := metas[chunkBlob]
	var blobs *blobStore
	if cfg.eagerMedia {
		blobData, err := readChunkData(r, blobMeta)
		if err != nil {
			return containerSections{}, err
		}
		blobs = &blobStore{reader: bytesReaderAt(blobData), size: uint64(len(blobData))}
	} else {
		baseOffset, err := toInt64(blobMeta.offset)
		if err != nil {
			return containerSections{}, err
		}
		blobs = &blobStore{
			reader:       r,
			baseOffset:   baseOffset,
			size:         blobMeta.size,
			closerNeeded: true,
		}
	}
	return containerSections{manifest: manifest, scene: scene, blobs: blobs}, nil
}

func readAtFull(r io.ReaderAt, data []byte, offset int64) error {
	n, err := r.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return err
	}
	if n != len(data) {
		return fmtErrorf("%w: truncated chunk payload", errInvalidCache)
	}
	return nil
}

func readChunkData(r io.ReaderAt, meta chunkMeta) ([]byte, error) {
	data := make([]byte, meta.size)
	offset, err := toInt64(meta.offset)
	if err != nil {
		return nil, err
	}
	if err := readAtFull(r, data, offset); err != nil {
		return nil, err
	}
	return data, nil
}

func (b *blobStore) attachCloser(closer io.Closer) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closerNeeded || b.closed || b.closer != nil {
		return false
	}
	b.closer = closer
	return true
}

func (b *blobStore) close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	closer := b.closer
	b.closer = nil
	b.reader = nil
	b.mu.Unlock()
	if closer != nil {
		return closer.Close()
	}
	return nil
}
