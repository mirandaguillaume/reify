package scanner

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkipDirs are directories excluded from scanning.
var SkipDirs = map[string]bool{
	".git":         true,
	"vendor":       true,
	"node_modules": true,
	"__pycache__":  true,
	".next":        true,
	"dist":         true,
	"build":        true,
	".claude":      true,
	".github":      true,
	"public":       true, // Hugo/static site build output
	".venv":        true,
	"venv":         true,
	"env":          true,
	".tox":         true,
	"coverage":     true,
	".nyc_output":  true,
	"target":       true, // Rust/Java build output
}

// langMap maps file extensions to language names.
var langMap = map[string]string{
	".go":   "Go",
	".ts":   "TypeScript",
	".tsx":  "TypeScript",
	".js":   "JavaScript",
	".jsx":  "JavaScript",
	".py":   "Python",
	".rs":   "Rust",
	".java": "Java",
	".rb":   "Ruby",
	".cs":   "C#",
	".cpp":  "C++",
	".c":    "C",
	".swift": "Swift",
	".kt":    "Kotlin",
	".php":   "PHP",
	".scala": "Scala",
	".ex":    "Elixir",
	".exs":   "Elixir",
}

// SourceExts are file extensions that represent actual source code.
var SourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".rb": true, ".cs": true,
	".cpp": true, ".c": true, ".swift": true, ".kt": true,
	".php": true, ".scala": true, ".ex": true, ".exs": true,
	".sql": true, ".sh": true,
}

// configExts are configuration files worth tracking.
var configExts = map[string]bool{
	".yaml": true, ".yml": true, ".toml": true, ".json": true,
}

// noiseFiles are filenames excluded from the index regardless of extension.
var noiseFiles = map[string]bool{
	"CHANGELOG.md": true, "CHANGES.md": true, "HISTORY.md": true,
	"LICENSE": true, "LICENSE.md": true, "LICENSE.txt": true,
	"SECURITY.md": true, "CODE_OF_CONDUCT.md": true, "CONDUCT.md": true,
	"AUTHORS.md": true, "CONTRIBUTORS.md": true,
	".release-please-manifest.json": true, "release-please-config.json": true,
	"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"go.sum": true, "Cargo.lock": true, "Gemfile.lock": true,
	"poetry.lock":    true,
	"composer.lock":  true,
	"phpunit.xml":    true,
	"phpstan.neon":   true,
}

// repeatedManifests are config files that appear in many sub-packages (mono-repos).
// They are kept at root level but stripped from deeper directories.
var repeatedManifests = map[string]bool{
	"composer.json":  true,
	"package.json":   true,
	"tsconfig.json":  true,
	"Cargo.toml":     true,
	".eslintrc.json": true,
}

// monorepoBoilerplate are config files repeated in every workspace/package of a mono-repo.
// Always excluded from the index — they add no signal for an agent.
var monorepoBoilerplate = map[string]bool{
	"project.json":         true, // Nx
	"tsconfig.lib.json":    true,
	"tsconfig.spec.json":   true,
	"tsconfig.app.json":    true,
	"tsconfig.build.json":  true,
	"tsconfig.storybook.json": true,
	"jest.config.ts":       true,
	"jest.config.js":       true,
	"jest-e2e.config.ts":   true,
	".prettierrc.js":       true,
	".prettierrc":          true,
	".eslintrc.js":         true,
	".eslintrc.json":       true,
	"vite.config.ts":       true,
	"vite.config.js":       true,
}

// infraDirs are directories that contain infrastructure/devops, not application code.
var infraDirs = map[string]bool{
	"docker":   true,
	"charts":   true,
	"fixtures": true,
	".husky":   true,
	".idea":    true,
	".vscode":  true,
	".nx":      true,
	".grepai":  true,
	".serena":  true,
	".cache":   true,
	"reports":  true,
	"tmp":      true,
}

