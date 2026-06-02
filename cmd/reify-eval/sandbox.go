package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// buildPrompt composes the agent prompt: the task, plus the formulation's
// rule text when present. For the `absent` control (rule == ""), no rule
// section is injected — the agent sees only the task.
func buildPrompt(f *Fixture, rule string) string {
	var b strings.Builder
	b.WriteString("Task:\n")
	b.WriteString(f.Task)
	b.WriteString("\n")
	if strings.TrimSpace(rule) != "" {
		b.WriteString("\nRules:\n")
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	return b.String()
}

// AttemptResult is one agent run's deterministic outcome.
type AttemptResult struct {
	Violated     bool
	Solved       bool
	ChangedPaths []string
	AgentError   bool // claude exited non-zero / reported is_error
}

const sandboxImage = "reify-eval-sandbox"

// runAttempt runs one agent attempt for (fixture, formulation, model,
// effort): copies the fixture repo to a throwaway dir, commits a
// baseline, runs claude headless in Docker over it, then derives the
// deterministic verdict from the git diff. The temp dir is the blast
// radius; nothing touches the original fixture.
func runAttempt(f *Fixture, rule, model, effort string) (*AttemptResult, error) {
	work, err := os.MkdirTemp("", "reify-eval-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)

	if err := copyTree(f.RepoDir, work); err != nil {
		return nil, err
	}
	if err := gitBaseline(work); err != nil {
		return nil, err
	}

	prompt := buildPrompt(f, rule)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// --dangerously-skip-permissions: safe inside the throwaway container.
	// --bare: the fixture rule is the ONLY instruction context.
	docker := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", work+":/work",
		"-e", "ANTHROPIC_API_KEY",
		sandboxImage,
		"claude", "-p", prompt,
		"--model", model, "--effort", effort,
		"--bare", "--dangerously-skip-permissions",
		"--output-format", "json")
	docker.Env = os.Environ() // forwards ANTHROPIC_API_KEY
	agentErr := docker.Run() != nil

	nameOnly, err := exec.Command("git", "-C", work, "diff", "--name-only").Output()
	if err != nil {
		return nil, err
	}
	changed := parseChangedPaths(string(nameOnly))

	targetContent, _ := os.ReadFile(filepath.Join(work, f.TargetFile))

	return &AttemptResult{
		Violated:     isViolated(changed, f.Scope),
		Solved:       isTaskSolved(string(targetContent), f.SolvedMarker),
		ChangedPaths: changed,
		AgentError:   agentErr,
	}, nil
}

// copyTree recursively copies src into dst (dst must exist).
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// gitBaseline makes the work dir a git repo with one baseline commit, so
// post-run `git diff` shows exactly the agent's changes.
func gitBaseline(dir string) error {
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "eval@reify.local"},
		{"config", "user.name", "reify-eval"},
		{"add", "-A"},
		{"commit", "-q", "-m", "baseline"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v: %s", args, err, out)
		}
	}
	return nil
}
