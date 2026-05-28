package llmutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"yaml fenced", "```yaml\nfindings:\n  - category: guardrails\n```", "findings:\n  - category: guardrails", false},
		{"yml fenced", "```yml\nfindings:\n  - category: security\n```", "findings:\n  - category: security", false},
		{"plain fenced", "```\nfindings:\n  - category: ordering\n```", "findings:\n  - category: ordering", false},
		{"no fences", "findings:\n  - category: context", "findings:\n  - category: context", false},
		{"text before fence", "Here are findings:\n```yaml\nfindings:\n  - category: guardrails\n```\nEnd.", "findings:\n  - category: guardrails", false},
		{"multiple fences", "```yaml\nfindings:\n  - category: guardrails\n```\nMore text\n```yaml\n- category: security\n```", "findings:\n  - category: guardrails\n- category: security", false},
		{"empty input", "", "", true},
		{"whitespace only", "   \n\t  ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractYAML(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
