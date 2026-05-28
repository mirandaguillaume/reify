package generator

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadContracts reads all .md and .template.md files from a contracts directory.
// Returns a map from contract name (filename without extension) to content.
// Supports both "name.md" and "name.template.md" naming conventions.
// Returns an empty map if the directory doesn't exist.
func LoadContracts(contractsDir string) map[string]string {
	contracts := make(map[string]string)

	entries, err := os.ReadDir(contractsDir)
	if err != nil {
		return contracts // missing dir is fine
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(contractsDir, entry.Name()))
		if err != nil {
			continue
		}
		name := entry.Name()
		name = strings.TrimSuffix(name, ".template.md")
		name = strings.TrimSuffix(name, ".md")
		contracts[name] = strings.TrimSpace(string(data))
	}

	return contracts
}

// FormatContractSection returns a markdown section for output format templates.
// Only includes contracts that match the skill's produces list.
// Returns empty string if no matching contracts found.
// When contractsDir is set, generates file references instead of inlining content.
func FormatContractSection(produces []string, contracts map[string]string) string {
	return FormatContractSectionWithDir(produces, contracts, "")
}

// FormatContractSectionWithDir generates output format references.
// If contractsDir is non-empty, produces file pointers; otherwise inlines content.
func FormatContractSectionWithDir(produces []string, contracts map[string]string, contractsDir string) string {
	if len(contracts) == 0 || len(produces) == 0 {
		return ""
	}

	var sections []string
	for _, p := range produces {
		_, ok := contracts[p]
		if !ok {
			continue
		}
		if contractsDir != "" {
			// Try .template.md first, fall back to .md
			tmplPath := filepath.Join(contractsDir, p+".template.md")
			mdPath := filepath.Join(contractsDir, p+".md")
			ref := tmplPath
			if _, err := os.Stat(tmplPath); err != nil {
				ref = mdPath
			}
			sections = append(sections, "- **"+ToTitle(p)+"**: `"+ref+"`")
		} else {
			sections = append(sections, "### Output: "+ToTitle(p)+"\n\n"+contracts[p])
		}
	}

	if len(sections) == 0 {
		return ""
	}

	if contractsDir != "" {
		return "## Output\n" + strings.Join(sections, "\n") + "\n"
	}
	return "## Output Format\n\n" + strings.Join(sections, "\n\n") + "\n"
}
