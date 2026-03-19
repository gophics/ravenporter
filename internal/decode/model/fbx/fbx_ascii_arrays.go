package fbx

import (
	"bytes"
	"strconv"

	"github.com/gophics/ravenporter/internal/decutil"
)

func scanASCIIFields[T any](s *decutil.LineScanner, capacity int, parse func([]byte) (T, bool)) []T {
	var result []T
	if capacity > 0 {
		result = make([]T, 0, capacity)
	}
	for line := s.Next(); line != nil; line = s.Next() {
		if bytes.ContainsRune(line, '}') {
			break
		}
		line = bytes.TrimPrefix(line, bArrayPrefix)
		line = bytes.TrimSpace(line)
		line = bytes.TrimSuffix(line, bComma)
		for field := range bytes.SplitSeq(line, bComma) {
			field = bytes.TrimSpace(field)
			if len(field) == 0 {
				continue
			}
			if v, ok := parse(field); ok {
				result = append(result, v)
			}
		}
	}
	return result
}

func parseASCIIIntArray(s *decutil.LineScanner, capacity int) []int32 {
	return scanASCIIFields(s, capacity, func(b []byte) (int32, bool) {
		v, err := strconv.Atoi(decutil.Bstr(b))
		return int32(v), err == nil //nolint:gosec // bounded
	})
}

func parseASCIIFloatArray(s *decutil.LineScanner, capacity int) []float64 {
	return scanASCIIFields(s, capacity, func(b []byte) (float64, bool) {
		f, err := strconv.ParseFloat(decutil.Bstr(b), 64)
		return f, err == nil
	})
}

func parseASCIIFloatTriples(s *decutil.LineScanner, capacity int) [][3]float32 {
	values := scanASCIIFields(s, capacity, parseFloat32FieldB)
	out := make([][3]float32, 0, len(values)/vecStride)
	for i := 0; i+2 < len(values); i += 3 {
		out = append(out, [3]float32{values[i], values[i+1], values[i+2]})
	}
	return out
}

func parseASCIIFloatPairs(s *decutil.LineScanner, capacity int) [][2]float32 {
	values := scanASCIIFields(s, capacity, parseFloat32FieldB)
	out := make([][2]float32, 0, len(values)/uvsPerVertex)
	for i := 0; i+1 < len(values); i += uvsPerVertex {
		out = append(out, [2]float32{values[i], values[i+1]})
	}
	return out
}

func parseFloat32FieldB(b []byte) (float32, bool) {
	f, err := strconv.ParseFloat(decutil.Bstr(b), 32)
	return float32(f), err == nil
}

func extractASCIICapacity(line []byte) int {
	idx := bytes.IndexByte(line, '*')
	if idx < 0 {
		return 0
	}
	rest := line[idx+1:]
	end := bytes.IndexAny(rest, " {")
	if end < 0 {
		end = len(rest)
	}
	v, _ := strconv.Atoi(decutil.Bstr(bytes.TrimSpace(rest[:end]))) //nolint:errcheck // silently ignore on unparseable
	return v
}

func extractASCIIName(line []byte) string {
	_, after, ok := bytes.Cut(line, bQuote)
	if !ok {
		return defaultMeshName
	}
	name, _, ok := bytes.Cut(after, bQuote)
	if !ok {
		return defaultMeshName
	}
	if _, suffix, found := bytes.Cut(name, bNameSep); found {
		name = suffix
	}
	return string(name)
}
