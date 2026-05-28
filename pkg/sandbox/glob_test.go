package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobMatch_Simple(t *testing.T) {
	assert.True(t, GlobMatch("*.go", "main.go"))
	assert.False(t, GlobMatch("*.go", "main.rs"))
	assert.True(t, GlobMatch("src/main.go", "src/main.go"))
}

func TestGlobMatch_DoubleStarPrefix(t *testing.T) {
	assert.True(t, GlobMatch("**/*.go", "main.go"))
	assert.True(t, GlobMatch("**/*.go", "src/main.go"))
	assert.True(t, GlobMatch("**/*.go", "src/pkg/main.go"))
	assert.False(t, GlobMatch("**/*.go", "readme.md"))
}

func TestGlobMatch_DoubleStarMid(t *testing.T) {
	assert.True(t, GlobMatch("src/**/*.go", "src/main.go"))
	assert.True(t, GlobMatch("src/**/*.go", "src/pkg/handler.go"))
	assert.True(t, GlobMatch("src/**/*.go", "src/a/b/c.go"))
	assert.False(t, GlobMatch("src/**/*.go", "test/main.go"))
}

func TestGlobMatch_DoubleStarSuffix(t *testing.T) {
	assert.True(t, GlobMatch("src/**", "src/main.go"))
	assert.True(t, GlobMatch("src/**", "src/a/b/c"))
	assert.False(t, GlobMatch("src/**", "test/file"))
}

func TestGlobMatch_ExactMatch(t *testing.T) {
	assert.True(t, GlobMatch(".env", ".env"))
	assert.False(t, GlobMatch(".env", "other/.env"))
}

func TestGlobMatch_DenyPattern(t *testing.T) {
	assert.True(t, GlobMatch("**/.env", ".env"))
	assert.True(t, GlobMatch("**/.env", "config/.env"))
	assert.True(t, GlobMatch("**/credentials*", "credentials.json"))
	assert.True(t, GlobMatch("**/credentials*", "secrets/credentials.yaml"))
}
