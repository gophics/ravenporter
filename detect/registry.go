package detect

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/gophics/ravenporter/ir"
)

// Decoder reads a specific asset format and populates the IR.
type Decoder interface {
	Probe(r io.ReadSeeker) bool
	Decode(r ReadSeekerAt, opts DecodeOptions) (*ir.Asset, error)
	Extensions() []string
	FormatName() string
}

// ReadSeekerAt is the universal stream handle for decoders.
type ReadSeekerAt interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// SeekableFS abstracts read-only file access for resolving external references.
type SeekableFS interface {
	Open(name string) (io.ReadCloser, error)
}

// DecodeReporter receives direct dependency and provenance notes discovered
// during decode without coupling decoders to higher-level pipeline types.
type DecodeReporter interface {
	AddDependency(kind, path, relation, reportedBy string)
	AddProvenanceNote(key, value string)
}

type DecodeOptions struct {
	Context         context.Context
	FS              SeekableFS
	Reporter        DecodeReporter
	MaxFileSize     int64
	MaxVertices     int
	MaxImagePixels  int
	MaxAudioSamples int
}

type nopFS struct{}

func (n nopFS) Open(string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (opt *DecodeOptions) Sanitize() {
	if opt.Context == nil {
		opt.Context = context.Background()
	}
	if opt.FS == nil {
		opt.FS = nopFS{}
	}

	if opt.MaxVertices <= 0 {
		opt.MaxVertices = math.MaxInt32
	}
	if opt.MaxImagePixels <= 0 {
		opt.MaxImagePixels = math.MaxInt32
	}
	if opt.MaxAudioSamples <= 0 {
		opt.MaxAudioSamples = math.MaxInt32
	}
}

// Registration declares a decoder binding for a specific asset format.
type Registration struct {
	Format  ir.FormatID
	Decoder Decoder
}

// Registry holds format decoders keyed by FormatID.
type Registry struct {
	mu       sync.RWMutex
	order    []ir.FormatID
	decoders map[ir.FormatID]Decoder
}

// NewRegistry creates a decoder registry from declarative registrations.
func NewRegistry(registrations ...Registration) *Registry {
	registry := &Registry{
		order:    make([]ir.FormatID, 0, len(registrations)),
		decoders: make(map[ir.FormatID]Decoder, len(registrations)),
	}
	registry.RegisterAll(registrations...)
	return registry
}

// Register adds a decoder for a format.
func (r *Registry) Register(format ir.FormatID, d Decoder) {
	r.mu.Lock()
	if _, exists := r.decoders[format]; !exists {
		r.order = append(r.order, format)
	}
	r.decoders[format] = d
	r.mu.Unlock()
}

// RegisterAll adds multiple decoder registrations to the registry.
func (r *Registry) RegisterAll(registrations ...Registration) {
	r.mu.Lock()
	for _, registration := range registrations {
		if _, exists := r.decoders[registration.Format]; !exists {
			r.order = append(r.order, registration.Format)
		}
		r.decoders[registration.Format] = registration.Decoder
	}
	r.mu.Unlock()
}

// Lookup returns the decoder for a format, if registered.
func (r *Registry) Lookup(format ir.FormatID) (Decoder, bool) {
	r.mu.RLock()
	d, ok := r.decoders[format]
	r.mu.RUnlock()
	return d, ok
}

// Formats returns all registered format IDs.
func (r *Registry) Formats() []ir.FormatID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]ir.FormatID, 0, len(r.decoders))
	for id := range r.decoders {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// Extensions returns the registered file extensions in canonical sorted order.
func (r *Registry) Extensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	unique := make(map[string]struct{})
	for _, decoder := range r.decoders {
		for _, ext := range decoder.Extensions() {
			if ext == "" {
				continue
			}
			unique[strings.ToLower(ext)] = struct{}{}
		}
	}

	extensions := make([]string, 0, len(unique))
	for ext := range unique {
		extensions = append(extensions, ext)
	}
	slices.Sort(extensions)
	return extensions
}

// SupportsExtension returns true if any registered decoder handles this extension.
func (r *Registry) SupportsExtension(ext string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, d := range r.decoders {
		for _, e := range d.Extensions() {
			if strings.EqualFold(e, ext) {
				return true
			}
		}
	}
	return false
}

func (r *Registry) Detect(reader io.ReadSeeker, filename string) (ir.FormatID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var header [dispatchBufSize]byte
	pos, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return ir.FormatUnknown, err
	}
	n, _ := reader.Read(header[:]) //nolint:errcheck // n checked below
	if _, err := reader.Seek(pos, io.SeekStart); err != nil {
		return ir.FormatUnknown, err
	}
	buf := header[:n]

	if id := matchMagic(buf); id != ir.FormatUnknown {
		if _, ok := r.decoders[id]; ok {
			return id, nil
		}
	}

	for _, id := range r.order {
		d := r.decoders[id]
		if d.Probe(reader) {
			return id, nil
		}
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		for _, id := range r.order {
			d := r.decoders[id]
			for _, e := range d.Extensions() {
				if strings.EqualFold(e, ext) {
					return id, nil
				}
			}
		}
	}

	return ir.FormatUnknown, nil
}
