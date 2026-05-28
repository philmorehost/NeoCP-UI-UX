package util

import "testing"

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1GB", 1024},
		{"100MB", 100},
		{"1TB", 1024 * 1024},
		{"unlimited", -1},
		{"500", 500},
		{"2 g", 2048},
	}

	for _, tt := range tests {
		got, err := ParseByteSize(tt.input)
		if err != nil {
			t.Errorf("ParseByteSize(%q) error: %v", tt.input, err)
		}
		if got != tt.expected {
			t.Errorf("ParseByteSize(%q) = %d; want %d", tt.input, got, tt.expected)
		}
	}
}
