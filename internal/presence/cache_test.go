package presence

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFreshWithinTTLSkipsAReprobe: a snapshot younger than the TTL is
// fresh enough to serve from cache, so a live fleet is not re-read on
// every invocation.
func TestFreshWithinTTLSkipsAReprobe(t *testing.T) {
	seen := at("2026-08-20T12:00:00Z")
	now := at("2026-08-20T12:00:20Z")
	c := Cache{"box": {At: seen}}

	assert.True(t, c.Fresh("box", 30*time.Second, now))
}

// TestFreshExpiresPastTTL: once the snapshot is older than the TTL the
// host is re-probed, keyed on the host so one host's age does not stale
// another's.
func TestFreshExpiresPastTTL(t *testing.T) {
	seen := at("2026-08-20T12:00:00Z")
	now := at("2026-08-20T12:01:00Z")
	c := Cache{"box": {At: seen}}

	assert.False(t, c.Fresh("box", 30*time.Second, now))
	assert.False(t, c.Fresh("other", 30*time.Second, now),
		"a host with no snapshot is never fresh")
}

// TestLoadMissingFileIsColdStart: an absent cache file is a cold start,
// an empty cache rather than an error.
func TestLoadMissingFileIsColdStart(t *testing.T) {
	c := Load(filepath.Join(t.TempDir(), "nope.json"))
	assert.Empty(t, c)
}

// TestStoreThenLoadRoundTrips: a stored cache reads back with the same
// panes and timestamps, so last-known presence survives between runs.
func TestStoreThenLoadRoundTrips(t *testing.T) {
	seen := at("2026-08-20T11:55:00Z")
	path := filepath.Join(t.TempDir(), "sub", "presence.json")
	want := Cache{"box": {
		Panes: []herdr.Pane{pane("box:p1")},
		At:    seen,
	}}

	require.NoError(t, Store(path, want))
	got := Load(path)

	require.Len(t, got, 1)
	assert.Equal(t, seen.UTC(), got["box"].At.UTC())
	require.Len(t, got["box"].Panes, 1)
	assert.Equal(t, "box:p1", got["box"].Panes[0].PaneID)
}

// TestCachePathIsUnderTheUserCacheDir keeps the cache where a cache
// belongs, named for frit.
func TestCachePathIsUnderTheUserCacheDir(t *testing.T) {
	p, err := CachePath()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("frit", "presence.json"),
		filepath.Join(filepath.Base(filepath.Dir(p)), filepath.Base(p)))
}
