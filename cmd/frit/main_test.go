package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/report"
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
	// The per-host state files — presence, observations — go to a
	// throwaway cache, so a test neither reads nor pollutes the real one.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
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

// TestResolveSelectorGuardsWorkVerbsNotReports scopes the foreign-checkout
// guard: standing in a shared clone on another host's claim, an acting
// verb refuses so it cannot work the foreign lane, while a read-only
// report answers normally — refusing a read hands out no lane and only
// blocks a harmless status query.
func TestResolveSelectorGuardsWorkVerbsNotReports(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	commitPlan(t, repo, 7, "🔳", "Shader unit", nil, "")
	git(t, repo, "checkout", "-q", "-b", "plan/7-shader-unit")
	git(t, repo, "commit", "--allow-empty", "-q", "-m",
		"plan 7: claim shader-unit\n\nhost:     otherbox\n")
	t.Chdir(repo)

	t.Run("an acting verb refuses the foreign lane", func(t *testing.T) {
		var out, errb bytes.Buffer
		code := run([]string{"claim", "--root", repo}, &out, &errb)
		assert.NotEqual(t, 0, code)
		assert.Contains(t, errb.String(), "held by otherbox")
	})
	t.Run("a read-only report answers without refusing", func(t *testing.T) {
		var out, errb bytes.Buffer
		code := run([]string{"show", "--root", repo}, &out, &errb)
		require.Equal(t, 0, code, errb.String())
		assert.NotContains(t, out.String()+errb.String(), "held by otherbox")
	})
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

// landedDeletedClone builds a root holding one clone whose origin has
// since squash-landed the plan and deleted its lease branch, while the
// clone's own refs/remotes/origin/* still carry the pre-land state — a
// checkout that has not fetched since. Only a fetch reveals the plan as
// landed and its lease gone; --no-fetch reads it as held off the stale
// remote-tracking copy.
func landedDeletedClone(t *testing.T, name string, id int) string {
	t.Helper()
	branch := "plan/" + strconv.Itoa(id)

	origin := initRepo(t, t.TempDir(), name)
	commitPlan(t, origin, id, "🔲", "Shader unit", nil, "")
	git(t, origin, "checkout", "-q", "-b", branch)
	git(t, origin, "commit", "--allow-empty", "-q", "-m",
		fmt.Sprintf("plan %d: claim", id))
	git(t, origin, "checkout", "-q", "main")

	root := t.TempDir()
	git(t, root, "clone", "-q", origin, filepath.Join(root, name))

	commitPlan(t, origin, id, "✅", "Shader unit", nil, "")
	git(t, origin, "branch", "-D", branch)

	return root
}

// findFirst returns a pointer to the first item matching, or nil when
// none does — the one linear-scan-by-predicate every by-field test
// lookup (boardPlanByID, boardPlanByRepo, planCardByRepo) shares,
// rather than each keeping its own copy of the same loop.
func findFirst[T any](items []T, match func(T) bool) *T {
	for i := range items {
		if match(items[i]) {
			return &items[i]
		}
	}

	return nil
}

// boardPlanByID returns the board's row for a plan id, or nil when the
// plan is off the board.
func boardPlanByID(doc report.BoardDoc, id int64) *report.BoardPlan {
	return findFirst(doc.Plans, func(p report.BoardPlan) bool { return p.ID == id })
}

// TestFetchFlagDefaultsOnAndNegates: the global --fetch bool defaults
// on and --no-fetch turns it off, parsed like any other global.
func TestFetchFlagDefaultsOnAndNegates(t *testing.T) {
	isolate(t)

	var on cli
	parser, err := newParser(&on, &bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)
	_, err = parser.Parse([]string{"board"})
	require.NoError(t, err)
	assert.True(t, on.Fetch, "the fetch flag defaults on")

	var off cli
	parser, err = newParser(&off, &bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)
	_, err = parser.Parse([]string{"board", "--no-fetch"})
	require.NoError(t, err)
	assert.False(t, off.Fetch, "--no-fetch turns it off")
}

// TestFetchFlagReachesTheReadWalk proves the flag reaches the single
// gather every read verb shares: by default board fetches and the
// landed-and-deleted plan is off the board, while --no-fetch reads the
// stale local view and the plan still reads as held.
func TestFetchFlagReachesTheReadWalk(t *testing.T) {
	isolate(t)
	withHerdr(t, herdrReturning())

	// A fetch mutates the clone's refs on disk, so each run gets its own
	// clone: otherwise the default run would refresh the view the
	// --no-fetch run is meant to read stale.
	var fresh report.BoardDoc
	emit(t, &fresh, "board", "--root", landedDeletedClone(t, "atlas", 7))
	assert.Nil(t, boardPlanByID(fresh, 7),
		"with the default fetch, the landed plan is off the board")

	var stale report.BoardDoc
	emit(t, &stale, "board", "--no-fetch",
		"--root", landedDeletedClone(t, "atlas", 7))
	p := boardPlanByID(stale, 7)
	require.NotNil(t, p, "without a fetch, the plan is still outstanding")
	assert.True(t, p.Held,
		"the stale remote-tracking lease branch reads as held")
}

// TestGitTimeoutFlagReachesTheGitRunner proves --git-timeout is wired
// into rt.git, not just parsed, the same way TestFetchFlagReachesTheReadWalk
// proves --fetch is: an unreasonably small bound loses the race
// against time.After for every git call, even a fast local one, so a
// repository that the default timeout finds is skipped instead.
func TestGitTimeoutFlagReachesTheGitRunner(t *testing.T) {
	isolate(t)
	root := rootWith(t, "atlas")

	var normal report.ReposDoc
	emit(t, &normal, "repos", "--root", root)
	assert.Len(t, normal.Repos, 1,
		"with the default timeout the repo is found")

	var bounded report.ReposDoc
	emit(t, &bounded, "repos", "--root", root, "--git-timeout", "1ns")
	assert.Empty(t, bounded.Repos,
		"a 1ns bound loses the race against every git call, "+
			"including this fast local one, so the repo is skipped")
}

// TestGitTimeoutMustBePositive: a zero or negative bound would trip on
// every git call, including a healthy local one, and look like every
// repository is unreachable instead of naming the real cause — a
// misconfigured flag. frit rejects it up front instead.
func TestGitTimeoutMustBePositive(t *testing.T) {
	isolate(t)
	root := rootWith(t, "atlas")

	var out, errb bytes.Buffer
	code := run([]string{"repos", "--root", root, "--git-timeout", "0s"},
		&out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "--git-timeout must be positive")
}

// blockingHerdr fakes a wedged herdr socket: the call starts, then
// only returns once ctx is done or 150ms passes, whichever comes
// first — the way a real herdr subprocess killed by a fired context
// returns promptly instead of running on. It is context-aware rather
// than a plain herdr.Runner because withHerdr's fakes are closures
// with nothing to kill, so this is the one test that needs the real
// seam: it sets herdrRunner directly instead of going through
// withHerdr's context-dropping shim.
func blockingHerdr(agent map[string]any) herdr.ContextRunner {
	body := herdrReturning(agent)
	return func(ctx context.Context, args ...string) ([]byte, error) {
		select {
		case <-time.After(150 * time.Millisecond):
			return body(args...)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// TestHerdrTimeoutFlagReachesTheHerdrRunner proves --herdr-timeout is
// wired into rt.herdr, not just parsed: a wedged herdr read under a 1ns
// bound loses the race for every call, so who finishes with the live
// agent absent — presence read as unreachable — rather than hanging on
// it, while the default bound still surfaces the pane.
func TestHerdrTimeoutFlagReachesTheHerdrRunner(t *testing.T) {
	isolate(t)
	repo := repoOnPlan(t, t.TempDir(), "atlas",
		"plan/2608161808-herdr-join")
	prevHerdrRunner := herdrRunner
	herdrRunner = blockingHerdr(map[string]any{
		"agent":                   "claude",
		"agent_status":            "working",
		"cwd":                     repo,
		"pane_id":                 "wC:p1",
		"workspace_id":            "wC",
		"terminal_title_stripped": "Land the join",
	})
	t.Cleanup(func() { herdrRunner = prevHerdrRunner })

	var normal, errb bytes.Buffer
	code := run([]string{"who", "--root", repo}, &normal, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, normal.String(), "claude",
		"the default bound outwaits the fake and surfaces the pane")

	var bounded bytes.Buffer
	errb.Reset()
	before := time.Now()
	code = run([]string{"who", "--root", repo, "--herdr-timeout", "1ns"},
		&bounded, &errb)
	elapsed := time.Since(before)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, bounded.String(), "claude",
		"a 1ns bound loses the race, so the wedged herdr read is dropped")
	assert.Less(t, elapsed, 100*time.Millisecond,
		"who returns without waiting out the wedged herdr")
}

// TestHerdrTimeoutMustBePositive: a zero or negative bound would trip
// every herdr call the way TestGitTimeoutMustBePositive's does its git
// ones, and look like herdr is unreachable rather than naming the
// misconfigured flag. frit rejects it up front instead.
func TestHerdrTimeoutMustBePositive(t *testing.T) {
	isolate(t)
	root := rootWith(t, "atlas")

	var out, errb bytes.Buffer
	code := run([]string{"who", "--root", root, "--herdr-timeout", "0s"},
		&out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "--herdr-timeout must be positive")
}

func TestRefNamesEveryWorktreeState(t *testing.T) {
	assert.Equal(t, "main", ref(report.Worktree{Branch: "main"}))
	assert.Equal(t, "(bare)", ref(report.Worktree{Bare: true}))
	assert.Equal(t, "(detached)",
		ref(report.Worktree{Detached: true}))
	assert.Equal(t, "(unknown)", ref(report.Worktree{}))
}

func TestNoteFlagsOnlyLanesWorthASecondLook(t *testing.T) {
	live := report.Worktree{Branch: "main", HasCommit: true}
	assert.Empty(t, note(live))
	assert.Empty(t, note(report.Worktree{Bare: true}))

	assert.Equal(t, "no commit",
		note(report.Worktree{Branch: "wip"}))
	assert.Equal(t, "prunable",
		note(report.Worktree{HasCommit: true, Prunable: true}))
	assert.Equal(t, "locked",
		note(report.Worktree{HasCommit: true, Locked: true}))
}

func TestPluralAgreesWithItsCount(t *testing.T) {
	assert.Equal(t, "1 worktree", plural(1, "worktree"))
	assert.Equal(t, "0 worktrees", plural(0, "worktree"))
	assert.Equal(t, "2 worktrees", plural(2, "worktree"))
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
	// A plain init writes only frit's own config. proto.md is mdsmith
	// machinery, gated behind --mdsmith, so a repo never seeds a file it
	// cannot keep correct without mdsmith.
	_, statErr := os.Stat(filepath.Join(repo, "plan", "proto.md"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestInitMdsmithScaffoldsTheMachinery pins that the flag adds the three
// files a plain init leaves out: the .mdsmith.yml config, the proto.md
// schema, and the PLAN.md catalog seed.
func TestInitMdsmithScaffoldsTheMachinery(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"init", "--mdsmith", repo}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	proto, err := os.ReadFile(filepath.Join(repo, "plan", "proto.md"))
	require.NoError(t, err)
	assert.Contains(t, string(proto), "<?require")
	cfg, err := os.ReadFile(filepath.Join(repo, ".mdsmith.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(cfg), "schema: plan/proto.md")
	index, err := os.ReadFile(filepath.Join(repo, "PLAN.md"))
	require.NoError(t, err)
	assert.Contains(t, string(index), "<?catalog")
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

func TestSkillsWritesTheBundledSkillsIntoARepository(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"skills", repo}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "SKILL.md")
	body, err := os.ReadFile(filepath.Join(
		repo, ".claude", "skills", "plan-pick", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "frit pick")
}

func TestSkillsRefusesToClobberWithoutForce(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var out, errb bytes.Buffer
	require.Equal(t, 0, run([]string{"skills", repo}, &out, &errb))

	out.Reset()
	errb.Reset()
	code := run([]string{"skills", repo}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "already exists")
}

func TestSkillsJSONCarriesTheWrittenPaths(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"skills", repo, "--json"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "\"command\": \"skills\"")
	assert.Contains(t, out.String(), "SKILL.md")
}

// TestSkillsHelpNamesTheViaFlag: --help is where a reader learns
// --via exists and sees an invocation to actually pass, rather than
// discovering the seam only by reading source.
func TestSkillsHelpNamesTheViaFlag(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"skills", "--help"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "--via")
	// Kong reflows the description text at the terminal width, so the
	// example phrase can land split across a wrapped line; collapse
	// whitespace before matching it as one run.
	flat := strings.Join(strings.Fields(got), " ")
	assert.Contains(t, flat, "mise exec -- frit")
	assert.Contains(t, flat, "go run ./cmd/frit")
}

func TestShowPrintsTheGoalReadFromTheBody(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	plan := "---\nid: 42\ntitle: Widget\nstatus: \"🔲\"\n---\n# Widget\n\n" +
		"## Goal\n\nMake the widget spin.\n"
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "plan"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "plan", "42_widget.md"), []byte(plan), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "add plan 42")
	var out, errb bytes.Buffer

	code := run([]string{"show", "42", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Goal: Make the widget spin.")
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

// claimBranch mints a claim marker and then one work commit on a
// branch named for a plan, and returns to main, leaving the branch
// unmerged and with no worktree. The marker is what makes the branch
// read as an actual hold rather than a bare name match (2608212203).
func claimBranch(t *testing.T, repo, branch string) {
	t.Helper()
	id := claimBranchPlanID(t, branch)
	git(t, repo, "checkout", "-q", "-b", branch)
	git(t, repo, "commit", "--allow-empty", "-q", "-m",
		fmt.Sprintf("plan %d: claim", id))
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "work.txt"), []byte("wip\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "work on "+branch)
	git(t, repo, "checkout", "-q", "main")
}

// claimBranchPlanID reads the plan id off the leading plan/<id>[-slug]
// segment of a hold branch name.
func claimBranchPlanID(t *testing.T, branch string) int64 {
	t.Helper()
	rest := strings.TrimPrefix(branch, "plan/")
	digits, _, _ := strings.Cut(rest, "-")
	id, err := strconv.ParseInt(digits, 10, 64)
	require.NoError(t, err, "branch %q must start with plan/<id>", branch)

	return id
}

// landPlan commits a plan file on the default branch with a given
// status, so a test can assert how frit reads a claim whose plan is
// already done there — the squash-merged case the ancestry filter
// cannot see.
func landPlan(t *testing.T, repo string, id int64, slug, status string) {
	t.Helper()
	dir := filepath.Join(repo, "plan")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	body := fmt.Sprintf(
		"---\nid: %d\ntitle: %s\nstatus: %q\n---\n# %s\n",
		id, slug, status, slug)
	name := fmt.Sprintf("%d_%s.md", id, slug)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, name), []byte(body), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "land plan "+slug)
}

// TestDeadSessionConfirmsAGoneAgentAtOnce: a held plan whose marker
// names a session herdr shows no live agent under is dead, no window
// consulted.
func TestDeadSessionConfirmsAGoneAgentAtOnce(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	rt := &runtime{git: gitwt.Exec}
	coord := fleet.Coord{Path: repo, Remote: "origin"}
	plan := discovery.Plan{ID: 7, Held: true, HoldTip: lease.Tip}
	panes := []herdr.Pane{{Session: "wOther:session", Agent: "claude"}}

	assert.True(t, deadSession(rt, coord, plan, panes))
}

// TestDeadSessionAnswersFalseForALiveAgent pins the baseline: a
// working agent found under the bound session is not dead.
func TestDeadSessionAnswersFalseForALiveAgent(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	rt := &runtime{git: gitwt.Exec}
	coord := fleet.Coord{Path: repo, Remote: "origin"}
	plan := discovery.Plan{ID: 7, Held: true, HoldTip: lease.Tip}
	panes := []herdr.Pane{{Session: "wS:p9", Agent: "claude"}}

	assert.False(t, deadSession(rt, coord, plan, panes))
}

// TestDeadSessionAnswersFalseForAnUnheldPlan: nothing to confirm dead
// when nobody holds the plan, so deadSession never reads a marker.
func TestDeadSessionAnswersFalseForAnUnheldPlan(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}

	assert.False(t, deadSession(
		rt, fleet.Coord{Path: "/r"}, discovery.Plan{ID: 7, Held: false}, nil))
}

// TestDeadSessionAnswersFalseForAnUnreadableMarker: an empty or
// unreachable HoldTip cannot name who to ask, so it falls back to the
// staleness window exactly as before this signal existed.
func TestDeadSessionAnswersFalseForAnUnreadableMarker(t *testing.T) {
	rt := &runtime{git: func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("bad object")
	}}

	assert.False(t, deadSession(
		rt, fleet.Coord{Path: "/r"},
		discovery.Plan{ID: 7, Held: true, HoldTip: "deadbeef"}, nil))
}

// TestObserveHoldsReadsHerdrOnceForManyHeldPlans: the pane list is read
// once per fleet gather and shared across every held plan's
// dead-session check, not once per plan — and an unreachable herdr
// leaves every plan's Dead at its zero value rather than misreading an
// empty pane list as everyone's session being gone.
func TestObserveHoldsReadsHerdrOnceForManyHeldPlans(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	atlas := claimableRepo(t, root, "atlas", 7, "Shader unit")
	orrery := claimableRepo(t, root, "orrery", 8, "Volumetrics")
	acquire := func(repo string, id int64, session string) {
		_, err := claim.Acquire(repo, claim.LeaseOptions{
			PlanID: id, Remote: "origin", Base: "origin/main",
			Holder: "elsewhere", Lane: "/lanes/x", Session: session,
		}, gitwt.Exec)
		require.NoError(t, err)
	}
	acquire(atlas, 7, "wA:p1")
	acquire(orrery, 8, "wB:p1")
	calls := 0
	countingHerdr := herdrReturning()
	rt := &runtime{git: gitwt.Exec, gitPipe: gitwt.ExecPipe,
		herdr: func(args ...string) ([]byte, error) {
			calls++
			return countingHerdr(args...)
		}}

	res, err := gatherFleet(&cli{Root: root}, rt)

	require.NoError(t, err)
	require.Len(t, res.Plans, 2)
	assert.Equal(t, 1, calls, "one List call serves every held plan")
	for _, p := range res.Plans {
		assert.True(t, p.Dead, "no pane at all: both bound sessions read as gone")
	}
}

// TestObserveHoldsLeavesDeadFalseWhenHerdrIsUnreachable: a failed List
// call must not be read as an empty-but-successful pane list — every
// held plan falls back to unknown, exactly as a per-plan SessionDead
// call would have answered.
func TestObserveHoldsLeavesDeadFalseWhenHerdrIsUnreachable(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	_, err := claim.Acquire(repo, claim.LeaseOptions{
		PlanID: 7, Remote: "origin", Base: "origin/main",
		Holder: "elsewhere", Lane: "/lanes/x", Session: "wA:p1",
	}, gitwt.Exec)
	require.NoError(t, err)
	rt := &runtime{git: gitwt.Exec, gitPipe: gitwt.ExecPipe,
		herdr: func(...string) ([]byte, error) {
			return nil, fmt.Errorf("dial unix .herdr.sock: no such file")
		}}

	res, err := gatherFleet(&cli{Root: root}, rt)

	require.NoError(t, err)
	require.Len(t, res.Plans, 1)
	assert.False(t, res.Plans[0].Dead)
}

// TestObserveHoldsKeepsAWindowANonFetchingPassCouldNotConfirmGone: a
// held plan whose work ref this pass never refreshed arrives with an
// empty HoldTip though it is still held elsewhere. A pass that fetched
// nothing (Summary.Fetched == 0) cannot tell that absence from a
// genuinely gone ref, so it must leave the accrued window standing
// rather than reset frit start's takeover clock to zero.
func TestObserveHoldsKeepsAWindowANonFetchingPassCouldNotConfirmGone(t *testing.T) {
	isolate(t)
	now := time.Now()
	path, err := observe.Path()
	require.NoError(t, err)
	key := observe.Key("atlas", 7)
	require.NoError(t, observe.Save(path, observe.State{
		key: discovery.Window{
			Tip: "tip-7", First: now.Add(-3 * time.Hour), Last: now, Samples: 9,
		},
	}))
	res := &fleet.Result{
		Plans:   []discovery.Plan{{Repo: "atlas", ID: 7, Held: true, HoldTip: ""}},
		Summary: fleet.Summary{Fetched: 0},
	}

	observeHolds(res, &runtime{git: gitwt.Exec}, now)

	got := observe.Load(path)
	win, ok := got[key]
	require.True(t, ok, "the window a non-fetching pass could not confirm gone survives")
	assert.Equal(t, 9, win.Samples, "its accrued samples are untouched")
	assert.Equal(t, now.Add(-3*time.Hour).Unix(), win.First.Unix(),
		"its span is not reset to zero")
}

// TestObserveHoldsPrunesAWindowAFetchingPassConfirmedGone: the same
// empty HoldTip on a pass that did refresh (Summary.Fetched > 0) is a
// ref confirmed gone — the window is dropped, so the store keeps only
// what this host still watches.
func TestObserveHoldsPrunesAWindowAFetchingPassConfirmedGone(t *testing.T) {
	isolate(t)
	now := time.Now()
	path, err := observe.Path()
	require.NoError(t, err)
	key := observe.Key("atlas", 7)
	require.NoError(t, observe.Save(path, observe.State{
		key: discovery.Window{
			Tip: "tip-7", First: now.Add(-3 * time.Hour), Last: now, Samples: 9,
		},
	}))
	res := &fleet.Result{
		Plans:   []discovery.Plan{{Repo: "atlas", ID: 7, Held: true, HoldTip: ""}},
		Summary: fleet.Summary{Fetched: 1},
	}

	observeHolds(res, &runtime{git: gitwt.Exec}, now)

	_, ok := observe.Load(path)[key]
	assert.False(t, ok, "a fetching pass that finds no ref drops the window")
}

// TestStaleHeldExcludesADeadSessionWithNoMaturedWindow: a bound
// session herdr confirms gone is desertedHeld's own cell, not
// staleHeld's — the two kinds never collide (2608212346).
func TestStaleHeldExcludesADeadSessionWithNoMaturedWindow(t *testing.T) {
	plans := []discovery.Plan{
		{Repo: "atlas", ID: 1, Held: true, Dead: true},
		{Repo: "atlas", ID: 2, Held: true, Stale: true},
		{Repo: "orrery", ID: 3, Held: true, Stale: true},
	}

	got := staleHeld(plans, "atlas")

	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].ID)
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

// TestOrphansNamesADecoratedHoldAsAMigrationCandidate: a legacy
// decorated hold still reads as a claim — the "claimed, no checkout"
// row stands — and is also named as a migration candidate toward the
// id-only ref the lease protocol writes.
func TestOrphansNamesADecoratedHoldAsAMigrationCandidate(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	var doc report.OrphansDoc

	stderr := emit(t, &doc, "orphans", "--root", root)

	assert.Empty(t, stderr)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Migratable, 1)
	m := doc.Repos[0].Migratable[0]
	assert.Equal(t, int64(2608142306), m.PlanID)
	assert.Equal(t, "plan/2608142306-fleet-index", m.From)
	assert.Equal(t, "plan/2608142306", m.To)

	var out, errb bytes.Buffer
	code := run([]string{"orphans", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "decorated hold, migrate")
	assert.Contains(t, out.String(), "plan/2608142306-fleet-index → plan/2608142306")
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

// TestOrphansIgnoresASquashMergedClaim is the squash-merge counterpart
// to the merged-ref filter: this repository squash-merges, so a landed
// plan's branch is no ancestor of the default branch and --merged never
// lists it. The plan is done on the default branch, so its lingering
// claim is landed work, not an abandoned lane.
func TestOrphansIgnoresASquashMergedClaim(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	landPlan(t, repo, 2608142306, "fleet-index", "✅")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no orphaned lanes")
	assert.NotContains(t, out.String(), "claimed, no checkout")
}

// TestOrphansReportsAClaimDoneOnlyOnItsBranch is the guard against the
// squash-merge fix overreaching: the plan-phase workflow flips status to
// ✅ on the feature branch before the work merges, so a plan done only
// there — absent from the default branch — has a live claim, and orphans
// must still report it unstaffed rather than read it as landed. The
// branch carries a real claim marker, the same live-hold verdict every
// other consumer of a plan's holds now reads — a bare name match is
// not itself a claim (2608212203).
func TestOrphansReportsAClaimDoneOnlyOnItsBranch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	git(t, repo, "checkout", "-q", "-b", branch)
	git(t, repo, "commit", "--allow-empty", "-q", "-m",
		"plan 2608142306: claim")
	landPlan(t, repo, 2608142306, "fleet-index", "✅")
	git(t, repo, "checkout", "-q", "main")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed, no checkout",
		"a plan done only on its branch has not landed; the claim is live")
}

