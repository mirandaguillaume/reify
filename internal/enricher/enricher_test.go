package enricher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleContext() *scanner.CodebaseContext {
	return &scanner.CodebaseContext{
		Root: ".",
		Structure: []scanner.DirEntry{
			{Path: "cmd/reify", Files: []string{"main.go"}},
			{Path: "pkg/model", Files: []string{"skill.go", "agent.go"}},
			{Path: "internal/cmd", Files: []string{"build.go", "score.go"}},
		},
		Stack: scanner.StackInfo{
			Languages: []scanner.LangInfo{
				{Name: "Go", FileCount: 25, Percentage: 100},
			},
			Deps: []scanner.DepInfo{
				{Name: "go", Version: "1.24", Kind: "runtime"},
				{Name: "cobra", Version: "v1.10.0", Kind: "direct"},
				{Name: "testify", Version: "v1.11.0", Kind: "direct"},
			},
		},
		Commands: []scanner.CommandInfo{
			{Name: "test", Command: "go test ./...", Source: "go.mod"},
			{Name: "build", Command: "go build ./...", Source: "go.mod"},
		},
		Symbols: []scanner.SymbolEntry{
			{Package: "model", Name: "SkillBehavior", Kind: "struct", File: "pkg/model/skill.go", Exported: true},
			{Package: "model", Name: "ValidateSkill", Kind: "func", File: "pkg/model/validate.go", Exported: true},
			{Package: "spec", Name: "TargetGenerator", Kind: "interface", File: "pkg/spec/generator.go", Exported: true},
			{Package: "cmd", Name: "RunBuild", Kind: "func", File: "internal/cmd/build.go", Exported: true},
		},
	}
}

func TestRenderIndex(t *testing.T) {
	idx := RenderIndex(sampleContext())

	assert.Contains(t, idx, "# Codebase Index")
	assert.Contains(t, idx, "|root: ./")
	assert.Contains(t, idx, "|IMPORTANT: Read relevant files before making assumptions")
	assert.Contains(t, idx, "|cmd/reify:{main.go}")
	assert.Contains(t, idx, "|pkg/model:{skill.go,agent.go}")
	assert.Contains(t, idx, "|internal/cmd:{build.go,score.go}")
}

func TestRenderStack(t *testing.T) {
	stack := RenderStack(sampleContext())

	assert.Contains(t, stack, "# Stack")
	assert.Contains(t, stack, "|Go (100% of source, 25 files)")
	assert.Contains(t, stack, "|cobra v1.10.0")
	assert.Contains(t, stack, "|testify v1.11.0")
	// Runtime and dev deps are excluded.
	assert.NotContains(t, stack, "go 1.24")
}

func TestRenderSymbolsIndex(t *testing.T) {
	idx := RenderSymbolsIndex(sampleContext())

	assert.Contains(t, idx, "# Key Symbols")
	assert.Contains(t, idx, "symbols/<pkg>.md")
	assert.Contains(t, idx, "|pkg/model:SkillBehavior(struct),ValidateSkill(func)")
	assert.Contains(t, idx, "|pkg/spec:TargetGenerator(interface)")
	assert.Contains(t, idx, "|internal/cmd:RunBuild(func)")
}

func TestRenderSymbolsIndex_Empty(t *testing.T) {
	ctx := &scanner.CodebaseContext{}
	assert.Empty(t, RenderSymbolsIndex(ctx))
}

func TestRenderSymbolsIndex_SkipsMethodsAndConsts(t *testing.T) {
	ctx := &scanner.CodebaseContext{
		Symbols: []scanner.SymbolEntry{
			{Package: "cmd", Name: "RunBuild", Kind: "func", File: "internal/cmd/build.go", Exported: true},
			{Package: "cmd", Name: "Start", Kind: "method", File: "internal/cmd/build.go", Exported: true},
			{Package: "cmd", Name: "MaxRetries", Kind: "const", File: "internal/cmd/build.go", Exported: true},
		},
	}
	idx := RenderSymbolsIndex(ctx)

	assert.Contains(t, idx, "RunBuild(func)")
	assert.NotContains(t, idx, "Start(method)")
	assert.NotContains(t, idx, "MaxRetries(const)")
}

