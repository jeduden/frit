package repocfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// write drops a .frit.yml into a fresh directory and returns it.
func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, FileName), []byte(body), 0o600))

	return dir
}

func TestDefaultIsTheCanonicalConvention(t *testing.T) {
	got := Default()

	assert.Equal(t, "plan", got.PlanDir)
	assert.Equal(t, []string{"plan/{id}-*"}, got.Holds)
}

func TestLoadOnARepoWithNoFileGetsTheDefaults(t *testing.T) {
	got, err := Load(t.TempDir())

	require.NoError(t, err)
	assert.Equal(t, Default(), got,
		"the common case needs no file at all")
}

func TestLoadReadsMultiplePatterns(t *testing.T) {
	dir := write(t, `holds:
  - "plan/{id}-*"
  - "*/plan-{id}-*"
  - "plan-{id}-*"
`)

	got, err := Load(dir)

	require.NoError(t, err)
	require.Len(t, got.Holds, 3)
	assert.Equal(t, "*/plan-{id}-*", got.Holds[1])
}

func TestLoadKeepsDefaultsForOmittedKeys(t *testing.T) {
	dir := write(t, "holds:\n  - \"lane/{id}\"\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, "plan", got.PlanDir,
		"overriding holds must not reset where plans live")
	assert.Equal(t, []string{"lane/{id}"}, got.Holds)
}

func TestLoadOverridesPlanDirAlone(t *testing.T) {
	dir := write(t, "plan-dir: docs/plans\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, "docs/plans", got.PlanDir)
	assert.Equal(t, Default().Holds, got.Holds)
}

// TestLoadHonoursADeclaredEmptyHoldList pins the difference between
// omitting the key and saying "this repository has no claims".
func TestLoadHonoursADeclaredEmptyHoldList(t *testing.T) {
	dir := write(t, "holds: []\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Empty(t, got.Holds)

	holds, err := got.Compiled()
	require.NoError(t, err)
	_, ok := holds.Match("plan/2608142306-anything")
	assert.False(t, ok)
}

func TestLoadFailsOnMalformedYAML(t *testing.T) {
	dir := write(t, "holds: [unclosed\n")

	_, err := Load(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), FileName)
}

func TestCompiledSurfacesABadPattern(t *testing.T) {
	cfg := Config{Holds: []string{"plan/{id}-*", "broken"}}

	_, err := cfg.Compiled()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
}

func TestDefaultPatternMatchesTheCanonicalBranch(t *testing.T) {
	holds, err := Default().Compiled()
	require.NoError(t, err)

	id, ok := holds.Match("plan/2608142306-fleet-index")

	assert.True(t, ok)
	assert.Equal(t, int64(2608142306), id)
}