// ScanCodebase walks the filesystem rooted at root and collects objective codebase facts.
func ScanCodebase(root string) (*CodebaseContext, error) {
	ctx := &CodebaseContext{Root: root}

	dirFiles := map[string][]string{}
	dirHasSource := map[string]bool{}
	dirSourceCount := map[string]int{}
	langCount := map[string]int{}
	totalSourceFiles := 0

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}

		// Skip excluded directories.
		if d.IsDir() {
			if SkipDirs[d.Name()] || infraDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(d.Name())

		// Track language stats.
		if lang, ok := langMap[ext]; ok {
			langCount[lang]++
			totalSourceFiles++
		}

		// Track directory structure for significant files.
		name := d.Name()
		dir := filepath.Dir(rel)
		if noiseFiles[name] || isHashedAsset(name) || monorepoBoilerplate[name] {
			// Skip changelogs, lock files, hashed build assets, mono-repo boilerplate.
		} else if SourceExts[ext] {
			dirFiles[dir] = append(dirFiles[dir], name)
			dirHasSource[dir] = true
			dirSourceCount[dir]++
		} else if configExts[ext] || name == "Makefile" || name == "Dockerfile" {
			// Skip repeated manifests (composer.json, package.json) in sub-dirs.
			if repeatedManifests[name] && dir != "." {
				// only noise in mono-repos
			} else {
				dirFiles[dir] = append(dirFiles[dir], name)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Build structure: prioritize dirs with source code, drop config-only deep dirs.
	ctx.Structure = buildStructure(dirFiles, dirHasSource, dirSourceCount)

	// Build language info.
	ctx.Stack.Languages = buildLanguages(langCount, totalSourceFiles)

	// Detect deps and commands from manifest files.
	ctx.Stack.Deps, ctx.Commands = detectManifests(root)

	// Extract Go symbols if this is a Go project.
	hasGo := false
	for _, lang := range ctx.Stack.Languages {
		if lang.Name == "Go" {
			hasGo = true
			break
		}
	}
	if hasGo {
		symbols, err := ExtractSymbols(root)
		if err == nil {
			ctx.Symbols = symbols
		}
	}

	return ctx, nil
}

// maxStructureDirs limits the index to prevent explosion on large projects.
const maxStructureDirs = 80

// maxFilesPerDir caps the number of files shown per directory entry.
const maxFilesPerDir = 15

// isTestDir returns true for directories that contain tests/fixtures.
func isTestDir(dir string) bool {
	lower := strings.ToLower(dir)
	for _, seg := range strings.Split(lower, string(filepath.Separator)) {
		switch seg {
		case "test", "tests", "testing", "fixtures", "fixture", "testdata", "__tests__", "spec", "specs":
			return true
		}
	}
	return false
}

// truncatePath returns the first N segments of a path.
// truncatePath("src/Symfony/Component/Console/Command", 4) → "src/Symfony/Component/Console"
func truncatePath(p string, depth int) string {
	parts := strings.Split(p, string(filepath.Separator))
	if len(parts) > depth {
		parts = parts[:depth]
	}
	return strings.Join(parts, string(filepath.Separator))
}

func buildStructure(dirFiles map[string][]string, dirHasSource map[string]bool, dirSourceCount map[string]int) []DirEntry {
	// Collect all dirs.
	dirs := make([]string, 0, len(dirFiles))
	for d := range dirFiles {
		dirs = append(dirs, d)
	}

	// Drop config-only dirs deeper than 3 levels (test fixtures).
	filtered := dirs[:0]
	for _, d := range dirs {
		if !dirHasSource[d] && strings.Count(d, string(filepath.Separator)) >= 3 {
			continue
		}
		filtered = append(filtered, d)
	}
	dirs = filtered

	// If under budget, return flat list sorted alphabetically.
	if len(dirs) <= maxStructureDirs {
		return buildFlatEntries(dirs, dirFiles)
	}

	// Too many dirs: collapse to a depth that fits within budget.
	// Source dirs get priority over test dirs, weighted by source file count.
	return buildCollapsedEntries(dirs, dirFiles, dirSourceCount)
}

// buildFlatEntries returns one entry per directory (small project behavior).
func buildFlatEntries(dirs []string, dirFiles map[string][]string) []DirEntry {
	sort.Strings(dirs)
	entries := make([]DirEntry, 0, len(dirs))
	for _, d := range dirs {
		files := dirFiles[d]
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		if len(files) > maxFilesPerDir {
			files = files[:maxFilesPerDir]
		}
		entries = append(entries, DirEntry{Path: d, Files: files})
	}
	return entries
}

// maxCollapsedFilesPerDir caps files shown per entry in collapsed mode (tighter than flat).
const maxCollapsedFilesPerDir = 8

// collapseAtDepth groups dirs by their prefix at the given depth and returns
// entries sorted by combined weight (source count + TF-IDF uniqueness).
func collapseAtDepth(dirs []string, dirFiles map[string][]string, dirSourceCount map[string]int, depth int, idf map[string]float64) []DirEntry {
	type group struct {
		rootFiles   []string // files directly at the prefix path
		allDirs     []string // all dirs that fall under this prefix
		sourceCount int      // total source files in the entire subtree
	}

	groups := map[string]*group{}
	var order []string
	for _, d := range dirs {
		prefix := truncatePath(d, depth)
		g, exists := groups[prefix]
		if !exists {
			g = &group{}
			groups[prefix] = g
			order = append(order, prefix)
		}
		if d == prefix {
			g.rootFiles = dirFiles[d]
		}
		g.allDirs = append(g.allDirs, d)
		g.sourceCount += dirSourceCount[d]
	}

	// Sort by combined weight (source count + TF-IDF uniqueness) descending.
	sort.Slice(order, func(i, j int) bool {
		gi, gj := groups[order[i]], groups[order[j]]
		wi := combinedWeight(gi.sourceCount, DirTFIDF(order[i], idf))
		wj := combinedWeight(gj.sourceCount, DirTFIDF(order[j], idf))
		if wi != wj {
			return wi > wj
		}
		return order[i] < order[j] // stable tie-break: alphabetical
	})

	entries := make([]DirEntry, 0, len(order))
	for _, prefix := range order {
		g := groups[prefix]

		// Collect all files in the subtree for entropy-based cap.
		var subtreeFiles []string
		for _, d := range g.allDirs {
			subtreeFiles = append(subtreeFiles, dirFiles[d]...)
		}
		fileCap := adaptiveFileCap(subtreeFiles)

		var files []string
		if len(g.rootFiles) > 0 {
			// Has root files — show them, then fill remaining with child dir hints.
			files = append(files, g.rootFiles...)
			if len(g.allDirs) > 1 {
				remaining := fileCap - len(files)
				if remaining > 0 {
					hints := immediateChildDirs(prefix, g.allDirs)
					if len(hints) > remaining {
						hints = hints[:remaining]
					}
					files = append(files, hints...)
				}
			}
		} else if g.sourceCount > 0 {
			// No root files but has source in subdirs — show child dir hints.
			files = immediateChildDirs(prefix, g.allDirs)
		} else {
			continue // no source at all, skip
		}

		sort.Strings(files)
		if len(files) > fileCap {
			files = files[:fileCap]
		}
		entries = append(entries, DirEntry{Path: prefix, Files: files})
	}
	return entries
}

// immediateChildDirs extracts the unique immediate sub-directory names from a
// list of full directory paths, relative to the given prefix. Each name gets
// a "/" suffix to distinguish it from files (e.g. "pages/", "components/").
func immediateChildDirs(prefix string, subdirs []string) []string {
	prefixDepth := strings.Count(prefix, string(filepath.Separator)) + 1
	seen := map[string]bool{}
	var result []string
	for _, d := range subdirs {
		if d == prefix {
			continue
		}
		parts := strings.Split(d, string(filepath.Separator))
		if len(parts) > prefixDepth {
			child := parts[prefixDepth] + "/"
			if !seen[child] {
				seen[child] = true
				result = append(result, child)
			}
		}
	}
	sort.Strings(result)
	return result
}

// buildCollapsedEntries handles large projects by collapsing directories.
// It separates source (non-test) dirs from test dirs and collapses each group
// independently, giving source dirs priority in the budget.
func buildCollapsedEntries(dirs []string, dirFiles map[string][]string, dirSourceCount map[string]int) []DirEntry {
	// Compute IDF once across all dirs for TF-IDF weighting.
	idf := DirNameIDF(dirs)

	// Stratified allocation: group by top-level segment, allocate budget
	// proportionally to source file count (Neyman 1934).
	strata := identifyStrata(dirs, dirSourceCount)
	allocateBudget(strata, maxStructureDirs, 0.2)

	// Collapse each stratum independently within its allocated budget.
	var allEntries []DirEntry
	for _, s := range strata {
		if s.Budget <= 0 {
			continue
		}
		entries := findBestCollapse(s.Dirs, dirFiles, dirSourceCount, s.Budget, idf)
		allEntries = append(allEntries, entries...)
	}

	// Deduplicate same-path entries and drop empty ones.
	return deduplicateEntries(allEntries)
}

// deduplicateEntries merges entries with the same path (keeping the one with
// more files) and drops entries with no files.
func deduplicateEntries(entries []DirEntry) []DirEntry {
	byPath := map[string]DirEntry{}
	for _, e := range entries {
		if len(e.Files) == 0 {
			continue
		}
		if existing, ok := byPath[e.Path]; ok {
			if len(e.Files) > len(existing.Files) {
				byPath[e.Path] = e
			}
		} else {
			byPath[e.Path] = e
		}
	}
	deduped := make([]DirEntry, 0, len(byPath))
	for _, e := range byPath {
		deduped = append(deduped, e)
	}
	sort.Slice(deduped, func(i, j int) bool { return deduped[i].Path < deduped[j].Path })
	return deduped
}

// findBestCollapse finds the deepest collapse depth that fits within maxEntries.
// Since collapseAtDepth sorts by source-file weight, truncation naturally
// keeps the most code-dense groups.
func findBestCollapse(dirs []string, dirFiles map[string][]string, dirSourceCount map[string]int, maxEntries int, idf map[string]float64) []DirEntry {
	if len(dirs) == 0 {
		return nil
	}
	// Try from deepest to shallowest.
	for depth := 8; depth >= 1; depth-- {
		entries := collapseAtDepth(dirs, dirFiles, dirSourceCount, depth, idf)
		if len(entries) <= maxEntries {
			return entries
		}
	}
	// Extreme fallback: depth 1, take top N by weight (already sorted).
	entries := collapseAtDepth(dirs, dirFiles, dirSourceCount, 1, idf)
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}
	return entries
}

func buildLanguages(langCount map[string]int, total int) []LangInfo {
	langs := make([]LangInfo, 0, len(langCount))
	for name, count := range langCount {
		pct := 0.0
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		langs = append(langs, LangInfo{
			Name:       name,
			FileCount:  count,
			Percentage: pct,
		})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].FileCount > langs[j].FileCount
	})
	return langs
}

