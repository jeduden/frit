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
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit"}}, lease.Retired,
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
	assert.Equal(t, []Retired{{Branch: "plan/7-shader-unit", Rescue: rescue}},
		lease.Retired)
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

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	assert.Empty(t, gitCmd(t, work, "ls-remote", "origin",
		"refs/heads/plan/7-shader-unit"))
	assert.Equal(t, tip, gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit"),
		"the local branch a worktree stands on is left alone")
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
		lease.Retired)
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

	_, err := TakeoverDecorated(work, leaseOptions("box-b", "/lanes/b"),
		map[string]string{"plan/7-shader-unit": tip}, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, local, gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit"))
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
