package opus

import (
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/pool"
)

const (
	pageHeaderSize   = 27
	flagContinuation = 0x01
	flagBOS          = 0x02
	flagEOS          = 0x04
	demuxBufSize     = 65536
	lacingBufSize    = 256
)

var (
	errBadMagic = errors.New("opus: bad page magic")
)

type oggPage struct {
	version      uint8
	headerType   uint8
	granulePos   int64
	serial       uint32
	sequenceNo   uint32
	checksum     uint32
	segmentCount uint8
	lacingVals   []uint8
	body         []byte
}

type oggDemuxer struct {
	r             detect.ReadSeekerAt
	buf           []byte
	lacingBuf     []uint8
	bodyBuf       []byte
	packetBuf     []byte
	page          oggPage
	packet        oggPacket
	primarySerial uint32
	serialKnown   bool
}

func acquireOggDemuxer(r detect.ReadSeekerAt) *oggDemuxer {
	return &oggDemuxer{
		r:         r,
		buf:       pool.GetBuffer(demuxBufSize),
		lacingBuf: make([]uint8, lacingBufSize),
		bodyBuf:   pool.GetBuffer(demuxBufSize),
		packetBuf: pool.GetBuffer(demuxBufSize),
	}
}

func releaseOggDemuxer(d *oggDemuxer) {
	pool.PutBuffer(d.buf)
	pool.PutBuffer(d.bodyBuf)
	pool.PutBuffer(d.packetBuf)
	d.r = nil
	d.buf = nil
	d.bodyBuf = nil
	d.packetBuf = nil
}

func (d *oggDemuxer) readPage() error {
	header := d.buf[:pageHeaderSize]
	_, err := io.ReadFull(d.r, header)
	if err != nil {
		return err
	}

	if string(header[:4]) != "OggS" {
		return errBadMagic
	}

	d.page.version = header[4]
	d.page.headerType = header[5]
	d.page.granulePos = int64(binread.ReadU64LE(header[6:14])) //nolint:gosec
	d.page.serial = binread.ReadU32LE(header[14:18])
	d.page.sequenceNo = binread.ReadU32LE(header[18:22])
	d.page.checksum = binread.ReadU32LE(header[22:26])
	d.page.segmentCount = header[26]
	d.page.lacingVals = d.lacingBuf[:header[26]]

	if d.page.segmentCount > 0 {
		_, err = io.ReadFull(d.r, d.page.lacingVals)
		if err != nil {
			return err
		}
	}

	bodyLen := 0
	for _, v := range d.page.lacingVals {
		bodyLen += int(v)
	}

	if bodyLen > 0 {
		d.page.body = d.bodyBuf[:bodyLen]
		_, err = io.ReadFull(d.r, d.page.body)
		if err != nil {
			return err
		}
	} else {
		d.page.body = d.bodyBuf[:0]
	}

	return nil
}

type oggPacket struct {
	data   []byte
	bos    bool
	eos    bool
	serial uint32
}

func (d *oggDemuxer) readNextPacket() error {
	body := d.packetBuf[:0]
	bos := false
	eos := false
	var serial uint32

	for {
		if err := d.readPage(); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, errBadMagic) {
				if d.resync() {
					continue
				}
			}
			if len(body) > 0 {
				break
			}
			return err
		}

		isBOS := (d.page.headerType & flagBOS) != 0

		if !d.serialKnown && isBOS {
			d.primarySerial = d.page.serial
			d.serialKnown = true
		}

		if d.serialKnown && d.page.serial != d.primarySerial {
			if isBOS {
				d.primarySerial = d.page.serial
			} else {
				continue
			}
		}

		serial = d.page.serial

		if len(body) == 0 {
			bos = isBOS
		}
		eos = (d.page.headerType & flagEOS) != 0

		body = append(body, d.page.body...)

		if len(d.page.lacingVals) > 0 && d.page.lacingVals[len(d.page.lacingVals)-1] < 255 {
			break
		}
		if len(d.page.lacingVals) == 0 {
			break
		}
	}

	d.packet.data = body
	d.packet.bos = bos
	d.packet.eos = eos
	d.packet.serial = serial
	return nil
}

func (d *oggDemuxer) resync() bool {
	var scan [4096]byte
	n, err := d.r.Read(scan[:])
	if err != nil && err != io.EOF {
		return false
	}
	for i := 0; i < n-3; i++ {
		if scan[i] == 'O' && scan[i+1] == 'g' && scan[i+2] == 'g' && scan[i+3] == 'S' {
			if _, err := d.r.Seek(int64(i-n), io.SeekCurrent); err == nil {
				return true
			}
		}
	}
	return false
}
