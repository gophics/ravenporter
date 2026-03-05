package ogg

import (
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

type huffNode struct {
	left  *huffNode
	right *huffNode
	value int
}

func buildHuffmanTree(lengths []uint8) (*huffNode, error) {
	root := &huffNode{value: -1}

	maxLen := 0
	for _, l := range lengths {
		if int(l) > maxLen {
			maxLen = int(l)
		}
	}

	if maxLen == 0 {
		return root, nil
	}

	nextPath := make([]uint32, maxLen+1)

	for i, l := range lengths {
		if l == 0 {
			continue
		}

		code := nextPath[l]

		current := root
		for bits := int(l) - 1; bits >= 0; bits-- {
			bit := (code >> bits) & 1
			if bit == 0 {
				if current.left == nil {
					current.left = &huffNode{value: -1}
				}
				current = current.left
			} else {
				if current.right == nil {
					current.right = &huffNode{value: -1}
				}
				current = current.right
			}
		}

		if current.value != -1 || current.left != nil || current.right != nil {
			return nil, decutil.DecodeErr(ir.FormatOGG, "malformed huffman tree collision", nil)
		}

		current.value = i

		for j := int(l); j > 0; j-- {
			if nextPath[j]&1 == 0 {
				nextPath[j]++
				break
			}
			nextPath[j]++
		}
		for j := int(l) + 1; j <= maxLen; j++ {
			nextPath[j] = nextPath[j-1] << 1
		}
	}

	return root, nil
}

func readHuffman(br *bitReader, root *huffNode) (int, error) {
	if root == nil {
		return -1, decutil.DecodeErr(ir.FormatOGG, "huffman decode with nil tree", nil)
	}
	current := root
	for current.left != nil || current.right != nil {
		bit := br.readBit()
		if bit == 0 {
			if current.left == nil {
				return -1, decutil.DecodeErr(ir.FormatOGG, "huffman decode missing branch 0", nil)
			}
			current = current.left
		} else {
			if current.right == nil {
				return -1, decutil.DecodeErr(ir.FormatOGG, "huffman decode missing branch 1", nil)
			}
			current = current.right
		}
	}
	return current.value, nil
}
