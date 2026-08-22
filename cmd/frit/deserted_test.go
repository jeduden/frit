package main

import (
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDesertedHeldListsATokenBehindTheTip: a held plan herdr confirms
// gone, whose only local worktree's persisted token is behind origin's
// current tip, is a dead end — self-resume cannot recover it — and
// orphans surfaces it before any takeover window matures (2608212346).
func TestDesertedHeldListsATokenBehindTheTip(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane, Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	// Somebody else moves the ref past the token this lane persisted.
	ghost := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "ghost", Lane: "/lanes/ghost"}
	taken, err := claim.Takeover(repo, ghost, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)

	rt := &runtime{git: gitwt.Exec}
	coord := fleet.Coord{Path: repo, Remote: "origin"}
	plan := discovery.Plan{
		Repo: "atlas", ID: 7, Held: true, Dead: true,
		Holds: []string{"plan/7"}, HoldTip: taken.Tip,
	}
	worktrees := []gitwt.Worktree{{Path: lane, Branch: "plan/7"}}

	got := desertedHeld(rt, []discovery.Plan{plan}, "atlas", worktrees, coord, true)

	require.Len(t, got, 1)
	assert.Equal(t, int64(7), got[0].ID)
}

// TestDesertedHeldExcludesALiveBoundSession: the herdr veto holds — a
// plan whose session is not confirmed dead is never a deserted hold,
// no matter its token.
func TestDesertedHeldExcludesALiveBoundSession(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: true, Dead: false}

	got := desertedHeld(
		rt, []discovery.Plan{plan}, "atlas", nil, fleet.Coord{}, true)

	assert.Empty(t, got)
}

// TestDesertedHeldExcludesAResumableToken: a persisted token that
// still matches origin's tip means self-resume can recover the lane
// with no operator action, so it is not a dead end.
func TestDesertedHeldExcludesAResumableToken(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane, Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	rt := &runtime{git: gitwt.Exec}
	coord := fleet.Coord{Path: repo, Remote: "origin"}
	plan := discovery.Plan{
		Repo: "atlas", ID: 7, Held: true, Dead: true,
		Holds: []string{"plan/7"}, HoldTip: renewed.Tip,
	}
	worktrees := []gitwt.Worktree{{Path: lane, Branch: "plan/7"}}

	got := desertedHeld(rt, []discovery.Plan{plan}, "atlas", worktrees, coord, true)

	assert.Empty(t, got, "self-resume can recover it, so it is not a dead end")
}

// TestDesertedHeldDoesNotCollideWithAMaturedStaleHold: a matured
// takeover window stays a stale-hold, never a deserted hold — the two
// kinds are told apart, not merged.
func TestDesertedHeldDoesNotCollideWithAMaturedStaleHold(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{
		Repo: "atlas", ID: 7, Held: true, Dead: true, Stale: true,
	}

	got := desertedHeld(
		rt, []discovery.Plan{plan}, "atlas", nil, fleet.Coord{}, true)

	assert.Empty(t, got, "a matured hold is staleHeld's cell, not desertedHeld's")
	assert.Len(t, staleHeld([]discovery.Plan{plan}, "atlas"), 1)
}
