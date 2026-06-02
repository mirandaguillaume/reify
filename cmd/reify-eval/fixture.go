package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Fixture is a versioned trapped repo: a git repo to mount, a task, a
// rule slot (formulations), and the two code-verifiable checkers'
// parameters. The fixture IS the experiment; a changed fixture is a new
// experiment.
type Fixture struct {
	Intention    string            `json:"intention"`
	TrapType     string            `json:"trap_type"` // "normal" | "impossible"
	Scope        string            `json:"scope"`
	Task         string            `json:"task"`
	TargetFile   string            `json:"target_file"`
	SolvedMarker string            `json:"solved_marker"`
	Formulations map[string]string `json:"formulations"` // name -> rule text ("" = absent)

	Dir     string `json:"-"` // fixture root
	RepoDir string `json:"-"` // <Dir>/repo, the dir to mount
}

// loadFixture reads <dir>/fixture.json and locates <dir>/repo.
func loadFixture(dir string) (*Fixture, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if err != nil {
		return nil, fmt.Errorf("read fixture manifest: %w", err)
	}
	var f Fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse fixture manifest: %w", err)
	}
	if len(f.Formulations) == 0 {
		return nil, fmt.Errorf("fixture has no formulations")
	}
	if _, ok := f.Formulations["absent"]; !ok {
		return nil, fmt.Errorf("fixture must define the 'absent' control formulation")
	}
	f.Dir = dir
	f.RepoDir = filepath.Join(dir, "repo")
	if _, err := os.Stat(f.RepoDir); err != nil {
		return nil, fmt.Errorf("fixture repo dir missing: %w", err)
	}
	return &f, nil
}
