package main

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
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
// no matured window — is refused; only that lane's own token can
// release it.
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
