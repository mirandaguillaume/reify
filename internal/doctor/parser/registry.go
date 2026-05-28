package parser

import (
	"fmt"
	"sort"
)

// FormatParser detects and parses agent definition files in a specific format.
type FormatParser interface {
	Format() string
	Detect(path string, content []byte) bool
	Parse(content []byte) (*AgentAnalysis, error)
	Validate(original, rewritten []byte) error
}

// ParserFactory creates a FormatParser instance.
type ParserFactory func() FormatParser

var parsers = make(map[string]ParserFactory)

// parserOrder maintains deterministic iteration order for DetectFormat.
var parserOrder []string

// Register adds a parser factory to the registry.
func Register(name string, factory ParserFactory) {
	if _, exists := parsers[name]; !exists {
		parserOrder = append(parserOrder, name)
	}
	parsers[name] = factory
}

// Get returns a parser by name.
func Get(name string) (FormatParser, error) {
	factory, ok := parsers[name]
	if !ok {
		return nil, fmt.Errorf("unknown parser %q (available: %v)", name, RegisteredFormats())
	}
	return factory(), nil
}

// DetectFormat iterates registered parsers in registration order and returns the first match.
func DetectFormat(path string, content []byte) (FormatParser, error) {
	for _, name := range parserOrder {
		factory := parsers[name]
		p := factory()
		if p.Detect(path, content) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no parser detected for %s", path)
}

// RegisteredFormats returns the names of all registered parsers in sorted order.
func RegisteredFormats() []string {
	names := make([]string, 0, len(parsers))
	for k := range parsers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
