package main

import (
	"testing"
	"time"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/presence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToHostsIsTheRemoteRosterWithoutLocal: the local host is read
// directly, so the fan-out roster is the configured remotes only.
func TestToHostsIsTheRemoteRosterWithoutLocal(t *testing.T) {
	assert.Equal(t, []herdr.Host{"box", "borg"},
		toHosts([]string{"box", "borg"}))
	assert.Empty(t, toHosts(nil))
}

// TestFleetPresenceReadsTheLocalSocketWhenNoHosts: with no hosts
// configured, fleetPresence is exactly the old single-socket read — the
// faked local runner's panes, no problems, no error.
func TestFleetPresenceReadsTheLocalSocketWhenNoHosts(t *testing.T) {
	rt := &runtime{
		git: gitwt.Exec,
		herdr: herdrReturning(map[string]any{
			"agent": "claude", "agent_status": "idle",
			"cwd": "/w", "pane_id": "w1:p1",
		}),
	}

	panes, probs, err := fleetPresence(&cli{}, rt)
	require.NoError(t, err)
	assert.Empty(t, probs)
	require.Len(t, panes, 1)
	assert.Equal(t, "w1:p1", panes[0].PaneID)
}

// TestFleetPresenceSurfacesTheLocalSocketError: a local socket frit
// cannot read is returned as an error, so a caller still tells
// "presence unknown" from "nobody live".
func TestFleetPresenceSurfacesTheLocalSocketError(t *testing.T) {
	rt := &runtime{
		git:   gitwt.Exec,
		herdr: func(...string) ([]byte, error) { return nil, assert.AnError },
	}

	_, _, err := fleetPresence(&cli{}, rt)
	assert.Error(t, err)
}

// TestFleetPresenceFallsBackToLocalWithoutACachePath: if no cache
// location can be resolved, the fan-out has nowhere to fall back, so
// fleetPresence reads the local socket and reaches no remote rather than
// returning nothing.
func TestFleetPresenceFallsBackToLocalWithoutACachePath(t *testing.T) {
	// os.UserCacheDir fails when neither is set, forcing CachePath's error.
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	rt := &runtime{
		git: gitwt.Exec,
		herdr: herdrReturning(map[string]any{
			"agent": "claude", "agent_status": "idle",
			"cwd": "/w", "pane_id": "local:p1",
		}),
	}

	panes, probs, err := fleetPresence(&cli{Hosts: []string{"box"}}, rt)
	require.NoError(t, err)
	assert.Empty(t, probs)
	require.Len(t, panes, 1)
	assert.Equal(t, "local:p1", panes[0].PaneID)
}

// TestHostProblemsFlagsStaleAndUnreachable: a fresh host is silent, an
// unreachable one with nothing cached is flagged, and one served from
// cache is flagged with how stale it is.
func TestHostProblemsFlagsStaleAndUnreachable(t *testing.T) {
	probs := hostProblems([]presence.Status{
		{Host: "fresh", Fresh: true, Seen: true},
		{Host: "cold", Fresh: false, Seen: false},
		{Host: "stale", Fresh: false, Seen: true, Age: 5 * time.Minute},
	})

	require.Len(t, probs, 2)
	assert.Equal(t, "host cold", probs[0].name)
	assert.Contains(t, probs[0].err.Error(), "no cached presence")
	assert.Equal(t, "host stale", probs[1].name)
	assert.Contains(t, probs[1].err.Error(), "5m")
}
