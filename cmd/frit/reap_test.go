package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

// TestReapSquashMergedBranchIsReapedEvenNotAnAncestor is the
// squash-merge counterpart: the plan is done on the default branch,
// but the checkout's own branch was never merged there, so
// merge-base --is-ancestor alone would miss it. Frit's own landed
// check is the authority reap deletes on.
func TestReapSquashMergedBranchIsReapedEvenNotAnAncestor(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := strandedCheckout(t, root, repo, "atlas-squashed", branch)
	landPlan(t, repo, 2608142306, "fleet-index", "✅")
	var out, errb bytes.Buffer

	code := run([]string{"reap", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "reaped")
	_, statErr := os.Stat(lane)
	assert.ErrorIs(t, statErr, os.ErrNotExist,
		"a squash-merged plan's checkout is reaped on its own status")
	assert.False(t, branchExists(t, repo, branch))
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

// TestReapDropsAnUnstaffedHoldWithGo: a plan claimed on its canonical
// id-only ref but never staffed with a real worktree — a claim whose
// herdr worktree stand-up never happened, the shape a faked herdr
// leaves behind — has its hold dropped under --go.
func TestReapDropsAnUnstaffedHoldWithGo(t *testing.T) {
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
	assert.Contains(t, out.String(), "7")
	_, err = holdRef(t, repo, 7)
	assert.Error(t, err, "the unstaffed hold no longer resolves")
}

// TestReapWithoutGoLeavesTheHoldStanding: the same unstaffed hold,
// without --go, is untouched.
func TestReapWithoutGoLeavesTheHoldStanding(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	cr, _ := startHerdr()
	withHerdr(t, cr)
	var claimed bytes.Buffer
	code := run([]string{"claim", "7", "--root", root}, &claimed, &claimed)
	require.Equal(t, 0, code, claimed.String())

	var out, errb bytes.Buffer
	code = run([]string{"reap", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
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

// TestReapRefusesAnUnstaffedHoldFencedByAnotherMachine: the hold moved
// since reap observed its tip — another machine took it over — so the
// scavenge CAS loses and reap refuses rather than drop a lease that
// may be actively renewing.
func TestReapRefusesAnUnstaffedHoldFencedByAnotherMachine(t *testing.T) {
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
	assert.NotEmpty(t, tip, "the fenced hold is left standing")
}

// TestReapParksUnlandedWorkBeforeDroppingTheHold: real work landed on
// the hold ref itself before its worktree vanished — a crashed
// machine, say — so reap must park it to a rescue ref before the hold
// is dropped, never discard it.
func TestReapParksUnlandedWorkBeforeDroppingTheHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	cr, _ := startHerdr()
	withHerdr(t, cr)
	var claimed bytes.Buffer
	code := run([]string{"claim", "7", "--root", root}, &claimed, &claimed)
	require.Equal(t, 0, code, claimed.String())

	git(t, repo, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "work.txt"), []byte("wip\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "unlanded work")
	git(t, repo, "push", "-q", "origin", "plan/7")
	git(t, repo, "checkout", "-q", "main")

	var out, errb bytes.Buffer
	code = run([]string{"reap", "--go", "--root", root}, &out, &errb)

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
