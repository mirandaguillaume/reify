package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeAnnotation(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"percent", "50% complete", "50%25 complete"},
		{"newline", "line1\nline2", "line1%0Aline2"},
		{"carriage_return", "line1\r\nline2", "line1%0D%0Aline2"},
		{"double_colon", "key::value", "key - value"},
		{"combined", "100%\nfoo::bar", "100%25%0Afoo - bar"},
		{"clean", "no special chars", "no special chars"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, escapeAnnotation(tc.input))
		})
	}
}
