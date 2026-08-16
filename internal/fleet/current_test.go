package fleet

import (
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
