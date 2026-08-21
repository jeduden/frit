package observe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/presence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKey pins the state key: repo and plan id, the pair a plan is
// keyed by on one host.
func TestKey(t *testing.T) {
	assert.Equal(t, "atlas:7", Key("atlas", 7))
	assert.Equal(t, "orrery:2608202144", Key("orrery", 2608202144))
}

// TestLoadMissingIsAColdStart: no file, or an unreadable one, is an
// empty state — losing the observer's memory only delays a takeover.
func TestLoadMissingIsAColdStart(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "absent.json"))
	assert.NotNil(t, got)
	assert.Empty(t, got)

	garbled := filepath.Join(t.TempDir(), "garbled.json")
	require.NoError(t, os.WriteFile(garbled, []byte("{not json"), 0o600))
	got = Load(garbled)
	assert.Empty(t, got)
}

// TestSaveThenLoadRoundTrips: a window survives the file intact, so
// the next run continues the same observation.
func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "observations.json")
	first := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	state := State{
		Key("atlas", 7): discovery.Window{
			Tip:     "abc123",
			First:   first,
			Last:    first.Add(90 * time.Minute),
			Samples: 5,
			Voided:  "",
		},
	}

	require.NoError(t, Save(path, state), "Save creates the directory")

	got := Load(path)
	assert.Equal(t, state, got)
}

// TestPathIsBesideThePresenceCache: the observation state lives where
// the presence cache lives — one frit directory of per-host state.
func TestPathIsBesideThePresenceCache(t *testing.T) {
	obsPath, err := Path()
	require.NoError(t, err)
	presPath, err := presence.CachePath()
	require.NoError(t, err)

	assert.Equal(t, filepath.Dir(presPath), filepath.Dir(obsPath))
	assert.Equal(t, "observations.json", filepath.Base(obsPath))
}
