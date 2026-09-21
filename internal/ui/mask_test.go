package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name, secret, want string
	}{
		{"long secret keeps twelve cells and the ellipsis", "dcs_live_0123456789abcdef", "dcs_live_012…"},
		{"thirteen characters reveal twelve", "abcdefghijklm", "abcdefghijkl…"},
		{"exactly twelve has nothing to hide", "abcdefghijkl", ""},
		{"shorter than twelve is hidden", "short", ""},
		{"empty stays empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MaskSecret(tc.secret))
		})
	}
}
