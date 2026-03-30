package event

import (
	"testing"
)

func TestHashContentNormalization(t *testing.T) {
	tests := []struct {
		name     string
		content1 string
		content2 string
		same     bool
	}{
		{"same text", "hello", "hello", true},
		{"different text", "hello", "world", false},
		{"LF vs CRLF", "line1\nline2", "line1\r\nline2", true},
		{"Unicode NFC vs NFD", "\u00F1", "n\u0303", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h1 := HashContent(tt.content1)
			h2 := HashContent(tt.content2)
			if (h1 == h2) != tt.same {
				t.Errorf("%s: expected same=%v, got same=%v (h1=%s, h2=%s)", tt.name, tt.same, h1 == h2, h1, h2)
			}
		})
	}
}
