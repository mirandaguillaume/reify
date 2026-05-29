package main

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEmergentTagsPrefixStripping(t *testing.T) {
	// "Labels:" prefix (case-insensitive) is stripped before tokenizing.
	assert.Equal(t, []string{"foo_bar", "baz"}, parseEmergentTags("Labels: foo_bar baz"))
	assert.Equal(t, []string{"foo_bar", "baz"}, parseEmergentTags("LABELS: foo_bar baz"))
	assert.Equal(t, []string{"alpha"}, parseEmergentTags("tag: alpha"))
	assert.Equal(t, []string{"alpha"}, parseEmergentTags("Tags: alpha"))
	assert.Equal(t, []string{"alpha"}, parseEmergentTags("answer: alpha"))
}

func TestParseEmergentTagsHyphenToUnderscore(t *testing.T) {
	// Internal hyphens become underscores after the leading hyphen is trimmed
	// from the token's edges (trim set includes '-').
	assert.Equal(t, []string{"foo_bar"}, parseEmergentTags("foo-bar"))
	// A leading hyphen is trimmed away, not converted.
	assert.Equal(t, []string{"bar"}, parseEmergentTags("-bar"))
	assert.Equal(t, []string{"a_b_c"}, parseEmergentTags("a-b-c"))
}

func TestParseEmergentTagsDedup(t *testing.T) {
	// Duplicates collapse, first-seen order preserved.
	assert.Equal(t, []string{"foo", "bar"}, parseEmergentTags("foo bar foo bar"))
	// "foo-bar" and "foo_bar" normalise to the same token.
	assert.Equal(t, []string{"foo_bar"}, parseEmergentTags("foo-bar foo_bar"))
}

func TestParseEmergentTagsFiveTagCap(t *testing.T) {
	// Seven distinct tags supplied, only the first five are kept.
	got := parseEmergentTags("a b c d e f g")
	require.Len(t, got, 5)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, got)
}

func TestParseEmergentTagsSeparators(t *testing.T) {
	// Comma, slash, pipe, semicolon, tab, newline all split tokens.
	assert.Equal(t, []string{"a", "b", "c"}, parseEmergentTags("a,b,c"))
	assert.Equal(t, []string{"a", "b", "c"}, parseEmergentTags("a/b/c"))
	assert.Equal(t, []string{"a", "b", "c"}, parseEmergentTags("a|b|c"))
	assert.Equal(t, []string{"a", "b", "c"}, parseEmergentTags("a;b;c"))
	assert.Equal(t, []string{"a", "b", "c"}, parseEmergentTags("a\tb\nc"))
}

func TestParseEmergentTagsMixedCaseLowercased(t *testing.T) {
	assert.Equal(t, []string{"foobar", "baz"}, parseEmergentTags("FooBar BAZ"))
}

func TestParseEmergentTagsEmptyInput(t *testing.T) {
	got := parseEmergentTags("")
	assert.Len(t, got, 0)
	assert.Nil(t, got)
}

func TestParseEmergentTagsWhitespaceOnly(t *testing.T) {
	got := parseEmergentTags("   \t\n  ")
	assert.Len(t, got, 0)
}

func TestParseEmergentTagsGarbagePunctuationDropped(t *testing.T) {
	// Tokens made entirely of punctuation normalise to "" and are skipped.
	got := parseEmergentTags("!!! @@@ ###")
	assert.Len(t, got, 0)
	// Mixed: punctuation noise around a real token is stripped.
	assert.Equal(t, []string{"foo"}, parseEmergentTags("!!! foo @@@"))
}

func TestParseEmergentTagsBacktickQuoteWrapping(t *testing.T) {
	// Surrounding backticks/quotes around the whole string are trimmed.
	assert.Equal(t, []string{"foo", "bar"}, parseEmergentTags("`foo bar`"))
	assert.Equal(t, []string{"foo"}, parseEmergentTags("\"foo\""))
}

func TestFilterSnakeCaseBasic(t *testing.T) {
	// Uppercase and punctuation are dropped (no lowercasing happens here).
	assert.Equal(t, "oo", filterSnakeCase("Foo"))
	// Hyphen and '!' are not in [a-z0-9_], so they are dropped entirely here
	// and the surviving letters concatenate.
	assert.Equal(t, "foobar", filterSnakeCase("foo-bar!!"))
}

func TestFilterSnakeCaseDoubleUnderscoreCollapse(t *testing.T) {
	assert.Equal(t, "a_b", filterSnakeCase("a__b"))
	assert.Equal(t, "a_b", filterSnakeCase("a___b"))
	assert.Equal(t, "a_b_c", filterSnakeCase("a__b__c"))
}

func TestFilterSnakeCaseLeadingTrailingUnderscoreTrim(t *testing.T) {
	assert.Equal(t, "abc", filterSnakeCase("_abc_"))
	assert.Equal(t, "abc", filterSnakeCase("__abc__"))
	assert.Equal(t, "a_b", filterSnakeCase("_a_b_"))
}

func TestFilterSnakeCaseAllInvalid(t *testing.T) {
	assert.Equal(t, "", filterSnakeCase("!!!"))
	assert.Equal(t, "", filterSnakeCase("ABC"))
	assert.Equal(t, "", filterSnakeCase("___"))
	assert.Equal(t, "", filterSnakeCase(""))
}

func TestFilterSnakeCaseKeepsValid(t *testing.T) {
	assert.Equal(t, "foo_bar", filterSnakeCase("foo_bar"))
	assert.Equal(t, "abc123", filterSnakeCase("abc123"))
}