func detectManifests(root string) ([]DepInfo, []CommandInfo) {
	var deps []DepInfo
	var cmds []CommandInfo

	// go.mod
	if d, c := parseGoMod(filepath.Join(root, "go.mod")); d != nil {
		deps = append(deps, d...)
		cmds = append(cmds, c...)
	}

	// package.json
	if d, c := parsePackageJSON(filepath.Join(root, "package.json")); d != nil {
		deps = append(deps, d...)
		cmds = append(cmds, c...)
	}

	// composer.json (PHP)
	if d, c := parseComposerJSON(filepath.Join(root, "composer.json")); d != nil {
		deps = append(deps, d...)
		cmds = append(cmds, c...)
	}

	// Makefile
	if c := parseMakefile(filepath.Join(root, "Makefile")); c != nil {
		cmds = append(cmds, c...)
	}

	return deps, cmds
}

func parseGoMod(path string) ([]DepInfo, []CommandInfo) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	var deps []DepInfo
	var cmds []CommandInfo
	inRequire := false
	isIndirect := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect Go version for commands.
		if strings.HasPrefix(line, "go ") {
			ver := strings.TrimPrefix(line, "go ")
			deps = append(deps, DepInfo{Name: "go", Version: ver, Kind: "runtime"})
			cmds = append(cmds, CommandInfo{Name: "test", Command: "go test ./...", Source: "go.mod"})
			cmds = append(cmds, CommandInfo{Name: "build", Command: "go build ./...", Source: "go.mod"})
			continue
		}

		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			isIndirect = false
			continue
		}

		if inRequire {
			isIndirect = strings.Contains(line, "// indirect")
			if isIndirect {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, DepInfo{
					Name:    goModShortName(parts[0]),
					Version: parts[1],
					Kind:    "direct",
				})
			}
		}
	}

	return deps, cmds
}

