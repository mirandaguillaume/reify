package scanner

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EvalResult holds the evaluation metrics for one scenario.
type EvalResult struct {
	Scenario      string
	Coverage      float64 // % of must-include dirs found in index (0-100)
	SignalToNoise float64 // % of entries that are source, not test/config noise (0-100)
	BudgetUsed    int     // number of entries used out of maxStructureDirs
	ByteSize      int     // total bytes of rendered index
}

// evaluate runs the scanner on a fixture root and computes metrics
// against a ground truth set of must-include directory prefixes.
func evaluate(t *testing.T, root string, mustInclude []string) EvalResult {
	t.Helper()
	ctx, err := ScanCodebase(root)
	require.NoError(t, err)

	// Compute coverage: does each must-include dir appear as a prefix of some entry?
	found := 0
	for _, mi := range mustInclude {
		for _, e := range ctx.Structure {
			if e.Path == mi || strings.HasPrefix(e.Path, mi+"/") || strings.HasPrefix(mi, e.Path+"/") || strings.HasPrefix(mi, e.Path) {
				found++
				break
			}
		}
	}
	coverage := 0.0
	if len(mustInclude) > 0 {
		coverage = float64(found) / float64(len(mustInclude)) * 100
	}

	// Signal-to-noise: % of entries that are NOT test dirs.
	sourceEntries := 0
	for _, e := range ctx.Structure {
		if !isTestDir(e.Path) {
			sourceEntries++
		}
	}
	snr := 0.0
	if len(ctx.Structure) > 0 {
		snr = float64(sourceEntries) / float64(len(ctx.Structure)) * 100
	}

	// Byte size: inline rendering to avoid import cycle with enricher.
	var buf strings.Builder
	for _, entry := range ctx.Structure {
		buf.WriteString("|")
		buf.WriteString(entry.Path)
		buf.WriteString(":{")
		buf.WriteString(strings.Join(entry.Files, ","))
		buf.WriteString("}\n")
	}

	return EvalResult{
		Coverage:      coverage,
		SignalToNoise: snr,
		BudgetUsed:    len(ctx.Structure),
		ByteSize:      buf.Len(),
	}
}

// ---------------------------------------------------------------------------
// Scenario 1: Mono-repo Nx (front-core pattern)
// ---------------------------------------------------------------------------

func buildMonorepoFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	apps := []string{"app-a", "app-b", "app-c", "app-d", "app-e"}
	subdirs := []string{"pages", "components", "hooks", "services"}

	for _, app := range apps {
		for _, sub := range subdirs {
			dir := filepath.Join(root, "apps", app, sub)
			// Each subdir gets 5-20 source files.
			n := 5 + len(app)*3 // deterministic variety
			if n > 20 {
				n = 20
			}
			for i := 0; i < n; i++ {
				writeFile(t, filepath.Join(dir, fmt.Sprintf("file%d.tsx", i)), "export {}\n")
			}
		}
		// Root config for each app.
		writeFile(t, filepath.Join(root, "apps", app, "next.config.js"), "module.exports = {}\n")
	}

	// 40 libs with 1-3 files each.
	for i := 0; i < 40; i++ {
		dir := filepath.Join(root, "libs", fmt.Sprintf("lib-%02d", i), "src")
		writeFile(t, filepath.Join(dir, "index.ts"), "export {}\n")
		if i%3 == 0 {
			writeFile(t, filepath.Join(dir, "utils.ts"), "export {}\n")
		}
		if i%5 == 0 {
			writeFile(t, filepath.Join(dir, "types.ts"), "export {}\n")
		}
	}

	// 30 e2e test scenarios.
	for i := 0; i < 30; i++ {
		dir := filepath.Join(root, "tests", "e2e", fmt.Sprintf("scenario-%02d", i))
		writeFile(t, filepath.Join(dir, "test.spec.ts"), "it('works', () => {})\n")
	}

	// Scripts.
	writeFile(t, filepath.Join(root, "scripts", "build.sh"), "#!/bin/bash\n")
	writeFile(t, filepath.Join(root, "scripts", "deploy.sh"), "#!/bin/bash\n")

	// Root package.json.
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"monorepo","scripts":{"build":"nx build"}}`)

	return root
}

var monorepoMustInclude = []string{
	"apps/app-a", "apps/app-b", "apps/app-c", "apps/app-d", "apps/app-e",
	"libs",
	"scripts",
}

func TestEval_Monorepo(t *testing.T) {
	root := buildMonorepoFixture(t)
	result := evaluate(t, root, monorepoMustInclude)

	t.Logf("Monorepo: coverage=%.1f%% signal=%.1f%% budget=%d/%d bytes=%d",
		result.Coverage, result.SignalToNoise, result.BudgetUsed, maxStructureDirs, result.ByteSize)

	assert.GreaterOrEqual(t, result.Coverage, 85.0, "must-include coverage")
	assert.GreaterOrEqual(t, result.SignalToNoise, 70.0, "signal-to-noise ratio")
	assert.LessOrEqual(t, result.BudgetUsed, maxStructureDirs, "within budget")
}

// ---------------------------------------------------------------------------
// Scenario 2: Deep framework (Symfony pattern)
// ---------------------------------------------------------------------------

func buildDeepFrameworkFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// 15 components with varying depth and file counts.
	components := []string{
		"Console", "HttpFoundation", "HttpKernel", "Routing", "EventDispatcher",
		"DependencyInjection", "Config", "Filesystem", "Finder", "Validator",
		"Form", "Security", "Cache", "Mailer", "Messenger",
	}
	for _, comp := range components {
		base := filepath.Join(root, "src", "Framework", "Component", comp)
		// Root source files (2-8 per component).
		n := 2 + len(comp)%7
		for i := 0; i < n; i++ {
			writeFile(t, filepath.Join(base, fmt.Sprintf("%s%d.php", comp, i)), "<?php\n")
		}
		// Some components have sub-packages.
		if len(comp) > 6 {
			writeFile(t, filepath.Join(base, "Command", "RunCommand.php"), "<?php\n")
			writeFile(t, filepath.Join(base, "Event", "RequestEvent.php"), "<?php\n")
		}
	}

	// 5 bridges.
	bridges := []string{"Doctrine", "Twig", "PhpUnit", "Monolog", "ProxyManager"}
	for _, br := range bridges {
		writeFile(t, filepath.Join(root, "src", "Framework", "Bridge", br, br+"Bridge.php"), "<?php\n")
	}

	// 3 bundles.
	bundles := []string{"FrameworkBundle", "SecurityBundle", "TwigBundle"}
	for _, bu := range bundles {
		writeFile(t, filepath.Join(root, "src", "Framework", "Bundle", bu, bu+".php"), "<?php\n")
	}

	// Tests mirroring source structure.
	for _, comp := range components {
		writeFile(t, filepath.Join(root, "tests", "Component", comp, comp+"Test.php"), "<?php\n")
	}

	// Root composer.json.
	writeFile(t, filepath.Join(root, "composer.json"), `{"require":{"php":">=8.2"}}`)

	return root
}

var deepFrameworkMustInclude = []string{
	"src/Framework/Component/Console",
	"src/Framework/Component/HttpFoundation",
	"src/Framework/Component/HttpKernel",
	"src/Framework/Component/Routing",
	"src/Framework/Component/EventDispatcher",
	"src/Framework/Component/DependencyInjection",
	"src/Framework/Component/Validator",
	"src/Framework/Component/Form",
	"src/Framework/Component/Security",
	"src/Framework/Component/Cache",
	"src/Framework/Bridge/Doctrine",
	"src/Framework/Bridge/Twig",
	"src/Framework/Bundle/FrameworkBundle",
	"src/Framework/Bundle/SecurityBundle",
}

func TestEval_DeepFramework(t *testing.T) {
	root := buildDeepFrameworkFixture(t)
	result := evaluate(t, root, deepFrameworkMustInclude)

	t.Logf("Framework: coverage=%.1f%% signal=%.1f%% budget=%d/%d bytes=%d",
		result.Coverage, result.SignalToNoise, result.BudgetUsed, maxStructureDirs, result.ByteSize)

	assert.GreaterOrEqual(t, result.Coverage, 85.0, "must-include coverage")
	assert.GreaterOrEqual(t, result.SignalToNoise, 65.0, "signal-to-noise ratio")
	assert.LessOrEqual(t, result.BudgetUsed, maxStructureDirs, "within budget")
}

// ---------------------------------------------------------------------------
// Scenario 3: Repetitive micro-services
// ---------------------------------------------------------------------------

func buildMicroservicesFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	services := []string{
		"auth", "users", "orders", "payments", "notifications",
		"inventory", "shipping", "analytics", "search", "gateway",
	}
	innerDirs := []string{"controllers", "services", "models", "utils", "helpers"}

	for _, svc := range services {
		for _, inner := range innerDirs {
			dir := filepath.Join(root, "services", svc, inner)
			n := 3 + len(svc)%4
			for i := 0; i < n; i++ {
				writeFile(t, filepath.Join(dir, fmt.Sprintf("%s%d.ts", inner, i)), "export {}\n")
			}
		}
		writeFile(t, filepath.Join(root, "services", svc, "index.ts"), "export {}\n")
	}

	// Shared libraries.
	writeFile(t, filepath.Join(root, "shared", "utils", "logger.ts"), "export {}\n")
	writeFile(t, filepath.Join(root, "shared", "utils", "http.ts"), "export {}\n")
	writeFile(t, filepath.Join(root, "shared", "helpers", "format.ts"), "export {}\n")
	writeFile(t, filepath.Join(root, "shared", "config", "database.ts"), "export {}\n")

	// Tests.
	for _, svc := range services {
		writeFile(t, filepath.Join(root, "tests", "integration", svc, "test.spec.ts"), "it('ok',()=>{})\n")
	}

	return root
}

var microservicesMustInclude = []string{
	"services/auth", "services/users", "services/orders",
	"services/payments", "services/notifications",
	"services/inventory", "services/shipping",
	"services/analytics", "services/search", "services/gateway",
	"shared/utils", "shared/helpers",
}

func TestEval_Microservices(t *testing.T) {
	root := buildMicroservicesFixture(t)
	result := evaluate(t, root, microservicesMustInclude)

	t.Logf("Microservices: coverage=%.1f%% signal=%.1f%% budget=%d/%d bytes=%d",
		result.Coverage, result.SignalToNoise, result.BudgetUsed, maxStructureDirs, result.ByteSize)

	assert.GreaterOrEqual(t, result.Coverage, 85.0, "must-include coverage")
	assert.GreaterOrEqual(t, result.SignalToNoise, 65.0, "signal-to-noise ratio")
	assert.LessOrEqual(t, result.BudgetUsed, maxStructureDirs, "within budget")
}
