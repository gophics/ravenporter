package imgutil

import (
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

func ImageFormatFromPath(path string) ir.ImageFormat {
	switch {
	case decutil.HasASCIISuffixFold(path, ".png"):
		return ir.ImagePNG
	case decutil.HasASCIISuffixFold(path, ".jpg"), decutil.HasASCIISuffixFold(path, ".jpeg"):
		return ir.ImageJPEG
	case decutil.HasASCIISuffixFold(path, ".webp"):
		return ir.ImageWebP
	case decutil.HasASCIISuffixFold(path, ".ktx"), decutil.HasASCIISuffixFold(path, ".ktx2"):
		return ir.ImageKTX
	case decutil.HasASCIISuffixFold(path, ".dds"):
		return ir.ImageDDS
	case decutil.HasASCIISuffixFold(path, ".bmp"):
		return ir.ImageBMP
	case decutil.HasASCIISuffixFold(path, ".tga"):
		return ir.ImageTGA
	case decutil.HasASCIISuffixFold(path, ".hdr"):
		return ir.ImageHDR
	case decutil.HasASCIISuffixFold(path, ".psd"):
		return ir.ImagePSD
	case decutil.HasASCIISuffixFold(path, ".tiff"), decutil.HasASCIISuffixFold(path, ".tif"):
		return ir.ImageTIFF
	case decutil.HasASCIISuffixFold(path, ".exr"):
		return ir.ImageEXR
	default:
		return ""
	}
}
