package sync

import (
	"testing"
)

func TestContentHash(t *testing.T) {
	// Must match plugin's contentHash for sync compatibility
	tests := []struct {
		in   string
		want string
	}{
		{"", "sha256:00"},
		{"# Hi", "sha256:n22m4"},
	}
	for _, tt := range tests {
		got := ContentHash(tt.in)
		if got != tt.want {
			t.Errorf("ContentHash(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	// Smoke: different inputs produce different hashes
	if a, b := ContentHash("a"), ContentHash("b"); a == b {
		t.Errorf("ContentHash(\"a\") == ContentHash(\"b\")")
	}
}