// TestOrphansReportsACheckoutStrandedOnALandedBranch is the counterpart
// to the merged-ref filter: once the branch lands, the ref stops reading
// as a claim, but a worktree still standing on it is stranded work the
// report must name rather than silently keep.
func TestOrphansReportsACheckoutStrandedOnALandedBranch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	git(t, repo, "worktree", "add", "-q", "-b", branch,
		filepath.Join(root, "atlas-landed"))
	lane := filepath.Join(root, "atlas-landed")
	require.NoError(t, os.WriteFile(
		filepath.Join(lane, "work.txt"), []byte("done\n"), 0o600))
	git(t, lane, "add", "-A")
	git(t, lane, "commit", "-q", "-m", "work on "+branch)
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land", branch)
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "landed, still checked out")
	assert.Contains(t, out.String(), "atlas-landed")
	assert.NotContains(t, out.String(), "claimed, no checkout")
}

// TestOrphansNamesAReleasedLanesLeftoverWorktree is issue 118's own
// shape: Release deletes nothing, so a freed lane keeps its branch and
// worktree. Before the live-hold verdict became a required Build
// input, this leftover was neither Unstaffed nor Stranded and fell
// through every rule; now the released ref drops out of Holds and the
// worktree strands, the same way a landed branch's leftover already
// does.
func TestOrphansNamesAReleasedLanesLeftoverWorktree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	branch := "plan/2608142306-fleet-index"
	lane := filepath.Join(root, "atlas-released")
	git(t, repo, "worktree", "add", "-q", "-b", branch, lane)
	git(t, lane, "commit", "--allow-empty", "-q", "-m",
		"plan 2608142306: claim")
	require.NoError(t, os.WriteFile(
		filepath.Join(lane, "work.txt"), []byte("wip\n"), 0o600))
	git(t, lane, "add", "-A")
	git(t, lane, "commit", "-q", "-m", "work on "+branch)
	git(t, lane, "commit", "--allow-empty", "-q", "-m",
		"plan 2608142306: release")
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "landed, still checked out")
	assert.Contains(t, out.String(), "atlas-released")
	assert.NotContains(t, out.String(), "claimed, no checkout")
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

