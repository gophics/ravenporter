package obj

import (
	"io"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

func ParseMTLForTest(r io.Reader, scene *ir.Asset) {
	raw, _ := decutil.ReadAllLimit(r, 0)
	parseMTLBytes(raw, scene)
}
