package bvh

import (
	"bytes"
	"errors"

	"github.com/gophics/ravenporter/internal/decutil"
)

type joint struct {
	name     string
	parent   int
	offset   [3]float32
	channels [][]byte
}

var errNoHierarchy = errors.New("missing HIERARCHY section")

func parseHierarchy(s *decutil.LineScanner) ([]joint, error) {
	line := s.Next()
	if !bytes.Equal(line, headerBVH) {
		return nil, errNoHierarchy
	}

	joints := make([]joint, 0, initialJointCap)
	stack := make([]int, 0, initialJointCap)
	fieldsBuf := make([][]byte, 0, initialFieldCap)

	for {
		line = s.Next()
		if line == nil || bytes.Equal(line, kwMotionB) {
			break
		}

		fieldsBuf = decutil.SplitByteFields(line, fieldsBuf)
		keyword := fieldsBuf[0]

		switch {
		case bytes.Equal(keyword, kwRootB) || bytes.Equal(keyword, kwJointB):
			if len(fieldsBuf) < minJointFields {
				continue
			}
			j := joint{name: string(fieldsBuf[1]), parent: -1}
			if len(stack) > 0 {
				j.parent = stack[len(stack)-1]
			}
			idx := len(joints)
			joints = append(joints, j)
			stack = append(stack, idx)

		case bytes.Equal(keyword, kwEndB):
			if len(stack) == 0 {
				continue
			}
			j := joint{name: endSite, parent: stack[len(stack)-1]}
			joints = append(joints, j)
			stack = append(stack, len(joints)-1)

		case keyword[0] == '{':

		case keyword[0] == '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case bytes.Equal(keyword, kwOffsetB):
			if len(fieldsBuf) >= offsetFields && len(joints) > 0 {
				idx := len(joints) - 1
				joints[idx].offset = [3]float32{
					decutil.ParseF32(string(fieldsBuf[1])),
					decutil.ParseF32(string(fieldsBuf[2])),
					decutil.ParseF32(string(fieldsBuf[3])),
				}
			}

		case bytes.Equal(keyword, kwChanB):
			if len(joints) > 0 && len(fieldsBuf) > chanFieldsSkip {
				idx := len(joints) - 1
				chans := make([][]byte, len(fieldsBuf)-chanFieldsSkip)
				copy(chans, fieldsBuf[chanFieldsSkip:])
				joints[idx].channels = chans
			}
		}
	}

	return joints, nil
}
