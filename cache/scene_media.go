package cache

import (
	"math"
	"time"

	"github.com/gophics/ravenporter/ir"
)

const (
	minCuePointBytes    = 8
	minFontKerningBytes = 12
	minLODLevelBytes    = 8
)

func writeAudioSlice(enc *encoder, clips []*ir.AudioClip, blobs *blobBuilder) {
	enc.count(len(clips))
	for _, clip := range clips {
		enc.bool(clip != nil)
		if clip == nil {
			continue
		}
		enc.string(clip.Name)
		enc.string(string(clip.Format))
		enc.int(clip.SampleRate)
		enc.string(string(clip.Layout))
		enc.int(int(clip.BitDepth))
		enc.i64(int64(clip.Duration))
		enc.int(clip.LoopStart)
		enc.int(clip.LoopEnd)
		writeAudioMetadata(enc, clip.Metadata, blobs)
		compressed, err := clip.CompressedBytes()
		if err != nil {
			enc.fail(err)
			return
		}
		writeBlobRef(enc, blobs.Add("audio", compressed))
		enc.string(string(clip.SourceCodec))
	}
}

func readAudioSlice(dec *decoder, blobs *blobStore) ([]*ir.AudioClip, error) {
	count := dec.count(minOptionalEntryBytes)
	clips := make([]*ir.AudioClip, count)
	for i := range clips {
		if !dec.bool() {
			continue
		}
		clip := &ir.AudioClip{
			Name:       dec.string(),
			Format:     ir.AudioFormat(dec.string()),
			SampleRate: int(dec.i32()),
			Layout:     ir.ChannelLayout(dec.string()),
			BitDepth:   ir.BitDepth(dec.i32()),
			Duration:   time.Duration(dec.i64()),
			LoopStart:  int(dec.i32()),
			LoopEnd:    int(dec.i32()),
		}
		metadata, err := readAudioMetadata(dec, blobs)
		if err != nil {
			return nil, err
		}
		clip.Metadata = metadata
		clip.SetCompressedLoader(newBlobLoader(blobs, readBlobRef(dec)))
		clip.SourceCodec = ir.AudioFormat(dec.string())
		clip.SampleDecode = restoreCachedAudioDecode(clip)
		clips[i] = clip
	}
	return clips, nil
}

func writeAudioMetadata(enc *encoder, metadata ir.AudioMetadata, blobs *blobBuilder) {
	enc.string(metadata.Title)
	enc.string(metadata.Artist)
	enc.string(metadata.Album)
	enc.string(metadata.Genre)
	enc.string(metadata.Comment)
	writeBlobRef(enc, blobs.Add("audio artwork", metadata.Artwork))
	enc.count(len(metadata.CuePoints))
	for _, cue := range metadata.CuePoints {
		enc.string(cue.Name)
		enc.int(cue.Sample)
	}
}

func readAudioMetadata(dec *decoder, blobs *blobStore) (ir.AudioMetadata, error) {
	metadata := ir.AudioMetadata{
		Title:   dec.string(),
		Artist:  dec.string(),
		Album:   dec.string(),
		Genre:   dec.string(),
		Comment: dec.string(),
	}
	artwork, err := blobs.bytes(readBlobRef(dec))
	if err != nil {
		return ir.AudioMetadata{}, err
	}
	metadata.Artwork = artwork
	count := dec.count(minCuePointBytes)
	metadata.CuePoints = make([]ir.CuePoint, count)
	for i := range metadata.CuePoints {
		metadata.CuePoints[i] = ir.CuePoint{
			Name:   dec.string(),
			Sample: int(dec.i32()),
		}
	}
	return metadata, nil
}

func writeFontSlice(enc *encoder, fonts []*ir.Font, blobs *blobBuilder) {
	enc.count(len(fonts))
	for _, font := range fonts {
		enc.bool(font != nil)
		if font == nil {
			continue
		}
		enc.string(font.Name)
		enc.string(string(font.Format))
		enc.string(font.Family)
		enc.string(font.Subfamily)
		enc.string(font.PostScript)
		enc.bool(font.Vector != nil)
		if font.Vector != nil {
			writeVectorFont(enc, font.Vector, blobs)
		}
		enc.bool(font.Bitmap != nil)
		if font.Bitmap != nil {
			writeBitmapFont(enc, font.Bitmap)
		}
		writeStringMap(enc, font.Metadata)
	}
}

