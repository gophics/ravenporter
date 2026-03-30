package assetio

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAndReadAbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.png")
	require.NoError(t, os.WriteFile(path, []byte{0x89, 0x50, 0x4E, 0x47}, 0o644))

	result, err := ResolveAndRead(path, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Data, 4)
	assert.Equal(t, "png", string(result.FormatID))
}

func TestResolveAndReadRelativeWithBaseDir(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "tex.jpg"), []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0o644))

	result, err := ResolveAndRead("tex.jpg", tmp)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "jpeg", string(result.FormatID))
}

func TestResolveAndReadRelativeNoBaseDir(t *testing.T) {
	result, err := ResolveAndRead("missing.png", "")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveAndReadMissingFile(t *testing.T) {
	result, err := ResolveAndRead("nonexistent.png", t.TempDir())
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestResolveAndReadUnknownFormat(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "data.xyz")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	result, err := ResolveAndRead(path, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, string(result.FormatID))
}

func TestReadFromFSNil(t *testing.T) {
	result, err := ReadFromFS("", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestReadFromFSSuccess(t *testing.T) {
	fsys := &stubSeekableFS{
		readCloser: io.NopCloser(strings.NewReader("\x89PNG\r\n\x1a\n")),
	}

	result, err := ReadFromFS("albedo.png", fsys)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []byte("\x89PNG\r\n\x1a\n"), result.Data)
	assert.Equal(t, "png", string(result.FormatID))
}

func TestReadFromFSReadErrorIncludesCloseError(t *testing.T) {
	readErr := errors.New("read failed")
	closeErr := errors.New("close failed")

	fsys := &stubSeekableFS{
		readCloser: errReadCloser{
			readErr:  readErr,
			closeErr: closeErr,
		},
	}

	result, err := ReadFromFS("bad.png", fsys)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, readErr)
	assert.ErrorIs(t, err, closeErr)
}

func TestReadFromFSCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	fsys := &stubSeekableFS{
		readCloser: errReadCloser{
			reader:   strings.NewReader("data"),
			closeErr: closeErr,
		},
	}

	result, err := ReadFromFS("data.bin", fsys)
	require.ErrorIs(t, err, closeErr)
	assert.Nil(t, result)
}

type stubSeekableFS struct {
	readCloser io.ReadCloser
	err        error
}

func (s *stubSeekableFS) Open(_ string) (io.ReadCloser, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.readCloser, nil
}

var _ detect.SeekableFS = (*stubSeekableFS)(nil)

type errReadCloser struct {
	reader   *strings.Reader
	readErr  error
	closeErr error
}

func (e errReadCloser) Read(p []byte) (int, error) {
	if e.readErr != nil {
		return 0, e.readErr
	}
	return e.reader.Read(p)
}

func (e errReadCloser) Close() error {
	return e.closeErr
}
