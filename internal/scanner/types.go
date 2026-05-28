package scanner

// EnrichMode controls how codebase context is attached to generated skills.
type EnrichMode string

const (
	// EnrichNone disables codebase enrichment (default).
	EnrichNone EnrichMode = ""
	// EnrichIndex writes separate context files and appends a pointer to each SKILL.md.
	EnrichIndex EnrichMode = "index"
	// EnrichFull inlines codebase context directly into each SKILL.md.
	EnrichFull EnrichMode = "full"
)

// CodebaseContext holds the objective (O) facts collected by the scanner.
type CodebaseContext struct {
	Root      string
	Structure []DirEntry
	Stack     StackInfo
	Commands  []CommandInfo
	Symbols   []SymbolEntry
}

// SymbolEntry represents an exported symbol found via static analysis.
type SymbolEntry struct {
	Package  string // "model"
	Name     string // "SkillBehavior"
	Kind     string // "struct", "func", "interface", "const", "var", "type"
	File     string // relative path, e.g. "pkg/model/skill.go"
	Exported bool
}

// DirEntry represents a directory with its significant files.
type DirEntry struct {
	Path  string   // relative path from root, e.g. "pkg/model"
	Files []string // significant filenames (no path prefix)
}

// StackInfo holds detected language and dependency information.
type StackInfo struct {
	Languages []LangInfo
	Deps      []DepInfo
}

// LangInfo describes a detected language and its prevalence.
type LangInfo struct {
	Name       string // "Go", "TypeScript", "Python"
	Extension  string // ".go", ".ts", ".py"
	FileCount  int
	Percentage float64 // 0-100
}

// DepInfo describes a detected dependency.
type DepInfo struct {
	Name    string // e.g. "cobra", "testify"
	Version string // e.g. "v1.10.0"
	Kind    string // "direct", "dev", "peer"
}

// CommandInfo describes a detected build/test/lint command.
type CommandInfo struct {
	Name    string // "build", "test", "lint"
	Command string // e.g. "go test ./..."
	Source  string // "Makefile", "package.json", "go.mod"
}
