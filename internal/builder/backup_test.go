package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileWithBackup_NewFileNoBak(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, writeFileWithBackup(p, []byte("v1"), 0644))

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "v1", string(got))
	// no prior file => no .bak
	_, err = os.Stat(p + ".bak")
	assert.True(t, os.IsNotExist(err), "no backup for a brand-new file")
}

func TestWriteFileWithBackup_DifferentContentMakesBak(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, os.WriteFile(p, []byte("hand-edited"), 0644))

	require.NoError(t, writeFileWithBackup(p, []byte("compiled"), 0644))

	// new content landed
	got, _ := os.ReadFile(p)
	assert.Equal(t, "compiled", string(got))
	// old content preserved in .bak
	bak, err := os.ReadFile(p + ".bak")
	require.NoError(t, err)
	assert.Equal(t, "hand-edited", string(bak))
}

func TestWriteFileWithBackup_IdenticalContentNoBak(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, os.WriteFile(p, []byte("same"), 0644))

	require.NoError(t, writeFileWithBackup(p, []byte("same"), 0644))

	// identical rewrite => no churn backup
	_, err := os.Stat(p + ".bak")
	assert.True(t, os.IsNotExist(err), "identical content must not create a .bak")
}
