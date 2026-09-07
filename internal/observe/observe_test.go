package observe

import (
	"errors"
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

// TestSaveFailsWhenDirCannotBeCreated: a file where the parent
// directory belongs makes MkdirAll fail, the same trick
// internal/presence's TestReadSurvivesAnUnwritableCache uses.
func TestSaveFailsWhenDirCannotBeCreated(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	path := filepath.Join(file, "sub", "observations.json")

	err := Save(path, State{})

	assert.Error(t, err)
}

// TestSaveFailsWhenStateCannotMarshal: a Window whose First lands
// outside time.Time.MarshalJSON's year range is the only shape this
// package's own values can take to make json.Marshal fail.
func TestSaveFailsWhenStateCannotMarshal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.json")
	state := State{
		Key("atlas", 7): discovery.Window{
			First: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	err := Save(path, state)

	assert.Error(t, err)
}

// TestSaveFailsWhenTempFileCannotBeCreated: an existing but unwritable
// directory lets MkdirAll no-op while os.CreateTemp still fails.
func TestSaveFailsWhenTempFileCannotBeCreated(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission bit this test relies on")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	path := filepath.Join(dir, "observations.json")

	err := Save(path, State{})

	assert.Error(t, err)
}

// fakeTempFile lets a test force Save's Write or Close to fail
// without a real *os.File misbehaving into it on demand.
type fakeTempFile struct {
	name     string
	writeErr error
	closeErr error
	wrote    bool
	closed   bool
}

func (f *fakeTempFile) Write(p []byte) (int, error) {
	f.wrote = true
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func (f *fakeTempFile) Close() error {
	f.closed = true
	return f.closeErr
}

func (f *fakeTempFile) Name() string { return f.name }

// TestSaveFailsWhenWriteFails: Save returns a swapped createTemp's
// Write failure and never reaches os.Rename.
func TestSaveFailsWhenWriteFails(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{name: filepath.Join(dir, "fake"), writeErr: errors.New("disk full")}
	restore := createTemp
	createTemp = func(string, string) (tempFile, error) { return fake, nil }
	t.Cleanup(func() { createTemp = restore })
	path := filepath.Join(dir, "observations.json")

	err := Save(path, State{})

	assert.ErrorContains(t, err, "disk full")
	assert.NoFileExists(t, path)
}

// TestSaveFailsWhenCloseFails: Save returns a swapped createTemp's
// Close failure and never reaches os.Rename.
func TestSaveFailsWhenCloseFails(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{name: filepath.Join(dir, "fake"), closeErr: errors.New("stale handle")}
	restore := createTemp
	createTemp = func(string, string) (tempFile, error) { return fake, nil }
	t.Cleanup(func() { createTemp = restore })
	path := filepath.Join(dir, "observations.json")

	err := Save(path, State{})

	assert.ErrorContains(t, err, "stale handle")
	assert.NoFileExists(t, path)
}

// TestSaveFailsWhenRenameFails: a destination that is itself an
// existing directory makes the final os.Rename fail after a real temp
// file already wrote and closed cleanly.
func TestSaveFailsWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observations.json")
	require.NoError(t, os.Mkdir(path, 0o755))

	err := Save(path, State{})

	assert.Error(t, err)
}

// TestPathFailsWithNoCacheDir: the one combination os.UserCacheDir
// refuses on Linux — neither $XDG_CACHE_HOME nor $HOME set.
func TestPathFailsWithNoCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	_, err := Path()

	assert.Error(t, err)
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
