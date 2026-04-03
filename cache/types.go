package cache

import (
	"errors"
	"io"
	"io/fs"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/ir"
)

// Option configures cache writing.
type Option func(*writeConfig) error

// ReadOption configures cache reading.
type ReadOption func(*readConfig) error

// Asset is a cooked runtime asset.
type Asset struct {
	Manifest Manifest
	Asset    *ir.Asset

	store *blobStore

	meshes     map[string][]int
	materials  map[string][]int
	animations map[string][]int
	nodes      map[string][]int
}

// Manifest describes the cooked asset provenance and summary.
type Manifest struct {
	FormatVersion uint16                   `json:"format_version"`
	SourceFormat  ir.FormatID              `json:"source_format,omitempty"`
	SourceProfile ravenporter.Profile      `json:"source_profile,omitempty"`
	Dependencies  []ravenporter.Dependency `json:"dependencies,omitempty"`
	Notes         map[string][]string      `json:"notes,omitempty"`
	Summary       ravenporter.AssetSummary `json:"summary"`
}

type writeConfig struct {
	imagePixels           ImagePixelsMode
	maxEmbeddedMediaBytes int64
}

type readConfig struct {
	eagerMedia bool
}

// Write stores a cooked asset in the RavenPorter cache container format.
func Write(w io.Writer, result *ravenporter.Result, options ...Option) error {
	cfg, err := resolveWriteOptions(options...)
	if err != nil {
		return err
	}
	_ = cfg

	if err := Validate(result); err != nil {
		return err
	}

	manifest := manifestFromResult(result)
	manifestData, err := marshalManifest(manifest)
	if err != nil {
		return err
	}

	asset, err := prepareAssetForWrite(result.Asset, cfg)
	if err != nil {
		return err
	}

	blobs := newBlobBuilder(cfg)
	sceneData, err := marshalScene(asset, blobs, cfg)
	if err != nil {
		return err
	}
	if err := blobs.Err(); err != nil {
		return err
	}

	return writeContainer(w, []chunk{
		{id: chunkManifest, data: manifestData},
		{id: chunkScene, data: sceneData},
		{id: chunkBlob, data: blobs.Bytes()},
	})
}

// Read loads a cooked asset from an io.ReaderAt.
func Read(r io.ReaderAt, size int64, options ...ReadOption) (*Asset, error) {
	cfg, err := resolveReadOptions(options...)
	if err != nil {
		return nil, err
	}
	_ = cfg

	sections, err := readContainer(r, size, cfg)
	if err != nil {
		return nil, err
	}

	manifest, err := unmarshalManifest(sections.manifest)
	if err != nil {
		return nil, errors.Join(err, sections.blobs.close())
	}
	decodedAsset, err := unmarshalScene(sections.scene, sections.blobs)
	if err != nil {
		return nil, errors.Join(err, sections.blobs.close())
	}
	if cfg.eagerMedia {
		if err := materializeAssetMedia(decodedAsset); err != nil {
			return nil, errors.Join(err, sections.blobs.close())
		}
	}

	asset := &Asset{
		Manifest: manifest,
		Asset:    decodedAsset,
		store:    sections.blobs,
	}
	asset.buildIndexes()
	return asset, nil
}

// Close releases any reader-backed media storage held by the cooked asset.
func (a *Asset) Close() error {
	if a == nil || a.store == nil {
		return nil
	}
	return a.store.close()
}

// Open loads a cooked asset from the local filesystem.
func Open(path string, options ...ReadOption) (*Asset, error) {
	file, err := fsOpen(path)
	if err != nil {
		return nil, err
	}
	return readOpenFile(file, options...)
}

// OpenFS loads a cooked asset from an arbitrary fs.FS.
func OpenFS(fsys fs.FS, path string, options ...ReadOption) (*Asset, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	if readerAt, ok := file.(io.ReaderAt); ok {
		if osFile, ok := file.(interface {
			io.ReaderAt
			io.Closer
			Stat() (fs.FileInfo, error)
		}); ok {
			return readOpenFile(osFile, options...)
		}
		info, err := file.Stat()
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}
		asset, err := Read(readerAt, info.Size(), options...)
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}
		if asset.store != nil && asset.store.attachCloser(file) {
			return asset, nil
		}
		return asset, file.Close()
	}

	data, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	asset, err := Read(bytesReaderAt(data), int64(len(data)), options...)
	return asset, err
}

// Validate verifies that a result can be stored as a runtime cache asset.
func Validate(result *ravenporter.Result) error {
	if result == nil {
		return errNilResult
	}
	if result.Asset == nil {
		return errNilAsset
	}
	for i, tex := range result.Asset.Textures {
		if tex == nil {
			continue
		}
		if tex.ImageIndex < 0 || tex.ImageIndex >= len(result.Asset.Images) {
			continue
		}
		image := result.Asset.Images[tex.ImageIndex]
		if image == nil || image.SourcePath == "" {
			continue
		}
		if isDataURI(image.SourcePath) {
			if _, err := decodeDataURI(image.SourcePath); err != nil {
				return err
			}
			continue
		}
		if !isDataURI(image.SourcePath) {
			return fmtErrorf("cache: texture[%d] still references external path %q", i, image.SourcePath)
		}
	}
	for i, mat := range result.Asset.Materials {
		if mat == nil {
			continue
		}
		if err := validateMaterialProperties(mat.Properties); err != nil {
			return fmtErrorf("cache: material[%d] %q: %w", i, mat.Name, err)
		}
	}
	return nil
}

func resolveWriteOptions(options ...Option) (writeConfig, error) {
	cfg := writeConfig{
		imagePixels: ImagePixelsNever,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return writeConfig{}, err
		}
	}
	return cfg, nil
}

func resolveReadOptions(options ...ReadOption) (readConfig, error) {
	cfg := readConfig{}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return readConfig{}, err
		}
	}
	return cfg, nil
}

func readOpenFile(file interface {
	io.ReaderAt
	io.Closer
	Stat() (fs.FileInfo, error)
}, options ...ReadOption) (*Asset, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	asset, err := Read(file, info.Size(), options...)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if asset.store != nil && asset.store.attachCloser(file) {
		return asset, nil
	}
	return asset, file.Close()
}
