package testutil

import (
	"errors"
	"io"
)

type OversizeReadSeeker struct {
	Size  int64
	Pos   int64
	Reads int
}

func NewOversizeReadSeeker(size int64) *OversizeReadSeeker {
	return &OversizeReadSeeker{Size: size}
}

func (r *OversizeReadSeeker) Read([]byte) (int, error) {
	r.Reads++
	return 0, errors.New("unexpected read")
}

func (r *OversizeReadSeeker) ReadAt([]byte, int64) (int, error) {
	r.Reads++
	return 0, errors.New("unexpected read")
}

func (r *OversizeReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.Pos = offset
	case io.SeekCurrent:
		r.Pos += offset
	case io.SeekEnd:
		r.Pos = r.Size + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if r.Pos < 0 {
		return 0, errors.New("negative position")
	}
	return r.Pos, nil
}
