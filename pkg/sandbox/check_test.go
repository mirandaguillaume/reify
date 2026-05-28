package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAccess_NilPolicy(t *testing.T) {
	reason, ok := CheckAccess(nil, "any/path", "read")
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckAccess_ReadAllowed(t *testing.T) {
	p := &FileAccessPolicy{ReadGlobs: []string{"src/**"}}
	_, ok := CheckAccess(p, "src/main.go", "read")
	assert.True(t, ok)
}

func TestCheckAccess_ReadDenied(t *testing.T) {
	p := &FileAccessPolicy{ReadGlobs: []string{"src/**"}}
	reason, ok := CheckAccess(p, "secrets/.env", "read")
	assert.False(t, ok)
	assert.Contains(t, reason, "not in allowed")
}

func TestCheckAccess_WriteAllowed(t *testing.T) {
	p := &FileAccessPolicy{WriteGlobs: []string{"src/**"}}
	_, ok := CheckAccess(p, "src/new.go", "write")
	assert.True(t, ok)
}

func TestCheckAccess_WriteDenied(t *testing.T) {
	p := &FileAccessPolicy{WriteGlobs: []string{"src/**"}}
	_, ok := CheckAccess(p, "etc/passwd", "write")
	assert.False(t, ok)
}

func TestCheckAccess_DenyTakesPrecedence(t *testing.T) {
	p := &FileAccessPolicy{
		ReadGlobs: []string{"**"},
		DenyGlobs: []string{"**/.env"},
	}
	reason, ok := CheckAccess(p, ".env", "read")
	assert.False(t, ok)
	assert.Contains(t, reason, "denied by glob")
}

func TestCheckAccess_EmptyReadGlobs_AllowAll(t *testing.T) {
	p := &FileAccessPolicy{} // no read globs
	_, ok := CheckAccess(p, "anywhere/file.txt", "read")
	assert.True(t, ok)
}

func TestCheckAccess_EmptyWriteGlobs_AllowAll(t *testing.T) {
	p := &FileAccessPolicy{} // no write globs
	_, ok := CheckAccess(p, "anywhere/file.txt", "write")
	assert.True(t, ok)
}

func TestCheckAccess_PathTraversalBlocked(t *testing.T) {
	p := &FileAccessPolicy{ReadGlobs: []string{"src/**"}}
	// "src/../etc/passwd" should be cleaned to "etc/passwd" which is outside src/
	_, ok := CheckAccess(p, "src/../etc/passwd", "read")
	assert.False(t, ok, "path traversal via .. should be blocked after filepath.Clean")
}

func TestCheckAccess_DeleteUsesWriteGlobs(t *testing.T) {
	p := &FileAccessPolicy{WriteGlobs: []string{"tmp/**"}}
	_, ok := CheckAccess(p, "tmp/cache.dat", "delete")
	assert.True(t, ok)

	_, ok = CheckAccess(p, "src/main.go", "delete")
	assert.False(t, ok)
}