func parsePackageJSON(path string) ([]DepInfo, []CommandInfo) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, nil
	}

	var deps []DepInfo
	for name, ver := range pkg.Dependencies {
		deps = append(deps, DepInfo{Name: name, Version: ver, Kind: "direct"})
	}
	for name, ver := range pkg.DevDependencies {
		deps = append(deps, DepInfo{Name: name, Version: ver, Kind: "dev"})
	}
	// Sort for deterministic output.
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	var cmds []CommandInfo
	for name, cmd := range pkg.Scripts {
		cmds = append(cmds, CommandInfo{Name: name, Command: cmd, Source: "package.json"})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })

	return deps, cmds
}

func parseComposerJSON(path string) ([]DepInfo, []CommandInfo) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var composer struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
		Scripts    map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(data, &composer); err != nil {
		return nil, nil
	}

	var deps []DepInfo
	for name, ver := range composer.Require {
		kind := "direct"
		if name == "php" || strings.HasPrefix(name, "ext-") {
			kind = "runtime"
		}
		deps = append(deps, DepInfo{Name: name, Version: ver, Kind: kind})
	}
	for name, ver := range composer.RequireDev {
		deps = append(deps, DepInfo{Name: name, Version: ver, Kind: "dev"})
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	var cmds []CommandInfo
	// composer scripts can be strings or arrays — we extract the key names as commands.
	for name := range composer.Scripts {
		cmds = append(cmds, CommandInfo{Name: name, Command: "composer " + name, Source: "composer.json"})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })

	// Add standard PHP commands.
	cmds = append(cmds, CommandInfo{Name: "test", Command: "vendor/bin/phpunit", Source: "composer.json"})

	return deps, cmds
}

