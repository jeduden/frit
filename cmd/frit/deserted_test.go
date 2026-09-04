package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
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

	got := desertedHeld(rt, []discovery.Plan{plan}, "atlas", worktrees, coord)

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
		rt, []discovery.Plan{plan}, "atlas", nil, fleet.Coord{})

	assert.Empty(t, got)
}

// TestDesertedHeldListsATokenMatchWithNoLiveSession: a matching token
// alone is not proof of self-resume — it is exactly what a dead
// session's own checkout still carries. With herdr confirming no live
// agent sits in that checkout, the hold is a dead end and orphans must
// surface it rather than trust the stale token (2608221754 phase 1).
func TestDesertedHeldListsATokenMatchWithNoLiveSession(t *testing.T) {
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

	rt := &runtime{git: gitwt.Exec, herdr: herdrReturning()}
	coord := fleet.Coord{Path: repo, Remote: "origin"}
	plan := discovery.Plan{
		Repo: "atlas", ID: 7, Held: true, Dead: true,
		Holds: []string{"plan/7"}, HoldTip: renewed.Tip,
	}
	worktrees := []gitwt.Worktree{{Path: lane, Branch: "plan/7"}}

	got := desertedHeld(rt, []discovery.Plan{plan}, "atlas", worktrees, coord)

	require.Len(t, got, 1,
		"no live session sits in the checkout, so the token cannot resume it")
	assert.Equal(t, int64(7), got[0].ID)
}

// TestDesertedHeldExcludesAResumableToken: a persisted token that
// still matches origin's tip, in a checkout herdr confirms is staffed
// right now, means self-resume can recover the lane with no operator
// action, so it is not a dead end.
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

	rt := &runtime{git: gitwt.Exec, herdr: herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"cwd":          lane,
		"pane_id":      "wNew:p1",
	})}
	coord := fleet.Coord{Path: repo, Remote: "origin"}
	plan := discovery.Plan{
		Repo: "atlas", ID: 7, Held: true, Dead: true,
		Holds: []string{"plan/7"}, HoldTip: renewed.Tip,
	}
	worktrees := []gitwt.Worktree{{Path: lane, Branch: "plan/7"}}

	got := desertedHeld(rt, []discovery.Plan{plan}, "atlas", worktrees, coord)

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
		rt, []discovery.Plan{plan}, "atlas", nil, fleet.Coord{})

	assert.Empty(t, got, "a matured hold is staleHeld's cell, not desertedHeld's")
	assert.Len(t, staleHeld([]discovery.Plan{plan}, "atlas"), 1)
}

// TestDesertedRefusalExcludesAnUnheldPlan: Dead is only ever herdr's
// answer about a bound session, which observeHolds only reads for a
// held plan — but desertedRefusal reads Dead and Stale alone, with no
// Held check of its own. Even from this exact lane, an unheld plan
// must not manufacture a "deserted hold" refusal, so the check does
// not rest solely on that upstream invariant holding forever.
func TestDesertedRefusalExcludesAnUnheldPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane, Session: "wOld:p1"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	t.Chdir(lane)
	rt := &runtime{git: gitwt.Exec}
	plan := discovery.Plan{Repo: "atlas", ID: 7, Held: false, Dead: true}

	got, _, _ := desertedRefusal(nil, rt, plan, lane)

	assert.Empty(t, got, "nobody holds this plan, so there is nothing to yield")
}

// TestStartNamesYieldForADesertedLaneOnThisHost: run from the lane's
// own worktree, herdr confirms the bound session gone, and the ref
// has moved past this lane's own persisted token — self-resume cannot
// recover it, so start refuses and names yield rather than blindly
// taking its own dead lane over and orphaning whatever it committed
// locally (S77).
func TestStartNamesYieldForADesertedLaneOnThisHost(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	// Somebody else moves the ref past the token this lane persisted.
	ghost := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "ghost", Lane: "/lanes/ghost",
		Session: "wGhost:p1"}
	_, err = claim.Takeover(repo, ghost, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "refused")
	assert.Contains(t, got, "yield 7")
	assert.NotContains(t, got, "started plan 7")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "holder:  ghost",
		"the dead lane's own ref was never seized")
}

