package ogg

import (
	"io"
	"strconv"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	pageSizeMax    = 65307
	pageHeaderSize = 27
	flagContinued  = 0x01
	flagBOS        = 0x02
	flagEOS        = 0x04
	maxLacing      = 255
	maxBodySize    = 65025
)

type oggPage struct {
	version      uint8
	headerType   uint8
	granulePos   int64
	serial       uint32
	sequence     uint32
	checksum     uint32
	segments     uint8
	lacingValues []uint8
	body         []byte
}

type oggDemuxer struct {
	r          io.Reader
	headerBuf  []byte
	lacingBuf  []uint8
	bodyBuf    []byte
	pktBuf     []byte
	offset     int64
	page       oggPage
	segmentIdx int
	bodyOffset int
	pageLoaded bool
}

func newDemuxer(r io.Reader) *oggDemuxer {
	return &oggDemuxer{
		r:         r,
		headerBuf: make([]byte, pageHeaderSize),
		lacingBuf: make([]uint8, maxLacing),
		bodyBuf:   make([]byte, maxBodySize),
		pktBuf:    make([]byte, 0, maxBodySize),
	}
}

func (d *oggDemuxer) readPage() error {
	_, err := io.ReadFull(d.r, d.headerBuf)
	if err != nil {
		return err
	}
	d.offset += pageHeaderSize

	if string(d.headerBuf[:4]) != pageMagic {
		return decutil.DecodeErr(ir.FormatOGG, "bad page magic", nil)
	}

	d.page = oggPage{
		version:    d.headerBuf[4],
		headerType: d.headerBuf[5],
		granulePos: int64(binread.ReadU64LE(d.headerBuf[6:14])), //nolint:gosec
		serial:     binread.ReadU32LE(d.headerBuf[14:18]),
		sequence:   binread.ReadU32LE(d.headerBuf[18:22]),
		checksum:   binread.ReadU32LE(d.headerBuf[22:26]),
		segments:   d.headerBuf[26],
	}

	p := &d.page
	if p.version != 0 {
		return decutil.DecodeErr(ir.FormatOGG, "unsupported version: "+strconv.Itoa(int(p.version)), nil)
	}

	if p.segments > 0 {
		p.lacingValues = d.lacingBuf[:p.segments]
		_, err = io.ReadFull(d.r, p.lacingValues)
		if err != nil {
			return err
		}
		d.offset += int64(p.segments)
	}

	var bodySize int
	for _, l := range p.lacingValues {
		bodySize += int(l)
	}

	if bodySize > 0 {
		p.body = d.bodyBuf[:bodySize]
		_, err = io.ReadFull(d.r, p.body)
		if err != nil {
			return err
		}
		d.offset += int64(bodySize)
	}

	return nil
}

type oggPacket struct {
	data   []byte
	serial uint32
	bos    bool
	eos    bool
}

func (d *oggDemuxer) readNextPacket() (oggPacket, error) {
	d.pktBuf = d.pktBuf[:0]
	var isBOS, isEOS bool

	for {
		if !d.pageLoaded || d.segmentIdx >= len(d.page.lacingValues) {
			if err := d.readPage(); err != nil {
				return oggPacket{}, err
			}
			d.pageLoaded = true
			d.segmentIdx = 0
			d.bodyOffset = 0
		}

		if d.segmentIdx == 0 {
			if d.page.headerType&flagBOS != 0 {
				isBOS = true
			}
			if d.page.headerType&flagEOS != 0 {
				isEOS = true
			}
		}

		for d.segmentIdx < len(d.page.lacingValues) {
			l := d.page.lacingValues[d.segmentIdx]
			d.pktBuf = append(d.pktBuf, d.page.body[d.bodyOffset:d.bodyOffset+int(l)]...)
			d.bodyOffset += int(l)
			d.segmentIdx++

			if l < maxLacing {
				return oggPacket{
					data:   d.pktBuf,
					serial: d.page.serial,
					bos:    isBOS,
					eos:    isEOS,
				}, nil
			}
		}
	}
}

const (
	granulePosOff    = 6
	granulePosEnd    = 14
	lastPageScanSize = 65536
)

func readLastGranulePos(r io.ReadSeeker) int64 {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil || size < pageHeaderSize {
		return 0
	}

	scanSize := int64(lastPageScanSize)
	if scanSize > size {
		scanSize = size
	}

	if _, err := r.Seek(-scanSize, io.SeekEnd); err != nil {
		return 0
	}

	buf := pool.GetBuffer(int(scanSize))
	defer pool.PutBuffer(buf)

	n, err := io.ReadFull(r, buf[:scanSize])
	if err != nil && n == 0 {
		return 0
	}
	buf = buf[:n]

	lastOff := -1
	for i := n - pageHeaderSize; i >= 0; i-- {
		if buf[i] == 'O' && i+3 < n && buf[i+1] == 'g' && buf[i+2] == 'g' && buf[i+3] == 'S' {
			lastOff = i
			break
		}
	}
	if lastOff < 0 || lastOff+granulePosEnd > n {
		return 0
	}

	return int64(binread.ReadU64LE(buf[lastOff+granulePosOff : lastOff+granulePosEnd])) //nolint:gosec
}
