package gltf

import (
	"path/filepath"

	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	mimePNG  = "image/png"
	mimeJPEG = "image/jpeg"
	mimeWebP = "image/webp"
	mimeKTX2 = "image/ktx2"

	wrapClampToEdge    = 33071
	wrapMirroredRepeat = 33648

	filterNearest = 9728
)

func (d *doc) convertTextures() []*ir.Texture {
	arr := d.root.GetArray(keyTextures)
	if len(arr) == 0 {
		return nil
	}
	samplers := d.root.GetArray(keySamplers)

	bulk := make([]ir.Texture, len(arr))
	out := make([]*ir.Texture, len(arr))
	for i, t := range arr {
		d.convertTexture(t, samplers, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func (d *doc) convertImages() []*ir.ImageAsset {
	arr := d.root.GetArray(keyImages)
	if len(arr) == 0 {
		return nil
	}

	bulk := make([]ir.ImageAsset, len(arr))
	out := make([]*ir.ImageAsset, len(arr))
	for i, img := range arr {
		d.convertImage(img, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func (d *doc) convertImage(img *fastjson.Value, out *ir.ImageAsset) {
	name := string(img.GetStringBytes(keyName))
	uri := string(img.GetStringBytes(keyURI))
	mime := string(img.GetStringBytes(keyMimeType))
	format := imageFormat(mime)
	if format == "" {
		format = imgutil.ImageFormatFromPath(uri)
	}
	if name == "" {
		name = imageName(uri, format)
	}

	out.Name = name
	out.Format = format
	out.ColorSpace = ir.ColorSRGB
	out.MipLevels = defaultMipLevels
	out.SourceFormat = formatIDFromImageFormat(format)

	if bvVal := img.Get(keyBufferView); bvVal != nil {
		bvIdx := bvVal.GetInt()
		if bvIdx >= 0 && bvIdx < len(d.bufs.views) {
			bv := d.bufs.views[bvIdx]
			if bv.buffer >= 0 && bv.buffer < len(d.bufs.buffers) && d.bufs.buffers[bv.buffer] != nil {
				out.Compressed = append([]byte(nil), d.bufs.buffers[bv.buffer][bv.byteOffset:bv.byteOffset+bv.byteLength]...)
			}
		}
		return
	}

	out.SourcePath = uri
}

func (d *doc) convertTexture(t *fastjson.Value, samplers []*fastjson.Value, tex *ir.Texture) {
	tex.Name = string(t.GetStringBytes(keyName))
	tex.MipLevels = defaultMipLevels
	tex.WrapS = ir.WrapRepeat
	tex.WrapT = ir.WrapRepeat

	if si := t.Get(keySampler); si != nil {
		idx := si.GetInt()
		if idx >= 0 && idx < len(samplers) {
			s := samplers[idx]
			tex.WrapS = wrapMode(s.GetInt(keyWrapS))
			tex.WrapT = wrapMode(s.GetInt(keyWrapT))
			tex.MinFilter = filterMode(s.GetInt(keyMinFilter))
			tex.MagFilter = filterMode(s.GetInt(keyMagFilter))
		}
	}

	srcIdx, ok := textureSourceIndex(t)
	if !ok {
		return
	}
	if srcIdx < 0 || srcIdx >= len(d.root.GetArray(keyImages)) {
		return
	}
	tex.ImageIndex = srcIdx
}

func imageFormat(mime string) ir.ImageFormat {
	switch mime {
	case mimePNG:
		return ir.ImagePNG
	case mimeJPEG:
		return ir.ImageJPEG
	case mimeWebP:
		return ir.ImageWebP
	case mimeKTX2:
		return ir.ImageKTX
	default:
		return ir.ImageFormat(mime)
	}
}

func imageName(uri string, format ir.ImageFormat) string {
	if uri != "" {
		return filepath.Base(uri)
	}
	if format != "" {
		return string(format)
	}
	return "image"
}

func formatIDFromImageFormat(format ir.ImageFormat) ir.FormatID {
	switch format {
	case ir.ImagePNG:
		return ir.FormatPNG
	case ir.ImageJPEG:
		return ir.FormatJPEG
	case ir.ImageWebP:
		return ir.FormatWebP
	case ir.ImageKTX:
		return ir.FormatKTX
	case ir.ImageDDS:
		return ir.FormatDDS
	case ir.ImageBMP:
		return ir.FormatBMP
	case ir.ImageTGA:
		return ir.FormatTGA
	case ir.ImageHDR:
		return ir.FormatHDR
	case ir.ImagePSD:
		return ir.FormatPSD
	case ir.ImageTIFF:
		return ir.FormatTIFF
	case ir.ImageEXR:
		return ir.FormatEXR
	default:
		return ir.FormatUnknown
	}
}

func textureSourceIndex(texture *fastjson.Value) (int, bool) {
	if ext := texture.Get(keyExtensions, keyEXTTextureWebP); ext != nil {
		if src := ext.Get(keySource); src != nil {
			return src.GetInt(), true
		}
	}
	if ext := texture.Get(keyExtensions, keyKHRTextureBasisu); ext != nil {
		if src := ext.Get(keySource); src != nil {
			return src.GetInt(), true
		}
	}
	srcVal := texture.Get(keySource)
	if srcVal == nil {
		return 0, false
	}
	return srcVal.GetInt(), true
}

func wrapMode(w int) ir.TextureWrap {
	switch w {
	case wrapClampToEdge:
		return ir.WrapClamp
	case wrapMirroredRepeat:
		return ir.WrapMirror
	default:
		return ir.WrapRepeat
	}
}

func filterMode(f int) ir.TextureFilter {
	switch f {
	case filterNearest:
		return ir.FilterNearest
	default:
		return ir.FilterLinear
	}
}
