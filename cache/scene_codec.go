package cache

import (
	"maps"
	"slices"

	"github.com/gophics/ravenporter/ir"
)

const (
	propertyFloat32 = uint8(iota + 1)
	propertyInt
	propertyBool
	propertyString
	propertyVec3
	propertyVec4
)

const (
	minNodeBytes             = 142
	minMaterialPropertyBytes = 5
	minStringMapEntryBytes   = 8
	minRuneIntEntryBytes     = 8
	minGlyphEntryBytes       = 32
	vec2Bytes                = 8
	vec3Bytes                = 12
	vec4Bytes                = 16
	jointBytes               = 8
)

type blobRef struct {
	present bool
	offset  uint64
	size    uint64
}

type blobBuilder struct {
	data     []byte
	maxBytes int64
	err      error
}

func newBlobBuilder(cfg writeConfig) *blobBuilder {
	return &blobBuilder{maxBytes: cfg.maxEmbeddedMediaBytes}
}

func (b *blobBuilder) Add(label string, data []byte) blobRef {
	if len(data) == 0 {
		return blobRef{}
	}
	if b.err != nil {
		return blobRef{}
	}
	if b.maxBytes > 0 {
		nextSize := int64(len(b.data)) + int64(len(data))
		if nextSize > b.maxBytes {
			b.err = fmtErrorf(
				"cache: embedded media exceeds limit while adding %s (%d > %d bytes)",
				label, nextSize, b.maxBytes,
			)
			return blobRef{}
		}
	}
	ref := blobRef{
		present: true,
		offset:  uint64(len(b.data)),
		size:    uint64(len(data)),
	}
	b.data = append(b.data, data...)
	return ref
}

func (b *blobBuilder) Bytes() []byte {
	return b.data
}

func (b *blobBuilder) Err() error {
	if b == nil {
		return nil
	}
	return b.err
}

func (b *blobStore) bytes(ref blobRef) ([]byte, error) {
	if !ref.present {
		return nil, nil
	}
	b.mu.Lock()
	if b.closed || b.reader == nil {
		b.mu.Unlock()
		return nil, fmtErrorf("%w: media source is closed", errInvalidCache)
	}
	reader := b.reader
	baseOffset := b.baseOffset
	size := b.size
	b.mu.Unlock()
	if ref.offset > size || ref.size > size || ref.offset+ref.size > size {
		return nil, fmtErrorf("%w: blob reference out of bounds", errInvalidCache)
	}
	out := make([]byte, ref.size)
	offset, err := toInt64(ref.offset)
	if err != nil {
		return nil, err
	}
	return out, readAtFull(reader, out, baseOffset+offset)
}

func marshalScene(asset *ir.Asset, blobs *blobBuilder, cfg writeConfig) ([]byte, error) {
	enc := &encoder{}
	writeScene(enc, asset, blobs, cfg)
	if enc.err != nil {
		return nil, enc.err
	}
	if err := blobs.Err(); err != nil {
		return nil, err
	}
	return enc.bytes(), nil
}

func unmarshalScene(data []byte, blobs *blobStore) (*ir.Asset, error) {
	dec := &decoder{data: data}
	asset, err := readScene(dec, blobs)
	if err != nil {
		return nil, err
	}
	if dec.err != nil {
		return nil, dec.err
	}
	if dec.remaining() != 0 {
		return nil, fmtErrorf("%w: trailing scene data", errInvalidCache)
	}
	return asset, nil
}

func writeScene(enc *encoder, asset *ir.Asset, blobs *blobBuilder, cfg writeConfig) {
	enc.string(asset.Name)
	enc.int(int(asset.UpAxis))
	enc.f64(asset.Unit)
	enc.int(asset.DefaultScene)
	writeSceneMetadata(enc, asset.Metadata)
	enc.ints(asset.RootNodes)
	writeSceneEntries(enc, asset.Scenes)

	enc.count(len(asset.Nodes))
	for i := range asset.Nodes {
		writeNode(enc, asset.Nodes[i])
	}

	writeMeshSlice(enc, asset.Meshes)
	writeMaterialSlice(enc, asset.Materials)
	writeTextureSlice(enc, asset.Textures)
	writeAnimationSlice(enc, asset.Animations)
	writeSkeletonSlice(enc, asset.Skeletons)
	writeCameraSlice(enc, asset.Cameras)
	writeLightSlice(enc, asset.Lights)
	writeAudioSlice(enc, asset.AudioClips, blobs)
	writeFontSlice(enc, asset.Fonts, blobs)
	writeImageSlice(enc, asset.Images, blobs, cfg)
	writeLODGroupSlice(enc, asset.LODGroups)
	writeCollisionSlice(enc, asset.CollisionMeshes)
}