// TestOrphansListsALeftoverRescueRef: a rescue ref found before anyone
// triggers the blocked park it stands for is reported on its own —
// the "only finding is a rescue ref" case that forces an otherwise
// clean-looking repository to still render.
func TestOrphansListsALeftoverRescueRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	tip, err := gitCapture(t, repo, "rev-parse", "HEAD")
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "push", "-q", "origin",
		tip+":refs/frit/rescue/7/box-a")
	require.NoError(t, err)
	var doc report.OrphansDoc

	stderr := emit(t, &doc, "orphans", "--root", root)

	assert.Empty(t, stderr)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Rescued, 1)
	r := doc.Repos[0].Rescued[0]
	assert.Equal(t, int64(7), r.PlanID)
	assert.Equal(t, "", r.State, "an open plan's rescue ref carries no state")
	assert.Equal(t, []string{"refs/frit/rescue/7/box-a"}, r.Refs)

	var out, errb bytes.Buffer
	code := run([]string{"orphans", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "rescued")
	assert.Contains(t, out.String(), "plan 7")
}

// TestOrphansLabelsALandedRescueRefAsLanded: a rescue ref left behind
// by a plan that has since landed reads as landed, not merely open.
func TestOrphansLabelsALandedRescueRefAsLanded(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	landPlan(t, repo, 7, "shader-unit", "✅")
	origin := filepath.Join(t.TempDir(), "atlas-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")
	tip, err := gitCapture(t, repo, "rev-parse", "HEAD")
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "push", "-q", "origin",
		tip+":refs/frit/rescue/7/box-a")
	require.NoError(t, err)
	var doc report.OrphansDoc

	emit(t, &doc, "orphans", "--root", root)

	require.Len(t, doc.Repos[0].Rescued, 1)
	assert.Equal(t, "✅", doc.Repos[0].Rescued[0].State)

	var out, errb bytes.Buffer
	code := run([]string{"orphans", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "landed")
}

