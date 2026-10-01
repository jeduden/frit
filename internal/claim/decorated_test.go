package claim

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decoratedHold builds the #204 shape on repo: a legacy decorated
// branch for plan 7 cut from main, its one commit a legacy claim, and
// — when withWork — a commit of real, unlanded work on top. pushed
// says whether origin carries it too. It returns the branch's tip and
// leaves main checked out.
func decoratedHold(
	t *testing.T, repo, branch string, withWork, pushed bool,
) string {
	t.Helper()
	gitCmd(t, repo, "checkout", "-q", "-b", branch, "main")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m",
		"plan 7: claim shader-unit")
	if withWork {
		require.NoError(t, os.WriteFile(
			filepath.Join(repo, "legacy.txt"), []byte("legacy wip\n"), 0o600))
		gitCmd(t, repo, "add", "-A")
		gitCmd(t, repo, "commit", "-q", "-m", "legacy work")
	}
	if pushed {
		gitCmd(t, repo, "push", "-q", "origin", branch)
	}
	tip := gitCmd(t, repo, "rev-parse", "HEAD")
	gitCmd(t, repo, "checkout", "-q", "main")

	return tip
}

// TestTakeoverDecoratedRetiresTheBranchAndMintsTheLease (#204): a
// matured hold made of a decorated branch alone has no work ref to
// CAS a takeover marker onto. The takeover deletes the decorated
// branch from origin by CAS on the tip the window matured on, then
// acquires the id-only lease — epoch 1, a fresh claim, since a legacy
// claim carries no epoch chain to extend. A claim-only chain parks
// nothing.
func TestTakeoverDecoratedRetiresTheBranchAndMintsTheLease(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, 1, lease.Epoch)
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit", DeletedOnOrigin: true}}, lease.Retired,
		"a claim-only chain is retired with nothing to park")
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7-shader-unit")
	assert.Empty(t, gone, "the decorated branch is deleted from origin")
	_, localErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, localErr, "its local copy, standing nowhere, goes too")
	_, trackingErr := gitCapture(t, work, "rev-parse", "--verify",
		"refs/remotes/origin/plan/7-shader-unit")
	assert.Error(t, trackingErr,
		"no remote-tracking copy is left for the next gather to read as a hold")
	held := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, held, lease.Tip, "the id-only lease now holds the plan")
	subject := gitCmd(t, work, "log", "-1", "--format=%s", lease.Tip)
	assert.Equal(t, "plan 7: claim", subject)
}

// TestTakeoverDecoratedParksUnlandedWorkFirst: work on the decorated
// branch that never landed is parked to the rescue ref before the
// branch is deleted, the same park-before-delete every scavenge keeps.
func TestTakeoverDecoratedParksUnlandedWorkFirst(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", true, true)

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	rescue := "refs/frit/rescue/7/box-b-" + tip
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit", Rescue: rescue,
		DeletedOnOrigin: true}}, lease.Retired)
	assert.Contains(t, gitCmd(t, work, "ls-remote", "origin", rescue), tip,
		"the rescue ref carries the decorated tip")
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin",
		"refs/heads/plan/7-shader-unit"))
}

