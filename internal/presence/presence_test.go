package presence

import (
	"errors"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// at is a fixed clock, so an age is an exact duration rather than a
// race against the wall.
func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}

	return t
}

func pane(id string) herdr.Pane {
	return herdr.Pane{Agent: "claude", Status: "working", PaneID: id}
}

// TestReconcileServesAFreshReadLive: a host that answered is live, its
// panes are the ones just read, and the cache records the read.
func TestReconcileServesAFreshReadLive(t *testing.T) {
	now := at("2026-08-20T12:00:00Z")
	results := []herdr.HostResult{
		{Host: "box", Panes: []herdr.Pane{pane("box:p1")}},
	}

	statuses, next := Reconcile(results, Cache{}, now)

	require.Len(t, statuses, 1)
	assert.True(t, statuses[0].Fresh)
	assert.True(t, statuses[0].Seen)
	assert.Equal(t, time.Duration(0), statuses[0].Age)
	require.Len(t, statuses[0].Panes, 1)
	assert.Equal(t, "box:p1", statuses[0].Panes[0].PaneID)
	assert.Equal(t, now, next["box"].At)
}

// TestReconcileRendersADeadHostStale is the phase gate: an unreachable
// host never fails the read, and instead renders its last-known panes
// with the age since they were read.
func TestReconcileRendersADeadHostStale(t *testing.T) {
	seen := at("2026-08-20T11:55:00Z")
	now := at("2026-08-20T12:00:00Z")
	prior := Cache{"box": {Panes: []herdr.Pane{pane("box:p1")}, At: seen}}
	results := []herdr.HostResult{
		{Host: "box", Err: errors.New("ssh: connect: no route to host")},
	}

	statuses, next := Reconcile(results, prior, now)

	require.Len(t, statuses, 1)
	assert.False(t, statuses[0].Fresh)
	assert.True(t, statuses[0].Seen)
	assert.Equal(t, 5*time.Minute, statuses[0].Age)
	require.Len(t, statuses[0].Panes, 1, "last-known panes still render")
	assert.Equal(t, "box:p1", statuses[0].Panes[0].PaneID)
	// A failed read must not overwrite the last good snapshot.
	assert.Equal(t, seen, next["box"].At)
}

// TestReconcileNeverBlocksTheBoard: a dead host beside a live one leaves
// the live host's answer standing — one unreachable machine does not
// fail the whole read.
func TestReconcileNeverBlocksTheBoard(t *testing.T) {
	seen := at("2026-08-20T11:59:00Z")
	now := at("2026-08-20T12:00:00Z")
	prior := Cache{"dead": {Panes: []herdr.Pane{pane("dead:p1")}, At: seen}}
	results := []herdr.HostResult{
		{Host: "dead", Err: errors.New("timed out")},
		{Host: "live", Panes: []herdr.Pane{pane("live:p1")}},
	}

	statuses, _ := Reconcile(results, prior, now)
	require.Len(t, statuses, 2)

	by := map[herdr.Host]Status{}
	for _, s := range statuses {
		by[s.Host] = s
	}
	assert.False(t, by["dead"].Fresh)
	assert.True(t, by["dead"].Seen)
	assert.True(t, by["live"].Fresh)
	assert.Equal(t, "live:p1", by["live"].Panes[0].PaneID)
}

// TestReconcileNeverSeenHostIsEmptyNotFabricated: a dead host with no
// snapshot to fall back on renders unseen rather than inventing panes.
func TestReconcileNeverSeenHostIsEmptyNotFabricated(t *testing.T) {
	now := at("2026-08-20T12:00:00Z")
	results := []herdr.HostResult{
		{Host: "cold", Err: errors.New("no route")},
	}

	statuses, _ := Reconcile(results, Cache{}, now)
	require.Len(t, statuses, 1)
	assert.False(t, statuses[0].Fresh)
	assert.False(t, statuses[0].Seen)
	assert.Empty(t, statuses[0].Panes)
}
