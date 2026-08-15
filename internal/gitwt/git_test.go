package gitwt

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner records the arguments it was called with and replays a
// canned answer, so the wiring can be tested without a real git.
func fakeRunner(out string, err error, got *[]string) Runner {
	return func(dir string, args ...string) ([]byte, error) {
		*got = append([]string{dir}, args...)
		return []byte(out), err
	}
}

func TestListAsksGitForPorcelainAndParsesIt(t *testing.T) {
	var got []string
	run := fakeRunner(porcelainFixture, nil, &got)

	wts, err := List("/repo", run)

	require.NoError(t, err)
	assert.Equal(t,
		[]string{"/repo", "worktree", "list", "--porcelain"}, got)
	require.Len(t, wts, 6)
	assert.Equal(t, "feature/fast-path", wts[0].Branch)
}

func TestListPropagatesGitFailure(t *testing.T) {
	var got []string
	boom := errors.New("not a git repository")
	run := fakeRunner("", boom, &got)

	_, err := List("/nope", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestCommonDirTrimsTrailingNewline(t *testing.T) {
	var got []string
	run := fakeRunner("/repo/.git\n", nil, &got)

	dir, err := CommonDir("/repo", run)

	require.NoError(t, err)
	assert.Equal(t, "/repo/.git", dir)
	assert.Equal(t, []string{
		"/repo", "rev-parse", "--path-format=absolute",
		"--git-common-dir",
	}, got)
}

func TestCommonDirPropagatesGitFailure(t *testing.T) {
	var got []string
	run := fakeRunner("", errors.New("boom"), &got)

	_, err := CommonDir("/nope", run)

	require.Error(t, err)
}

func TestExecReportsStderrOnFailure(t *testing.T) {
	dir := t.TempDir()

	_, err := Exec(dir, "rev-parse", "--git-common-dir")

	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not a git repo")
}

func TestExecAgainstARealRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := initRepo(t)

	out, err := Exec(dir, "worktree", "list", "--porcelain")

	require.NoError(t, err)
	wts := ParseWorktreeList(out)
	require.Len(t, wts, 1)
	assert.Equal(t, "main", wts[0].Branch)
	assert.True(t, wts[0].HasCommit())
}

// initRepo builds a real one-commit repository and returns its path.
// Identity is set inline so the test does not depend on the host's
// global git config.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

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