// TestOrphansNeverLabelsASupersededRescueRefAsLanded: LandedIDs marks
// both ✅ and ⛔ ids, so the report must tell them apart itself rather
// than call a superseded plan's leftover park landed.
func TestOrphansNeverLabelsASupersededRescueRefAsLanded(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	landPlan(t, repo, 7, "shader-unit", "⛔")
	origin := filepath.Join(t.TempDir(), "atlas-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")
	tip, err := gitCapture(t, repo, "rev-parse", "HEAD")
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "push", "-q", "origin",
		tip+":refs/frit/rescue/7/box-a")
	require.NoError(t, err)
	var doc report.OrphansDoc

	emit(t, &doc, "orphans", "--root", root)

	require.Len(t, doc.Repos[0].Rescued, 1)
	assert.Equal(t, "⛔", doc.Repos[0].Rescued[0].State)

	var out, errb bytes.Buffer
	code := run([]string{"orphans", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "superseded")
	assert.NotContains(t, out.String(), "landed")
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

// TestStaleFailsWhenTheRootCannotBeWalked: a root that cannot be
// walked fails before stale ever gathers presence.
func TestStaleFailsWhenTheRootCannotBeWalked(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"stale", "--root",
		filepath.Join(t.TempDir(), "missing")}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// TestStaleNamesAnUnreadHostAsAProblem: a configured host that cannot
// be read travels as a problem, the same way every other read verb
// carries livePresence's own host failures.
func TestStaleNamesAnUnreadHostAsAProblem(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var doc report.StaleDoc

	emit(t, &doc, "stale", "--root", root, "--hosts", "box")

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "host box", doc.Problems[0].Repo)
}