func readScene(dec *decoder, blobs *blobStore) (*ir.Asset, error) {
	asset := &ir.Asset{
		Name:         dec.string(),
		UpAxis:       ir.Axis(dec.i32()),
		Unit:         dec.f64(),
		DefaultScene: int(dec.i32()),
		Metadata:     readSceneMetadata(dec),
		RootNodes:    dec.ints(),
		Scenes:       readSceneEntries(dec),
	}

	nodeCount := dec.count(minNodeBytes)
	asset.Nodes = make([]ir.Node, nodeCount)
	for i := range asset.Nodes {
		node, err := readNode(dec)
		if err != nil {
			return nil, err
		}
		asset.Nodes[i] = node
	}

	asset.Meshes = readMeshSlice(dec)
	materials, err := readMaterialSlice(dec)
	if err != nil {
		return nil, err
	}
	asset.Materials = materials
	textures := readTextureSlice(dec)
	if dec.err != nil {
		return nil, dec.err
	}
	asset.Textures = textures
	asset.Animations = readAnimationSlice(dec)
	asset.Skeletons = readSkeletonSlice(dec)
	asset.Cameras = readCameraSlice(dec)
	asset.Lights = readLightSlice(dec)
	audio, err := readAudioSlice(dec, blobs)
	if err != nil {
		return nil, err
	}
	asset.AudioClips = audio
	asset.Fonts = readFontSlice(dec, blobs)
	images, err := readImageSlice(dec, blobs)
	if err != nil {
		return nil, err
	}
	asset.Images = images
	asset.LODGroups = readLODGroupSlice(dec)
	asset.CollisionMeshes = readCollisionSlice(dec)
	asset.NormalizeGraph()
	return asset, nil
}

func writeSceneMetadata(enc *encoder, metadata ir.AssetMetadata) {
	enc.string(string(metadata.SourceFormat))
	enc.string(metadata.SourceVersion)
	enc.string(metadata.Generator)
	enc.string(metadata.CreationTime)
	writeStringMap(enc, metadata.ExtraProperties)
}

func readSceneMetadata(dec *decoder) ir.AssetMetadata {
	return ir.AssetMetadata{
		SourceFormat:    ir.FormatID(dec.string()),
		SourceVersion:   dec.string(),
		Generator:       dec.string(),
		CreationTime:    dec.string(),
		ExtraProperties: readStringMap(dec),
	}
}

func writeNode(enc *encoder, node ir.Node) {
	enc.string(node.Name)
	writeTransform(enc, node.Transform)
	enc.bool(node.Visible)
	enc.int(int(node.Mobility))
	enc.int(node.ParentIndex)
	enc.int(node.MeshIndex)
	enc.int(node.SkinIndex)
	enc.int(node.CameraIndex)
	enc.int(node.LightIndex)
	enc.int(node.LODGroupIndex)
	enc.bool(node.IsJoint)
	enc.bool(node.IsCollision)
	enc.float32s(node.MorphWeights)
	enc.ints(node.Children)
	writeMaterialProperties(enc, node.Extras)
}

func readNode(dec *decoder) (ir.Node, error) {
	node := ir.Node{
		Name:          dec.string(),
		Transform:     readTransform(dec),
		Visible:       dec.bool(),
		Mobility:      ir.MobilityState(dec.i32()),
		ParentIndex:   int(dec.i32()),
		MeshIndex:     int(dec.i32()),
		SkinIndex:     int(dec.i32()),
		CameraIndex:   int(dec.i32()),
		LightIndex:    int(dec.i32()),
		LODGroupIndex: int(dec.i32()),
		IsJoint:       dec.bool(),
		IsCollision:   dec.bool(),
		MorphWeights:  dec.float32s(),
		Children:      dec.ints(),
	}
	extras, err := readMaterialProperties(dec)
	if err != nil {
		return ir.Node{}, err
	}
	node.Extras = extras
	return node, nil
}

func writeSceneEntries(enc *encoder, scenes []*ir.Scene) {
	enc.count(len(scenes))
	for _, scene := range scenes {
		enc.bool(scene != nil)
		if scene == nil {
			continue
		}
		enc.string(scene.Name)
		enc.ints(scene.RootNodes)
	}
}

func readSceneEntries(dec *decoder) []*ir.Scene {
	count := dec.count(minOptionalEntryBytes)
	scenes := make([]*ir.Scene, count)
	for i := range scenes {
		if !dec.bool() {
			continue
		}
		scenes[i] = &ir.Scene{
			Name:      dec.string(),
			RootNodes: dec.ints(),
		}
	}
	return scenes
}

