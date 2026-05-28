package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestScanCodebase_GoProject(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "go.mod"), `module example.com/myapp

go 1.22

require (
	github.com/spf13/cobra v1.10.0
	github.com/stretchr/testify v1.11.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
)
`)
	writeFile(t, filepath.Join(root, "cmd/main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "pkg/model/skill.go"), "package model\n")
	writeFile(t, filepath.Join(root, "pkg/model/agent.go"), "package model\n")
	writeFile(t, filepath.Join(root, "internal/cmd/build.go"), "package cmd\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	// Structure
	assert.NotEmpty(t, ctx.Structure)
	found := map[string]bool{}
	for _, e := range ctx.Structure {
		found[e.Path] = true
	}
	assert.True(t, found["cmd"], "should have cmd dir")
	assert.True(t, found["pkg/model"], "should have pkg/model dir")
	assert.True(t, found["internal/cmd"], "should have internal/cmd dir")

	// Languages
	require.NotEmpty(t, ctx.Stack.Languages)
	assert.Equal(t, "Go", ctx.Stack.Languages[0].Name)
	assert.Equal(t, 4, ctx.Stack.Languages[0].FileCount)
	assert.InDelta(t, 100.0, ctx.Stack.Languages[0].Percentage, 0.1)

	// Deps
	depNames := map[string]bool{}
	for _, d := range ctx.Stack.Deps {
		depNames[d.Name] = true
	}
	assert.True(t, depNames["go"], "should detect go runtime")
	assert.True(t, depNames["cobra"], "should detect cobra")
	assert.True(t, depNames["testify"], "should detect testify")
	assert.False(t, depNames["go-spew"], "should skip indirect deps")

	// Commands
	cmdNames := map[string]bool{}
	for _, c := range ctx.Commands {
		cmdNames[c.Name] = true
	}
	assert.True(t, cmdNames["test"], "should detect test command")
	assert.True(t, cmdNames["build"], "should detect build command")
}

func TestScanCodebase_NodeProject(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "package.json"), `{
  "dependencies": {
    "react": "^18.0.0",
    "next": "^14.0.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  },
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "test": "jest"
  }
}`)
	writeFile(t, filepath.Join(root, "src/app.tsx"), "export default function App() {}\n")
	writeFile(t, filepath.Join(root, "src/utils.ts"), "export const x = 1\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	// Languages
	require.NotEmpty(t, ctx.Stack.Languages)
	assert.Equal(t, "TypeScript", ctx.Stack.Languages[0].Name)

	// Deps
	depNames := map[string]bool{}
	for _, d := range ctx.Stack.Deps {
		depNames[d.Name] = true
	}
	assert.True(t, depNames["react"])
	assert.True(t, depNames["next"])
	assert.True(t, depNames["typescript"])

	// Commands
	cmdNames := map[string]bool{}
	for _, c := range ctx.Commands {
		cmdNames[c.Name] = true
	}
	assert.True(t, cmdNames["dev"])
	assert.True(t, cmdNames["build"])
	assert.True(t, cmdNames["test"])
}

func TestScanCodebase_SkipsDotGitAndVendor(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, ".git/config"), "gitconfig")
	writeFile(t, filepath.Join(root, "vendor/lib/lib.go"), "package lib\n")
	writeFile(t, filepath.Join(root, "node_modules/pkg/index.js"), "module.exports = {}\n")
	writeFile(t, filepath.Join(root, "src/main.go"), "package main\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	for _, e := range ctx.Structure {
		assert.NotContains(t, e.Path, ".git")
		assert.NotContains(t, e.Path, "vendor")
		assert.NotContains(t, e.Path, "node_modules")
	}

	// Only src/main.go should be counted.
	require.Len(t, ctx.Stack.Languages, 1)
	assert.Equal(t, 1, ctx.Stack.Languages[0].FileCount)
}

func TestScanCodebase_MixedLanguages(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "lib.go"), "package main\n")
	writeFile(t, filepath.Join(root, "script.py"), "print('hi')\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	require.Len(t, ctx.Stack.Languages, 2)
	// Go should come first (2 files > 1).
	assert.Equal(t, "Go", ctx.Stack.Languages[0].Name)
	assert.Equal(t, 2, ctx.Stack.Languages[0].FileCount)
	assert.Equal(t, "Python", ctx.Stack.Languages[1].Name)
	assert.Equal(t, 1, ctx.Stack.Languages[1].FileCount)
}

