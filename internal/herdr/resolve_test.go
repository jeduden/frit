package herdr

import (
	"errors"
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
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"},
		args...)
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

// eval resolves symlinks so a /tmp that is itself a symlink does not
// fail an otherwise-correct path comparison.
func eval(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)

	return real
}

// TestResolveWalksBackUpFromADriftedCWD is the whole point of using
// rev-parse rather than a string match: the pane's cwd is a
// subdirectory the shell wandered into, and the worktree root is still
// found from it.
func TestResolveWalksBackUpFromADriftedCWD(t *testing.T) {
	root := repoOnBranch(t, "plan/2608161808-herdr-join")
	sub := filepath.Join(root, "docs", "research")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	site := Resolve(sub, gitwt.Exec)

	assert.Equal(t, eval(t, root), eval(t, site.Root))
	assert.Equal(t, "plan/2608161808-herdr-join", site.Branch)
}

// TestResolveHasNoBranchOnDetachedHead keeps a detached checkout from
// inventing a branch: the root is real, the branch is honestly empty.
func TestResolveHasNoBranchOnDetachedHead(t *testing.T) {
	root := repoOnBranch(t, "main")
	gitCmd(t, root, "checkout", "-q", "--detach")

	site := Resolve(root, gitwt.Exec)

	assert.Equal(t, eval(t, root), eval(t, site.Root))
	assert.Empty(t, site.Branch)
}

// TestResolveIsEmptyOutsideARepository: a pane in no worktree is a fact
// to report, not a git error to fail on.
func TestResolveIsEmptyOutsideARepository(t *testing.T) {
	site := Resolve(t.TempDir(), gitwt.Exec)

	assert.Empty(t, site.Root)
	assert.Empty(t, site.Branch)
}

// TestResolveAsksGitForTheToplevelThenTheBranch pins the two calls the
// join is built on.
func TestResolveAsksGitForTheToplevelThenTheBranch(t *testing.T) {
	var calls [][]string
	run := func(dir string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{dir}, args...))
		if args[0] == "rev-parse" {
			return []byte("/repo\n"), nil
		}

		return []byte("plan/7-x\n"), nil
	}

	site := Resolve("/repo/sub/dir", run)

	assert.Equal(t, "/repo", site.Root)
	assert.Equal(t, "plan/7-x", site.Branch)
	require.Len(t, calls, 2)
	assert.Equal(t,
		[]string{"/repo/sub/dir", "rev-parse", "--show-toplevel"},
		calls[0])
	assert.Equal(t, []string{
		"/repo", "symbolic-ref", "--quiet", "--short", "HEAD",
	}, calls[1])
}

// roots maps each pane cwd to the worktree root git would report, and
// each root to the branch it is on, so one fake git answers the whole
// join.
func fakeJoinGit(roots, branches map[string]string) gitwt.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if args[0] == "rev-parse" {
			root, ok := roots[dir]
			if !ok {
				return nil, errors.New("not a git repository")
			}

			return []byte(root + "\n"), nil
		}

		return []byte(branches[dir] + "\n"), nil
	}
}

// TestJoinResolvesEachPaneToItsPlan is the cwd join end to end over a
// fake git: a claimed lane resolves to its plan id, and a branch that
// claims none is still carried rather than dropped.
func TestJoinResolvesEachPaneToItsPlan(t *testing.T) {
	git := fakeJoinGit(
		map[string]string{
			"/fleet/atlas/docs": "/fleet/atlas",
			"/elsewhere":        "/elsewhere",
		},
		map[string]string{
			"/fleet/atlas": "plan/2608161808-herdr-join",
			"/elsewhere":   "feature/side-quest",
		},
	)
	holds, err := repocfg.CompileAll([]string{"plan/{id}-*"})
	require.NoError(t, err)
	holdsFor := func(string) repocfg.Holds { return holds }

	panes := []Pane{
		{Agent: "claude", CWD: "/fleet/atlas/docs"},
		{Agent: "pi", CWD: "/elsewhere"},
	}
	lanes := Join(panes, git, holdsFor)

	require.Len(t, lanes, 2)
	assert.Equal(t, "atlas", lanes[0].Repo)
	assert.Equal(t, "plan/2608161808-herdr-join", lanes[0].Branch)
	assert.Equal(t, int64(2608161808), lanes[0].PlanID)
	assert.True(t, lanes[0].HasPlan())

	assert.Equal(t, "pi", lanes[1].Pane.Agent)
	assert.Equal(t, "elsewhere", lanes[1].Repo)
	assert.False(t, lanes[1].HasPlan(),
		"a pane off the convention is kept, not dropped")
}

// TestLiveRootsCollectsOnlyStaffedResolvableRoots is what makes stale
// agent-aware: only a pane with an agent that resolves to a root
// contributes, and each root appears once however many panes sit in it.
func TestLiveRootsCollectsOnlyStaffedResolvableRoots(t *testing.T) {
	git := fakeJoinGit(
		map[string]string{
			"/fleet/atlas/a": "/fleet/atlas",
			"/fleet/atlas/b": "/fleet/atlas",
			"/fleet/bare":    "/fleet/bare",
			"/no/repo":       "", // resolves, but rev-parse will error
		},
		map[string]string{},
	)

	roots := LiveRoots([]Pane{
		{Agent: "claude", CWD: "/fleet/atlas/a"},
		{Agent: "pi", CWD: "/fleet/atlas/b"},
		{Agent: "", CWD: "/fleet/bare"}, // bare pane, no agent
		{Agent: "claude", CWD: "/no/repo"},
	}, git)

	assert.Equal(t, map[string]bool{"/fleet/atlas": true}, roots)
}

// TestJoinKeepsAPaneInNoRepository carries an agent working somewhere
// git knows nothing about, root and all left empty.
func TestJoinKeepsAPaneInNoRepository(t *testing.T) {
	git := fakeJoinGit(map[string]string{}, map[string]string{})
	holdsFor := func(string) repocfg.Holds { return nil }

	lanes := Join([]Pane{{Agent: "claude", CWD: "/tmp/scratch"}},
		git, holdsFor)

	require.Len(t, lanes, 1)
	assert.Empty(t, lanes[0].Root)
	assert.Empty(t, lanes[0].Repo)
	assert.False(t, lanes[0].HasPlan())
}