// TestStartNamesThePaneForADesertedLaneAWorkingPaneAttends: the same
// deserted-lane fixture as TestStartNamesYieldForADesertedLaneOnThisHost
// — bound session confirmed gone, window unmatured, this exact lane's
// token behind origin's tip — but with herdr showing a pane actively
// working the branch. The refusal names that pane and leads with
// resuming it; `frit yield` stays only the trailing fallback, since
// nobody deserted a lane someone is sitting in.
func TestStartNamesThePaneForADesertedLaneAWorkingPaneAttends(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	// Somebody else moves the ref past the token this lane persisted.
	ghost := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "ghost", Lane: "/lanes/ghost",
		Session: "wGhost:p1"}
	_, err = claim.Takeover(repo, ghost, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id": "wLive:p1", "cwd": lane,
	}))
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "refused")
	assert.Contains(t, got, "wLive:p1", "the refusal names the live pane")
	assert.Contains(t, got, "frit open 7", "leads with resuming it")
	assert.NotContains(t, got, "started plan 7")
	require.True(t,
		strings.Index(got, "frit open") < strings.Index(got, "frit yield"),
		"resume leads; yield is only the trailing fallback: %q", got)
}

// TestStartNamesThePaneForADesertedLaneAnIdlePaneAttends: the same
// fixture, but the pane is idling between phases rather than working.
// A cheap guard that the reconciliation reads any live pane, not just
// a working one — nothing here should key off agent_status.
func TestStartNamesThePaneForADesertedLaneAnIdlePaneAttends(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	ghost := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "ghost", Lane: "/lanes/ghost",
		Session: "wGhost:p1"}
	_, err = claim.Takeover(repo, ghost, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "idle",
		"pane_id": "wLive:p1", "cwd": lane,
	}))
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "wLive:p1",
		"an idling pane is still named, not read as gone")
	assert.Contains(t, got, "frit open 7")
}

// TestStartNamesThePaneForAnUnparkedSuffixALivePaneAttends: the
// park-first guard's own fixture (from outside the lane, a divergent
// local commit past the pushed tip), but with a herdr pane sitting
// live on the branch elsewhere. The refusal names that pane and leads
// with resume rather than the park-first yield.
func TestStartNamesThePaneForAnUnparkedSuffixALivePaneAttends(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wOld:p1"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "plan/7")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "local work")
	localTip, err := gitCapture(t, repo, "rev-parse", "plan/7")
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "main")
	lane := liveLeaseLane(t, repo, "plan/7")
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id": "wLive:p1", "cwd": lane,
	}))
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "refused")
	assert.Contains(t, got, "wLive:p1", "the refusal names the live pane")
	assert.Contains(t, got, "frit open 7", "leads with resuming it")
	assert.NotContains(t, got, "started plan 7")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, localTip, tip, "nothing was taken over; the local commit stands")
}

// TestStartGoRefusalOfAnAttendedDesertedLaneCarriesInJSON: the
// resume-first wording rides in the JSON contract the same way every
// other pre-flight refusal does — `refused` names the pane, and
// `prompt_dispatched` stays false.
func TestStartGoRefusalOfAnAttendedDesertedLaneCarriesInJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	ghost := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "ghost", Lane: "/lanes/ghost",
		Session: "wGhost:p1"}
	_, err = claim.Takeover(repo, ghost, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id": "wLive:p1", "cwd": lane,
	}))
	var doc report.StartDoc

	emit(t, &doc, "start", "7", "--phase", "3", "--go", "--root", root)

	assert.Contains(t, doc.Refused, "wLive:p1")
	assert.Contains(t, doc.Refused, "frit open 7")
	assert.Contains(t, doc.Refused, "frit message 7",
		"the attended lane is offered the ask-the-agent remedy")
	assert.False(t, doc.PromptDispatched)
}

// TestClaimRefusesAnUnparkedSuffixFromOutsideTheLane: a held plan
// herdr confirms dead, window not matured, with a local commit on the
// plan branch past the pushed hold tip — the dead lane's own unparked
// work, never renewed. claim run from the primary, never having
// entered the lane, must not take this over: doing so would seize the
// ref at the pushed tip and silently orphan that commit. It refuses
// and names yield instead (2608221754 phase 2, S77).
func TestClaimRefusesAnUnparkedSuffixFromOutsideTheLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wOld:p1"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "plan/7")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "local work")
	localTip, err := gitCapture(t, repo, "rev-parse", "plan/7")
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "main")
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "refused")
	assert.Contains(t, got, "yield 7")
	assert.NotContains(t, got, "claimed plan 7")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, localTip, tip, "nothing was taken over; the local commit stands")
}

// TestStartRefusesAnUnparkedSuffixFromOutsideTheLane: the same fixture
// through start, which shares the guard with claim.
func TestStartRefusesAnUnparkedSuffixFromOutsideTheLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wOld:p1"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "plan/7")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "local work")
	localTip, err := gitCapture(t, repo, "rev-parse", "plan/7")
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "main")
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "refused")
	assert.Contains(t, got, "yield 7")
	assert.NotContains(t, got, "started plan 7")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, localTip, tip, "nothing was taken over; the local commit stands")
}
