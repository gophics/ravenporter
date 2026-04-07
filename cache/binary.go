package cache

import (
	"encoding/binary"
	"math"
)

const (
	uint16Bytes  = 2
	uint32Bytes  = 4
	uint64Bytes  = 8
	int32Bytes   = 4
	float32Bytes = 4
	shift8       = 8
	shift16      = 16
	shift24      = 24
	shift32      = 32
	shift40      = 40
	shift48      = 48
	shift56      = 56
)

type encoder struct {
	data []byte
	err  error
}

func (e *encoder) bytes() []byte {
	return e.data
}

func (e *encoder) fail(err error) {
	if e.err == nil {
		e.err = err
	}
}

func (e *encoder) bool(value bool) {
	if value {
		e.u8(1)
		return
	}
	e.u8(0)
}

func (e *encoder) u8(value uint8) {
	if e.err != nil {
		return
	}
	e.data = append(e.data, value)
}

func (e *encoder) u16(value uint16) {
	if e.err != nil {
		return
	}
	e.data = binary.LittleEndian.AppendUint16(e.data, value)
}

func (e *encoder) u32(value uint32) {
	if e.err != nil {
		return
	}
	e.data = binary.LittleEndian.AppendUint32(e.data, value)
}

func (e *encoder) u64(value uint64) {
	if e.err != nil {
		return
	}
	e.data = binary.LittleEndian.AppendUint64(e.data, value)
}

func (e *encoder) i32(value int32) {
	if e.err != nil {
		return
	}
	e.data = append(e.data,
		byte(value),
		byte(value>>shift8),
		byte(value>>shift16),
		byte(value>>shift24),
	)
}

func (e *encoder) i64(value int64) {
	if e.err != nil {
		return
	}
	e.data = append(e.data,
		byte(value),
		byte(value>>shift8),
		byte(value>>shift16),
		byte(value>>shift24),
		byte(value>>shift32),
		byte(value>>shift40),
		byte(value>>shift48),
		byte(value>>shift56),
	)
}

func (e *encoder) f32(value float32) {
	e.u32(math.Float32bits(value))
}

func (e *encoder) f64(value float64) {
	e.u64(math.Float64bits(value))
}

func (e *encoder) string(value string) {
	e.rawBytes([]byte(value))
}

func (e *encoder) rawBytes(value []byte) {
	if e.err != nil {
		return
	}
	e.count(len(value))
	if e.err != nil {
		return
	}
	e.data = append(e.data, value...)
}

func (e *encoder) ints(values []int) {
	e.count(len(values))
	for _, value := range values {
		e.int(value)
	}
}

func (e *encoder) float32s(values []float32) {
	e.count(len(values))
	for _, value := range values {
		e.f32(value)
	}
}

func (e *encoder) uint32s(values []uint32) {
	e.count(len(values))
	for _, value := range values {
		e.u32(value)
	}
}

func (e *encoder) runes(values []rune) {
	e.count(len(values))
	for _, value := range values {
		e.i32(value)
	}
}

func (e *encoder) int(value int) {
	if e.err != nil {
		return
	}
	encoded, err := toInt32(value)
	if err != nil {
		e.fail(err)
		return
	}
	e.i32(encoded)
}

func (e *encoder) count(value int) {
	if e.err != nil {
		return
	}
	encoded, err := toUint32(value)
	if err != nil {
		e.fail(err)
		return
	}
	e.u32(encoded)
}

type decoder struct {
	data []byte
	pos  int
	err  error
}

func (d *decoder) remaining() int {
	return len(d.data) - d.pos
}

func (d *decoder) fail(err error) {
	if d.err == nil {
		d.err = err
	}
}

func (d *decoder) slice(n int) []byte {
	if d.err != nil {
		return nil
	}
	if n < 0 || d.remaining() < n {
		d.fail(fmtErrorf("%w: truncated scene data", errInvalidCache))
		return nil
	}
	value := d.data[d.pos : d.pos+n]
	d.pos += n
	return value
}