func readFontSlice(dec *decoder, blobs *blobStore) []*ir.Font {
	count := dec.count(minOptionalEntryBytes)
	fonts := make([]*ir.Font, count)
	for i := range fonts {
		if !dec.bool() {
			continue
		}
		font := &ir.Font{
			Name:       dec.string(),
			Format:     ir.FontFormat(dec.string()),
			Family:     dec.string(),
			Subfamily:  dec.string(),
			PostScript: dec.string(),
		}
		if dec.bool() {
			font.Vector = readVectorFont(dec, blobs)
		}
		if dec.bool() {
			font.Bitmap = readBitmapFont(dec)
		}
		font.Metadata = readStringMap(dec)
		fonts[i] = font
	}
	return fonts
}

func writeVectorFont(enc *encoder, font *ir.VectorFontData, blobs *blobBuilder) {
	enc.int(font.UnitsPerEm)
	enc.int(font.Ascender)
	enc.int(font.Descender)
	enc.int(font.LineGap)
	enc.int(font.GlyphCount)
	enc.runes(font.Codepoints)
	writeRuneIntMap(enc, font.Advances)
	enc.count(len(font.Kerning))
	for _, pair := range font.Kerning {
		enc.i32(pair.First)
		enc.i32(pair.Second)
		enc.int(pair.Amount)
	}
	rawData, err := font.RawBytes()
	if err != nil {
		enc.fail(err)
		return
	}
	writeBlobRef(enc, blobs.Add("font", rawData))
}

func readVectorFont(dec *decoder, blobs *blobStore) *ir.VectorFontData {
	font := &ir.VectorFontData{
		UnitsPerEm: int(dec.i32()),
		Ascender:   int(dec.i32()),
		Descender:  int(dec.i32()),
		LineGap:    int(dec.i32()),
		GlyphCount: int(dec.i32()),
		Codepoints: dec.runes(),
		Advances:   readRuneIntMap(dec),
	}
	count := dec.count(minFontKerningBytes)
	font.Kerning = make([]ir.KerningPair, count)
	for i := range font.Kerning {
		font.Kerning[i] = ir.KerningPair{
			First:  dec.i32(),
			Second: dec.i32(),
			Amount: int(dec.i32()),
		}
	}
	font.SetRawBytesLoader(newBlobLoader(blobs, readBlobRef(dec)))
	return font
}

func writeBitmapFont(enc *encoder, font *ir.BitmapFontData) {
	enc.int(font.LineHeight)
	enc.int(font.Base)
	enc.int(font.GlyphCount)
	enc.string(font.AtlasPath)
	enc.int(font.AtlasIndex)
	writeGlyphMap(enc, font.Glyphs)
}

func readBitmapFont(dec *decoder) *ir.BitmapFontData {
	return &ir.BitmapFontData{
		LineHeight: int(dec.i32()),
		Base:       int(dec.i32()),
		GlyphCount: int(dec.i32()),
		AtlasPath:  dec.string(),
		AtlasIndex: int(dec.i32()),
		Glyphs:     readGlyphMap(dec),
	}
}

func writeImageSlice(enc *encoder, images []*ir.ImageAsset, blobs *blobBuilder, cfg writeConfig) {
	enc.count(len(images))
	for _, image := range images {
		enc.bool(image != nil)
		if image == nil {
			continue
		}
		enc.string(image.Name)
		enc.string(string(image.Format))
		enc.int(image.Width)
		enc.int(image.Height)
		enc.int(int(image.Channels))
		enc.string(string(image.ColorSpace))
		enc.int(image.MipLevels)
		compressed, err := image.CompressedBytes()
		if err != nil {
			enc.fail(err)
			return
		}
		writeBlobRef(enc, blobs.Add("image", compressed))
		enc.string(string(image.SourceFormat))
		enc.string(image.SourcePath)
		enc.int(int(image.CompressionFormat))
		writeStringMap(enc, image.Metadata)
		writePixelBuffer(enc, image.Pixels(), blobs, cfg)
	}
}

func readImageSlice(dec *decoder, blobs *blobStore) ([]*ir.ImageAsset, error) {
	count := dec.count(minOptionalEntryBytes)
	images := make([]*ir.ImageAsset, count)
	for i := range images {
		if !dec.bool() {
			continue
		}
		image := &ir.ImageAsset{
			Name:       dec.string(),
			Format:     ir.ImageFormat(dec.string()),
			Width:      int(dec.i32()),
			Height:     int(dec.i32()),
			Channels:   ir.ChannelCount(dec.i32()),
			ColorSpace: ir.ColorSpace(dec.string()),
			MipLevels:  int(dec.i32()),
		}
		image.SetCompressedLoader(newBlobLoader(blobs, readBlobRef(dec)))
		image.SourceFormat = ir.FormatID(dec.string())
		image.SourcePath = dec.string()
		compressionFormat, err := readGPUCompression(dec)
		if err != nil {
			return nil, err
		}
		image.CompressionFormat = compressionFormat
		image.Metadata = readStringMap(dec)
		pixels, err := readPixelBuffer(dec, blobs)
		if err != nil {
			return nil, err
		}
		if pixels != nil {
			image.SetPixels(pixels)
		}
		image.PixelDecode = restoreCachedImageDecode(image)
		images[i] = image
	}
	return images, nil
}

