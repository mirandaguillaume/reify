package analysis

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ASTSymbol represents a top-level code symbol extracted from tree-sitter.
type ASTSymbol struct {
	Kind      string // "function", "method", "class", "interface", "struct"
	Name      string
	StartLine int
	EndLine   int
	Body      string // full source text
	Changed   bool   // true if any diff hunk overlaps this symbol
}

// ASTContext holds parsed AST information for a single file.
type ASTContext struct {
	File     string
	Language string
	Symbols  []ASTSymbol
	Imports  []string
}

// langToGrammar maps our language names to gotreesitter grammar loaders.
var langToGrammar = map[string]func() *gotreesitter.Language{
	"go":         grammars.GoLanguage,
	"python":     grammars.PythonLanguage,
	"java":       grammars.JavaLanguage,
	"typescript": grammars.TypescriptLanguage,
	"javascript": grammars.JavascriptLanguage,
	"ruby":       grammars.RubyLanguage,
}

// symbolQueries maps languages to tree-sitter queries for extracting symbols.
// Each query captures "name" for the symbol name and "def" for the full definition.
var symbolQueries = map[string]string{
	"go": `
		(function_declaration name: (identifier) @name) @def
		(method_declaration name: (field_identifier) @name) @def
		(type_declaration (type_spec name: (type_identifier) @name type: (struct_type))) @def
		(type_declaration (type_spec name: (type_identifier) @name type: (interface_type))) @def
	`,
	"python": `
		(function_definition name: (identifier) @name) @def
		(class_definition name: (identifier) @name) @def
	`,
	"java": `
		(method_declaration name: (identifier) @name) @def
		(class_declaration name: (identifier) @name) @def
		(interface_declaration name: (identifier) @name) @def
	`,
	"typescript": `
		(function_declaration name: (identifier) @name) @def
		(class_declaration name: (type_identifier) @name) @def
		(interface_declaration name: (type_identifier) @name) @def
		(method_definition name: (property_identifier) @name) @def
	`,
	"javascript": `
		(function_declaration name: (identifier) @name) @def
		(class_declaration name: (identifier) @name) @def
		(method_definition name: (property_identifier) @name) @def
	`,
	"ruby": `
		(method name: (identifier) @name) @def
		(class name: (constant) @name) @def
		(module name: (constant) @name) @def
	`,
}

// importQueries maps languages to tree-sitter queries for extracting imports.
var importQueries = map[string]string{
	"go":         `(import_spec path: (interpreted_string_literal) @path)`,
	"python":     `(import_from_statement module_name: (dotted_name) @path)`,
	"java":       `(import_declaration (scoped_identifier) @path)`,
	"typescript": `(import_statement source: (string) @path)`,
	"javascript": `(import_statement source: (string) @path)`,
	"ruby":       `(call method: (identifier) @method arguments: (argument_list (string) @path) (#eq? @method "require"))`,
}

// ParseAST parses source code using tree-sitter and extracts symbols and imports.
// Returns nil with no error for unsupported languages.
func ParseAST(code []byte, language string) (*ASTContext, error) {
	grammarFn, ok := langToGrammar[language]
	if !ok {
		return nil, nil // unsupported language — graceful degradation
	}
	lang := grammarFn()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(code)
	if err != nil {
		return nil, err
	}

	ctx := &ASTContext{Language: language}

	// Extract symbols
	if queryStr, ok := symbolQueries[language]; ok {
		ctx.Symbols = extractSymbols(tree, lang, code, queryStr, language)
	}

	// Extract imports
	if queryStr, ok := importQueries[language]; ok {
		ctx.Imports = extractImports(tree, lang, code, queryStr)
	}

	return ctx, nil
}

// extractSymbols runs a tree-sitter query and extracts ASTSymbol entries.
func extractSymbols(tree *gotreesitter.Tree, lang *gotreesitter.Language, code []byte, queryStr string, language string) []ASTSymbol {
	q, err := gotreesitter.NewQuery(queryStr, lang)
	if err != nil {
		return nil
	}

	cursor := q.Exec(tree.RootNode(), lang, code)
	var symbols []ASTSymbol

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var name string
		var defNode *gotreesitter.Node

		for _, cap := range match.Captures {
			switch cap.Name {
			case "name":
				name = cap.Node.Text(code)
			case "def":
				defNode = cap.Node
			}
		}

		if name == "" || defNode == nil {
			continue
		}

		startLine := int(defNode.StartPoint().Row) + 1 // tree-sitter is 0-indexed
		endLine := int(defNode.EndPoint().Row) + 1
		body := defNode.Text(code)

		kind := inferKind(defNode.Type(lang), language)

		symbols = append(symbols, ASTSymbol{
			Kind:      kind,
			Name:      name,
			StartLine: startLine,
			EndLine:   endLine,
			Body:      body,
		})
	}

	return symbols
}

// extractImports runs a tree-sitter query and extracts import paths.
func extractImports(tree *gotreesitter.Tree, lang *gotreesitter.Language, code []byte, queryStr string) []string {
	q, err := gotreesitter.NewQuery(queryStr, lang)
	if err != nil {
		return nil
	}

	cursor := q.Exec(tree.RootNode(), lang, code)
	var imports []string

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		for _, cap := range match.Captures {
			if cap.Name == "path" {
				text := cap.Node.Text(code)
				// Strip quotes
				text = strings.Trim(text, `"'`)
				imports = append(imports, text)
			}
		}
	}

	return imports
}

// inferKind maps tree-sitter node types to our symbol kinds.
func inferKind(nodeType, language string) string {
	switch nodeType {
	case "function_declaration", "function_definition":
		return "function"
	case "method_declaration", "method_definition", "method":
		return "method"
	case "class_declaration", "class_definition", "class":
		return "class"
	case "interface_declaration":
		return "interface"
	}
	// Go-specific: type_declaration wrapping struct_type or interface_type
	if strings.Contains(nodeType, "type_declaration") || nodeType == "type_declaration" {
		return "struct" // caller should refine based on inner type
	}
	return "symbol"
}

// OverlayHunks marks symbols as Changed if any DiffHunk overlaps their line range.
func OverlayHunks(ctx *ASTContext, hunks []DiffHunk) {
	for i := range ctx.Symbols {
		for _, h := range hunks {
			// Check if any changed line in the hunk falls within the symbol range
			for _, line := range h.Lines {
				if line.Kind == LineContext {
					continue
				}
				lineNum := line.NewLine
				if lineNum == 0 {
					lineNum = line.OldLine
				}
				if lineNum >= ctx.Symbols[i].StartLine && lineNum <= ctx.Symbols[i].EndLine {
					ctx.Symbols[i].Changed = true
					break
				}
			}
			if ctx.Symbols[i].Changed {
				break
			}
		}
	}
}
