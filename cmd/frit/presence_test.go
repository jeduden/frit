package main

import (
	"errors"
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

// TestFleetPresenceMarksHostsUnreadWithoutACachePath: if no cache
// location can be resolved, the remotes cannot be read or reconciled, so
// fleetPresence reads the local socket alone — but the configured hosts
// went unread, and that travels back as a noPresence problem rather than
// reading as a clean local-only fleet a consumer could start a duplicate
// lane against.
func TestFleetPresenceMarksHostsUnreadWithoutACachePath(t *testing.T) {
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
	require.Len(t, panes, 1)
	assert.Equal(t, "local:p1", panes[0].PaneID, "the local socket still stands")
	require.Len(t, probs, 1, "the configured host went unread")
	assert.Equal(t, "host box", probs[0].name)
	assert.True(t, probs[0].noPresence,
		"no cache path means the remote was never read")
}

// TestUnreadHostsFlagsEveryConfiguredHost: with no cache path the
// remotes cannot be read, so every configured host comes back as a
// noPresence problem — an empty roster yields an empty, non-nil list.
func TestUnreadHostsFlagsEveryConfiguredHost(t *testing.T) {
	assert.Empty(t, unreadHosts(nil))

	probs := unreadHosts([]string{"box", "borg"})
	require.Len(t, probs, 2)
	assert.Equal(t, "host box", probs[0].name)
	assert.Equal(t, "host borg", probs[1].name)
	for _, p := range probs {
		assert.True(t, p.noPresence, "no cache path means nothing was read")
		assert.Contains(t, p.err.Error(), "no cache path")
	}
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
	assert.True(t, probs[0].noPresence,
		"a host with nothing cached read no presence at all")
	assert.Contains(t, probs[0].err.Error(), "no cached presence")
	assert.Equal(t, "host stale", probs[1].name)
	assert.False(t, probs[1].noPresence,
		"a stale host still served cached presence, just old")
	assert.Contains(t, probs[1].err.Error(), "5m")
}

// TestFleetPresenceStopsAtAMissingHerdr: a reader that stops on any
// local failure — board, who, the surveys — never pays for the remote
// fan-out a missing herdr would only have it discard.
func TestFleetPresenceStopsAtAMissingHerdr(t *testing.T) {
	isolate(t)
	withRemoteHerdr(t, map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id": "wBox:p1", "cwd": "/w",
	})
	rt := &runtime{git: gitwt.Exec, herdr: herdrNotInstalled}

	panes, probs, err := fleetPresence(&cli{Hosts: []string{"box"}}, rt)

	assert.Error(t, herdr.NotFound(err))
	assert.Empty(t, panes, "the hosts went unread")
	assert.Empty(t, probs)
}

// TestFleetPresencePastMissingReturnsAMissingHerdrWithNoHosts: with
// herdr not installed and no hosts to read past it, there are no panes
// and the not-found error comes back for the caller to tell apart.
func TestFleetPresencePastMissingReturnsAMissingHerdrWithNoHosts(t *testing.T) {
	rt := &runtime{git: gitwt.Exec, herdr: herdrNotInstalled}

	panes, probs, err := fleetPresencePastMissing(&cli{}, rt)

	assert.Error(t, herdr.NotFound(err))
	assert.Empty(t, panes)
	assert.Empty(t, probs)
}

// TestFleetPresencePastMissingReadsHostsPastAMissingHerdr: a herdr not
// installed here still lets the configured hosts be read, their panes
// returned beside the not-found error.
func TestFleetPresencePastMissingReadsHostsPastAMissingHerdr(t *testing.T) {
	isolate(t)
	withRemoteHerdr(t, map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id": "wBox:p1", "cwd": "/w",
	})
	rt := &runtime{git: gitwt.Exec, herdr: herdrNotInstalled}

	panes, _, err := fleetPresencePastMissing(&cli{Hosts: []string{"box"}}, rt)

	assert.Error(t, herdr.NotFound(err))
	require.Len(t, panes, 1)
	assert.Equal(t, herdr.Host("box"), panes[0].Host)
}

// TestFleetPresencePastMissingMarksHostsUnreadPastAMissingHerdr: with
// no cache path the hosts go unread, flagged as such, and the
// not-found error still comes back.
func TestFleetPresencePastMissingMarksHostsUnreadPastAMissingHerdr(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	rt := &runtime{git: gitwt.Exec, herdr: herdrNotInstalled}

	_, probs, err := fleetPresencePastMissing(&cli{Hosts: []string{"box"}}, rt)

	assert.Error(t, herdr.NotFound(err))
	require.Len(t, probs, 1)
	assert.True(t, probs[0].noPresence)
}

// TestFleetPresencePastMissingStopsAtAnUnreachableHerdr: only a herdr
// not installed is read past; a socket that failed any other way is
// returned before any host is read.
func TestFleetPresencePastMissingStopsAtAnUnreachableHerdr(t *testing.T) {
	rt := &runtime{git: gitwt.Exec, herdr: func(...string) ([]byte, error) {
		return nil, errors.New("dial refused")
	}}

	panes, probs, err := fleetPresencePastMissing(&cli{Hosts: []string{"box"}}, rt)

	require.Error(t, err)
	assert.NoError(t, herdr.NotFound(err))
	assert.Empty(t, panes)
	assert.Empty(t, probs)
}

// TestWithHostPresenceIsTheLocalPanesWhenNoHosts: with no hosts
// configured the local panes come back untouched and nothing is read.
func TestWithHostPresenceIsTheLocalPanesWhenNoHosts(t *testing.T) {
	local := []herdr.Pane{{PaneID: "w1:p1"}}

	panes, probs := withHostPresence(&cli{}, local)

	assert.Equal(t, local, panes)
	assert.Empty(t, probs)
}