func writeTransform(enc *encoder, transform ir.Transform) {
	writeVec3(enc, transform.Translation)
	writeVec4(enc, transform.Rotation)
	writeVec3(enc, transform.Scale)
	writeMat4(enc, transform.Matrix)
}

func readTransform(dec *decoder) ir.Transform {
	return ir.Transform{
		Translation: readVec3(dec),
		Rotation:    readVec4(dec),
		Scale:       readVec3(dec),
		Matrix:      readMat4(dec),
	}
}

func validateMaterialProperties(properties map[string]any) error {
	for _, key := range sortedAnyKeys(properties) {
		switch properties[key].(type) {
		case float32, int, bool, string, [3]float32, [4]float32:
		default:
			return fmtErrorf("unsupported material property %q type %T", key, properties[key])
		}
	}
	return nil
}

func writeMaterialProperties(enc *encoder, properties map[string]any) {
	if err := validateMaterialProperties(properties); err != nil {
		enc.fail(err)
		return
	}
	keys := sortedAnyKeys(properties)
	enc.count(len(keys))
	for _, key := range keys {
		enc.string(key)
		switch value := properties[key].(type) {
		case float32:
			enc.u8(propertyFloat32)
			enc.f32(value)
		case int:
			enc.u8(propertyInt)
			enc.i64(int64(value))
		case bool:
			enc.u8(propertyBool)
			enc.bool(value)
		case string:
			enc.u8(propertyString)
			enc.string(value)
		case [3]float32:
			enc.u8(propertyVec3)
			writeVec3(enc, value)
		case [4]float32:
			enc.u8(propertyVec4)
			writeVec4(enc, value)
		}
	}
}

func readMaterialProperties(dec *decoder) (map[string]any, error) {
	count := dec.count(minMaterialPropertyBytes)
	if count == 0 {
		return nil, nil
	}
	properties := make(map[string]any, count)
	for i := 0; i < count; i++ {
		key := dec.string()
		switch dec.u8() {
		case propertyFloat32:
			properties[key] = dec.f32()
		case propertyInt:
			value := dec.i64()
			if int64(int(value)) != value {
				return nil, fmtErrorf("%w: material property %q integer overflow", errInvalidCache, key)
			}
			properties[key] = int(value)
		case propertyBool:
			properties[key] = dec.bool()
		case propertyString:
			properties[key] = dec.string()
		case propertyVec3:
			properties[key] = readVec3(dec)
		case propertyVec4:
			properties[key] = readVec4(dec)
		default:
			return nil, fmtErrorf("%w: unknown material property type for %q", errInvalidCache, key)
		}
	}
	return properties, nil
}

func writeBlobRef(enc *encoder, ref blobRef) {
	enc.bool(ref.present)
	if !ref.present {
		return
	}
	enc.u64(ref.offset)
	enc.u64(ref.size)
}

func readBlobRef(dec *decoder) blobRef {
	if !dec.bool() {
		return blobRef{}
	}
	return blobRef{
		present: true,
		offset:  dec.u64(),
		size:    dec.u64(),
	}
}

func writeStringMap(enc *encoder, values map[string]string) {
	keys := slices.Sorted(maps.Keys(values))
	enc.count(len(keys))
	for _, key := range keys {
		enc.string(key)
		enc.string(values[key])
	}
}

func readStringMap(dec *decoder) map[string]string {
	count := dec.count(minStringMapEntryBytes)
	if count == 0 {
		return nil
	}
	values := make(map[string]string, count)
	for i := 0; i < count; i++ {
		values[dec.string()] = dec.string()
	}
	return values
}

func writeRuneIntMap(enc *encoder, values map[rune]int) {
	keys := sortedRuneKeys(values)
	enc.count(len(keys))
	for _, key := range keys {
		enc.i32(key)
		enc.int(values[key])
	}
}

func readRuneIntMap(dec *decoder) map[rune]int {
	count := dec.count(minRuneIntEntryBytes)
	if count == 0 {
		return nil
	}
	values := make(map[rune]int, count)
	for i := 0; i < count; i++ {
		values[dec.i32()] = int(dec.i32())
	}
	return values
}