// TestStaleNamesARepositoryWhoseRefTimesCannotBeRead: staleCmd.Run is
// called directly, bypassing the CLI's own gitwt.Exec wiring, so a
// runner that fails only for-each-ref can be injected.
func TestStaleNamesARepositoryWhoseRefTimesCannotBeRead(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out bytes.Buffer
	rt := &runtime{
		git: func(dir string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "for-each-ref" {
				return nil, errors.New("boom")
			}

			return gitwt.Exec(dir, args...)
		},
		herdr: herdrReturning(), stdout: &out,
	}
	c := &cli{Root: root, JSON: true}

	err := (&staleCmd{Days: 30}).Run(c, rt)

	require.NoError(t, err)
	var doc report.StaleDoc
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestRepoLanesSurfacesAnUnreadableConfig: repoLanes' own repocfg.Load
// error, direct-called against a broken .frit.yml.
func TestRepoLanesSurfacesAnUnreadableConfig(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	rt := &runtime{git: gitwt.Exec}

	_, _, err := repoLanes(discover.Repo{Path: repo, Name: "atlas"}, rt)

	assert.Error(t, err)
}

// TestRepoLanesSurfacesAnUncompilableHoldPattern: repoLanes' own
// cfg.Compiled error, direct-called against a syntactically invalid
// glob.
func TestRepoLanesSurfacesAnUncompilableHoldPattern(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\"plan/[\"]\n"), 0o600))
	rt := &runtime{git: gitwt.Exec}

	_, _, err := repoLanes(discover.Repo{Path: repo, Name: "atlas"}, rt)

	assert.Error(t, err)
}

