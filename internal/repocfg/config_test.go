package repocfg

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/discovery"
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
	assert.Equal(t, []string{"plan/{id}", "plan/{id}-*"}, got.Holds,
		"the lease's id-only ref and the decorated legacy shape both count")
}

// TestDefaultRemoteIsOriginAndBaseIsDerived pins that a lease pushes to
// origin by default, while base carries no literal default: it is
// resolved from git at use-time, so repocfg leaves it empty.
func TestDefaultRemoteIsOriginAndBaseIsDerived(t *testing.T) {
	got := Default()

	assert.Equal(t, "origin", got.Remote)
	assert.Empty(t, got.Base,
		"base has no literal default; it is derived from git")
}

func TestLoadOverridesRemoteAndBase(t *testing.T) {
	dir := write(t, "remote: upstream\nbase: origin/trunk\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, "upstream", got.Remote)
	assert.Equal(t, "origin/trunk", got.Base)
}

// TestLoadKeepsRemoteAndBaseDefaultsForOmittedKeys pins that overriding
// one unrelated key does not reset remote or base to something other
// than their defaults.
func TestLoadKeepsRemoteAndBaseDefaultsForOmittedKeys(t *testing.T) {
	dir := write(t, "plan-dir: docs/plans\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, "docs/plans", got.PlanDir)
	assert.Equal(t, "origin", got.Remote,
		"omitting remote keeps its default")
	assert.Empty(t, got.Base, "omitting base keeps it derived")
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

// TestDefaultTakeoverWindowAndSampleGap pins the staleness knobs' own
// defaults to the ones discovery already documents (F12): a
// repository that declares nothing still watches on the same clock.
func TestDefaultTakeoverWindowAndSampleGap(t *testing.T) {
	got := Default()

	assert.Equal(t, discovery.DefaultTakeoverWindow, got.TakeoverWindow)
	assert.Equal(t, discovery.DefaultSampleGap, got.SampleGap)
}

// TestLoadParsesTakeoverWindowAndSampleGap: a repo declaring its own
// clock overrides the defaults (F12) — the knobs travel with the
// repository rather than living on one observer's machine.
func TestLoadParsesTakeoverWindowAndSampleGap(t *testing.T) {
	dir := write(t, "takeover-window: 20m\nsample-gap: 5m\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, 20*time.Minute, got.TakeoverWindow)
	assert.Equal(t, 5*time.Minute, got.SampleGap)
}

// TestLoadKeepsStalenessDefaultsForOmittedKeys: overriding an
// unrelated key must not silently reset the staleness clock.
func TestLoadKeepsStalenessDefaultsForOmittedKeys(t *testing.T) {
	dir := write(t, "plan-dir: docs/plans\n")

	got, err := Load(dir)

	require.NoError(t, err)
	assert.Equal(t, discovery.DefaultTakeoverWindow, got.TakeoverWindow)
	assert.Equal(t, discovery.DefaultSampleGap, got.SampleGap)
}

// TestLoadRejectsAnUnparsableTakeoverWindow: a wrong value is a loud
// parse error, never a silent fallback to the default — someone tried
// to configure the clock and got it wrong.
func TestLoadRejectsAnUnparsableTakeoverWindow(t *testing.T) {
	dir := write(t, "takeover-window: soon\n")

	_, err := Load(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "takeover-window")
}

// TestLoadRejectsAnUnparsableSampleGap mirrors the window case for the
// other clock knob.
func TestLoadRejectsAnUnparsableSampleGap(t *testing.T) {
	dir := write(t, "sample-gap: eventually\n")

	_, err := Load(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sample-gap")
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
