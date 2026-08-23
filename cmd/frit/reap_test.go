package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strandedCheckout stands a worktree up on a fresh branch with a real
// commit, so it is never empty and never prunable — the plain
// "checked out on a landed branch" shape every reap fixture starts
// from. It does not land the branch itself; callers do that their own
// way — an ordinary merge or a squash — so one fixture serves every
// landed-evidence case.
func strandedCheckout(t *testing.T, root, repo, name, branch string) string {
	t.Helper()
	lane := filepath.Join(root, name)
	git(t, repo, "worktree", "add", "-q", "-b", branch, lane)
	require.NoError(t, os.WriteFile(
		filepath.Join(lane, "work.txt"), []byte("done\n"), 0o600))
	git(t, lane, "add", "-A")
	git(t, lane, "commit", "-q", "-m", "work on "+branch)

	return lane
}

// branchExists reports whether a branch still resolves in repo.
func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	_, err := gitCapture(t, repo, "rev-parse", "--verify", "--quiet",
		"refs/heads/"+branch)

	return err == nil
}

// TestReapRemovesALandedCheckoutAndDeletesItsBranchWithGo: a stranded
// lane landed by an ordinary merge is torn down under --go — the
// worktree is gone from disk, the branch no longer resolves, and the
// report names both.
func TestReapRemovesALandedCheckoutAndDeletesItsBranchWithGo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := strandedCheckout(t, root, repo, "atlas-landed", branch)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land", branch)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "reaped")
	assert.Contains(t, out.String(), "atlas-landed")
	assert.Contains(t, out.String(), branch)
	_, statErr := os.Stat(lane)
	assert.ErrorIs(t, statErr, os.ErrNotExist,
		"the landed checkout is removed from disk")
	assert.False(t, branchExists(t, repo, branch),
		"the landed branch no longer resolves")
}

// TestReapStreamsProgressAsStrandedLanesAreReaped: a --go reap of two
// stranded, landed checkouts announces each one to stderr as it is
// torn down, not only in the final stdout table — the cost here is a
// serial network push per lane, so a run must read as live, not hung.
func TestReapStreamsProgressAsStrandedLanesAreReaped(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branchA := "plan/1-a"
	branchB := "plan/2-b"
	strandedCheckout(t, root, repo, "atlas-a", branchA)
	strandedCheckout(t, root, repo, "atlas-b", branchB)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land a", branchA)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land b", branchB)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, errb.String(), "reaped")
	assert.Contains(t, errb.String(), branchA,
		"the first lane streams as it is reaped")
	assert.Contains(t, errb.String(), branchB,
		"the second lane streams as it is reaped")
}

// TestReapJSONLeavesStderrEmptyDuringStrandedTeardown pins the JSON
// contract: nothing reaches stderr under --json, because stdout is
// then the whole report.
func TestReapJSONLeavesStderrEmptyDuringStrandedTeardown(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/1-a"
	strandedCheckout(t, root, repo, "atlas-a", branch)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land a", branch)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root, "--json"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Empty(t, errb.String(), "the JSON path writes nothing to stderr")
}

// TestReapWithoutGoLeavesEverythingStanding: the same lane, without
// --go, is untouched — reap is a dry-run by default, and the report
// says what it would do rather than doing it.
func TestReapWithoutGoLeavesEverythingStanding(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := strandedCheckout(t, root, repo, "atlas-landed", branch)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land", branch)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "atlas-landed")
	assert.Contains(t, out.String(), branch)
	_, statErr := os.Stat(lane)
	assert.NoError(t, statErr, "nothing is removed without --go")
	assert.True(t, branchExists(t, repo, branch),
		"nothing is deleted without --go")
}

// TestReapRefusesABranchNotConfirmedLanded reproduces the S79 shape:
// a worktree's branch ref is dropped by plumbing rather than landed —
// merged never lists it, and no plan claims it done — so reap must
// refuse rather than tear the checkout down on the stranded
// classification alone.
func TestReapRefusesABranchNotConfirmedLanded(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := strandedCheckout(t, root, repo, "atlas-orphan", branch)
	git(t, repo, "update-ref", "-d", "refs/heads/"+branch)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "atlas-orphan")
	_, statErr := os.Stat(lane)
	assert.NoError(t, statErr,
		"an unconfirmed branch is left standing, even with --go")
}