func TestParseJudgeAnswerKeepsValidFacets(t *testing.T) {
	assert.Equal(t, []string{"context"}, parseJudgeAnswer("context"))
	assert.Equal(t, []string{"guardrails", "security"}, parseJudgeAnswer("guardrails security"))
}

func TestParseJudgeAnswerDropsUnknownTokens(t *testing.T) {
	// "workflow" and "topic" are not valid facets and are silently dropped.
	assert.Equal(t, []string{"strategy"}, parseJudgeAnswer("workflow strategy topic"))
	got := parseJudgeAnswer("workflow topic")
	assert.Len(t, got, 0)
}

func TestParseJudgeAnswerPrefixAndComma(t *testing.T) {
	// "Facets:" prefix stripped, comma separates the two valid facets.
	assert.Equal(t, []string{"guardrails", "security"}, parseJudgeAnswer("Facets: guardrails, security"))
	assert.Equal(t, []string{"context"}, parseJudgeAnswer("Facet: context"))
	assert.Equal(t, []string{"context"}, parseJudgeAnswer("Label: context"))
}

func TestParseJudgeAnswerDedup(t *testing.T) {
	assert.Equal(t, []string{"security"}, parseJudgeAnswer("security security security"))
	assert.Equal(t, []string{"context", "security"}, parseJudgeAnswer("context security context"))
}

func TestParseJudgeAnswerMixedCaseLowercased(t *testing.T) {
	// Tokenization lowercases, so capitalised facet names are still matched.
	assert.Equal(t, []string{"context"}, parseJudgeAnswer("Context"))
	assert.Equal(t, []string{"guardrails", "security"}, parseJudgeAnswer("GUARDRAILS SECURITY"))
}

func TestParseJudgeAnswerSeparators(t *testing.T) {
	// space, comma, semicolon, slash, tab, newline split — pipe does NOT.
	assert.Equal(t, []string{"context", "strategy"}, parseJudgeAnswer("context;strategy"))
	assert.Equal(t, []string{"context", "strategy"}, parseJudgeAnswer("context/strategy"))
	assert.Equal(t, []string{"context", "strategy"}, parseJudgeAnswer("context\tstrategy"))
	// Pipe is not a separator here, so "context|strategy" is one token and
	// fails the facet check.
	assert.Len(t, parseJudgeAnswer("context|strategy"), 0)
}

func TestParseJudgeAnswerEmptyAndNoneValid(t *testing.T) {
	assert.Len(t, parseJudgeAnswer(""), 0)
	assert.Len(t, parseJudgeAnswer("   "), 0)
	assert.Len(t, parseJudgeAnswer("foo bar baz"), 0)
}

func TestParseJudgeAnswerAllFiveFacets(t *testing.T) {
	got := parseJudgeAnswer("context, strategy, guardrails, observability, security")
	assert.Equal(t, []string{"context", "strategy", "guardrails", "observability", "security"}, got)
}

func TestParseIsValidFacetAllFiveTrue(t *testing.T) {
	for _, f := range classifier.AllFacets {
		assert.True(t, isValidFacet(string(f)), "expected %q to be valid", f)
	}
	// Explicit literals so the test fails if AllFacets ever changes silently.
	assert.True(t, isValidFacet("context"))
	assert.True(t, isValidFacet("strategy"))
	assert.True(t, isValidFacet("guardrails"))
	assert.True(t, isValidFacet("observability"))
	assert.True(t, isValidFacet("security"))
}

func TestParseIsValidFacetRejects(t *testing.T) {
	// Case-sensitive: capitalised form is rejected.
	assert.False(t, isValidFacet("Context"))
	assert.False(t, isValidFacet(""))
	assert.False(t, isValidFacet("other"))
	assert.False(t, isValidFacet("workflow"))
	assert.False(t, isValidFacet(" context"))
}

func TestParseClusterResponseHappyPathWithProseAndFence(t *testing.T) {
	in := "Here is the result you asked for:\n```json\n" +
		`{"clusters": {"naming": ["a", "b"], "io": ["c"]}, "outliers": ["z"]}` +
		"\n```\nThat's all."
	clusters, outliers, err := parseClusterResponse(in)
	require.NoError(t, err)
	require.NotNil(t, clusters)
	assert.Equal(t, []string{"a", "b"}, clusters["naming"])
	assert.Equal(t, []string{"c"}, clusters["io"])
	assert.Equal(t, []string{"z"}, outliers)
}

func TestParseClusterResponsePlainObject(t *testing.T) {
	clusters, outliers, err := parseClusterResponse(`{"clusters":{"x":["1"]},"outliers":["y"]}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, clusters["x"])
	assert.Equal(t, []string{"y"}, outliers)
}

func TestParseClusterResponseMissingBraces(t *testing.T) {
	_, _, err := parseClusterResponse("no json here at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON object found")

	_, _, err = parseClusterResponse("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON object found")

	// Only a closing brace, no opening brace.
	_, _, err = parseClusterResponse("just } here")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON object found")
}

func TestParseClusterResponseMalformedJSON(t *testing.T) {
	// Braces present but the content is not valid JSON.
	_, _, err := parseClusterResponse("{not valid json}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestParseClusterResponseEmptyObject(t *testing.T) {
	clusters, outliers, err := parseClusterResponse("prose {} more prose")
	require.NoError(t, err)
	assert.Nil(t, clusters)
	assert.Nil(t, outliers)
}
