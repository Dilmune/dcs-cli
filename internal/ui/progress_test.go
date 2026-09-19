package ui

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressWriter_WrapReader(t *testing.T) {
	data := []byte("hello world, this is test data for progress tracking")
	pw := NewProgressWriter(int64(len(data)), "Test")
	reader := pw.WrapReader(bytes.NewReader(data))

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, data, result)
	assert.Equal(t, int64(len(data)), pw.written)
}

func TestProgressWriter_ZeroTotal(t *testing.T) {
	pw := NewProgressWriter(0, "Empty")
	reader := pw.WrapReader(bytes.NewReader(nil))

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Empty(t, result)
}