func writeGlyphMap(enc *encoder, values map[rune]ir.BitmapGlyph) {
	keys := sortedRuneKeys(values)
	enc.count(len(keys))
	for _, key := range keys {
		glyph := values[key]
		enc.i32(key)
		enc.int(glyph.X)
		enc.int(glyph.Y)
		enc.int(glyph.Width)
		enc.int(glyph.Height)
		enc.int(glyph.XOffset)
		enc.int(glyph.YOffset)
		enc.int(glyph.Advance)
	}
}

func readGlyphMap(dec *decoder) map[rune]ir.BitmapGlyph {
	count := dec.count(minGlyphEntryBytes)
	if count == 0 {
		return nil
	}
	values := make(map[rune]ir.BitmapGlyph, count)
	for i := 0; i < count; i++ {
		key := dec.i32()
		values[key] = ir.BitmapGlyph{
			X:       int(dec.i32()),
			Y:       int(dec.i32()),
			Width:   int(dec.i32()),
			Height:  int(dec.i32()),
			XOffset: int(dec.i32()),
			YOffset: int(dec.i32()),
			Advance: int(dec.i32()),
		}
	}
	return values
}

func sortedRuneKeys[T any](values map[rune]T) []rune {
	return slices.Sorted(maps.Keys(values))
}

func sortedAnyKeys(values map[string]any) []string {
	return slices.Sorted(maps.Keys(values))
}

func writeVec2(enc *encoder, value [2]float32) {
	enc.f32(value[0])
	enc.f32(value[1])
}

func readVec2(dec *decoder) [2]float32 {
	return [2]float32{dec.f32(), dec.f32()}
}

func writeVec3(enc *encoder, value [3]float32) {
	for _, component := range value {
		enc.f32(component)
	}
}

func readVec3(dec *decoder) [3]float32 {
	return [3]float32{dec.f32(), dec.f32(), dec.f32()}
}

func writeVec4(enc *encoder, value [4]float32) {
	for _, component := range value {
		enc.f32(component)
	}
}

func readVec4(dec *decoder) [4]float32 {
	return [4]float32{dec.f32(), dec.f32(), dec.f32(), dec.f32()}
}

func writeMat4(enc *encoder, value [16]float32) {
	for _, component := range value {
		enc.f32(component)
	}
}

func readMat4(dec *decoder) [16]float32 {
	var value [16]float32
	for i := range value {
		value[i] = dec.f32() //nolint:gosec // i is produced by ranging over the fixed-size array.
	}
	return value
}

func writeBounds(enc *encoder, value [2][3]float32) {
	writeVec3(enc, value[0])
	writeVec3(enc, value[1])
}

func readBounds(dec *decoder) [2][3]float32 {
	return [2][3]float32{readVec3(dec), readVec3(dec)}
}

func writeVec2Slice(enc *encoder, values [][2]float32) {
	enc.count(len(values))
	for _, value := range values {
		writeVec2(enc, value)
	}
}

func readVec2Slice(dec *decoder) [][2]float32 {
	count := dec.count(vec2Bytes)
	values := make([][2]float32, count)
	for i := range values {
		values[i] = readVec2(dec)
	}
	return values
}

func writeVec3Slice(enc *encoder, values [][3]float32) {
	enc.count(len(values))
	for _, value := range values {
		writeVec3(enc, value)
	}
}

func readVec3Slice(dec *decoder) [][3]float32 {
	count := dec.count(vec3Bytes)
	values := make([][3]float32, count)
	for i := range values {
		values[i] = readVec3(dec)
	}
	return values
}

func writeVec4Slice(enc *encoder, values [][4]float32) {
	enc.count(len(values))
	for _, value := range values {
		writeVec4(enc, value)
	}
}

func readVec4Slice(dec *decoder) [][4]float32 {
	count := dec.count(vec4Bytes)
	values := make([][4]float32, count)
	for i := range values {
		values[i] = readVec4(dec)
	}
	return values
}

func writeColorSlice(enc *encoder, values [][4]float32) {
	writeVec4Slice(enc, values)
}

func readColorSlice(dec *decoder) [][4]float32 {
	return readVec4Slice(dec)
}

func writeJointSlice(enc *encoder, values [][4]uint16) {
	enc.count(len(values))
	for _, value := range values {
		for _, component := range value {
			enc.u16(component)
		}
	}
}

func readJointSlice(dec *decoder) [][4]uint16 {
	count := dec.count(jointBytes)
	values := make([][4]uint16, count)
	for i := range values {
		for j := range values[i] {
			values[i][j] = dec.u16()
		}
	}
	return values
}

func writeWeightSlice(enc *encoder, values [][4]float32) {
	writeVec4Slice(enc, values)
}

func readWeightSlice(dec *decoder) [][4]float32 {
	return readVec4Slice(dec)
}