func TestScanCodebase_Makefile(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "Makefile"), "build:\n\tgo build ./...\ntest:\n\tgo test ./...\n.PHONY: build test\n")
	writeFile(t, filepath.Join(root, "main.go"), "package main\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	cmdNames := map[string]bool{}
	for _, c := range ctx.Commands {
		if c.Source == "Makefile" {
			cmdNames[c.Name] = true
		}
	}
	assert.True(t, cmdNames["build"])
	assert.True(t, cmdNames["test"])
}

func TestGoModShortName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"github.com/spf13/cobra/v2", "cobra"},
		{"github.com/spf13/cobra", "cobra"},
		{"gopkg.in/yaml.v3", "yaml.v3"},
		{"github.com/fatih/color", "color"},
		{"github.com/stretchr/testify", "testify"},
		{"golang.org/x/sys", "sys"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, goModShortName(tt.input), "goModShortName(%q)", tt.input)
	}
}

func TestIsHashedAsset(t *testing.T) {
	assert.True(t, isHashedAsset("main.a1b2c3d4e5f6.js"))
	assert.True(t, isHashedAsset("flexsearch.433e941a8a573ebb9931fc16fc75266ab6b93f569ac2fb4f3dc66882e0416f4c.js"))
	assert.False(t, isHashedAsset("main.go"))
	assert.False(t, isHashedAsset("build.go"))
	assert.False(t, isHashedAsset("skill.yaml"))
}

func TestScanCodebase_SkipsNoiseFiles(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "CHANGELOG.md"), "# Changes\n")
	writeFile(t, filepath.Join(root, "LICENSE"), "MIT\n")
	writeFile(t, filepath.Join(root, "go.sum"), "hash\n")
	writeFile(t, filepath.Join(root, "package-lock.json"), "{}\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	// Only main.go should appear in structure.
	require.Len(t, ctx.Structure, 1)
	assert.Equal(t, []string{"main.go"}, ctx.Structure[0].Files)
}

func TestScanCodebase_SkipsHashedAssets(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "app.a1b2c3d4e5f6.js"), "//hashed\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	for _, e := range ctx.Structure {
		for _, f := range e.Files {
			assert.NotContains(t, f, "a1b2c3d4e5f6", "hashed asset should be filtered")
		}
	}
}

func TestScanCodebase_PHPProject(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "composer.json"), `{
  "require": {
    "php": ">=8.2",
    "ext-json": "*",
    "symfony/console": "^7.0",
    "symfony/http-kernel": "^7.0"
  },
  "require-dev": {
    "phpunit/phpunit": "^10.0"
  },
  "scripts": {
    "lint": "@php vendor/bin/phpstan analyse"
  }
}`)
	writeFile(t, filepath.Join(root, "src/Controller/HomeController.php"), "<?php\nclass HomeController {}\n")
	writeFile(t, filepath.Join(root, "src/Entity/User.php"), "<?php\nclass User {}\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	// Languages
	require.NotEmpty(t, ctx.Stack.Languages)
	assert.Equal(t, "PHP", ctx.Stack.Languages[0].Name)
	assert.Equal(t, 2, ctx.Stack.Languages[0].FileCount)

	// Deps
	depNames := map[string]string{}
	for _, d := range ctx.Stack.Deps {
		depNames[d.Name] = d.Kind
	}
	assert.Equal(t, "runtime", depNames["php"])
	assert.Equal(t, "runtime", depNames["ext-json"])
	assert.Equal(t, "direct", depNames["symfony/console"])
	assert.Equal(t, "dev", depNames["phpunit/phpunit"])

	// Commands
	cmdNames := map[string]bool{}
	for _, c := range ctx.Commands {
		cmdNames[c.Name] = true
	}
	assert.True(t, cmdNames["lint"])
	assert.True(t, cmdNames["test"])
}

func TestScanCodebase_DropsDeepConfigOnlyDirs(t *testing.T) {
	root := t.TempDir()

	// Source dir — should always appear.
	writeFile(t, filepath.Join(root, "src/Controller/Home.php"), "<?php\n")
	// Deep config-only dir (4 levels) — should be dropped.
	writeFile(t, filepath.Join(root, "src/Tests/Fixtures/Config/Validation/rules.yml"), "rules:\n")
	// Shallow config dir (1 level) — should appear.
	writeFile(t, filepath.Join(root, "config/services.yaml"), "services:\n")

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	paths := map[string]bool{}
	for _, e := range ctx.Structure {
		paths[e.Path] = true
	}
	assert.True(t, paths["src/Controller"], "source dir should appear")
	assert.True(t, paths["config"], "shallow config dir should appear")
	assert.False(t, paths["src/Tests/Fixtures/Config/Validation"], "deep config-only dir should be dropped")
}