func TestRenderSymbolShard(t *testing.T) {
	symbols := []scanner.SymbolEntry{
		{Package: "model", Name: "SkillBehavior", Kind: "struct", File: "pkg/model/skill.go"},
		{Package: "model", Name: "ValidateSkill", Kind: "func", File: "pkg/model/validate.go"},
		{Package: "model", Name: "MaxSize", Kind: "const", File: "pkg/model/skill.go"},
		{Package: "model", Name: "Start", Kind: "method", File: "pkg/model/skill.go"},
	}

	shard := RenderSymbolShard("pkg/model", symbols)

	assert.Contains(t, shard, "# pkg/model")
	assert.Contains(t, shard, "|struct: SkillBehavior")
	assert.Contains(t, shard, "|func: ValidateSkill")
	assert.Contains(t, shard, "|const: MaxSize")
	assert.Contains(t, shard, "|method: Start")
}

func TestSymbolShardFilename(t *testing.T) {
	assert.Equal(t, "pkg-model.md", SymbolShardFilename("pkg/model"))
	assert.Equal(t, "internal-cmd.md", SymbolShardFilename("internal/cmd"))
	assert.Equal(t, "root.md", SymbolShardFilename("."))
}

func TestRenderPointer_WithSymbols(t *testing.T) {
	ptr := RenderPointer(sampleContext(), "context")

	assert.Contains(t, ptr, "## Codebase Context")
	assert.Contains(t, ptr, "Read `context/index.md` for project structure")
	assert.Contains(t, ptr, "`context/stack.md`")
	assert.Contains(t, ptr, "`context/symbols.md`")
}

func TestRenderPointer_NoSymbols(t *testing.T) {
	ctx := &scanner.CodebaseContext{}
	ptr := RenderPointer(ctx, "context")

	assert.Contains(t, ptr, "`context/stack.md`")
	assert.NotContains(t, ptr, "symbols.md")
}

func TestRenderInline(t *testing.T) {
	inline := RenderInline(sampleContext())

	assert.Contains(t, inline, "## Codebase Context")
	assert.Contains(t, inline, "### Project Structure")
	assert.Contains(t, inline, "cmd/reify/")
	assert.Contains(t, inline, "  main.go")
	assert.Contains(t, inline, "### Stack")
	assert.Contains(t, inline, "**Go**")
	assert.Contains(t, inline, "### Dependencies")
	assert.Contains(t, inline, "cobra v1.10.0")
	assert.Contains(t, inline, "### Commands")
	assert.Contains(t, inline, "**test**: `go test ./...`")
	assert.Contains(t, inline, "### Key Symbols")
	assert.Contains(t, inline, "`SkillBehavior` (struct)")
	assert.Contains(t, inline, "`TargetGenerator` (interface)")
}

func TestRenderInline_NoDeps(t *testing.T) {
	ctx := sampleContext()
	ctx.Stack.Deps = nil
	ctx.Commands = nil

	inline := RenderInline(ctx)

	assert.NotContains(t, inline, "### Dependencies")
	assert.NotContains(t, inline, "### Commands")
}

func TestWriteContextFiles(t *testing.T) {
	dir := t.TempDir()
	contextDir := filepath.Join(dir, "context")

	err := WriteContextFiles(sampleContext(), contextDir)
	require.NoError(t, err)

	// index.md exists and has content.
	indexData, err := os.ReadFile(filepath.Join(contextDir, "index.md"))
	require.NoError(t, err)
	assert.Contains(t, string(indexData), "# Codebase Index")
	assert.Contains(t, string(indexData), "|pkg/model:{skill.go,agent.go}")

	// stack.md exists and has content.
	stackData, err := os.ReadFile(filepath.Join(contextDir, "stack.md"))
	require.NoError(t, err)
	assert.Contains(t, string(stackData), "# Stack")
	assert.Contains(t, string(stackData), "|Go (100% of source, 25 files)")

	// symbols.md exists and has content.
	symData, err := os.ReadFile(filepath.Join(contextDir, "symbols.md"))
	require.NoError(t, err)
	assert.Contains(t, string(symData), "# Key Symbols")
	assert.Contains(t, string(symData), "SkillBehavior(struct)")
}

func TestRenderIndex_Empty(t *testing.T) {
	ctx := &scanner.CodebaseContext{Root: "."}
	idx := RenderIndex(ctx)

	assert.Contains(t, idx, "# Codebase Index")
	assert.Contains(t, idx, "|root: ./")
	// No directory entries.
	assert.NotContains(t, idx, ":{")
}
