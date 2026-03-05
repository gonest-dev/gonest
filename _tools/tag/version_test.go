package main

import "testing"

func TestBumpVersion(t *testing.T) {
	tests := []struct {
		version  string
		level    string
		expected string
	}{
		{"v0.7.10", "minor", "v0.8.0"},
		{"v0.7.10", "patch", "v0.7.11"},
		{"0.7.10", "minor", "v0.8.0"},
		{"0.7.10", "patch", "v0.7.11"},
		{"v1.0.0", "minor", "v1.1.0"},
		{"v1.0.0", "patch", "v1.0.1"},
		{"v0.1", "patch", "v0.1.1"}, // Handles short versions by appending 0?
		// My implementation pads with 0s if < 3 parts.
		// "v0.1" -> "0.1" -> parts ["0", "1"] -> appends "0" -> ["0", "1", "0"]
		// Patch bump -> "0.1.1" -> "v0.1.1"
	}

	for _, tt := range tests {
		got := bumpVersion(tt.version, tt.level)
		if got != tt.expected {
			t.Errorf("bumpVersion(%q, %q) = %q; want %q", tt.version, tt.level, got, tt.expected)
		}
	}
}