// goModShortName extracts a readable name from a Go module path.
// "github.com/spf13/cobra/v2" → "cobra", "gopkg.in/yaml.v3" → "yaml.v3"
func goModShortName(modulePath string) string {
	segments := strings.Split(modulePath, "/")
	// Walk backwards to find the first segment that isn't just a version (v2, v3...).
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if len(seg) >= 2 && seg[0] == 'v' && seg[1] >= '0' && seg[1] <= '9' {
			continue // skip version suffixes like "v2", "v3"
		}
		return seg
	}
	return segments[len(segments)-1]
}

// isHashedAsset detects filenames with content hashes (e.g., "main.a1b2c3d4.js").
func isHashedAsset(name string) bool {
	// Pattern: name.HASH.ext where HASH is 8+ hex chars.
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return false
	}
	for _, part := range parts[1 : len(parts)-1] {
		if len(part) >= 8 && isHex(part) {
			return true
		}
	}
	return false
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func parseMakefile(path string) []CommandInfo {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var cmds []CommandInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Makefile targets are lines ending with ':'  not starting with tab.
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, ".") {
			target := strings.TrimSuffix(line, ":")
			target = strings.TrimSpace(target)
			if target != "" {
				cmds = append(cmds, CommandInfo{
					Name:    target,
					Command: "make " + target,
					Source:  "Makefile",
				})
			}
		}
	}
	return cmds
}
