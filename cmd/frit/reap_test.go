package main

import (
	"bytes"
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
