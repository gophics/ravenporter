//go:build integration

package integration

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

const (
	errStrCtxFail    = "pipeline must fail if context is canceled"
	errStrCtxAssert  = "error must assert to context.Canceled, got: %v"
	errStrCtxPrompt  = "cancellation must be prompt, took %v"
	maxCancelLatency = 100 * time.Millisecond
)

func TestRun_ContextCancellation_RealDecoder(t *testing.T) {
	path := filepath.Join(corpusDir(t, corpus.ModelOBJBunny), corpus.ModelOBJBunny)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, pipelineErr := pipeline.ImportPath(ctx, path)
	elapsed := time.Since(start)

	require.Error(t, pipelineErr, errStrCtxFail)
	assert.True(t, errors.Is(pipelineErr, context.Canceled), errStrCtxAssert, pipelineErr)
	assert.True(t, elapsed < maxCancelLatency, errStrCtxPrompt, elapsed)
}
