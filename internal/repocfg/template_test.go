package repocfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitWritesAFileThatLoadsBackAsTheDefaults is the property that
// matters: what init writes must parse, and must mean exactly what a
// repository with no file at all means.
func TestInitWritesAFileThatLoadsBackAsTheDefaults(t *testing.T) {
	dir := t.TempDir()

	path, err := Init(dir, false)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, FileName), path)

	got, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, Default(), got)
}

func TestInitWritesPatternsThatCompile(t *testing.T) {
	dir := t.TempDir()
	_, err := Init(dir, false)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	holds, err := cfg.Compiled()
	require.NoError(t, err)

	id, ok := holds.Match("plan/2608142306-fleet-index")
	assert.True(t, ok)
	assert.Equal(t, int64(2608142306), id)
}

func TestInitRefusesToClobberAnEditedFile(t *testing.T) {
	dir := t.TempDir()
	mine := "holds:\n  - \"lane/{id}\"\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, FileName), []byte(mine), 0o600))

	_, err := Init(dir, false)

	require.ErrorIs(t, err, ErrExists)
	after, readErr := os.ReadFile(filepath.Join(dir, FileName))
	require.NoError(t, readErr)
	assert.Equal(t, mine, string(after), "the edit survives")
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, FileName), []byte("holds: []\n"), 0o600))

	_, err := Init(dir, true)

	require.NoError(t, err)
	got, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, Default(), got)
}

func TestInitFailsOnAMissingDirectory(t *testing.T) {
	_, err := Init(filepath.Join(t.TempDir(), "absent"), false)

	require.Error(t, err)
}

func TestTemplateDocumentsEveryKeyItWrites(t *testing.T) {
	// A key that is written but not explained is how a config file
	// rots, so both keys must appear as prose as well as settings.
	assert.Contains(t, Template, "plan-dir:")
	assert.Contains(t, Template, "holds:")
	assert.Contains(t, Template, "{id}")
	assert.Contains(t, Template, "remote prefix stripped")
}

// TestTemplateWritesTheActiveRemoteKey pins that the remote a lease is
// pushed to is written as a live setting, built from the default.
func TestTemplateWritesTheActiveRemoteKey(t *testing.T) {
	assert.Contains(t, Template, "remote: origin")
}

// TestTemplateLeavesBaseCommented pins that base appears only as a
// commented example, because its default is derived from git and an
// active empty value would be wrong.
func TestTemplateLeavesBaseCommented(t *testing.T) {
	assert.Contains(t, Template, "# base: origin/main")
	assert.NotContains(t, Template, "\nbase:")
}