func writePixelBuffer(enc *encoder, pixels *ir.PixelBuffer, blobs *blobBuilder, _ writeConfig) {
	enc.bool(pixels != nil)
	if pixels == nil {
		return
	}
	writeBlobRef(enc, blobs.Add("image pixels", pixels.Data))
	enc.int(int(pixels.DataType))
	enc.int(int(pixels.BitDepth))
	enc.count(len(pixels.Mipmaps))
	for _, mip := range pixels.Mipmaps {
		writeBlobRef(enc, blobs.Add("image mipmap", mip))
	}
}

func readPixelBuffer(dec *decoder, blobs *blobStore) (*ir.PixelBuffer, error) {
	if !dec.bool() {
		return nil, nil
	}
	data, err := blobs.bytes(readBlobRef(dec))
	if err != nil {
		return nil, err
	}
	pixels := &ir.PixelBuffer{
		Data: data,
	}
	dataType, err := readDataType(dec)
	if err != nil {
		return nil, err
	}
	pixels.DataType = dataType
	pixels.BitDepth = ir.BitDepth(dec.i32())
	count := dec.count(minOptionalEntryBytes)
	pixels.Mipmaps = make([][]byte, count)
	for i := range pixels.Mipmaps {
		pixels.Mipmaps[i], err = blobs.bytes(readBlobRef(dec))
		if err != nil {
			return nil, err
		}
	}
	return pixels, nil
}

func newBlobLoader(blobs *blobStore, ref blobRef) func() ([]byte, error) {
	if !ref.present {
		return nil
	}
	return func() ([]byte, error) {
		return blobs.bytes(ref)
	}
}

func writeLODGroupSlice(enc *encoder, groups []*ir.LODGroup) {
	enc.count(len(groups))
	for _, group := range groups {
		enc.bool(group != nil)
		if group == nil {
			continue
		}
		enc.string(group.Name)
		enc.count(len(group.Levels))
		for _, level := range group.Levels {
			enc.f32(level.Threshold)
			enc.int(level.NodeIndex)
		}
	}
}

func readLODGroupSlice(dec *decoder) []*ir.LODGroup {
	count := dec.count(minOptionalEntryBytes)
	groups := make([]*ir.LODGroup, count)
	for i := range groups {
		if !dec.bool() {
			continue
		}
		group := &ir.LODGroup{Name: dec.string()}
		levelCount := dec.count(minLODLevelBytes)
		group.Levels = make([]ir.LODLevel, levelCount)
		for j := range group.Levels {
			group.Levels[j] = ir.LODLevel{
				Threshold: dec.f32(),
				NodeIndex: int(dec.i32()),
			}
		}
		groups[i] = group
	}
	return groups
}

func writeCollisionSlice(enc *encoder, collisions []*ir.CollisionMesh) {
	enc.count(len(collisions))
	for _, collision := range collisions {
		enc.bool(collision != nil)
		if collision == nil {
			continue
		}
		enc.int(int(collision.Type))
		enc.int(collision.MeshIndex)
		enc.int(collision.NodeIndex)
	}
}

func readGPUCompression(dec *decoder) (ir.GPUCompression, error) {
	value := dec.i32()
	if value < 0 || value > math.MaxUint8 {
		return 0, fmtErrorf("%w: invalid GPU compression %d", errInvalidCache, value)
	}
	return ir.GPUCompression(uint8(value)), nil
}

func readDataType(dec *decoder) (ir.DataType, error) {
	value := dec.i32()
	if value < 0 || value > math.MaxUint8 {
		return 0, fmtErrorf("%w: invalid pixel data type %d", errInvalidCache, value)
	}
	return ir.DataType(uint8(value)), nil
}

func readCollisionSlice(dec *decoder) []*ir.CollisionMesh {
	count := dec.count(minOptionalEntryBytes)
	collisions := make([]*ir.CollisionMesh, count)
	for i := range collisions {
		if !dec.bool() {
			continue
		}
		collisions[i] = &ir.CollisionMesh{
			Type:      ir.CollisionType(dec.i32()),
			MeshIndex: int(dec.i32()),
			NodeIndex: int(dec.i32()),
		}
	}
	return collisions
}
