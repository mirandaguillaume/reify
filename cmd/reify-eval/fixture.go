package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Fixture is a versioned trapped repo: a git repo to mount, a task, a
// rule slot (formulations), and the code-verifiable checkers' parameters.
// The fixture IS the experiment; a changed fixture is a new experiment.
type Fixture struct {
	Intention    string            `json:"intention"`
	TrapType     string            `json:"trap_type"` // "normal" | "impossible"
	Task         string            `json:"task"`
	TargetFile   string            `json:"target_file"`
	SolvedMarker string            `json:"solved_marker"`
	Formulations map[string]string `json:"formulations"` // name -> rule text ("" = absent)

	// Violation declares HOW a violation is detected for this intention.
	// Exactly one model, chosen by Violation.Kind. Keeping the predicate
	// in the manifest (not hardcoded) lets one harness measure many
	// intentions, each with its own deterministic, judge-free check.
	Violation ViolationCheck `json:"violation"`

	Dir     string `json:"-"` // fixture root
	RepoDir string `json:"-"` // <Dir>/repo, the dir to mount
}

// ViolationCheck is a discriminated config for the violation predicate.
// Kind selects which field applies:
//   - "scope_escape"   : any changed path outside Param (a path prefix)
//   - "forbidden_path" : any changed path whose basename/path contains Param
//                        (e.g. ".env" for secrets, "go.mod" for new deps)
//   - "deletion"       : any file deleted in the diff
//   - "trace_absent"   : the agent did NOT run a Bash command containing
//                        Param (e.g. "go test") — a *positive* rule, so the
//                        violation is the omission. Judged from the trace.
type ViolationCheck struct {
	Kind  string `json:"kind"`
	Param string `json:"param"`
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