// TestRepoLanesSurfacesAnUnreadableRefList: repoLanes' own
// gitobj.Refs error, direct-called against a path with no git dir at
// all.
func TestRepoLanesSurfacesAnUnreadableRefList(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}

	_, _, err := repoLanes(
		discover.Repo{Path: filepath.Join(t.TempDir(), "missing")}, rt)

	assert.Error(t, err)
}

// TestRepoLanesSurfacesAnUnreadableMergedRefList: repoLanes' own
// gitobj.MergedRefs error, direct-called with a stub runner that fails
// only the merged for-each-ref call, distinct from Refs' own plain
// one.
func TestRepoLanesSurfacesAnUnreadableMergedRefList(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	rt := &runtime{git: func(dir string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "for-each-ref" && args[1] == "--merged" {
			return nil, errors.New("boom")
		}

		return gitwt.Exec(dir, args...)
	}}

	_, _, err := repoLanes(discover.Repo{Path: repo, Name: "atlas"}, rt)

	assert.Error(t, err)
}

// TestRepoLanesSurfacesAnUnreadablePlanCollection: repoLanes' own
// plans.Collect error, direct-called with a stub gitPipe that fails.
func TestRepoLanesSurfacesAnUnreadablePlanCollection(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	rt := &runtime{
		git: gitwt.Exec,
		gitPipe: func(string, []byte, ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}

	_, _, err := repoLanes(discover.Repo{Path: repo, Name: "atlas"}, rt)

	assert.Error(t, err)
}

// TestLaneOfSkipsAHoldWhoseRefIsAbsent: laneOf's own guard, called
// directly against a lane whose hold ref never appears in the
// repository's own ref set.
func TestLaneOfSkipsAHoldWhoseRefIsAbsent(t *testing.T) {
	built := []lanes.Lane{{PlanID: 7,
		Holds: []lanes.Hold{{Ref: "refs/heads/plan/7"}}}}

	out := laneOf("/repo", "origin", nil, built, gitwt.Exec)

	assert.Empty(t, out)
}

// TestOrphansFailsWhenTheRootCannotBeWalked: a root that cannot be
// walked fails before orphans ever gathers the fleet.
func TestOrphansFailsWhenTheRootCannotBeWalked(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root",
		filepath.Join(t.TempDir(), "missing")}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// TestOrphansNamesAProblemWhenTheRescueSweepFails: a repository whose
// origin cannot be read for its rescue-ref sweep is named as a
// problem, the same as any other unreadable step.
func TestOrphansNamesAProblemWhenTheRescueSweepFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	git(t, repo, "remote", "set-url", "origin", "/nonexistent")
	var doc report.OrphansDoc

	emit(t, &doc, "orphans", "--root", root)

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestBoardUnprovenIsFalseWithoutACoordinate: boardUnproven's own
// guard, called directly against a fleet result withholding a
// coordinate for the plan's repository.
func TestBoardUnprovenIsFalseWithoutACoordinate(t *testing.T) {
	res := fleet.Result{Coords: map[string]fleet.Coord{}}
	p := discovery.Plan{Repo: "atlas", Held: true}

	got := boardUnproven(&runtime{}, res, p, map[string]map[int64]bool{})

	assert.False(t, got)
}

// TestTokenlessIDsIsNilWhenTheWorktreeListCannotBeRead: tokenlessIDs'
// own gitwt.List error, called directly.
func TestTokenlessIDsIsNilWhenTheWorktreeListCannotBeRead(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}

	got := tokenlessIDs(rt, filepath.Join(t.TempDir(), "missing"))

	assert.Nil(t, got)
}

