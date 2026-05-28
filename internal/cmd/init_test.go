package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitProject_CreatesStructure(t *testing.T) {
	dir := t.TempDir()

	result, err := InitProject(dir)
	require.NoError(t, err)

	assert.False(t, result.AlreadyInitialized)
	assert.Equal(t, dir, result.Path)

	// reify.yaml should exist
	_, err = os.Stat(filepath.Join(dir, "reify.yaml"))
	require.NoError(t, err)

	// skills/ directory should exist
	info, err := os.Stat(filepath.Join(dir, "skills"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// agents/ directory should exist
	info, err = os.Stat(filepath.Join(dir, "agents"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// example.skill.yaml should exist
	_, err = os.Stat(filepath.Join(dir, "skills", "example.skill.yaml"))
	require.NoError(t, err)
}

func TestInitProject_DetectsAlreadyInitialized(t *testing.T) {
	dir := t.TempDir()

	// Create reify.yaml to simulate existing project
	err := os.WriteFile(filepath.Join(dir, "reify.yaml"), []byte("version: 0.1.0\n"), 0644)
	require.NoError(t, err)

	result, err := InitProject(dir)
	require.NoError(t, err)

	assert.True(t, result.AlreadyInitialized)
	assert.Equal(t, dir, result.Path)

	// skills/ directory should NOT have been created
	_, err = os.Stat(filepath.Join(dir, "skills"))
	assert.True(t, os.IsNotExist(err))
}
