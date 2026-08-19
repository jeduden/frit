package main

import (
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHostsWithLocalLeadsWithTheLocalSocket: the empty Host comes first,
// so presence is read from the fleet on top of the local socket, not
// instead of it.
func TestHostsWithLocalLeadsWithTheLocalSocket(t *testing.T) {
	got := hostsWithLocal([]string{"box", "borg"})
	assert.Equal(t, []herdr.Host{"", "box", "borg"}, got)
}

// TestHostsWithLocalAloneIsJustLocal: with no remotes configured the
// list is the local socket by itself.
func TestHostsWithLocalAloneIsJustLocal(t *testing.T) {
	assert.Equal(t, []herdr.Host{""}, hostsWithLocal(nil))
}

// TestPresencePanesReadsTheLocalSocketWhenNoHosts: with no hosts
// configured, presencePanes is exactly the old single-socket read — the
// faked local runner's panes, and its error surfaced unchanged.
func TestPresencePanesReadsTheLocalSocketWhenNoHosts(t *testing.T) {
	rt := &runtime{
		git: gitwt.Exec,
		herdr: herdrReturning(map[string]any{
			"agent": "claude", "agent_status": "idle",
			"cwd": "/w", "pane_id": "w1:p1",
		}),
	}

	panes, err := presencePanes(&cli{}, rt)
	require.NoError(t, err)
	require.Len(t, panes, 1)
	assert.Equal(t, "w1:p1", panes[0].PaneID)
}