func (d *decoder) bool() bool {
	value := d.u8()
	if value > 1 {
		d.fail(fmtErrorf("%w: invalid boolean value %d", errInvalidCache, value))
		return false
	}
	return value == 1
}

func (d *decoder) u8() uint8 {
	data := d.slice(1)
	if data == nil {
		return 0
	}
	return data[0]
}

func (d *decoder) u16() uint16 {
	data := d.slice(uint16Bytes)
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

func (d *decoder) u32() uint32 {
	data := d.slice(uint32Bytes)
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}

func (d *decoder) u64() uint64 {
	data := d.slice(uint64Bytes)
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(data)
}

func (d *decoder) i32() int32 {
	data := d.slice(uint32Bytes)
	if data == nil {
		return 0
	}
	return int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16 | int32(data[3])<<24
}

func (d *decoder) i64() int64 {
	data := d.slice(uint64Bytes)
	if data == nil {
		return 0
	}
	return int64(data[0]) |
		int64(data[1])<<8 |
		int64(data[2])<<16 |
		int64(data[3])<<24 |
		int64(data[4])<<32 |
		int64(data[5])<<40 |
		int64(data[6])<<48 |
		int64(data[7])<<56
}

func (d *decoder) f32() float32 {
	return math.Float32frombits(d.u32())
}

func (d *decoder) f64() float64 {
	return math.Float64frombits(d.u64())
}

func (d *decoder) string() string {
	return string(d.rawBytes())
}

func (d *decoder) rawBytes() []byte {
	return d.slice(d.length())
}

func (d *decoder) ints() []int {
	count := d.count(int32Bytes)
	values := make([]int, count)
	for i := range values {
		values[i] = int(d.i32())
	}
	return values
}

func (d *decoder) float32s() []float32 {
	count := d.count(float32Bytes)
	values := make([]float32, count)
	for i := range values {
		values[i] = d.f32()
	}
	return values
}

func (d *decoder) uint32s() []uint32 {
	count := d.count(uint32Bytes)
	values := make([]uint32, count)
	for i := range values {
		values[i] = d.u32()
	}
	return values
}

func (d *decoder) runes() []rune {
	count := d.count(int32Bytes)
	values := make([]rune, count)
	for i := range values {
		values[i] = d.i32()
	}
	return values
}

func (d *decoder) length() int {
	length := d.u32()
	if d.err != nil {
		return 0
	}
	if uint64(length) > maxIntValue() {
		d.fail(fmtErrorf("%w: length %d exceeds int", errInvalidCache, length))
		return 0
	}
	intLength := int(length)
	if intLength > d.remaining() {
		d.fail(fmtErrorf("%w: truncated scene data", errInvalidCache))
		return 0
	}
	return intLength
}

func (d *decoder) count(minBytesPerItem int) int {
	count := d.u32()
	if d.err != nil {
		return 0
	}
	if uint64(count) > maxIntValue() {
		d.fail(fmtErrorf("%w: count %d exceeds int", errInvalidCache, count))
		return 0
	}
	intCount := int(count)
	if minBytesPerItem > 0 && intCount > 0 && intCount > d.remaining()/minBytesPerItem {
		d.fail(fmtErrorf("%w: invalid element count %d", errInvalidCache, count))
		return 0
	}
	return intCount
}

func toUint32(value int) (uint32, error) {
	if value < 0 || uint64(value) > math.MaxUint32 {
		return 0, fmtErrorf("cache: count %d exceeds uint32", value)
	}
	return uint32(value), nil //nolint:gosec // range validated above
}

func toInt32(value int) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmtErrorf("cache: value %d exceeds int32", value)
	}
	return int32(value), nil
}

func toUint64(value int) (uint64, error) {
	if value < 0 {
		return 0, fmtErrorf("cache: negative length %d", value)
	}
	return uint64(value), nil
}

func toInt64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmtErrorf("cache: value %d exceeds int64", value)
	}
	return int64(value), nil
}

func maxIntValue() uint64 {
	return uint64(^uint(0) >> 1)
}
