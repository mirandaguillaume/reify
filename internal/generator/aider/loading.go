package aider

import "github.com/mirandaguillaume/reify/pkg/spec"

// LoadingFiles emits the artefact that makes aider actually load CONVENTIONS.md.
// aider does not auto-discover a conventions file; it reads one only when named
// via `read:` in .aider.conf.yml (aider docs). So we always emit a root
// .aider.conf.yml with that entry.
//
// The file is written with backup (any existing .aider.conf.yml becomes .bak),
// because reify can't safely merge into an arbitrary YAML config — hence the
// warning telling the user to merge the `read:` line into their own conf.
func (g *aiderGenerator) LoadingFiles() ([]spec.ConfigFile, []string) {
	files := []spec.ConfigFile{{
		Path:    "../.aider.conf.yml",
		Content: "# Written by reify so aider loads the conventions file.\nread: CONVENTIONS.md\n",
	}}
	warnings := []string{
		"Wrote ../.aider.conf.yml with `read: CONVENTIONS.md` (any existing one was backed up to .bak) — " +
			"aider only loads CONVENTIONS.md when it is named there; if you already had an .aider.conf.yml, " +
			"merge the `read:` entry back into it.",
	}
	return files, warnings
}
