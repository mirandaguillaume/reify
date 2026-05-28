package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── T1.5: Config loading ────────────────────────────────────────────────────

func TestLoad_ConfigPresent(t *testing.T) {
	dir := t.TempDir()
	reifyDir := filepath.Join(dir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(reifyDir, "config.yaml"), []byte(`
doctor:
  provider: anthropic
  model: claude-opus-4-7
  confidence_threshold: high
  backup_retention: 336h
  debug: true
  registry_path: /custom/registry.yaml
`), 0644))

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.Doctor.Provider)
	assert.Equal(t, "claude-opus-4-7", cfg.Doctor.Model)
	assert.Equal(t, "high", cfg.Doctor.ConfidenceThreshold)
	assert.Equal(t, 336*time.Hour, cfg.Doctor.BackupRetention)
	assert.True(t, cfg.Doctor.Debug)
	assert.Equal(t, "/custom/registry.yaml", cfg.Doctor.RegistryPath)
}

func TestLoad_ConfigAbsent(t *testing.T) {
	dir := t.TempDir() // no .reify/config.yaml

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Doctor.Provider)
	assert.Equal(t, "", cfg.Doctor.Model)
	assert.Equal(t, time.Duration(0), cfg.Doctor.BackupRetention)
	assert.False(t, cfg.Doctor.Debug)
}

func TestLoad_MalformedConfig(t *testing.T) {
	dir := t.TempDir()
	reifyDir := filepath.Join(dir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(reifyDir, "config.yaml"), []byte(`
doctor:
  provider: [not a string
`), 0644))

	_, err := Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestLoad_NestedDirectoryLookup(t *testing.T) {
	// Config in root, look up from nested child dir.
	root := t.TempDir()
	reifyDir := filepath.Join(root, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(reifyDir, "config.yaml"), []byte(`
doctor:
  provider: ollama
`), 0644))

	nested := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0755))

	cfg, err := Load(nested)
	require.NoError(t, err)
	assert.Equal(t, "ollama", cfg.Doctor.Provider)
}

// ─── T2.3: Environment variable overrides ────────────────────────────────────

func TestApplyEnv_OverridesProvider(t *testing.T) {
	t.Setenv("REIFY_PROVIDER", "openrouter")
	cfg := &Config{}
	ApplyEnv(cfg)
	assert.Equal(t, "openrouter", cfg.Doctor.Provider)
}

func TestApplyEnv_OverridesModel(t *testing.T) {
	t.Setenv("REIFY_MODEL", "gpt-4o")
	cfg := &Config{}
	ApplyEnv(cfg)
	assert.Equal(t, "gpt-4o", cfg.Doctor.Model)
}

func TestApplyEnv_OverridesDebug(t *testing.T) {
	t.Setenv("REIFY_DEBUG", "true")
	cfg := &Config{}
	ApplyEnv(cfg)
	assert.True(t, cfg.Doctor.Debug)
}

func TestApplyEnv_EmptyEnvNoOverride(t *testing.T) {
	t.Setenv("REIFY_PROVIDER", "")
	cfg := &Config{Doctor: DoctorConfig{Provider: "ollama"}}
	ApplyEnv(cfg)
	assert.Equal(t, "ollama", cfg.Doctor.Provider, "empty env var must not override")
}

// ─── T3.5: Full precedence chain ─────────────────────────────────────────────

func TestPrecedence_FlagsWinOverEnvAndConfig(t *testing.T) {
	// Config file sets provider=ollama
	dir := t.TempDir()
	reifyDir := filepath.Join(dir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(reifyDir, "config.yaml"), []byte(`
doctor:
  provider: ollama
`), 0644))

	// Env sets provider=anthropic
	t.Setenv("REIFY_PROVIDER", "anthropic")

	cfg, err := Load(dir)
	require.NoError(t, err)
	ApplyEnv(cfg)

	// Flag sets provider=openrouter → must win
	ApplyFlags(cfg, "openrouter", "", false, true)

	assert.Equal(t, "openrouter", cfg.Doctor.Provider)
}

func TestPrecedence_EnvWinsOverConfig(t *testing.T) {
	dir := t.TempDir()
	reifyDir := filepath.Join(dir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(reifyDir, "config.yaml"), []byte(`
doctor:
  provider: ollama
`), 0644))

	t.Setenv("REIFY_PROVIDER", "anthropic")

	cfg, err := Load(dir)
	require.NoError(t, err)
	ApplyEnv(cfg)
	ApplyFlags(cfg, "", "", false, false) // flag not changed — don't override

	assert.Equal(t, "anthropic", cfg.Doctor.Provider)
}

func TestApplyFlags_EmptyFlagsNoOverride(t *testing.T) {
	cfg := &Config{Doctor: DoctorConfig{Provider: "ollama", Model: "llama3"}}
	ApplyFlags(cfg, "", "", false, false)
	assert.Equal(t, "ollama", cfg.Doctor.Provider, "empty provider flag must not override")
	assert.Equal(t, "llama3", cfg.Doctor.Model, "empty model flag must not override")
}

// ─── Debug precedence edge cases ─────────────────────────────────────────────

func TestApplyEnv_DebugOneOverrides(t *testing.T) {
	t.Setenv("REIFY_DEBUG", "1")
	cfg := &Config{}
	ApplyEnv(cfg)
	assert.True(t, cfg.Doctor.Debug, "REIFY_DEBUG=1 must enable debug")
}

func TestApplyEnv_DebugFalseDisablesConfigDebug(t *testing.T) {
	t.Setenv("REIFY_DEBUG", "false")
	cfg := &Config{Doctor: DoctorConfig{Debug: true}} // config file set debug=true
	ApplyEnv(cfg)
	assert.False(t, cfg.Doctor.Debug, "REIFY_DEBUG=false must disable config-set debug")
}

func TestApplyEnv_DebugZeroDisablesConfigDebug(t *testing.T) {
	t.Setenv("REIFY_DEBUG", "0")
	cfg := &Config{Doctor: DoctorConfig{Debug: true}}
	ApplyEnv(cfg)
	assert.False(t, cfg.Doctor.Debug, "REIFY_DEBUG=0 must disable config-set debug")
}

func TestApplyFlags_DebugFalseOverridesConfig(t *testing.T) {
	cfg := &Config{Doctor: DoctorConfig{Debug: true}} // config file set debug=true
	// debugChanged=true means --debug=false was explicitly passed
	ApplyFlags(cfg, "", "", false, true)
	assert.False(t, cfg.Doctor.Debug, "--debug=false (debugChanged=true) must override config debug=true")
}

func TestApplyFlags_DebugFalseNoChangedNoOverride(t *testing.T) {
	cfg := &Config{Doctor: DoctorConfig{Debug: true}}
	// debugChanged=false means flag was not passed — must not override
	ApplyFlags(cfg, "", "", false, false)
	assert.True(t, cfg.Doctor.Debug, "debug=false without Changed must not override config debug=true")
}
