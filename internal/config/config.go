package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// DoctorConfig holds doctor-specific settings.
type DoctorConfig struct {
	Provider            string        `yaml:"provider"`
	Model               string        `yaml:"model"`
	ConfidenceThreshold string        `yaml:"confidence_threshold"`
	BackupRetention     time.Duration `yaml:"backup_retention"`
	Debug               bool          `yaml:"debug"`
	RegistryPath        string        `yaml:"registry_path,omitempty"`
}

// Config is the top-level config structure for .reify/config.yaml.
type Config struct {
	Doctor DoctorConfig `yaml:"doctor"`
}

// Load reads .reify/config.yaml starting from dir and walking up to the
// user's home directory or filesystem root. Returns a zero-value Config if
// no file is found.
func Load(dir string) (*Config, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return &Config{}, nil
	}

	home, _ := os.UserHomeDir()

	for {
		candidate := filepath.Join(dir, ".reify", "config.yaml")
		data, err := os.ReadFile(candidate)
		if err == nil {
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s: %w", candidate, err)
			}
			return &cfg, nil
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", candidate, err)
		}

		// Stop at home directory or filesystem root.
		if home != "" && dir == home {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root reached
		}
		dir = parent
	}

	return &Config{}, nil
}

// ApplyEnv overrides cfg fields from environment variables.
// Only non-empty env values are applied. REIFY_DEBUG accepts "true", "1"
// to enable and "false", "0" to disable.
func ApplyEnv(cfg *Config) {
	if v := os.Getenv("REIFY_PROVIDER"); v != "" {
		cfg.Doctor.Provider = v
	}
	if v := os.Getenv("REIFY_MODEL"); v != "" {
		cfg.Doctor.Model = v
	}
	if v := os.Getenv("REIFY_DEBUG"); v != "" {
		cfg.Doctor.Debug = v == "true" || v == "1"
	}
}

// ApplyFlags overrides cfg fields from CLI flag values.
// Only non-empty/non-zero string values are applied. debugChanged must be
// true when the --debug flag was explicitly set (use cmd.Flags().Changed).
func ApplyFlags(cfg *Config, provider, model string, debug bool, debugChanged bool) {
	if provider != "" {
		cfg.Doctor.Provider = provider
	}
	if model != "" {
		cfg.Doctor.Model = model
	}
	if debugChanged {
		cfg.Doctor.Debug = debug
	}
}
