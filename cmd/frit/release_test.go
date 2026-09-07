package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForeignHoldRefusalPointsAtClaimWhenTheSessionIsConfirmedDead:
// Dead is its own reason to send the caller to `frit claim`, distinct
// from Stale — a plan whose window has not matured can still be a
// takeover candidate the moment herdr confirms its bound session gone,
// and the refusal must say so rather than claim the hold is live.
func TestForeignHoldRefusalPointsAtClaimWhenTheSessionIsConfirmedDead(t *testing.T) {
	reason := foreignHoldRefusal(discovery.Plan{Dead: true})

	assert.Contains(t, reason, "frit claim")
	assert.NotContains(t, reason, "held live")
}

// TestForeignHoldRefusalStillNamesALiveHold pins the baseline: neither
// signal present reads as an ordinary live hold, worded as before.
func TestForeignHoldRefusalStillNamesALiveHold(t *testing.T) {
	reason := foreignHoldRefusal(discovery.Plan{Holds: []string{"plan/7-x"}})

	assert.Contains(t, reason, "held live")
}

// TestReleaseIsANoOpOnAnAbsentPlan: nothing has ever held the plan, so
// there is nothing to end — a no-op, not a refusal.
func TestReleaseIsANoOpOnAnAbsentPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "nothing")
}

// TestReleaseIsANoOpOnAnAlreadyReleasedPlan: a work ref whose tip is
// already a release marker is a lease that ended already; releasing it
// again is a no-op, not a fresh transition.
func TestReleaseIsANoOpOnAnAlreadyReleasedPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	_, err = claim.Release(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "already released")
}

// TestReleaseRefusesALiveForeignHold: a plan another lane holds live —
// no matured window — is refused; only that lane's own token can end
// it. The refusal names the same wait-or-take-over way out yield gives
// the identical hold, since both route it through refuseForeignHold.
func TestReleaseRefusesALiveForeignHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "takeover window",
		"a live foreign hold names the same way out yield does")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, tip, "the foreign lease is untouched")
}

// TestReleaseRefusesAMaturedForeignHold: a stale window opens claim's
// takeover door, not release's — release still refuses and says to
// take it over instead of waiting on a release that will not come.
func TestReleaseRefusesAMaturedForeignHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "claim")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, tip, "a matured foreign lease is taken over, not released")
}

// TestReleaseEndsTheLanesOwnLease: run from the lane's own worktree,
// release pushes a release marker CASed from the lane's own persisted
// token — no staleness window consulted.
func TestReleaseEndsTheLanesOwnLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "refused")
	assert.Contains(t, got, "released plan 7")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: release")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, renewed.Tip, parent,
		"the release is CASed from the lane's own persisted token")
}

// TestReleaseEndsALaneClaimAloneStoodUp: a lane `frit claim` alone
// stood up carries the same token a lane `start` created would, so
// release run from inside it ends the lease with no foreign-hold
// refusal in between — the resume shortcut a claim-only lane could not
// reach before this phase.
func TestReleaseEndsALaneClaimAloneStoodUp(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, _ := liveLaneHerdr(t, repo, claim.Branch(7))
	withHerdr(t, runner)
	var claimed struct {
		Worktree string `json:"worktree"`
	}
	emit(t, &claimed, "claim", "7", "--root", root)
	require.NotEmpty(t, claimed.Worktree)
	t.Chdir(claimed.Worktree)
	var doc struct {
		Released bool   `json:"released"`
		Refused  string `json:"refused"`
	}

	emit(t, &doc, "release", "7", "--root", root)

	assert.True(t, doc.Released)
	assert.Empty(t, doc.Refused)
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: release")
}

// TestReleaseNamesTheWayOutForATokenlessOwnLane: run from inside a
// lane this host claimed and stood up, its token then dropped — the
// S49 shape a legacy claim-only lane, or one whose token write never
// landed, leaves behind. release refuses, but never claims the hold is
// "held live by another lane": it is this very lane, just unable to
// prove itself. The refusal's own next_action names the honest way
// out — the same wording open already gives the identical hold.
func TestReleaseNamesTheWayOutForATokenlessOwnLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	t.Chdir(lane)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, out.String(), "held live",
		"this is the lane's own checkout, not a stranger's")
	assert.Contains(t, out.String(), "takeover window",
		"the table names the way out too")

	var doc struct {
		Refused    string `json:"refused"`
		NextAction string `json:"next_action"`
	}
	emit(t, &doc, "release", "7", "--root", root)

	assert.NotContains(t, doc.Refused, "held live",
		"this is the lane's own checkout, not a stranger's")
	assert.NotEmpty(t, doc.NextAction)
	assert.Contains(t, doc.NextAction, "takeover window")
}

// TestReleaseRecognizesALaneWhoseOwnCommitsAdvancedTheTip: the
// prescribed workflow is raw git commit/push on plan/<id>, with no
// frit transition between — origin's tip ends up a descendant of the
// lane's persisted token under the same epoch. release must still
// recognize this as the lane's own advance and succeed, re-anchoring
// to the fresh tip, rather than refuse it as held by another lane.
func TestReleaseRecognizesALaneWhoseOwnCommitsAdvancedTheTip(t *testing.T) {
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

	git(t, lane, "commit", "--allow-empty", "-q", "-m", "red: add failing test")
	git(t, lane, "commit", "--allow-empty", "-q", "-m", "green: make it pass")
	git(t, lane, "push", "-q", "origin", "plan/7")
	rawTip, err := gitCapture(t, lane, "rev-parse", "HEAD")
	require.NoError(t, err)

	t.Chdir(lane)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "refused")
	assert.Contains(t, got, "released plan 7")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: release")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, rawTip, parent,
		"the release re-anchors to origin's fresh tip, not the stale token")
}

