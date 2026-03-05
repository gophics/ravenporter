package ogg

type bitReader struct {
	data []byte
	pos  int
	bits uint64
	n    int
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (br *bitReader) readBits(n int) uint32 {
	for br.n < n {
		if br.pos >= len(br.data) {
			break
		}
		br.bits |= uint64(br.data[br.pos]) << br.n
		br.n += 8
		br.pos++
	}

	if br.n < n {
		br.n = 0
		return 0
	}

	res := uint32(br.bits & ((1 << n) - 1)) //nolint:gosec
	br.bits >>= n
	br.n -= n
	return res
}

func (br *bitReader) readBit() uint32 {
	return br.readBits(1)
}

func ilog(v int) int {
	res := 0
	for v > 0 {
		res++
		v >>= 1
	}
	return res
}