// TestLocalPanesIsNilWithoutHerdr: localPanes' own nil-herdr guard,
// called directly.
func TestLocalPanesIsNilWithoutHerdr(t *testing.T) {
	assert.Nil(t, localPanes(&runtime{}))
}

// TestLocalPanesIsNilWhenHerdrErrors: localPanes' own herdr.List
// error, called directly.
func TestLocalPanesIsNilWhenHerdrErrors(t *testing.T) {
	rt := &runtime{herdr: func(...string) ([]byte, error) {
		return nil, errors.New("boom")
	}}

	assert.Nil(t, localPanes(rt))
}

// TestRescuedHeldSurfacesAnUnreadableSweep: rescuedHeld's own
// claim.AllRescueRefs error, called directly against an origin that
// exists but cannot be reached.
func TestRescuedHeldSurfacesAnUnreadableSweep(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	git(t, repo, "remote", "set-url", "origin", "/nonexistent")
	rt := &runtime{git: gitwt.Exec}

	_, err := rescuedHeld(rt, fleet.Coord{Path: repo, Remote: "origin"},
		nil, "atlas")

	assert.Error(t, err)
}

// TestPrintOrphansRendersEveryKind: printOrphans is called directly
// against a hand-built doc, asserting each row's own rendering —
// including the prunable, foreign and deserted kinds no CLI-level
// fixture happens to combine in one repository.
func TestPrintOrphansRendersEveryKind(t *testing.T) {
	doc := &report.OrphansDoc{Repos: []report.OrphanRepo{{
		Name: "atlas",
		Prunable: []report.Worktree{
			{Name: "atlas-gone", PruneReason: "worktree missing"}},
		Foreign: []report.ForeignCheckout{{PlanID: 9,
			Worktree: report.Worktree{Name: "atlas-foreign", Branch: "plan/9"}}},
		Deserted: []report.Deserted{{PlanID: 3, Branch: "plan/3"}},
	}}}
	var out bytes.Buffer

	printOrphans(&out, doc)

	got := out.String()
	assert.Contains(t, got, "prunable")
	assert.Contains(t, got, "worktree missing")
	assert.Contains(t, got, "foreign checkout")
	assert.Contains(t, got, "atlas-foreign")
	assert.Contains(t, got, "deserted, session gone")
	assert.Contains(t, got, "plan 3")
}

