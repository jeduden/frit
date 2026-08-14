package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithNoArgsIsAUsageError(t *testing.T) {
	var out, errb bytes.Buffer

	code := run(nil, &out, &errb)

	assert.Equal(t, 2, code)
	assert.Contains(t, errb.String(), "usage:")
	assert.Empty(t, out.String())
}

func TestRunRejectsAnUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer

	code := run([]string{"summon"}, &out, &errb)

	assert.Equal(t, 2, code)
	assert.Contains(t, errb.String(), `unknown command "summon"`)
}

func TestRunPrintsVersion(t *testing.T) {
	var out, errb bytes.Buffer

	code := run([]string{"version"}, &out, &errb)

	assert.Equal(t, 0, code)
	assert.Equal(t, "dev\n", out.String())
}

func TestRunPrintsHelpToStdout(t *testing.T) {
	var out, errb bytes.Buffer

	code := run([]string{"help"}, &out, &errb)

	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "frit repos")
	assert.Empty(t, errb.String())
}

func TestReposListsRepositoriesAndWorktrees(t *testing.T) {
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/2608142306",
		filepath.Join(root, "atlas-fleet-index"))
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "atlas")
	assert.Contains(t, got, "2 worktrees")
	assert.Contains(t, got, "plan/2608142306")
	assert.Contains(t, got, "atlas-fleet-index")
}

func TestReposReportsAnEmptyRootPlainly(t *testing.T) {
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", t.TempDir()}, &out, &errb)

	require.Equal(t, 0, code)
	assert.Contains(t, out.String(), "no git repositories found")
}

func TestReposFailsOnAMissingRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", missing}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "frit:")
}

func TestReposRejectsAnUnknownFlag(t *testing.T) {
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--nope"}, &out, &errb)

	assert.Equal(t, 2, code)
}

func TestRefNamesEveryWorktreeState(t *testing.T) {
	assert.Equal(t, "main", ref(gitwt.Worktree{Branch: "main"}))
	assert.Equal(t, "(bare)", ref(gitwt.Worktree{Bare: true}))
	assert.Equal(t, "(detached)",
		ref(gitwt.Worktree{Detached: true}))
	assert.Equal(t, "(unknown)", ref(gitwt.Worktree{}))
}

func TestNoteFlagsOnlyLanesWorthASecondLook(t *testing.T) {
	live := gitwt.Worktree{Branch: "main", Head: sha('a')}
	assert.Empty(t, note(live))
	assert.Empty(t, note(gitwt.Worktree{Bare: true}))

	assert.Equal(t, "no commit",
		note(gitwt.Worktree{Branch: "wip", Head: sha('0')}))
	assert.Equal(t, "prunable",
		note(gitwt.Worktree{Head: sha('b'), Prunable: true}))
	assert.Equal(t, "locked",
		note(gitwt.Worktree{Head: sha('c'), Locked: true}))
}

// sha builds a 40-character object name out of one repeated byte.
func sha(c byte) string {
	return string(bytes.Repeat([]byte{c}, 40))
}

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

func TestPluralAgreesWithItsCount(t *testing.T) {
	assert.Equal(t, "1 worktree", plural(1, "worktree"))
	assert.Equal(t, "0 worktrees", plural(0, "worktree"))
	assert.Equal(t, "2 worktrees", plural(2, "worktree"))
}