// TestReleaseStillRefusesAGenuineTakeoverAfterItsOwnRenewal: a
// takeover marker minted at a new epoch from the observed tip
// descends from the original lane's token too, exactly like an
// ordinary raw commit does — but it is a foreign move, not the lane's
// own advance. release must still refuse it: the Phase 1 relaxation
// widens what counts as "own", it does not open the fence a genuine
// takeover sits behind (S86).
func TestReleaseStillRefusesAGenuineTakeoverAfterItsOwnRenewal(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	foreign := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	_, err = claim.Takeover(repo, foreign, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)

	t.Chdir(lane)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "refused")
	assert.Contains(t, got, "held live")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: takeover",
		"the takeover stands untouched")
}

// TestReleaseScavengesALandedRef: a hold whose work already merged is
// scavenged rather than released — the same evidence claim's own
// scavenge acts on.
func TestReleaseScavengesALandedRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, _ := landedLeaseRepo(t, root)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, out.String(), "refused")
	gone, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Empty(t, gone, "the landed ref is scavenged from origin")
}

// TestReleaseFailsWhenTheFleetCannotBeGathered: a root that does not
// exist fails the fleet walk itself, before release ever resolves a
// plan.
func TestReleaseFailsWhenTheFleetCannotBeGathered(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root",
		filepath.Join(t.TempDir(), "missing")}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// TestReleaseRefusesWithNoPlanGivenAndNoneInferred: an empty selector
// run outside any held checkout cannot infer a plan, and refuses
// rather than guess.
func TestReleaseRefusesWithNoPlanGivenAndNoneInferred(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	t.Chdir(t.TempDir())
	var out, errb bytes.Buffer

	code := run([]string{"release", "--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(),
		"no plan given and none inferred from the current directory")
}

// TestReleaseRefusesAnAmbiguousRepoName: two checkouts under root
// sharing a basename leave the fleet unable to tell which one the
// plan lives in, so release refuses rather than guess — the same
// treatment claim already gives it.
func TestReleaseRefusesAnAmbiguousRepoName(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repoA := initRepo(t, filepath.Join(root, "a"), "frontend")
	commitPlan(t, repoA, 7, "🔲", "Shader unit", nil, "")
	repoB := initRepo(t, filepath.Join(root, "b"), "frontend")
	commitPlan(t, repoB, 9, "🔲", "Other work", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"release", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "shared by another checkout")
}

// TestReleaseHeldWarnsWhenThePushFails: releaseHeld is called
// directly, bypassing Run's unconditional rt.git reassignment, so a
// runner that fails only the CAS push can be injected once the lane's
// own token is already proven. The lease stays standing and the
// failure is reported rather than swallowed.
func TestReleaseHeldWarnsWhenThePushFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	rt := &runtime{git: gitwt.Exec, gitPipe: gitwt.ExecPipe,
		herdr: herdrReturning()}
	res, err := gatherFleet(&cli{Root: root}, rt)
	require.NoError(t, err)
	plan, err := resolveSelector(rt, "7", res.Plans, true)
	require.NoError(t, err)
	coord := res.Coords[plan.Repo]
	doc := report.NewRelease(root, plan.Repo, plan.ID, plan.Title,
		claim.Branch(plan.ID))
	failPush := func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "push" {
			return nil, errors.New("boom")
		}

		return gitwt.Exec(dir, args...)
	}
	rt.git = failPush

	releaseHeld(rt, doc, plan, coord)

	assert.False(t, doc.Released)
	assert.Contains(t, doc.Warning, "release:",
		"the CAS push's failure is reported, not swallowed")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, renewed.Tip, tip, "the lease is untouched by the failed push")
}

// TestPrintReleaseNamesARescuedRef: the rendering branch a scavenge's
// rescue ref takes, direct-called against a hand-built doc.
func TestPrintReleaseNamesARescuedRef(t *testing.T) {
	doc := report.NewRelease("/root", "atlas", 7, "Shader unit", "plan/7")
	doc.ScavengedRef("plan/7", "refs/frit/rescue/7/host-abc")
	var out bytes.Buffer

	printRelease(&out, doc)

	assert.Contains(t, out.String(), "rescued: refs/frit/rescue/7/host-abc")
}

// TestPrintReleaseNamesAWarning: the rendering branch a non-fatal
// failure takes alongside a scavenge, direct-called against a
// hand-built doc.
func TestPrintReleaseNamesAWarning(t *testing.T) {
	doc := report.NewRelease("/root", "atlas", 7, "Shader unit", "plan/7")
	doc.Warn("release: boom")
	var out bytes.Buffer

	printRelease(&out, doc)

	assert.Contains(t, out.String(), "warning: release: boom")
}

// TestReleaseEmitsJSON decodes the report a consumer reads.
func TestReleaseEmitsJSON(t *testing.T) {
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
	var doc struct {
		Command  string `json:"command"`
		Released bool   `json:"released"`
		Branch   string `json:"branch"`
	}

	emit(t, &doc, "release", "7", "--root", root)

	assert.Equal(t, "release", doc.Command)
	assert.True(t, doc.Released)
	assert.Equal(t, "plan/7", doc.Branch)
}