// TestReapStrandedTeardownFailureRefusesOnlyThatLaneWithGo: one landed
// checkout git refuses to tear down (a leftover untracked file) must
// not hide an unrelated, unambiguously prunable worktree in the same
// repository — the same per-item fault isolation reapPruned's own
// worktree-remove failures already get.
func TestReapStrandedTeardownFailureRefusesOnlyThatLaneWithGo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")

	branch := "plan/1-bad"
	lane := strandedCheckout(t, root, repo, "atlas-bad", branch)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land", branch)
	require.NoError(t, os.WriteFile(
		filepath.Join(lane, "leftover.txt"), []byte("wip\n"), 0o600))

	goneLane := filepath.Join(root, "atlas-gone")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/2-gone", goneLane)
	require.NoError(t, os.RemoveAll(goneLane))

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "atlas-bad")
	_, statErr := os.Stat(lane)
	assert.NoError(t, statErr,
		"the lane git refused to tear down is left standing")
	assert.Contains(t, out.String(), "atlas-gone",
		"an unrelated prunable worktree is still reaped in the same run")
	listed, err := gitCapture(t, repo, "worktree", "list", "--porcelain")
	require.NoError(t, err)
	assert.NotContains(t, listed, "atlas-gone",
		"the unrelated prunable worktree is gone despite the other lane's failure")
}

// addOrigin gives a fixture repository a bare origin with main pushed,
// so a park has somewhere to push a rescue ref. The origin lives
// outside root so the fleet walk does not index it as a repository of
// its own.
func addOrigin(t *testing.T, repo string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")
}