func TestScanCodebase_TruncatesLargeProjects(t *testing.T) {
	root := t.TempDir()

	// Create 150 dirs with source files — should be capped.
	for i := 0; i < 150; i++ {
		dir := filepath.Join(root, "src", "pkg"+string(rune('A'+i/26))+string(rune('a'+i%26)))
		writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	}

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(ctx.Structure), maxStructureDirs)
}

func TestScanCodebase_WeightsByCodeDensity(t *testing.T) {
	root := t.TempDir()

	// Heavy app: 45 source files spread across 3 subdirs.
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(root, "apps/big-app/pages", "page"+string(rune('A'+i))+".tsx"), "export default function Page() {}\n")
	}
	for i := 0; i < 15; i++ {
		writeFile(t, filepath.Join(root, "apps/big-app/components", "comp"+string(rune('A'+i))+".tsx"), "export default function Comp() {}\n")
	}
	for i := 0; i < 10; i++ {
		writeFile(t, filepath.Join(root, "apps/big-app/hooks", "use"+string(rune('A'+i))+".ts"), "export function useHook() {}\n")
	}

	// 50 small libs — 1 source file each — to push source dir count > 80.
	for i := 0; i < 50; i++ {
		dir := filepath.Join(root, "libs", "lib-"+string(rune('A'+i/26))+string(rune('a'+i%26)), "src")
		writeFile(t, filepath.Join(dir, "index.ts"), "export {}\n")
	}

	// 40 test dirs.
	for i := 0; i < 40; i++ {
		dir := filepath.Join(root, "tests/e2e", "scenario"+string(rune('A'+i/26))+string(rune('a'+i%26)))
		writeFile(t, filepath.Join(dir, "test.ts"), "it('works', () => {})\n")
	}

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	// Total source dirs (3 big-app + 50 libs = 53) + 40 test dirs = 93 > 80.
	assert.LessOrEqual(t, len(ctx.Structure), maxStructureDirs)

	// apps/big-app should appear — it has the most source files (45).
	// After collapse, it should show child dir hints.
	paths := map[string]bool{}
	for _, e := range ctx.Structure {
		paths[e.Path] = true
	}

	// The heavy app must be visible — either as collapsed group or individual dirs.
	bigAppVisible := paths["apps/big-app"] ||
		paths["apps/big-app/pages"] ||
		paths["apps/big-app/components"] ||
		paths["apps/big-app/hooks"]
	assert.True(t, bigAppVisible, "heavy app dirs should appear in index")

	// If collapsed to apps/big-app, verify child dir hints.
	if paths["apps/big-app"] {
		var bigAppEntry DirEntry
		for _, e := range ctx.Structure {
			if e.Path == "apps/big-app" {
				bigAppEntry = e
				break
			}
		}
		fileSet := map[string]bool{}
		for _, f := range bigAppEntry.Files {
			fileSet[f] = true
		}
		assert.True(t, fileSet["components/"], "should show components/ hint")
		assert.True(t, fileSet["hooks/"], "should show hooks/ hint")
		assert.True(t, fileSet["pages/"], "should show pages/ hint")
	}

	// Test dirs should not dominate.
	testCount := 0
	for p := range paths {
		if isTestDir(p) {
			testCount++
		}
	}
	sourceCount := len(ctx.Structure) - testCount
	assert.Greater(t, sourceCount, 0, "source dirs should appear")
	assert.GreaterOrEqual(t, sourceCount, testCount, "source dirs should have at least as many entries as test dirs")
}

func TestImmediateChildDirs(t *testing.T) {
	subdirs := []string{
		"apps/bo",
		"apps/bo/pages",
		"apps/bo/common/components",
		"apps/bo/common/hooks",
		"apps/bo/cache",
	}
	result := immediateChildDirs("apps/bo", subdirs)
	assert.Equal(t, []string{"cache/", "common/", "pages/"}, result)
}

func TestScanCodebase_EmptyDir(t *testing.T) {
	root := t.TempDir()

	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	assert.Empty(t, ctx.Structure)
	assert.Empty(t, ctx.Stack.Languages)
	assert.Empty(t, ctx.Stack.Deps)
	assert.Empty(t, ctx.Commands)
}
