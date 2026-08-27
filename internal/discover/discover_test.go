package discover

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReposGroupsLinkedWorktreesUnderOneRepo(t *testing.T) {
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	// A linked worktree laid out as a sibling, the way the machines
	// this targets actually store them (<repo>-<slug>).
	git(t, repo, "worktree", "add", "-q", "-b", "plan/2608142306",
		filepath.Join(root, "atlas-fleet-index"))

	got, _, err := Repos(root, gitwt.Exec)

	require.NoError(t, err)
	require.Len(t, got, 1, "two checkouts, one repository")
	assert.Equal(t, "atlas", got[0].Name)
	require.Len(t, got[0].Worktrees, 2)
	assert.Equal(t, "plan/2608142306", got[0].Worktrees[1].Branch)
}

func TestReposFindsSeveralRepositories(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "alpha")
	initRepo(t, root, "beta")

	got, _, err := Repos(root, gitwt.Exec)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "alpha", got[0].Name, "sorted by name")
	assert.Equal(t, "beta", got[1].Name)
}

func TestReposDoesNotDescendIntoARepository(t *testing.T) {
	root := t.TempDir()
	outer := initRepo(t, root, "outer")
	initRepo(t, outer, "nested")

	got, _, err := Repos(root, gitwt.Exec)

	require.NoError(t, err)
	require.Len(t, got, 1, "a vendored checkout is not a fleet lane")
	assert.Equal(t, "outer", got[0].Name)
}

func TestReposSkipsNoiseDirectories(t *testing.T) {
	root := t.TempDir()
	initRepo(t, filepath.Join(root, "node_modules"), "dep")
	initRepo(t, filepath.Join(root, ".cache"), "hidden")

	got, _, err := Repos(root, gitwt.Exec)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReposOnARootWithNoRepositories(t *testing.T) {
	root := t.TempDir()
	require.NoError(t,
		os.MkdirAll(filepath.Join(root, "docs"), 0o750))

	got, _, err := Repos(root, gitwt.Exec)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReposFailsOnAMissingRoot(t *testing.T) {
	_, _, err := Repos(filepath.Join(t.TempDir(), "absent"), gitwt.Exec)

	require.Error(t, err)
}

// TestReposSkipsACandidateGitRefusesToAnswerFor: a candidate git
// refuses to answer for does not abort the walk or vanish silently —
// it is named in the skipped list so a caller can choose to surface
// it, and every other candidate is still read.
func TestReposSkipsACandidateGitRefusesToAnswerFor(t *testing.T) {
	root := t.TempDir()
	// A .git file that points nowhere: git errors, and the walk must
	// carry on rather than abort.
	broken := filepath.Join(root, "broken")
	require.NoError(t, os.MkdirAll(broken, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".git"),
		[]byte("gitdir: /nonexistent\n"), 0o600))
	initRepo(t, root, "good")

	got, skipped, err := Repos(root, gitwt.Exec)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "good", got[0].Name)
	require.Len(t, skipped, 1)
	assert.Equal(t, broken, skipped[0].Dir)
	assert.Error(t, skipped[0].Err)
}

func TestSkipDirNamesNoiseAndDotfiles(t *testing.T) {
	assert.True(t, skipDir(".git"))
	assert.True(t, skipDir("node_modules"))
	assert.True(t, skipDir(".hidden"))
	assert.False(t, skipDir("atlas"))
	assert.False(t, skipDir("plan"))
}

func TestIsWorkTreeAcceptsBothFileAndDirectoryGit(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, isWorkTree(dir))

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"),
		[]byte("gitdir: elsewhere\n"), 0o600))
	assert.True(t, isWorkTree(dir), "linked worktrees use a .git file")
}

// initRepo creates a one-commit repository at parent/name and returns
// its path.
func initRepo(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(dir, 0o750))

	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "frit-test")
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "README.md"), []byte("# fixture\n"), 0o600))
	git(t, dir, "add", "README.md")
	git(t, dir, "commit", "-q", "-m", "initial")

	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"},
		args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}
