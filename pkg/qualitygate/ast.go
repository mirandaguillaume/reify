package qualitygate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// structuralRequirement is an invariant extracted from a template's H2/H3 heading.
type structuralRequirement struct {
	heading   string // lowercased heading with prefix, e.g. "## verdict"
	needsBody bool   // section must have non-empty paragraph or code block
	needsList bool   // section must have at least one list item
}

// mdParser is the shared, stateless goldmark parser instance.
// goldmark creates a new parse context on each Parse call, so concurrent use is safe.
var mdParser = goldmark.New().Parser()

// parseMarkdown returns the goldmark AST document node for src.
func parseMarkdown(src []byte) gast.Node {
	return mdParser.Parse(text.NewReader(src))
}

// extractStructuralRequirements parses tmpl and returns structural invariants
// from H2/H3 headings. Returns nil if no headings exist (triggers heuristic
// fallback in ValidateProduces/ValidateConsumes — AC6, NFR40).
func extractStructuralRequirements(tmpl string) []structuralRequirement {
	if strings.TrimSpace(tmpl) == "" {
		return nil
	}
	src := []byte(tmpl)
	doc := parseMarkdown(src)

	var reqs []structuralRequirement

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		h, ok := child.(*gast.Heading)
		if !ok || (h.Level != 2 && h.Level != 3) {
			continue
		}
		prefix := "##"
		if h.Level == 3 {
			prefix = "###"
		}
		txt := strings.ToLower(strings.TrimSpace(headingText(h, src)))
		req := structuralRequirement{heading: prefix + " " + txt}

		// Scan siblings until the next heading of equal or higher level.
		for sib := child.NextSibling(); sib != nil; sib = sib.NextSibling() {
			if sh, isH := sib.(*gast.Heading); isH && sh.Level <= h.Level {
				break
			}
			switch sib.Kind() {
			case gast.KindParagraph:
				if paragraphNonEmpty(sib, src) {
					req.needsBody = true
				}
			case gast.KindCodeBlock, gast.KindFencedCodeBlock:
				req.needsBody = true
			case gast.KindList:
				req.needsList = true
			}
		}

		reqs = append(reqs, req)
	}

	if len(reqs) == 0 {
		return nil
	}
	return reqs
}

// extractOutputSection finds the first H2/H3 heading in outputDoc whose
// lowercased text matches heading (e.g. "## verdict"). Returns nil if absent.
func extractOutputSection(outputDoc gast.Node, heading string, src []byte) gast.Node {
	for child := outputDoc.FirstChild(); child != nil; child = child.NextSibling() {
		h, ok := child.(*gast.Heading)
		if !ok || (h.Level != 2 && h.Level != 3) {
			continue
		}
		prefix := "##"
		if h.Level == 3 {
			prefix = "###"
		}
		txt := strings.ToLower(strings.TrimSpace(headingText(h, src)))
		if prefix+" "+txt == heading {
			return child
		}
	}
	return nil
}

// validateASTStructure verifies each structural requirement against the output AST.
// Returns the first error encountered.
func validateASTStructure(reqs []structuralRequirement, outputDoc gast.Node, src []byte) error {
	for _, req := range reqs {
		node := extractOutputSection(outputDoc, req.heading, src)
		if node == nil {
			return fmt.Errorf("quality gate: heading %q not found", req.heading)
		}
		if !req.needsBody && !req.needsList {
			continue
		}

		level := node.(*gast.Heading).Level
		hasBody := false
		hasList := false

		for sib := node.NextSibling(); sib != nil; sib = sib.NextSibling() {
			if sh, isH := sib.(*gast.Heading); isH && sh.Level <= level {
				break
			}
			switch sib.Kind() {
			case gast.KindParagraph:
				if paragraphNonEmpty(sib, src) {
					hasBody = true
				}
			case gast.KindCodeBlock, gast.KindFencedCodeBlock:
				hasBody = true
			case gast.KindList:
				if sib.FirstChild() != nil {
					hasList = true
					hasBody = true
				}
			}
		}

		if req.needsBody && !hasBody && !hasList {
			return fmt.Errorf("quality gate: section %q is empty", req.heading)
		}
		if req.needsList && !hasList {
			return fmt.Errorf("quality gate: section %q requires a list", req.heading)
		}
	}
	return nil
}

// headingText returns the combined plain-text content of an ast.Heading node,
// walking all descendants so inline markup (Strong, Emphasis, CodeSpan) is
// transparent: `## **Verdict**` yields "Verdict" rather than "".
func headingText(h *gast.Heading, src []byte) string {
	var buf bytes.Buffer
	_ = gast.Walk(h, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if t, ok := n.(*gast.Text); ok {
			buf.Write(t.Segment.Value(src))
		}
		return gast.WalkContinue, nil
	})
	return buf.String()
}

// paragraphNonEmpty reports whether the paragraph contains at least one
// non-whitespace text segment anywhere in its subtree. Walking all descendants
// makes inline-only paragraphs (`**bold**`, `code`) correctly register as
// non-empty instead of always returning false.
func paragraphNonEmpty(n gast.Node, src []byte) bool {
	found := false
	_ = gast.Walk(n, func(child gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if t, ok := child.(*gast.Text); ok {
			if len(bytes.TrimSpace(t.Segment.Value(src))) > 0 {
				found = true
				return gast.WalkStop, nil
			}
		}
		return gast.WalkContinue, nil
	})
	return found
}

// ExtractDiffComments scans a unified diff for added lines (+) that contain code
// comments or annotation markers (TODO, FIXME, HACK, NOTE, //, #, /* */).
// Returns a []any slice of "[COMMENT] <text>" strings, one per unique comment line.
// Deterministic — no LLM call. Designed for use as the ast_extract_comments builtin.
func ExtractDiffComments(diff string) []any {
	commentPrefixes := []string{"//", "/*", "#", "<!--", "*"}
	annotationMarkers := []string{"TODO", "FIXME", "HACK", "NOTE", "XXX"}

	seen := make(map[string]bool)
	var results []any

	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if trimmed == "" {
			continue
		}

		isComment := false
		for _, prefix := range commentPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				isComment = true
				break
			}
		}
		if !isComment {
			for _, marker := range annotationMarkers {
				if strings.Contains(trimmed, marker+":") || strings.Contains(trimmed, marker+"(") {
					isComment = true
					break
				}
			}
		}

		if isComment {
			entry := "[COMMENT] " + trimmed
			if !seen[entry] {
				seen[entry] = true
				results = append(results, entry)
			}
		}
	}

	return results
}
