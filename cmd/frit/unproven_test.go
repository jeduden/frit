package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnprovenHeldListsAClaimOnlyLaneWithNoToken: a held plan whose
// only local worktree never carried a token — the S49 shape a
// claim-only lane leaves when its stand-up write never landed, or a
// legacy lane stood up before phase 1 of plan 2609050854 — is surfaced
// here, with no herdr call at all: tokenlessOwnLane is pure git.
func TestUnprovenHeldListsAClaimOnlyLaneWithNoToken(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")

	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: true, Holds: []string{"plan/7"}}
	worktrees := []gitwt.Worktree{{Path: lane, Branch: "plan/7"}}

	got := unprovenHeld(rt, []discovery.Plan{plan}, "atlas", worktrees)

	require.Len(t, got, 1)
	assert.Equal(t, int64(7), got[0].ID)
}

// TestUnprovenHeldExcludesALaneWhoseTokenProves: a lane that renewed
// its own token still proves the lease, so it is not this shape.
func TestUnprovenHeldExcludesALaneWhoseTokenProves(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	_, err = claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: true, Holds: []string{"plan/7"}}
	worktrees := []gitwt.Worktree{{Path: lane, Branch: "plan/7"}}

	got := unprovenHeld(rt, []discovery.Plan{plan}, "atlas", worktrees)

	assert.Empty(t, got, "a token that proves the lease is not this shape")
}

// TestUnprovenHeldExcludesAnUnheldPlan: nothing here to prove or not
// for a plan nobody holds.
func TestUnprovenHeldExcludesAnUnheldPlan(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: false}

	got := unprovenHeld(rt, []discovery.Plan{plan}, "atlas", nil)

	assert.Empty(t, got)
}

// TestUnprovenHeldExcludesAPlanWithNoLocalCheckout: no worktree on
// this host stands on the plan's branch at all, so there is nothing
// local to prove tokenless.
func TestUnprovenHeldExcludesAPlanWithNoLocalCheckout(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: true, Holds: []string{"plan/7"}}

	got := unprovenHeld(rt, []discovery.Plan{plan}, "atlas", nil)

	assert.Empty(t, got)
}

// TestOrphansNamesTheWayOutForATokenlessOwnLane: the S49 fixture, run
// through `frit orphans`, carries the plan in `unproven` with a
// non-empty `next_action`, and the table names the way out too.
func TestOrphansNamesTheWayOutForATokenlessOwnLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no token")
	assert.Contains(t, out.String(), "takeover window",
		"the table names the way out too")

	var doc report.OrphansDoc
	emit(t, &doc, "orphans", "--root", root)

	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Unproven, 1)
	assert.Equal(t, int64(7), doc.Repos[0].Unproven[0].PlanID)
	assert.NotEmpty(t, doc.Repos[0].Unproven[0].NextAction)
}

// TestBoardUnprovenReportsAClaimOnlyLaneWithNoToken: boardUnproven
// reuses tokenlessOwnLane against a repository's worktrees, cached so
// gitwt.List runs once per repository rather than once per held plan.
func TestBoardUnprovenReportsAClaimOnlyLaneWithNoToken(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")

	rt := &runtime{git: gitwt.Exec}
	res := fleet.Result{Coords: map[string]fleet.Coord{
		"atlas": {Path: repo, Remote: "origin"},
	}}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: true, Holds: []string{"plan/7"}}
	cache := map[string][]gitwt.Worktree{}

	assert.True(t, boardUnproven(rt, res, plan, cache))
	assert.False(t,
		boardUnproven(rt, res, discovery.Plan{Repo: "atlas", ID: 8, Held: true}, cache),
		"a different plan id on the same lane's branch proves nothing")
}

// TestBoardNamesTheWayOutForATokenlessOwnLane: the S49 fixture, run
// through `frit board`, carries next_action non-empty in both the
// table and --json.
func TestBoardNamesTheWayOutForATokenlessOwnLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	var out, errb bytes.Buffer

	code := run([]string{"board", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "takeover window")

	var doc report.BoardDoc
	emit(t, &doc, "board", "--root", root)

	require.Len(t, doc.Plans, 1)
	assert.NotEmpty(t, doc.Plans[0].NextAction)
}

// tokenlessLaneFixture builds the S49 shape phase 2 already tests
// against: this host's own lease, its worktree stood up by hand
// rather than by `frit claim`, so no token was ever persisted. It
// returns the lane path, to chdir into.
func tokenlessLaneFixture(t *testing.T, root string) string {
	t.Helper()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")

	return lane
}

// TestNextNamesTheWayOutFromATokenlessLane: run from inside the S49
// fixture's lane, next's next_action names the same wait-or-take-over
// wording open already gives the identical hold.
func TestNextNamesTheWayOutFromATokenlessLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	lane := tokenlessLaneFixture(t, root)
	t.Chdir(lane)
	var doc report.NextDoc

	emit(t, &doc, "next", "--root", root)

	assert.NotEmpty(t, doc.NextAction)
}

// TestShowNamesTheWayOutFromATokenlessLane is next's own test, for show.
func TestShowNamesTheWayOutFromATokenlessLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	lane := tokenlessLaneFixture(t, root)
	t.Chdir(lane)
	var doc report.ShowDoc

	emit(t, &doc, "show", "--root", root)

	assert.NotEmpty(t, doc.NextAction)
}

// TestPhaseNamesTheWayOutFromATokenlessLane is next's own test, for
// phase, which always runs from inside the lane.
func TestPhaseNamesTheWayOutFromATokenlessLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	lane := tokenlessLaneFixture(t, root)
	t.Chdir(lane)
	var doc report.PhaseDoc

	emit(t, &doc, "phase", "--root", root)

	assert.NotEmpty(t, doc.NextAction)
}

// TestNextLeavesTheWayOutEmptyForAHealthyLane: a lane whose own
// renewal persisted its token proves the lease, so next names no way
// out — the field is not merely "held", it is "unprovable".
func TestNextLeavesTheWayOutEmptyForAHealthyLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	_, err = claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	var doc report.NextDoc

	emit(t, &doc, "next", "--root", root)

	assert.Empty(t, doc.NextAction)
}
