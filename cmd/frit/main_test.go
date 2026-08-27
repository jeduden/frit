package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
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

// boardPlanByID returns the board's row for a plan id, or nil when the
// plan is off the board.
func boardPlanByID(doc report.BoardDoc, id int64) *report.BoardPlan {
	for i := range doc.Plans {
		if doc.Plans[i].ID == id {
			return &doc.Plans[i]
		}
	}

	return nil
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
// must still report it unstaffed rather than read it as landed.
func TestOrphansReportsAClaimDoneOnlyOnItsBranch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "checkout", "-q", "-b", "plan/2608142306-fleet-index")
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
