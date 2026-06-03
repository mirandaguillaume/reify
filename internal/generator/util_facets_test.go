package generator_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestFormatObservability_Empty(t *testing.T) {
	assert.Empty(t, generator.FormatObservability(model.ObservabilityFacet{}))
}

func TestFormatObservability_RendersTraceAndMetrics(t *testing.T) {
	out := generator.FormatObservability(model.ObservabilityFacet{
		TraceLevel: model.TraceLevelStandard,
		Metrics:    []string{"tokens", "issue_count"},
	})
	assert.Contains(t, out, "## Observability")
	assert.Contains(t, out, "Trace level: standard")
	assert.Contains(t, out, "Metrics: tokens, issue_count")
}

func TestFormatSecurity_Empty(t *testing.T) {
	assert.Empty(t, generator.FormatSecurity(model.SecurityFacet{}))
}

func TestFormatSecurity_RendersConstraints(t *testing.T) {
	out := generator.FormatSecurity(model.SecurityFacet{
		Filesystem: model.AccessReadOnly,
		Network:    model.NetworkAllowlist,
		Secrets:    []string{"STRIPE_SECRET_KEY", "DATABASE_URL"},
	})
	assert.Contains(t, out, "## Security")
	assert.Contains(t, out, "Filesystem: read-only")
	assert.Contains(t, out, "Network: allowlist")
	assert.Contains(t, out, "Secrets: STRIPE_SECRET_KEY, DATABASE_URL")
}

func TestFormatSecurity_FileAccess(t *testing.T) {
	out := generator.FormatSecurity(model.SecurityFacet{
		FileAccess: &model.FileAccessConfig{
			Read:  []string{"domain/**"},
			Write: []string{"adapters/**"},
			Deny:  []string{"**/*.env"},
		},
	})
	assert.Contains(t, out, "Read paths: domain/**")
	assert.Contains(t, out, "Write paths: adapters/**")
	assert.Contains(t, out, "Deny paths: **/*.env")
}

// The proven-efficacy contract these helpers exist to support: when both
// observability and security are present, security renders last (recency for
// least-privilege). This guards the ORDER the demo relies on.
func TestFacetSections_SecurityRendersAfterObservability(t *testing.T) {
	obs := generator.FormatObservability(model.ObservabilityFacet{TraceLevel: model.TraceLevelStandard})
	sec := generator.FormatSecurity(model.SecurityFacet{Filesystem: model.AccessReadOnly})
	combined := obs + sec
	assert.Less(t, strings.Index(combined, "## Observability"), strings.Index(combined, "## Security"))
}