// TestReapSquashMergedBranchIsReapedEvenNotAnAncestor is the
// squash-merge counterpart: the plan is done on the default branch,
// but the checkout's own branch was never merged there, so
// merge-base --is-ancestor alone would miss it. Frit's own landed
// check is the authority reap deletes on — and because that evidence
// is not tied to the branch tip, the tip's commits are parked to the
// plan's rescue ref before the branch is deleted, so a follow-up
// commit the squash never carried is moved, not destroyed.
func TestReapSquashMergedBranchIsReapedEvenNotAnAncestor(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := strandedCheckout(t, root, repo, "atlas-squashed", branch)
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/"+branch)
	require.NoError(t, err)
	landPlan(t, repo, 2608142306, "fleet-index", "✅")
	addOrigin(t, repo)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "reaped")
	_, statErr := os.Stat(lane)
	assert.ErrorIs(t, statErr, os.ErrNotExist,
		"a squash-merged plan's checkout is reaped on its own status")
	assert.False(t, branchExists(t, repo, branch))
	rescue, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/frit/rescue/2608142306/*")
	require.NoError(t, err)
	assert.Contains(t, rescue, tip,
		"the tip the squash never carried is parked before the delete")
}

// TestReapDryRunPreviewsTheRescueRef: the preview must not hide that a
// --go will move work — the document names the rescue ref the park
// would write.
func TestReapDryRunPreviewsTheRescueRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	strandedCheckout(t, root, repo, "atlas-squashed", branch)
	landPlan(t, repo, 2608142306, "fleet-index", "✅")
	addOrigin(t, repo)
	var doc report.ReapDoc

	stderr := emit(t, &doc, "reap", "--root", root)

	assert.Empty(t, stderr)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Reaped, 1)
	assert.Equal(t, "refs/frit/rescue/2608142306/"+hostname(),
		doc.Repos[0].Reaped[0].Rescue,
		"the dry run names where the work would be parked")
	rescue, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/frit/rescue/*")
	require.NoError(t, err)
	assert.Empty(t, rescue, "a dry run parks nothing")
}

// TestReapRefusesTheTeardownWhenTheParkIsRefused: a rescue ref already
// holding other work is exactly what the park exists to keep — the
// teardown is refused whole, worktree and branch both left standing,
// rather than deleting work that was never parked.
func TestReapRefusesTheTeardownWhenTheParkIsRefused(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := strandedCheckout(t, root, repo, "atlas-squashed", branch)
	landPlan(t, repo, 2608142306, "fleet-index", "✅")
	addOrigin(t, repo)
	// A foreign rescue already sits at this plan's ref name, at a tip
	// that is not the branch's — the create-only park must refuse it.
	foreign, err := gitCapture(t, repo, "rev-parse", "main")
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "push", "-q", "origin",
		foreign+":refs/frit/rescue/2608142306/"+hostname())
	require.NoError(t, err)
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	_, statErr := os.Stat(lane)
	assert.NoError(t, statErr, "a failed park leaves the checkout standing")
	assert.True(t, branchExists(t, repo, branch),
		"a failed park deletes no branch")
}

// TestReapIsQuietOnAHealthyRepository matches the rest of the ladder's
// reports: nothing stranded reads as nothing to say, not a blank
// table.
func TestReapIsQuietOnAHealthyRepository(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing to reap")
}

// holdRef reads a plan's canonical id-only hold ref, "" when absent.
func holdRef(t *testing.T, repo string, id int64) (string, error) {
	t.Helper()

	return gitCapture(t, repo, "rev-parse", "--verify", "--quiet",
		fmt.Sprintf("refs/heads/plan/%d", id))
}

// deadHold claims plan 7's canonical hold bound to a herdr session and
// then installs a herdr whose pane list carries no such session — the
// dead-holder shape (S61 family): the binding is positive evidence the
// agent is gone, no staleness window consulted. This is the one
// abandonment evidence a test can produce without aging a window.
func deadHold(t *testing.T, repo string) {
	t.Helper()
	_, err := claim.Acquire(repo, claim.LeaseOptions{
		PlanID: 7, Remote: "origin", Base: "origin/main",
		Holder: "elsewhere", Lane: "/lanes/x", Session: "wS:p9",
	}, gitwt.Exec)
	require.NoError(t, err)
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id":       "wO:p1",
		"agent_session": map[string]any{"value": "wOther:sess"},
	}))
}

// TestReapRefusesAFreshUnstaffedHold: "claimed, no local checkout" is
// not abandonment evidence — the checkout may be another machine's,
// or the claim seconds old with its worktree stand-up still pending.
// A hold whose lease is neither observed stale nor confirmed dead is
// refused, exactly the gate discovery.Ready and takeover honor.
func TestReapRefusesAFreshUnstaffedHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	cr, _ := startHerdr()
	withHerdr(t, cr)
	var claimed bytes.Buffer
	code := run([]string{"claim", "7", "--root", root}, &claimed, &claimed)
	require.Equal(t, 0, code, claimed.String())
	tip, err := holdRef(t, repo, 7)
	require.NoError(t, err)
	require.NotEmpty(t, tip, "the claim minted the canonical hold")

	var out, errb bytes.Buffer
	code = run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "stale or dead")
	_, err = holdRef(t, repo, 7)
	assert.NoError(t, err, "a live lease survives reap --go")
}

// TestReapDropsADeadSessionsHoldWithGo: a hold whose bound session
// herdr positively confirms gone is abandoned by the protocol's own
// evidence, and --go drops it through the scavenge.
func TestReapDropsADeadSessionsHoldWithGo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	deadHold(t, repo)

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "dropped")
	_, err := holdRef(t, repo, 7)
	assert.Error(t, err, "the dead session's hold no longer resolves")
	gone, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/heads/plan/7")
	require.NoError(t, err)
	assert.Empty(t, gone, "the hold is gone from origin too")
}

// TestReapWithoutGoLeavesTheHoldStanding: the same dead-session hold,
// without --go, is reported as a would-drop and left untouched.
func TestReapWithoutGoLeavesTheHoldStanding(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	deadHold(t, repo)

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "would drop")
	_, err := holdRef(t, repo, 7)
	assert.NoError(t, err, "nothing is dropped without --go")
}

// TestReapRefusesADecoratedUnstaffedHold: a legacy decorated hold —
// plan/<id>-slug rather than the lease protocol's own plan/<id> — is
// not claim.Scavenge's ref to CAS against, so reap refuses it with a
// migrate-first reason rather than silently doing nothing useful.
func TestReapRefusesADecoratedUnstaffedHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "migrate")
	assert.True(t, branchExists(t, repo, "plan/2608142306-fleet-index"),
		"a decorated hold is left standing")
}

// TestReapRefusesEveryDecoratedHoldOnAnUnstaffedLane: a lane claimed
// twice, on two decorated branches for the same plan and neither the
// lease protocol's own canonical ref, names both branches in the
// report rather than only the first one found — both need the same
// migration.
func TestReapRefusesEveryDecoratedHoldOnAnUnstaffedLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	claimBranch(t, repo, "plan/2608142306-other-slug")
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "plan/2608142306-fleet-index")
	assert.Contains(t, out.String(), "plan/2608142306-other-slug")
}

// TestReapRefusesAHoldAnotherMachineTookOver: a takeover winner is a
// live holder — fresh marker, no matured window, no dead session — so
// reap refuses it the same way it refuses any live lease; the CAS
// inside the scavenge is the second line of defense, not the first.
func TestReapRefusesAHoldAnotherMachineTookOver(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	cr, _ := startHerdr()
	withHerdr(t, cr)
	var claimed bytes.Buffer
	code := run([]string{"claim", "7", "--root", root}, &claimed, &claimed)
	require.Equal(t, 0, code, claimed.String())

	fenceWithATakeover(t, repo, 7)

	var out, errb bytes.Buffer
	code = run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	tip, err := holdRef(t, repo, 7)
	require.NoError(t, err)
	assert.NotEmpty(t, tip, "the taken-over hold is left standing")
}

// TestReapParksUnlandedWorkBeforeDroppingTheHold: real work landed on
// the hold ref itself before its holder died — a crashed machine, say
// — so reap must park it to a rescue ref before the hold is dropped,
// never discard it.
func TestReapParksUnlandedWorkBeforeDroppingTheHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	deadHold(t, repo)

	git(t, repo, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "work.txt"), []byte("wip\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "unlanded work")
	git(t, repo, "push", "-q", "origin", "plan/7")
	git(t, repo, "checkout", "-q", "main")

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	rescue, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/frit/rescue/7/*")
	require.NoError(t, err)
	assert.NotEmpty(t, rescue, "the unlanded work is parked, not dropped")
	_, err = holdRef(t, repo, 7)
	assert.Error(t, err, "the hold is still dropped once its work is parked")
}

// TestReapPrunesAPrunableWorktreeWithGo: a worktree git already
// considers removable — its directory gone from disk — is pruned
// under --go, and left standing without it.
func TestReapPrunesAPrunableWorktreeWithGo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	lane := filepath.Join(root, "atlas-gone")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/9-gone", lane)
	require.NoError(t, os.RemoveAll(lane))

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "atlas-gone")
	listed, err := gitCapture(t, repo, "worktree", "list", "--porcelain")
	require.NoError(t, err)
	assert.Contains(t, listed, "atlas-gone", "nothing is pruned without --go")

	out.Reset()
	errb.Reset()
	code = run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	listed, err = gitCapture(t, repo, "worktree", "list", "--porcelain")
	require.NoError(t, err)
	assert.NotContains(t, listed, "atlas-gone",
		"the prunable worktree is gone under --go")
}

// TestReapRemovesAnEmptyWorktreeWithGo: a worktree prepared but never
// worked — an unborn branch, all-zero HEAD — is removed under --go.
func TestReapRemovesAnEmptyWorktreeWithGo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	lane := filepath.Join(root, "atlas-empty")
	git(t, repo, "worktree", "add", "-q", "--orphan", "-b",
		"plan/42-empty", lane)

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "atlas-empty")
	_, statErr := os.Stat(lane)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestReapRemovesAnEmptyWorktreeWithoutAlsoRefusingItAsStranded pins
// the S79 double-classification hazard shut: an unborn worktree whose
// branch matches a hold pattern has no ref for lanes.Build to find, so
// it reads as a Stranded lane with no live hold and, independently, as
// Empty. Judging it against the landed evidence too would refuse it
// ("frit does not read this branch as landed") in the very same run
// the Empty pass safely reaps it in — a self-contradictory report.
// Only the Empty pass's kind should ever speak for this worktree.
func TestReapRemovesAnEmptyWorktreeWithoutAlsoRefusingItAsStranded(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	lane := filepath.Join(root, "atlas-empty")
	git(t, repo, "worktree", "add", "-q", "--orphan", "-b",
		"plan/42-empty", lane)

	var out, errb bytes.Buffer
	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, out.String(), "refused",
		"a worktree the Empty pass safely reaps is never also refused as stranded")
	_, statErr := os.Stat(lane)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
