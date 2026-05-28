// Package classifier maps instructions extracted from an agent file to one of
// the five Reify facets using an LLM. The previous static (keyword-based)
// classifier was removed because subjective prose cannot be classified
// reliably from surface lexical patterns.
package classifier

// Facet is one of the five Reify facets.
type Facet string

const (
	FacetContext       Facet = "context"
	FacetStrategy      Facet = "strategy"
	FacetGuardrails    Facet = "guardrails"
	FacetObservability Facet = "observability"
	FacetSecurity      Facet = "security"
)

// AllFacets lists every facet in canonical order.
var AllFacets = []Facet{
	FacetContext,
	FacetStrategy,
	FacetGuardrails,
	FacetObservability,
	FacetSecurity,
}

// Item is a single classified instruction extracted from an agent file.
type Item struct {
	Text    string
	Facet   Facet
	Section string
}

// Result holds the full classification of an agent file.
type Result struct {
	Format string
	Items  []Item
}

// ByFacet returns items grouped by facet, preserving AllFacets order.
func (r Result) ByFacet() map[Facet][]Item {
	m := make(map[Facet][]Item, len(AllFacets))
	for _, f := range AllFacets {
		m[f] = nil
	}
	for _, item := range r.Items {
		m[item.Facet] = append(m[item.Facet], item)
	}
	return m
}
