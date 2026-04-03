package cache

import (
	"bytes"
	"context"

	"github.com/gophics/ravenporter/detect"
	audioaiff "github.com/gophics/ravenporter/internal/decode/audio/aiff"
	audioflac "github.com/gophics/ravenporter/internal/decode/audio/flac"
	audiomp3 "github.com/gophics/ravenporter/internal/decode/audio/mp3"
	audioogg "github.com/gophics/ravenporter/internal/decode/audio/ogg"
	audioopus "github.com/gophics/ravenporter/internal/decode/audio/opus"
	audiowav "github.com/gophics/ravenporter/internal/decode/audio/wav"
	imagebmp "github.com/gophics/ravenporter/internal/decode/image/bmp"
	imagedds "github.com/gophics/ravenporter/internal/decode/image/dds"
	imageexr "github.com/gophics/ravenporter/internal/decode/image/exr"
	imagehdr "github.com/gophics/ravenporter/internal/decode/image/hdr"
	imagejpeg "github.com/gophics/ravenporter/internal/decode/image/jpeg"
	imagepng "github.com/gophics/ravenporter/internal/decode/image/png"
	imagepsd "github.com/gophics/ravenporter/internal/decode/image/psd"
	imagetga "github.com/gophics/ravenporter/internal/decode/image/tga"
	imagetiff "github.com/gophics/ravenporter/internal/decode/image/tiff"
	imagewebp "github.com/gophics/ravenporter/internal/decode/image/webp"
	"github.com/gophics/ravenporter/ir"
)

func restoreCachedImageDecode(image *ir.ImageAsset) ir.PixelDecodeFunc {
	if image == nil || image.IsGPUCompressed() {
		return nil
	}
	switch image.Format {
	case ir.ImageBMP, ir.ImageDDS, ir.ImageEXR, ir.ImageHDR, ir.ImageJPEG,
		ir.ImagePNG, ir.ImagePSD, ir.ImageTGA, ir.ImageTIFF, ir.ImageWebP:
	default:
		return nil
	}
	return func(img *ir.ImageAsset) (*ir.PixelBuffer, error) {
		raw, err := img.CompressedBytes()
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, nil
		}
		decoded, err := decodeCachedImage(img.Format, raw)
		if err != nil {
			return nil, err
		}
		if decoded == nil {
			return nil, nil
		}
		return clonePixelBuffer(decoded), nil
	}
}

func restoreCachedAudioDecode(clip *ir.AudioClip) ir.SampleDecodeFunc {
	if clip == nil {
		return nil
	}
	switch cachedAudioCodec(clip) {
	case ir.AudioAIFF, ir.AudioFLAC, ir.AudioMP3, ir.AudioOGG, ir.AudioOpus, ir.AudioWAV:
	default:
		return nil
	}
	return func(c *ir.AudioClip) ([]float32, error) {
		raw, err := c.CompressedBytes()
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, nil
		}
		return decodeCachedAudio(cachedAudioCodec(c), raw)
	}
}

func cachedAudioCodec(clip *ir.AudioClip) ir.AudioFormat {
	if clip.SourceCodec != "" {
		return clip.SourceCodec
	}
	return clip.Format
}

func decodeCachedImage(format ir.ImageFormat, raw []byte) (*ir.PixelBuffer, error) {
	var decoder interface {
		Decode(detect.ReadSeekerAt, detect.DecodeOptions) (*ir.Asset, error)
	}
	switch format {
	case ir.ImageBMP:
		decoder = &imagebmp.Decoder{}
	case ir.ImageDDS:
		decoder = &imagedds.Decoder{}
	case ir.ImageEXR:
		decoder = &imageexr.Decoder{}
	case ir.ImageHDR:
		decoder = &imagehdr.Decoder{}
	case ir.ImageJPEG:
		decoder = &imagejpeg.Decoder{}
	case ir.ImagePNG:
		decoder = &imagepng.Decoder{}
	case ir.ImagePSD:
		decoder = &imagepsd.Decoder{}
	case ir.ImageTGA:
		decoder = &imagetga.Decoder{}
	case ir.ImageTIFF:
		decoder = &imagetiff.Decoder{}
	case ir.ImageWebP:
		decoder = &imagewebp.Decoder{}
	default:
		return nil, nil
	}
	asset, err := decoder.Decode(bytes.NewReader(raw), detect.DecodeOptions{Context: context.Background()})
	if err != nil {
		return nil, err
	}
	if asset == nil || len(asset.Images) == 0 || asset.Images[0] == nil {
		return nil, nil
	}
	return asset.Images[0].DecodePixels()
}

func decodeCachedAudio(format ir.AudioFormat, raw []byte) ([]float32, error) {
	var decoder interface {
		Decode(detect.ReadSeekerAt, detect.DecodeOptions) (*ir.Asset, error)
	}
	switch format {
	case ir.AudioAIFF:
		decoder = &audioaiff.Decoder{}
	case ir.AudioFLAC:
		decoder = &audioflac.Decoder{}
	case ir.AudioMP3:
		decoder = &audiomp3.Decoder{}
	case ir.AudioOGG:
		decoder = &audioogg.Decoder{}
	case ir.AudioOpus:
		decoder = &audioopus.Decoder{}
	case ir.AudioWAV:
		decoder = &audiowav.Decoder{}
	default:
		return nil, nil
	}
	asset, err := decoder.Decode(bytes.NewReader(raw), detect.DecodeOptions{Context: context.Background()})
	if err != nil {
		return nil, err
	}
	if asset == nil || len(asset.AudioClips) == 0 || asset.AudioClips[0] == nil {
		return nil, nil
	}
	return asset.AudioClips[0].DecodeSamples()
}
