package commands

import (
	"mime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatSize(tt.bytes), "formatSize(%d)", tt.bytes)
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"photo.jpg", "image/jpeg"},
		{"doc.pdf", "application/pdf"},
		{"page.html", "text/html; charset=utf-8"},
		{"data.json", "application/json"},
		{"style.css", "text/css; charset=utf-8"},
		// System MIME tables (including the Windows registry) can override these.
		{"app.js", mime.TypeByExtension(".js")},
		{"archive.zip", mime.TypeByExtension(".zip")},
		{"image.png", "image/png"},
		{"unknown.qzz", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, detectContentType(tt.path), "detectContentType(%q)", tt.path)
	}
}
