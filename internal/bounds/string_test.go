package bounds_test

import (
	"testing"

	"github.com/atharvamhaske/flowtel/internal/bounds"
)

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected string
	}{
		{name: "under limit", input: "hello", limit: 10, expected: "hello"},
		{name: "at limit", input: "hello", limit: 5, expected: "hello"},
		{name: "over limit", input: "hello", limit: 3, expected: "hel"},
		{name: "invalid limit", input: "hello", limit: 0, expected: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := bounds.String(tt.input, tt.limit); got != tt.expected {
				t.Fatalf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}
