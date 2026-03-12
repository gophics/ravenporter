package obj

import (
	"bytes"

	"github.com/gophics/ravenporter/internal/decutil"
)

type byteScanner struct {
	data []byte
	pos  int
}

func (s *byteScanner) nextLine() ([]byte, bool) {
	if s.pos >= len(s.data) {
		return nil, false
	}
	start := s.pos
	i := bytes.IndexByte(s.data[start:], '\n')
	if i < 0 {
		s.pos = len(s.data)
		line := s.data[start:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		return line, true
	}
	end := start + i
	s.pos = end + 1
	if end > start && s.data[end-1] == '\r' {
		end--
	}
	return s.data[start:end], true
}

func (s *byteScanner) nextLogicalLine() ([]byte, bool) {
	line, ok := s.nextLine()
	if !ok {
		return nil, false
	}
	for len(line) > 0 && line[len(line)-1] == '\\' {
		line = line[:len(line)-1]
		next, ok := s.nextLine()
		if !ok {
			break
		}
		line = append(line, ' ')
		line = append(line, next...)
	}
	return line, true
}

const stackFields = 16

type fieldSplitter struct {
	stack [stackFields][]byte
	heap  [][]byte
	count int
}

func (f *fieldSplitter) split(line []byte) {
	f.count = 0
	f.heap = f.heap[:0]
	for len(line) > 0 {
		for len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			line = line[1:]
		}
		if len(line) == 0 {
			break
		}
		end := bytes.IndexAny(line, " \t")
		var field []byte
		if end < 0 {
			field = line
			line = nil
		} else {
			field = line[:end]
			line = line[end+1:]
		}
		if f.count < stackFields {
			f.stack[f.count] = field
		} else {
			f.heap = append(f.heap, field)
		}
		f.count++
	}
}

func (f *fieldSplitter) get(i int) []byte {
	if i >= f.count {
		return nil
	}
	if i < stackFields {
		return f.stack[i]
	}
	return f.heap[i-stackFields]
}

func parseFloatBytes(b []byte) (float32, error) {
	return decutil.ParseFloat32Bytes(b)
}

func parseIntBytes(b []byte) (int, error) {
	return decutil.ParseIntBytes(b)
}

func parseFaceVertexBytes(b []byte) (vertexRef, error) {
	i1 := bytes.IndexByte(b, '/')
	if i1 < 0 {
		v, err := parseIntBytes(b)
		if err != nil {
			return vertexRef{}, decodeErrCause(errBadFace.Error(), err)
		}
		return vertexRef{pos: v}, nil
	}

	v, err := parseIntBytes(b[:i1])
	if err != nil {
		return vertexRef{}, decodeErrCause(errBadFace.Error(), err)
	}
	ref := vertexRef{pos: v}
	rest := b[i1+1:]

	i2 := bytes.IndexByte(rest, '/')
	if i2 < 0 {
		if len(rest) > 0 {
			ref.uv, err = parseIntBytes(rest)
			if err != nil {
				return vertexRef{}, decodeErrCause(errBadFace.Error(), err)
			}
		}
		return ref, nil
	}

	if i2 > 0 {
		ref.uv, err = parseIntBytes(rest[:i2])
		if err != nil {
			return vertexRef{}, decodeErrCause(errBadFace.Error(), err)
		}
	}
	vnPart := rest[i2+1:]
	if len(vnPart) > 0 {
		ref.norm, err = parseIntBytes(vnPart)
		if err != nil {
			return vertexRef{}, decodeErrCause(errBadFace.Error(), err)
		}
	}
	return ref, nil
}

func parseFloat3Bytes(x, y, z []byte) ([3]float32, error) {
	fx, err := parseFloatBytes(x)
	if err != nil {
		return [3]float32{}, err
	}
	fy, err := parseFloatBytes(y)
	if err != nil {
		return [3]float32{}, err
	}
	fz, err := parseFloatBytes(z)
	if err != nil {
		return [3]float32{}, err
	}
	return [3]float32{fx, fy, fz}, nil
}