// TestTakeoverDecoratedRefusesAMovedBranch: the window matured on one
// tip; a holder that pushed to its decorated branch since moved it, so
// the takeover loses as a lost race naming the new tip — nothing is
// parked, deleted or minted (A2).
func TestTakeoverDecoratedRefusesAMovedBranch(t *testing.T) {
	work := originAndClone(t)
	observed := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	gitCmd(t, work, "checkout", "-q", "plan/7-shader-unit")
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "still here")
	gitCmd(t, work, "push", "-q", "origin", "plan/7-shader-unit")
	moved := gitCmd(t, work, "rev-parse", "HEAD")
	gitCmd(t, work, "checkout", "-q", "main")

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": observed}, gitwt.Exec)

	require.ErrorIs(t, err, ErrLostRace)
	var held *HeldError
	require.True(t, errors.As(err, &held))
	assert.Equal(t, moved, held.Tip)
	assert.Contains(t, gitCmd(t, work, "ls-remote", "origin",
		"refs/heads/plan/7-shader-unit"), moved, "nothing was deleted")
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7"),
		"nothing was minted")
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/*"),
		"nothing was parked")
}

// TestTakeoverDecoratedSparesALocalBranchAWorktreeStandsOn: the lane's
// own deserted checkout still stands on the decorated branch. Origin's
// copy is deleted and the lease minted, but the local branch survives
// — deleting it would leave that worktree's HEAD dangling (S79).
func TestTakeoverDecoratedSparesALocalBranchAWorktreeStandsOn(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	lane := filepath.Join(t.TempDir(), "lane")
	gitCmd(t, work, "worktree", "add", "-q", lane, "plan/7-shader-unit")

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin",
		"refs/heads/plan/7-shader-unit"))
	assert.Equal(t, tip, gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit"),
		"the local branch a worktree stands on is left alone")
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit",
		DeletedOnOrigin: true, LocalKept: true}}, lease.Retired,
		"the report says this host's copy still stands")
}

// TestTakeoverDecoratedParksAndDropsALocalOnlyBranch: a decorated hold
// never pushed lives on this host alone. With nothing on origin to
// CAS, its unlanded work is parked to origin's rescue ref before the
// local branch is deleted, so the takeover destroys nothing.
func TestTakeoverDecoratedParksAndDropsALocalOnlyBranch(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", true, false)

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	rescue := "refs/frit/rescue/7/box-b-" + tip
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit", Rescue: rescue}},
		lease.Retired, "nothing was on origin to delete")
	assert.Contains(t, gitCmd(t, work, "ls-remote", "origin", rescue), tip)
	_, localErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, localErr, "the parked local branch is dropped")
}

// TestTakeoverDecoratedKeepsALocalCopyThatMovedPastTheObservedTip: the
// local branch carries commits beyond the tip the window matured on —
// this host's own unpushed work. Origin's copy, still at the observed
// tip, is retired, but the local branch is kept rather than deleted
// out from under work nobody parked.
func TestTakeoverDecoratedKeepsALocalCopyThatMovedPastTheObservedTip(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	gitCmd(t, work, "checkout", "-q", "plan/7-shader-unit")
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "local only")
	local := gitCmd(t, work, "rev-parse", "HEAD")
	gitCmd(t, work, "checkout", "-q", "main")

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, local, gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit"))
	assert.True(t, lease.Retired[0].LocalKept)
}

// TestTakeoverDecoratedErrsWhenTheRemoteCannotBeRead: an unreadable
// origin is a fault, not an absent branch — reading it as "gone" would
// drop the local copy of a hold origin may still carry.
func TestTakeoverDecoratedErrsWhenTheRemoteCannotBeRead(t *testing.T) {
	readErr := errors.New("ls-remote: connection reset")
	failing := func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ls-remote" {
			return nil, readErr
		}

		return gitwt.Exec(dir, args...)
	}
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, failing)

	require.ErrorIs(t, err, readErr)
	assert.Equal(t, tip, gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit"))
}

// TestTakeoverDecoratedRefusesWhenTheParkConflicts: a rescue ref
// already standing at the content-addressed name with other work is a
// conflict, and the decorated branch is not deleted — nothing a park
// could not save is ever destroyed.
func TestTakeoverDecoratedRefusesWhenTheParkConflicts(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", true, true)
	other := gitCmd(t, work, "rev-parse", "origin/main")
	gitCmd(t, work, "push", "-q", "origin",
		other+":refs/frit/rescue/7/box-b-"+tip)

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)

	var conflict *RescueConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Contains(t, err.Error(), "not deleting plan/7-shader-unit")
	assert.Contains(t, gitCmd(t, work, "ls-remote", "origin",
		"refs/heads/plan/7-shader-unit"), tip, "the decorated branch stands")
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7"))
}

// TestTakeoverDecoratedLosesTheAcquireToAnotherMachine: the decorated
// branch is retired, but another machine minted plan/7 first — the
// acquire stays the arbiter, and its lost race is returned as is.
func TestTakeoverDecoratedLosesTheAcquireToAnotherMachine(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	winner, err := Acquire(cloneAgain(t, work),
		leaseOptions("box-c", "/lanes/c"), gitwt.Exec)
	require.NoError(t, err)

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)

	var held *HeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, winner.Tip, held.Tip)
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit", DeletedOnOrigin: true}}, lease.Retired,
		"the branch already retired is still reported, though the acquire lost")
}

// deleteFailing wraps the real runner so the decorated branch's delete
// push fails; after it, ls-remote fails too when confirmUnreadable.
func deleteFailing(confirmUnreadable bool) gitwt.Runner {
	failed := false

	return func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "push" &&
			args[len(args)-1] == ":refs/heads/plan/7-shader-unit" {
			failed = true

			return nil, errors.New("push: connection reset")
		}
		if failed && confirmUnreadable && len(args) > 0 && args[0] == "ls-remote" {
			return nil, errors.New("ls-remote: connection reset")
		}

		return gitwt.Exec(dir, args...)
	}
}

// TestTakeoverDecoratedReportsAnUnconfirmedDelete: the delete push
// failed and origin could not be read back either, so whether the
// branch is gone is unknown — reported as such, and nothing is minted.
func TestTakeoverDecoratedReportsAnUnconfirmedDelete(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, deleteFailing(true))

	var unconfirmed *UnconfirmedDeleteError
	require.ErrorAs(t, err, &unconfirmed)
	assert.Equal(t, "refs/heads/plan/7-shader-unit", unconfirmed.Ref)
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7"))
}

// TestTakeoverDecoratedFaultsOnADeleteTheServerRefuses: the delete push
// failed while origin still carries the branch at the very tip the
// window matured on — a protected branch or a hook refused it, nobody
// moved it. That is a fault naming the push's own error, not a lost
// race that would restart the window over a branch that never moved.
func TestTakeoverDecoratedFaultsOnADeleteTheServerRefuses(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, deleteFailing(false))

	var refused *DeleteRefusedError
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, tip, refused.Holder)
	assert.ErrorContains(t, err, "connection reset", "the push's own error is kept")
	assert.NotErrorIs(t, err, ErrLostRace)
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7"))
}

// TestTakeoverDecoratedLosesToABranchThatMovesDuringTheDelete: the
// holder pushed between the read and the delete, so the CAS delete
// fails with a new tip on origin — a lost race naming it, and nothing
// minted.
func TestTakeoverDecoratedLosesToABranchThatMovesDuringTheDelete(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	holder := cloneAgain(t, work)
	moved := ""
	racing := func(dir string, args ...string) ([]byte, error) {
		if moved == "" && len(args) > 0 && args[0] == "push" &&
			args[len(args)-1] == ":refs/heads/plan/7-shader-unit" {
			gitCmd(t, holder, "checkout", "-q", "plan/7-shader-unit")
			gitCmd(t, holder, "commit", "--allow-empty", "-q", "-m", "still here")
			gitCmd(t, holder, "push", "-q", "origin", "plan/7-shader-unit")
			moved = gitCmd(t, holder, "rev-parse", "HEAD")
		}

		return gitwt.Exec(dir, args...)
	}

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, racing)

	var held *HeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, moved, held.Tip)
	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7"))
}

// TestTakeoverDecoratedSkipsABranchAlreadyGone: the decorated branch
// vanished from origin, and this clone holds no copy, between the read
// that matured the window and the takeover. Nothing holds the plan
// through it any more, so it is skipped — not reported as a lost race
// — and the lease is acquired.
func TestTakeoverDecoratedSkipsABranchAlreadyGone(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	gitCmd(t, work, "push", "-q", "origin", ":refs/heads/plan/7-shader-unit")
	gitCmd(t, work, "branch", "-q", "-D", "plan/7-shader-unit")

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)

	require.NoError(t, err)
	assert.Empty(t, lease.Retired, "nothing was left to retire")
	assert.Contains(t, gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7"),
		lease.Tip)
}

// TestTakeoverDecoratedTakesADeleteThatLandedDespiteAnError: the
// delete push reported an error, yet origin no longer carries the
// branch — a connection dropped after the ref transaction committed.
// Gone is gone: the retirement stands and the lease is acquired.
func TestTakeoverDecoratedTakesADeleteThatLandedDespiteAnError(t *testing.T) {
	work := originAndClone(t)
	tip := decoratedHold(t, work, "plan/7-shader-unit", false, true)
	droppedAfter := func(dir string, args ...string) ([]byte, error) {
		out, err := gitwt.Exec(dir, args...)
		if err == nil && len(args) > 0 && args[0] == "push" &&
			args[len(args)-1] == ":refs/heads/plan/7-shader-unit" {
			return out, errors.New("push: connection reset after commit")
		}

		return out, err
	}

	lease, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, droppedAfter)

	require.NoError(t, err)
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit", DeletedOnOrigin: true}},
		lease.Retired)
}