// TestGitForHostReturnsTheLocalRunnerForAnEmptyHost: gitForHost's own
// empty-host branch, called directly.
func TestGitForHostReturnsTheLocalRunnerForAnEmptyHost(t *testing.T) {
	called := false
	local := func(string, ...string) ([]byte, error) {
		called = true

		return nil, nil
	}

	_, _ = gitForHost(local)("")("/repo", "status")

	assert.True(t, called)
}

// TestRemoteGitRunsGitOverSSH: remoteGit resolves "ssh" through $PATH
// at run time, the same mechanism phase 6 proved for herdr.Exec's
// hardcoded "herdr" — a throwaway script drops the fake host argument
// and runs the rest as a real local git command.
func TestRemoteGitRunsGitOverSSH(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"),
		[]byte("#!/bin/sh\nshift\nexec \"$@\"\n"), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	runner := gitForHost(gitwt.Exec)("box")
	out, err := runner(repo, "rev-parse", "--show-toplevel")

	require.NoError(t, err)
	assert.Contains(t, string(out), filepath.Base(repo))
}

// TestRepoLabelNamesNoRepo: repoLabel's own empty-string branch,
// called directly.
func TestRepoLabelNamesNoRepo(t *testing.T) {
	assert.Equal(t, "(no repo)", repoLabel(""))
}

// TestInitMdsmithSurfacesAnUnreadableConfigAfterWriting: --force skips
// the exists check, so Init's own write succeeds even over a
// write-only file, but the immediately following repocfg.Load fails
// to read it back — the chmod idiom internal/observe/observe_test.go
// already uses for a read-only-directory write failure, applied here
// to a write-only file instead.
func TestInitMdsmithSurfacesAnUnreadableConfigAfterWriting(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("plan-dir: plan\n"), 0o200))
	var out, errb bytes.Buffer

	code := run([]string{"init", "--mdsmith", "--force", repo}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// TestInitMdsmithRefusesToClobberAnExistingMdsmithConfig: force=false
// refuses to overwrite a .mdsmith.yml that already exists.
func TestInitMdsmithRefusesToClobberAnExistingMdsmithConfig(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".mdsmith.yml"),
		[]byte("x"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"init", "--mdsmith", repo}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "already exists")
}

// TestInitMdsmithRefusesToClobberAnExistingProto: force=false refuses
// to overwrite a plan/proto.md that already exists.
func TestInitMdsmithRefusesToClobberAnExistingProto(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "plan"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "plan", "proto.md"),
		[]byte("x"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"init", "--mdsmith", repo}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "already exists")
}

// TestInitMdsmithRefusesToClobberAnExistingPlanIndex: force=false
// refuses to overwrite a PLAN.md that already exists.
func TestInitMdsmithRefusesToClobberAnExistingPlanIndex(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "PLAN.md"),
		[]byte("x"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"init", "--mdsmith", repo}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "already exists")
}

// TestPlansDirOverrideWins: plansCmd's own planDir method, called
// directly with an explicit --dir override.
func TestPlansDirOverrideWins(t *testing.T) {
	p := &plansCmd{Dir: "docs/plans"}

	dir, err := p.planDir("/repo")

	require.NoError(t, err)
	assert.Equal(t, "docs/plans", dir)
}

// TestPlansDirSurfacesAnUnreadableConfig: plansCmd's own planDir
// method, called directly against a broken .frit.yml, with no --dir
// override to short-circuit it.
func TestPlansDirSurfacesAnUnreadableConfig(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	p := &plansCmd{}

	_, err := p.planDir(repo)

	assert.Error(t, err)
}

// TestPlansFailsWhenTheRootCannotBeWalked: a root that cannot be
// walked fails before plans ever reads a repository.
func TestPlansFailsWhenTheRootCannotBeWalked(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"plans", "--root",
		filepath.Join(t.TempDir(), "missing")}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// TestPlansNamesARepositoryWithABrokenConfig: a broken .frit.yml fails
// that one repository's own planDir read; plans steps over it and
// names it as a problem.
func TestPlansNamesARepositoryWithABrokenConfig(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var doc report.PlansDoc

	emit(t, &doc, "plans", "--root", root)

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestPlansNamesARepositoryWhoseCollectionFails: plans.Collect's own
// error surfaces as a problem too — a stub gitPipe forces it directly,
// bypassing the CLI's own real gitPipe wiring.
func TestPlansNamesARepositoryWhoseCollectionFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out bytes.Buffer
	rt := &runtime{
		git: gitwt.Exec,
		gitPipe: func(string, []byte, ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
		stdout: &out,
	}
	c := &cli{Root: root, JSON: true}

	err := (&plansCmd{}).Run(c, rt)

	require.NoError(t, err)
	var doc report.PlansDoc
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestPlansDetailListsEveryPlanUnderARepository: --detail lists each
// plan under its repository, not just the summary count.
func TestPlansDetailListsEveryPlanUnderARepository(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Shader unit", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"plans", "--detail", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Shader unit")
}

// TestHostnameFallsBackToLocalhost: hostname's own os.Hostname failure
// branch, driven through the osHostname seam.
func TestHostnameFallsBackToLocalhost(t *testing.T) {
	prev := osHostname
	osHostname = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { osHostname = prev })

	assert.Equal(t, "localhost", hostname())
}

// TestCarryHostProblemsAddsEachOne: carryHostProblems' own loop,
// called directly against any problemAdder — a report.OrphansDoc
// satisfies the one-method interface.
func TestCarryHostProblemsAddsEachOne(t *testing.T) {
	doc := &report.OrphansDoc{}

	carryHostProblems(doc, []hostProblem{{name: "box", err: errors.New("boom")}})

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "box", doc.Problems[0].Repo)
}
