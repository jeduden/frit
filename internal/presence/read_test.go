package presence

import (
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentsFor is a `herdr agent list` reply naming its one pane after the
// host, so a test can tell whose panes came back.
func agentsFor(host string) []byte {
	return []byte(`{"result":{"agents":[
		{"agent":"claude","agent_status":"working",
		 "cwd":"/w","pane_id":"` + host + `:p1"}]}}`)
}

// probeRecorder is an exec that records which hosts it was asked to
// reach, so a test can prove a fresh host was served without a probe.
type probeRecorder struct {
	mu     sync.Mutex
	probed []string
	fail   map[string]error
}

func (p *probeRecorder) exec(name string, args ...string) ([]byte, error) {
	host := "" // local: command("", args) → name "herdr", no host in argv
	if name == "ssh" {
		host = args[0]
	}

	p.mu.Lock()
	p.probed = append(p.probed, host)
	p.mu.Unlock()

	if err, ok := p.fail[host]; ok {
		return nil, err
	}

	return agentsFor(host), nil
}

func opts() Options {
	return Options{TTL: 30 * time.Second, Timeout: time.Second}
}

// TestReadUnionsEveryHostsPanes: the fan-out returns one flat list of
// panes across the local host and the remotes, so a caller wanting the
// union is unchanged.
func TestReadUnionsEveryHostsPanes(t *testing.T) {
	now := at("2026-08-20T12:00:00Z")
	path := filepath.Join(t.TempDir(), "presence.json")
	rec := &probeRecorder{}

	panes, _ := Read(
		[]herdr.Host{"", "box"}, rec.exec, path, opts(), now)

	ids := paneIDs(panes)
	assert.Equal(t, []string{":p1", "box:p1"}, ids)
}

// TestReadRendersADeadHostFromCache: an unreachable host contributes
// its last-known panes with an age, while a reachable host beside it
// still returns — the board is not blocked.
func TestReadRendersADeadHostFromCache(t *testing.T) {
	seen := at("2026-08-20T11:58:00Z")
	now := at("2026-08-20T12:00:00Z")
	path := filepath.Join(t.TempDir(), "presence.json")
	require.NoError(t, Store(path, Cache{
		"box": {Panes: []herdr.Pane{pane("box:p1")}, At: seen},
	}))
	rec := &probeRecorder{fail: map[string]error{
		"box": errors.New("no route to host"),
	}}

	panes, statuses := Read(
		[]herdr.Host{"", "box"}, rec.exec, path, opts(), now)

	assert.Contains(t, paneIDs(panes), "box:p1")

	by := statusByHost(statuses)
	assert.False(t, by["box"].Fresh)
	assert.True(t, by["box"].Seen)
	assert.Equal(t, 2*time.Minute, by["box"].Age)
}

// TestReadServesAFreshRemoteWithoutAProbe: a remote host whose snapshot
// is younger than the TTL is served from cache, so a live fleet is not
// re-read on every invocation.
func TestReadServesAFreshRemoteWithoutAProbe(t *testing.T) {
	seen := at("2026-08-20T11:59:50Z") // 10s ago, inside the 30s TTL
	now := at("2026-08-20T12:00:00Z")
	path := filepath.Join(t.TempDir(), "presence.json")
	require.NoError(t, Store(path, Cache{
		"box": {Panes: []herdr.Pane{pane("box:cached")}, At: seen},
	}))
	rec := &probeRecorder{}

	panes, _ := Read(
		[]herdr.Host{"", "box"}, rec.exec, path, opts(), now)

	assert.NotContains(t, rec.probed, "box", "fresh host must not be probed")
	assert.Contains(t, rec.probed, "", "local host is always probed")
	assert.Contains(t, paneIDs(panes), "box:cached")
}

// TestReadAlwaysProbesTheLocalHost: the local socket has no round-trip
// to spend, so it is read every time even when its snapshot is fresh.
func TestReadAlwaysProbesTheLocalHost(t *testing.T) {
	seen := at("2026-08-20T11:59:55Z")
	now := at("2026-08-20T12:00:00Z")
	path := filepath.Join(t.TempDir(), "presence.json")
	require.NoError(t, Store(path, Cache{
		"": {Panes: []herdr.Pane{pane("stale-local")}, At: seen},
	}))
	rec := &probeRecorder{}

	Read([]herdr.Host{""}, rec.exec, path, opts(), now)
	assert.Equal(t, []string{""}, rec.probed)
}

// TestReadPersistsFreshSnapshots: a successful read updates the cache on
// disk, so the next invocation has a newer last-known to fall back on.
func TestReadPersistsFreshSnapshots(t *testing.T) {
	now := at("2026-08-20T12:00:00Z")
	path := filepath.Join(t.TempDir(), "presence.json")
	rec := &probeRecorder{}

	Read([]herdr.Host{"box"}, rec.exec, path, opts(), now)

	stored := Load(path)
	assert.Equal(t, now, stored["box"].At)
}

// TestReadSurvivesAnUnwritableCache: a cache-write failure must not
// block the board — Read still returns the panes it gathered.
func TestReadSurvivesAnUnwritableCache(t *testing.T) {
	now := at("2026-08-20T12:00:00Z")
	// A file where the parent directory is expected makes MkdirAll fail.
	file := filepath.Join(t.TempDir(), "afile")
	require.NoError(t, Store(file, Cache{}))
	path := filepath.Join(file, "presence.json")
	rec := &probeRecorder{}

	panes, _ := Read([]herdr.Host{"box"}, rec.exec, path, opts(), now)
	assert.Contains(t, paneIDs(panes), "box:p1")
}

// TestReadPrunesHostsNoLongerConfigured: a host dropped from the roster
// is not probed and must not linger in the cache forever — Read keeps
// only the hosts it was asked about.
func TestReadPrunesHostsNoLongerConfigured(t *testing.T) {
	seen := at("2026-08-20T11:59:55Z")
	now := at("2026-08-20T12:00:00Z")
	path := filepath.Join(t.TempDir(), "presence.json")
	require.NoError(t, Store(path, Cache{
		"gone": {Panes: []herdr.Pane{pane("gone:p1")}, At: seen},
		"box":  {Panes: []herdr.Pane{pane("box:old")}, At: seen},
	}))
	rec := &probeRecorder{}

	Read([]herdr.Host{"box"}, rec.exec, path, opts(), now)

	stored := Load(path)
	_, keptGone := stored["gone"]
	assert.False(t, keptGone, "a dropped host is pruned from the cache")
	_, keptBox := stored["box"]
	assert.True(t, keptBox, "a configured host stays")
}

func paneIDs(panes []herdr.Pane) []string {
	ids := make([]string, 0, len(panes))
	for _, p := range panes {
		ids = append(ids, p.PaneID)
	}
	sort.Strings(ids)

	return ids
}

func statusByHost(statuses []Status) map[herdr.Host]Status {
	by := map[herdr.Host]Status{}
	for _, s := range statuses {
		by[s.Host] = s
	}

	return by
}
