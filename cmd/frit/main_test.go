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

// isolate points config discovery at an empty working directory and
// clears the environment frit reads, so a test only sees the inputs
// it sets itself. Without it a developer's own ~/.config/frit would
// leak into the result.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("FRIT_ROOT", "")
	t.Setenv("FRIT_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir())
}

// rootWith builds a root directory holding one repository of the
// given name, and returns the root.
func rootWith(t *testing.T, repo string) string {
	t.Helper()
	root := t.TempDir()
	initRepo(t, root, repo)

	return root
}

func TestNoCommandIsAUsageError(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run(nil, &out, &errb)

	assert.Equal(t, 2, code)
	assert.Contains(t, errb.String(), "frit")
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"summon"}, &out, &errb)

	assert.Equal(t, 2, code)
}

func TestHelpExitsZeroAndListsCommands(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"--help"}, &out, &errb)

	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "repos")
}

func TestVersionPrintsTheBuildVersion(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"version"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Equal(t, "dev\n", out.String())
}

func TestReposListsRepositoriesAndWorktrees(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/2608142306",
		filepath.Join(root, "atlas-fleet-index"))
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "2 worktrees")
	assert.Contains(t, got, "plan/2608142306")
	assert.Contains(t, got, "atlas-fleet-index")
}

func TestReposReportsAnEmptyRootPlainly(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", t.TempDir()}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no git repositories found")
}

func TestReposFailsOnAMissingRoot(t *testing.T) {
	isolate(t)
	missing := filepath.Join(t.TempDir(), "absent")
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", missing}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "frit:")
}

func TestRootComesFromTheEnvironment(t *testing.T) {
	isolate(t)
	t.Setenv("FRIT_ROOT", rootWith(t, "from-env"))
	var out, errb bytes.Buffer

	code := run([]string{"repos"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "from-env")
}

func TestRootComesFromARepoLocalConfigFile(t *testing.T) {
	isolate(t)
	writeConfig(t, ".frit.yml", rootWith(t, "from-config"))
	var out, errb bytes.Buffer

	code := run([]string{"repos"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "from-config")
}

func TestRootComesFromAnExplicitConfigFlag(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "explicit.yml")
	writeConfigAt(t, path, rootWith(t, "from-explicit"))
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--config", path}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "from-explicit")
}

func TestFlagBeatsEnvironment(t *testing.T) {
	isolate(t)
	t.Setenv("FRIT_ROOT", rootWith(t, "from-env"))
	var out, errb bytes.Buffer

	code := run([]string{"repos", "--root", rootWith(t, "from-flag")},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "from-flag")
	assert.NotContains(t, out.String(), "from-env")
}

func TestEnvironmentBeatsConfigFile(t *testing.T) {
	isolate(t)
	writeConfig(t, ".frit.yml", rootWith(t, "from-config"))
	t.Setenv("FRIT_ROOT", rootWith(t, "from-env"))
	var out, errb bytes.Buffer

	code := run([]string{"repos"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "from-env")
	assert.NotContains(t, out.String(), "from-config")
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

func TestPluralAgreesWithItsCount(t *testing.T) {
	assert.Equal(t, "1 worktree", plural(1, "worktree"))
	assert.Equal(t, "0 worktrees", plural(0, "worktree"))
	assert.Equal(t, "2 worktrees", plural(2, "worktree"))
}

// sha builds a 40-character object name out of one repeated byte.
func sha(c byte) string {
	return string(bytes.Repeat([]byte{c}, 40))
}

// writeConfig writes a config file into the current directory.
func writeConfig(t *testing.T, name, root string) {
	t.Helper()
	writeConfigAt(t, name, root)
}

func writeConfigAt(t *testing.T, path, root string) {
	t.Helper()
	body := "root: " + root + "\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
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

func TestInitWritesAConfigIntoARepository(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"init", repo}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), ".frit.yml")
	body, err := os.ReadFile(filepath.Join(repo, ".frit.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "plan/{id}-*")
}

func TestInitRefusesToClobberWithoutForce(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var out, errb bytes.Buffer
	require.Equal(t, 0, run([]string{"init", repo}, &out, &errb))

	out.Reset()
	errb.Reset()
	code := run([]string{"init", repo}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "already exists")
}

func TestInitForceOverwrites(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var out, errb bytes.Buffer
	require.Equal(t, 0, run([]string{"init", repo}, &out, &errb))

	code := run([]string{"init", repo, "--force"}, &out, &errb)

	assert.Equal(t, 0, code, errb.String())
}

// TestPlansHonoursEachRepositorysPlanDir is the payoff of per-repo
// config: a repository that keeps plans somewhere else is indexed
// correctly with no flag at all.
func TestPlansHonoursEachRepositorysPlanDir(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.MkdirAll(
		filepath.Join(repo, "docs", "plans"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "docs", "plans", "a.md"),
		[]byte("---\nid: 7\ntitle: Elsewhere\nstatus: \"🔳\"\n---\n# E\n"),
		0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan in an unusual place")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, ".frit.yml"),
		[]byte("plan-dir: docs/plans\n"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"plans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "1 plan")
}

// claimBranch commits one file on a branch named for a plan and
// returns to main, leaving the branch unmerged and with no worktree.
func claimBranch(t *testing.T, repo, branch string) {
	t.Helper()
	git(t, repo, "checkout", "-q", "-b", branch)
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "work.txt"), []byte("wip\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "work on "+branch)
	git(t, repo, "checkout", "-q", "main")
}

func TestOrphansReportsAClaimWithNoCheckout(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed, no checkout")
	assert.Contains(t, out.String(), "2608142306")
}

// TestOrphansIgnoresAMergedClaim is the merged-ref filter end to end:
// finished work must not read as an abandoned claim.
func TestOrphansIgnoresAMergedClaim(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land",
		"plan/2608142306-fleet-index")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no orphaned lanes")
}

func TestOrphansReportsAWorktreeThatNeverStarted(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	// --orphan gives an unborn branch, which is how a worktree ends
	// up with an all-zero HEAD: prepared, never worked.
	git(t, repo, "worktree", "add", "-q", "--orphan", "-b",
		"plan/42-empty", filepath.Join(root, "atlas-empty"))
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "atlas-empty")
}

func TestOrphansIsQuietOnAHealthyRepository(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no orphaned lanes")
}

func TestOrphansHonoursARepositoryWithNoHoldPatterns(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: []\n"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no orphaned lanes",
		"a repo declaring no pattern reports no claims")
}

func TestStaleIsQuietWhenEverythingIsFresh(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"stale", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no worktree idle longer than 30")
}

func TestStaleReportsAnOldWorktree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	// Everything committed just now is older than zero days.
	code := run([]string{"stale", "--root", root, "--days", "0"},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "atlas")
}
