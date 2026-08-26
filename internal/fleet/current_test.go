package fleet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitCmd runs git in dir for test setup, failing loudly.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// repoOnBranch builds a one-commit repository sitting on branch.
func repoOnBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "t@example.com")
	gitCmd(t, dir, "config", "user.name", "frit-test")
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "README.md"), []byte("x\n"), 0o600))
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", "init")
	if branch != "main" {
		gitCmd(t, dir, "checkout", "-q", "-b", branch)
	}

	return dir
}

// planHolds is the canonical convention, compiled once for a test.
func planHolds(t *testing.T) repocfg.Holds {
	t.Helper()
	holds, err := repocfg.CompileAll([]string{"plan/{id}-*"})
	require.NoError(t, err)

	return holds
}

// markClaim writes a claim marker on the current branch, recording the
// host that took it — the same subject and host line the lease mints, so
// the reader under test finds it.
func markClaim(t *testing.T, dir string, id int64, slug, host string) {
	t.Helper()
	msg := fmt.Sprintf("plan %d: claim %s\n\nhost:     %s\n", id, slug, host)
	gitCmd(t, dir, "commit", "--allow-empty", "-q", "-m", msg)
}

// TestForeignHoldNamesAnotherHostsHolder is the guard that stops a
// shared checkout handing one agent another's lane: standing on a claim
// whose marker records a different host, the preflight names that host.
func TestForeignHoldNamesAnotherHostsHolder(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161809-discovery")
	markClaim(t, root, 2608161809, "discovery", "otherbox")
	holds := planHolds(t)

	host, foreign := ForeignHold(root, "thisbox", gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	assert.True(t, foreign, "a claim held by another host is foreign")
	assert.Equal(t, "otherbox", host)
}

// TestForeignHoldIsSilentOnThisHostsOwnClaim: standing on a lane this
// host itself took is not foreign, so the verb proceeds.
func TestForeignHoldIsSilentOnThisHostsOwnClaim(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161809-discovery")
	markClaim(t, root, 2608161809, "discovery", "thisbox")
	holds := planHolds(t)

	_, foreign := ForeignHold(root, "thisbox", gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	assert.False(t, foreign, "an own-host claim is not foreign")
}

// TestForeignHoldIsSilentWithoutAMarker fails open: a claim branch
// carrying no frit marker — objects not fetched, or a non-frit branch —
// must not read as a foreign hold and block the verb.
func TestForeignHoldIsSilentWithoutAMarker(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161809-discovery")
	holds := planHolds(t)

	_, foreign := ForeignHold(root, "thisbox", gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	assert.False(t, foreign, "an unreadable marker never refuses")
}

// TestCurrentPlanIDReadsTheLaneAWorktreeIsOn is the cwd form of the
// selector: standing in a worktree on a plan branch, the plan is the
// one that branch claims.
func TestCurrentPlanIDReadsTheLaneAWorktreeIsOn(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161809-discovery")
	holds := planHolds(t)

	repo, id, ok := CurrentPlanID(root, gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	require.True(t, ok)
	assert.Equal(t, int64(2608161809), id)
	assert.Equal(t, filepath.Base(root), repo,
		"the id travels with the repository that owns it")
}

// TestCurrentPlanIDWalksUpFromADriftedCWD: the id resolves from a
// subdirectory the shell wandered into, not only the worktree root.
func TestCurrentPlanIDWalksUpFromADriftedCWD(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161809-discovery")
	sub := filepath.Join(root, "internal", "discovery")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	holds := planHolds(t)

	repo, id, ok := CurrentPlanID(sub, gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	require.True(t, ok)
	assert.Equal(t, int64(2608161809), id)
	assert.Equal(t, filepath.Base(root), repo)
}

func TestCurrentPlanIDReportsNoneOffTheConvention(t *testing.T) {
	root := repoOnBranch(t, "feature/side-quest")
	holds := planHolds(t)

	_, _, ok := CurrentPlanID(root, gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	assert.False(t, ok)
}

func TestCurrentPlanIDReportsNoneOutsideAnyRepository(t *testing.T) {
	_, _, ok := CurrentPlanID(t.TempDir(), gitwt.Exec,
		func(string) repocfg.Holds { return nil })

	assert.False(t, ok)
}

// TestCurrentLaneReadsTheLaneAWorktreeIsOn is CurrentPlanID's sibling:
// the same cwd join, but also handing back the worktree root, so a
// caller that already knows the plan id can re-read a file from disk
// without asking herdr.Resolve a second time.
func TestCurrentLaneReadsTheLaneAWorktreeIsOn(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161809-discovery")
	holds := planHolds(t)

	repo, id, wtRoot, ok := CurrentLane(root, gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	require.True(t, ok)
	assert.Equal(t, int64(2608161809), id)
	assert.Equal(t, filepath.Base(root), repo)
	assert.Equal(t, root, wtRoot)
}

func TestCurrentLaneReportsNoneOffTheConvention(t *testing.T) {
	root := repoOnBranch(t, "feature/side-quest")
	holds := planHolds(t)

	_, _, _, ok := CurrentLane(root, gitwt.Exec,
		func(string) repocfg.Holds { return holds })

	assert.False(t, ok)
}
