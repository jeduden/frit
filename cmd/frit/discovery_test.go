package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commitPlan writes a plan file on the current branch and commits it.
// deps is the depends-on list, empty for none; body is appended after
// the front matter so a test can add phase sections.
func commitPlan(
	t *testing.T, repo string, id int, status, title string,
	deps []int, body string,
) {
	t.Helper()
	writePlanFile(t, repo, id, status, title, deps, "", body)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", fmt.Sprintf("plan %d", id))
}

// writePlanFile lays down one plan file with full front matter, without
// committing it. phases is a raw YAML block for the phases: key, empty
// for none.
func writePlanFile(
	t *testing.T, repo string, id int, status, title string,
	deps []int, phases, body string,
) {
	t.Helper()
	path := filepath.Join(repo, "plan", fmt.Sprintf("%d_%s.md", id,
		slugify(title)))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))

	var fm strings.Builder
	fmt.Fprintf(&fm, "---\nid: %d\ntitle: %s\nstatus: %q\n", id, title,
		status)
	if len(deps) > 0 {
		parts := make([]string, len(deps))
		for i, d := range deps {
			parts[i] = fmt.Sprint(d)
		}
		fmt.Fprintf(&fm, "depends-on: [%s]\n", strings.Join(parts, ", "))
	}
	if phases != "" {
		fm.WriteString(phases)
	}
	fm.WriteString("---\n# " + title + "\n")
	if body != "" {
		fm.WriteString("\n" + body + "\n")
	}

	require.NoError(t, os.WriteFile(path, []byte(fm.String()), 0o600))
}

// slugify makes a filename-safe slug from a title.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}

		return '-'
	}, s)

	return strings.Trim(s, "-")
}

// TestDiscoveryCarriesABrokenRepoAsAProblem is the JSON contract for
// the discovery verbs: a repository frit could not read travels in the
// document, nothing is written beside it, and the readable plans still
// come back.
func TestDiscoveryCarriesABrokenRepoAsAProblem(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	good := initRepo(t, root, "atlas")
	commitPlan(t, good, 1, "🔲", "Readable", nil, "")
	broken := initRepo(t, root, "busted")
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var doc report.ReadyDoc

	stderr := emit(t, &doc, "ready", "--root", root)

	assert.Empty(t, stderr, "under --json nothing goes to stderr")
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "busted", doc.Problems[0].Repo)
	assert.NotEmpty(t, doc.Problems[0].Message)
	require.Len(t, doc.Plans, 1, "the readable repo is still answered")
	assert.Equal(t, int64(1), doc.Plans[0].ID)
}

// TestDiscoveryProblemGoesToStderrInTheTable is the same failure in the
// other rendering: the table stays on stdout, the failure beside it.
func TestDiscoveryProblemGoesToStderrInTheTable(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	broken := initRepo(t, root, "busted")
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"find", "anything", "--root", root}, &out, &errb)

	require.Equal(t, 0, code)
	assert.Contains(t, out.String(), "no plan matches")
	assert.Contains(t, errb.String(), "frit: busted:")
}

// commitNonPlan drops a markdown file with no front matter into the
// plan directory — a PLAN.md index or a stray note, the kind of file a
// real plan directory keeps beside its plans.
func commitNonPlan(t *testing.T, repo, name string) {
	t.Helper()
	path := filepath.Join(repo, "plan", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path,
		[]byte("# Just a heading\n\nNo front matter here.\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "a non-plan "+name)
}

// TestNotAPlanIsHiddenByDefaultAndShownWithAll: a front-matterless file
// in the plan directory is noise, held back from the report unless
// --all asks for it. A genuine plan is unaffected.
func TestNotAPlanIsHiddenByDefaultAndShownWithAll(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Real plan", nil, "")
	commitNonPlan(t, repo, "notes.md")

	var quiet, errb bytes.Buffer
	require.Equal(t, 0, run([]string{"find", "real", "--root", root},
		&quiet, &errb))
	assert.NotContains(t, errb.String(), "no front matter",
		"a non-plan is held back by default")
	assert.Contains(t, quiet.String(), "Real plan")

	var out, errb2 bytes.Buffer
	require.Equal(t, 0, run([]string{"find", "real", "--all", "--root", root},
		&out, &errb2))
	assert.Contains(t, errb2.String(), "no front matter",
		"--all surfaces the non-plan")
	assert.Contains(t, errb2.String(), "notes.md")
}

// TestPlansHidesNonPlansUntilAll: frit plans reads the same directory
// and applies the same rule down its own problem path.
func TestPlansHidesNonPlansUntilAll(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Real plan", nil, "")
	commitNonPlan(t, repo, "PLAN.md")

	var out, errb bytes.Buffer
	require.Equal(t, 0, run([]string{"plans", "--root", root}, &out, &errb))
	assert.NotContains(t, errb.String(), "no front matter")

	out.Reset()
	errb.Reset()
	require.Equal(t, 0,
		run([]string{"plans", "--all", "--root", root}, &out, &errb))
	assert.Contains(t, errb.String(), "no front matter")
}

// TestNotAPlanIsAbsentFromJSONUntilAll: the same rule holds in the
// document, so a consumer is not handed the noise either until it asks.
func TestNotAPlanIsAbsentFromJSONUntilAll(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Real plan", nil, "")
	commitNonPlan(t, repo, "PLAN.md")

	var plain report.ReadyDoc
	emit(t, &plain, "ready", "--root", root)
	assert.Empty(t, plain.Problems, "no non-plan noise by default")
	require.Len(t, plain.Plans, 1)

	var all report.ReadyDoc
	emit(t, &all, "ready", "--all", "--root", root)
	require.Len(t, all.Problems, 1)
	assert.Contains(t, all.Problems[0].Message, "no front matter")
}

// TestBoardShowsUnfinishedWithHolderAndAgent is the whole point of the
// board: an in-progress plan, the lane that holds it, the machine, and
// the agent live on that lane.
func TestBoardShowsUnfinishedWithHolderAndAgent(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 100, "🔳", "Underway", nil, "")

	// A worktree on the hold branch, one commit ahead so the claim is
	// live rather than merged. The claim marker beneath the wip commit
	// is what makes this an actual hold, not merely a name match
	// (2608212203).
	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-underway", wt)
	git(t, wt, "commit", "--allow-empty", "-q", "-m", "plan 100: claim")
	require.NoError(t, os.WriteFile(
		filepath.Join(wt, "work.txt"), []byte("wip\n"), 0o600))
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "wip")

	withHerdr(t, herdrReturning(map[string]any{
		"agent":                   "claude",
		"agent_status":            "working",
		"cwd":                     wt,
		"pane_id":                 "w1:p1",
		"terminal_title_stripped": "on it",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"board", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Underway")
	assert.Contains(t, got, "underway",
		"the holding lane, its redundant plan/<id>- prefix trimmed")
	assert.Contains(t, got, "claude", "the agent on that lane")
	assert.Contains(t, got, "working")
}

// TestBoardWipHidesNotStarted: --wip keeps only what is under way.
func TestBoardWipHidesNotStarted(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔳", "Moving", nil, "")
	commitPlan(t, repo, 2, "🔲", "Not begun", nil, "")
	withHerdr(t, herdrReturning())

	var full, errb bytes.Buffer
	require.Equal(t, 0, run([]string{"board", "--root", root}, &full, &errb))
	assert.Contains(t, full.String(), "Not begun", "default carries both")

	var wip, errb2 bytes.Buffer
	require.Equal(t, 0,
		run([]string{"board", "--wip", "--root", root}, &wip, &errb2))
	assert.Contains(t, wip.String(), "Moving")
	assert.NotContains(t, wip.String(), "Not begun", "--wip drops 🔲")
}

// TestBoardMarksAgentUnknownWithoutHerdr: a missing socket leaves the
// git board standing with the agent column an honest unknown.
func TestBoardMarksAgentUnknownWithoutHerdr(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔳", "Moving", nil, "")
	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: no such file")
	})
	var out, errb bytes.Buffer

	code := run([]string{"board", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Moving")
	assert.Contains(t, out.String(), "?", "agent state is unknown, not absent")
}

func TestBoardIsQuietWhenNothingIsOutstanding(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "All done", nil, "")
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"board", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing outstanding")
}

func TestBoardEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 100, "🔳", "Underway", nil, "")
	claimBranch(t, repo, "plan/100-underway")
	withHerdr(t, herdrReturning())
	var doc report.BoardDoc

	emit(t, &doc, "board", "--root", root)

	assert.Equal(t, "board", doc.Command)
	assert.True(t, doc.Presence)
	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(100), doc.Plans[0].ID)
	assert.True(t, doc.Plans[0].Held)
	assert.Equal(t, []string{"plan/100-underway"}, doc.Plans[0].Holds)
	assert.NotEmpty(t, doc.Plans[0].Host)
}

// TestBoardReportsAMaturedHoldsAge: the board document carries the
// same stale flag and age ready and pick already answer with, so a
// consumer of the board's JSON sees a takeover candidate without a
// second read.
func TestBoardReportsAMaturedHoldsAge(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Underway")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	withHerdr(t, herdrReturning())
	var doc report.BoardDoc

	emit(t, &doc, "board", "--root", root)

	require.Len(t, doc.Plans, 1)
	assert.True(t, doc.Plans[0].Stale)
	assert.Greater(t, doc.Plans[0].StaleSeconds, int64(0))
}

// TestOrphansReportsAMaturedHold: the held-stale cell of the
// verb-state table — orphans names a takeover candidate beside its
// other kinds, without a second `board` read to spot it.
func TestOrphansReportsAMaturedHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Underway")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "stale")
	assert.Contains(t, out.String(), "plan 7")
}

// TestBoardSortByID orders the board oldest-first, and --reverse flips
// it, over the command instead of its default status order.
func TestBoardSortByID(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 300, "🔳", "Newer plan", nil, "")
	commitPlan(t, repo, 100, "🔳", "Older plan", nil, "")
	withHerdr(t, herdrReturning())

	var out, errb bytes.Buffer
	require.Equal(t, 0,
		run([]string{"board", "--sort", "id", "--root", root}, &out, &errb),
		errb.String())
	assert.Less(t, strings.Index(out.String(), "Older"),
		strings.Index(out.String(), "Newer"), "oldest id first")

	var rev, errb2 bytes.Buffer
	require.Equal(t, 0, run(
		[]string{"board", "--sort", "id", "--reverse", "--root", root},
		&rev, &errb2))
	assert.Less(t, strings.Index(rev.String(), "Newer"),
		strings.Index(rev.String(), "Older"), "--reverse: newest first")
}

// TestFindSortByRepoGroups: --sort reaches the plan lists too.
func TestFindSortByRepoGroups(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	atlas := initRepo(t, root, "atlas")
	commitPlan(t, atlas, 1, "🔲", "Orbit control", nil, "")
	orrery := initRepo(t, root, "orrery")
	commitPlan(t, orrery, 2, "🔲", "Orbit view", nil, "")
	var out, errb bytes.Buffer

	require.Equal(t, 0, run(
		[]string{"find", "orbit", "--sort", "repo", "--root", root},
		&out, &errb), errb.String())
	assert.Less(t, strings.Index(out.String(), "atlas"),
		strings.Index(out.String(), "orrery"))
}

func TestSortRejectsAnUnknownKey(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--sort", "bogus", "--root", root},
		&out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "unknown sort")
}

// phasesBlock builds a phases: front-matter block, one entry per given
// status, numbered from one.
func phasesBlock(statuses ...string) string {
	var b strings.Builder
	b.WriteString("phases:\n")
	for i, status := range statuses {
		fmt.Fprintf(&b, "  - { n: %d, title: 'Phase %d', status: %q }\n",
			i+1, i+1, status)
	}

	return b.String()
}

// TestReadyListsAPlanWithDepsDoneAndWithholdsAnUnmetOne is the flagship
// end to end: a plan whose every dependency is ✅ is startable, and one
// with an unmet edge is not.
func TestReadyListsAPlanWithDepsDoneAndWithholdsAnUnmetOne(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Foundation", nil, "")
	commitPlan(t, repo, 2, "✅", "Middle", nil, "")
	commitPlan(t, repo, 3, "🔲", "Startable now", []int{1, 2}, "")
	commitPlan(t, repo, 4, "🔳", "Underway", nil, "")
	commitPlan(t, repo, 5, "🔲", "Still blocked", []int{4}, "")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Startable now")
	assert.NotContains(t, got, "Still blocked",
		"a plan waiting on an unfinished plan is withheld")
}

// TestReadyWithholdsAHeldPlan: a not-started plan a lane already claims
// is not offered, even with no dependencies.
func TestReadyWithholdsAHeldPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 2608161809, "🔲", "Claimed already", nil, "")
	claimBranch(t, repo, "plan/2608161809-discovery")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing startable")
}

func TestReadyIsQuietWhenNothingIsStartable(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "All done here", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing startable")
}

func TestReadyEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Foundation", nil, "")
	commitPlan(t, repo, 3, "🔲", "Startable now", []int{1}, "")
	var doc report.ReadyDoc

	emit(t, &doc, "ready", "--root", root)

	assert.Equal(t, "ready", doc.Command)
	assert.NotEmpty(t, doc.Host)
	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(3), doc.Plans[0].ID)
	assert.Equal(t, "atlas", doc.Plans[0].Repo)
	assert.Equal(t, []int64{1}, doc.Plans[0].DependsOn)
}

// commitPhasedPlan writes and commits a plan carrying a phase ledger.
func commitPhasedPlan(
	t *testing.T, repo string, id int, status, title string,
	statuses ...string,
) {
	t.Helper()
	writePlanFile(t, repo, id, status, title, nil,
		phasesBlock(statuses...), "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", fmt.Sprintf("plan %d", id))
}

// TestNextReportsTheFirstOpenPhase is the rule end to end: next skips
// the done phases and stops at the first still open.
func TestNextReportsTheFirstOpenPhase(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "✅", "🔳", "🔲")
	var out, errb bytes.Buffer

	code := run([]string{"next", "100", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "phase 2")
	assert.NotContains(t, got, "phase 1", "a done phase is skipped")
}

// TestNextInfersThePlanFromTheCwd is the third selector form: standing
// in a worktree on a plan branch, next means the plan that worktree is
// working, with no id typed.
func TestNextInfersThePlanFromTheCwd(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "checkout", "-q", "-b", "plan/100-layered")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "✅", "🔲")
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"next", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "phase 2")
}

// TestNextFromCwdResolvesWithinTheRepo is the cwd selector's guard
// against a false ambiguity: two repos share plan id 100, but standing
// in one, next resolves that repo's plan 100 rather than refusing
// because the id also exists elsewhere.
func TestNextFromCwdResolvesWithinTheRepo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	atlas := initRepo(t, root, "atlas")
	git(t, atlas, "checkout", "-q", "-b", "plan/100-alpha")
	commitPhasedPlan(t, atlas, 100, "🔳", "Atlas hundred", "✅", "🔲")
	orrery := initRepo(t, root, "orrery")
	commitPlan(t, orrery, 100, "🔲", "Orrery hundred", nil, "")
	t.Chdir(atlas)
	var doc report.NextDoc

	emit(t, &doc, "next", "--root", root)

	assert.Equal(t, "atlas", doc.Plan.Repo,
		"the cwd pins the repo; the shared id is not ambiguous")
	assert.Equal(t, int64(100), doc.Plan.ID)
	assert.Equal(t, "2", doc.Phase.N)
}

// TestNextInsideItsOwnLaneReadsTheWorkingTreeCopy: a lane that closed
// phase 1 in its own worktree, but has not merged that commit, still
// reads as done there — next is not fooled by the stale open phase 1
// the default branch still carries.
func TestNextInsideItsOwnLaneReadsTheWorkingTreeCopy(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "🔳", "🔲")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	writePlanFile(t, wt, 100, "🔳", "Layered work", nil,
		phasesBlock("✅", "🔳"), "")
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "phase 1 done")
	t.Chdir(wt)
	var out, errb bytes.Buffer

	code := run([]string{"next", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "phase 2",
		"the lane's own copy already closed phase 1")
	assert.NotContains(t, got, "phase 1")
}

// TestNextOutsideTheLaneStillReadsTheDefaultBranch: a diverging lane
// exists, but standing outside it, next still reports the
// default-branch version, unchanged.
func TestNextOutsideTheLaneStillReadsTheDefaultBranch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "🔳", "🔲")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	writePlanFile(t, wt, 100, "🔳", "Layered work", nil,
		phasesBlock("✅", "🔳"), "")
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "phase 1 done")
	var out, errb bytes.Buffer

	code := run([]string{"next", "100", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "phase 1",
		"outside the lane, next still reads the default branch")
}

func TestNextEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "✅", "🔳")
	var doc report.NextDoc

	emit(t, &doc, "next", "layered", "--root", root)

	assert.Equal(t, "next", doc.Command)
	assert.True(t, doc.HasPhase)
	assert.Equal(t, "2", doc.Phase.N)
	assert.Equal(t, int64(100), doc.Plan.ID)
}

// executionTableBody builds an `## Execution` section with one row
// per given (phase, design, implement, gate) tuple, the shape phase 2
// parses tier and gate out of.
func executionTableBody(rows [][4]string) string {
	var b strings.Builder
	b.WriteString("## Execution\n\n")
	b.WriteString("| Phase | Design | Implement | Gate |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r[0], r[1], r[2], r[3])
	}

	return b.String()
}

// TestNextPrintsTheExecutionTierAndGate: the phase next points at
// carries the tier and gate its Execution row names — tier the more
// demanding of Design and Implement — in both the table and the JSON.
func TestNextPrintsTheExecutionTierAndGate(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	writePlanFile(t, repo, 100, "🔳", "Layered work", nil,
		phasesBlock("✅", "🔳"),
		executionTableBody([][4]string{
			{"1 first", "sonnet", "sonnet", "test one"},
			{"2 second", "sonnet", "opus", "test two"},
		}))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")
	var doc report.NextDoc

	emit(t, &doc, "next", "100", "--root", root)

	assert.True(t, doc.HasPhase)
	assert.Equal(t, "opus", doc.Phase.Tier,
		"the more demanding of sonnet and opus")
	assert.Equal(t, "test two", doc.Phase.Gate)
	assert.Empty(t, doc.Problems, "the phase carries a row")

	var out, errb bytes.Buffer
	code := run([]string{"next", "100", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "opus")
	assert.Contains(t, out.String(), "test two")
}

// TestNextReportsAMissingExecutionRowAsAProblem: the phase next
// points at has no row in the Execution table, so its tier and gate
// stay blank and the gap is said explicitly rather than rendered as
// if the plan asked for nothing.
func TestNextReportsAMissingExecutionRowAsAProblem(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	writePlanFile(t, repo, 100, "🔳", "Layered work", nil,
		phasesBlock("✅", "🔳"),
		executionTableBody([][4]string{
			{"1 first", "sonnet", "sonnet", "test one"},
		}))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")
	var doc report.NextDoc

	emit(t, &doc, "next", "100", "--root", root)

	assert.True(t, doc.HasPhase)
	assert.Empty(t, doc.Phase.Tier, "no row means no invented tier")
	assert.Empty(t, doc.Phase.Gate)
	require.Len(t, doc.Problems, 1)
	assert.Contains(t, doc.Problems[0].Message, "phase 2")
	assert.Contains(t, doc.Problems[0].Message, "no Execution row")
}

// TestNextDerivesTheLedgerFromHeadingsAndPrintsThePhaseBody covers
// frit's own plan convention: no front-matter `phases:` list, just
// `## Phase N` sections. next still finds a phase to point at — the
// first one, since section state carries no status to skip by — and
// prints the section's own prose, not just its title.
func TestNextDerivesTheLedgerFromHeadingsAndPrintsThePhaseBody(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	writePlanFile(t, repo, 100, "🔳", "Section tracked", nil, "",
		"## Phase 1: First sitting\n\n"+
			"Read the fixture and confirm it parses.\n\n"+
			"## Phase 2: Second sitting\n\nWire the second half.\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")
	var doc report.NextDoc

	emit(t, &doc, "next", "100", "--root", root)

	require.True(t, doc.HasPhase)
	assert.Equal(t, "1", doc.Phase.N)
	assert.Equal(t, "First sitting", doc.Phase.Title)
	assert.Empty(t, doc.Phase.Status, "section state carries no status")
	assert.Equal(t, "Read the fixture and confirm it parses.",
		doc.Phase.Body)

	var out, errb bytes.Buffer
	code := run([]string{"next", "100", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Read the fixture and confirm it parses.")
}

// TestShowByDefaultShowsOnlyBlockers: the default view walks the
// upstream chain but prunes the done edges, because a finished
// dependency blocks nothing.
func TestShowByDefaultShowsOnlyBlockers(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Bedrock", nil, "")
	commitPlan(t, repo, 2, "🔳", "Middle layer", []int{1}, "")
	commitPlan(t, repo, 3, "🔲", "Top", []int{2}, "")
	var out, errb bytes.Buffer

	code := run([]string{"show", "3", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Top")
	assert.Contains(t, got, "Middle layer")
	assert.NotContains(t, got, "Bedrock",
		"a done dependency blocks nothing and is pruned by default")
}

// TestShowAllShowsEveryDependency: --all (aliased --deps) keeps the
// done edges too, the whole upstream tree.
func TestShowAllShowsEveryDependency(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Bedrock", nil, "")
	commitPlan(t, repo, 2, "🔳", "Middle layer", []int{1}, "")
	commitPlan(t, repo, 3, "🔲", "Top", []int{2}, "")

	for _, flag := range []string{"--all", "--deps"} {
		var out, errb bytes.Buffer
		code := run([]string{"show", "3", flag, "--root", root}, &out, &errb)

		require.Equal(t, 0, code, errb.String())
		got := out.String()
		assert.Contains(t, got, "Bedrock", "%s keeps the done edges", flag)
		assert.Contains(t, got, "Middle layer")
	}
}

// TestShowSaysWhenNothingBlocksAPlan: a plan whose every dependency is
// done reads plainly, not as a bare line.
func TestShowSaysWhenNothingBlocksAPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Bedrock", nil, "")
	commitPlan(t, repo, 2, "🔲", "Rests on done work", []int{1}, "")
	var out, errb bytes.Buffer

	code := run([]string{"show", "2", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Rests on done work")
	assert.Contains(t, got, "nothing blocks it")
	assert.NotContains(t, got, "Bedrock")
}

func TestShowEmitsTheDependencyTree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Bedrock", nil, "")
	commitPlan(t, repo, 3, "🔲", "Top", []int{1}, "")
	var doc report.ShowDoc

	emit(t, &doc, "show", "3", "--root", root)

	assert.Equal(t, "show", doc.Command)
	assert.Equal(t, int64(3), doc.Tree.ID)
	require.Len(t, doc.Tree.Deps, 1)
	assert.Equal(t, int64(1), doc.Tree.Deps[0].ID)
	assert.True(t, doc.Tree.Deps[0].Found)
}

// TestShowInsideItsOwnLaneReadsTheWorkingTreeCopy mirrors Phase 1's
// case for next: a lane that has rewritten its own Goal, but not
// merged that commit, reads its own Goal, not the stale one the
// default branch still carries.
func TestShowInsideItsOwnLaneReadsTheWorkingTreeCopy(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 100, "🔳", "Layered work", nil,
		"## Goal\n\nShip the default-branch goal.\n")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	writePlanFile(t, wt, 100, "✅", "Layered work", nil, "",
		"## Goal\n\nShip the lane's own goal.\n")
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "rewrite the goal")
	t.Chdir(wt)
	var out, errb bytes.Buffer

	code := run([]string{"show", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Ship the lane's own goal.")
	assert.NotContains(t, got, "Ship the default-branch goal.")
}

// TestShowAppliesTheLaneOverrideConsistentlyThroughACycle: a plan
// reached twice in its own dependency cycle — once as the tree's root,
// once again through the cycle back to it — reads the same
// lane-overridden copy both times, not the root's lane view paired
// with a stale default-branch echo the second time the walk reaches it.
func TestShowAppliesTheLaneOverrideConsistentlyThroughACycle(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔳", "Alpha", []int{2}, "")
	commitPlan(t, repo, 2, "🔲", "Beta", []int{1}, "")

	wt := filepath.Join(root, "atlas-1")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/1-alpha", wt)
	writePlanFile(t, wt, 1, "✅", "Alpha", []int{2}, "", "")
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "close it out")
	t.Chdir(wt)
	var doc report.ShowDoc

	emit(t, &doc, "show", "--root", root)

	require.Equal(t, "✅", doc.Tree.Status,
		"the root reads the lane's own status")
	require.Len(t, doc.Tree.Deps, 1)
	beta := doc.Tree.Deps[0]
	require.Len(t, beta.Deps, 1)
	assert.Equal(t, "✅", beta.Deps[0].Status,
		"the cycle's second visit to plan 1 reads the same lane copy, "+
			"not the stale default branch")
}

// TestShowInsideItsOwnLaneReadsItsOwnDependsOn: a dependency edge the
// lane added to its own file, but has not merged, still walks —
// source: lane means the whole document reflects the lane's file, not
// only its status and Goal.
func TestShowInsideItsOwnLaneReadsItsOwnDependsOn(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔳", "Alpha", nil, "")
	commitPlan(t, repo, 2, "🔲", "Beta", nil, "")

	wt := filepath.Join(root, "atlas-1")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/1-alpha", wt)
	writePlanFile(t, wt, 1, "🔳", "Alpha", []int{2}, "", "")
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "add a dependency the lane hasn't merged")
	t.Chdir(wt)
	var out, errb bytes.Buffer

	code := run([]string{"show", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Beta",
		"the lane's own unmerged depends-on edge is walked")
}

// TestSelectorAmbiguityPrintsCandidatesAndExitsNonZero is the acceptance
// criterion for the selector: a fragment matching two plans is refused
// with its candidates, not resolved by a guess.
func TestSelectorAmbiguityPrintsCandidatesAndExitsNonZero(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Shared word alpha", nil, "")
	commitPlan(t, repo, 2, "🔲", "Shared word beta", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"next", "Shared word", "--root", root}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "matches 2 plans")
	assert.Contains(t, errb.String(), "alpha")
	assert.Contains(t, errb.String(), "beta")
}

func TestSelectorNotFoundExitsNonZero(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Something", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"show", "raymarch", "--root", root}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "no plan matches")
}

// TestFindMatchesAcrossBranches is the payoff of reading every ref: a
// plan that lives only on a side branch is still found by its topic.
func TestFindMatchesAcrossBranches(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "checkout", "-q", "-b", "plan/500-raymarch")
	commitPlan(t, repo, 500, "🔲", "Raymarch the gas giants", nil, "")
	git(t, repo, "checkout", "-q", "main")
	var out, errb bytes.Buffer

	code := run([]string{"find", "raymarch", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Raymarch the gas giants")
}

func TestFindMatchesASummary(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	path := filepath.Join(repo, "plan", "1_a.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(
		"---\nid: 1\ntitle: A plain title\nstatus: \"🔲\"\n"+
			"summary: walk the signed distance field\n---\n# A\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 1")
	var out, errb bytes.Buffer

	code := run([]string{"find", "signed distance", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "A plain title")
}

func TestFindIsQuietOnNoMatch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Something", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"find", "raymarch", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no plan matches")
}

func TestFindEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Raymarch the gas giants", nil, "")
	var doc report.FindDoc

	emit(t, &doc, "find", "raymarch", "--root", root)

	assert.Equal(t, "find", doc.Command)
	assert.Equal(t, "raymarch", doc.Query)
	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(7), doc.Plans[0].ID)
}

// TestPickRanksAndTrims: the most-unblocking plan comes first, and -n
// trims the list.
func TestPickRanksAndTrims(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Frees nothing", nil, "")
	commitPlan(t, repo, 2, "🔲", "Frees two downstream", nil, "")
	commitPlan(t, repo, 3, "🔳", "Waits on two", []int{2}, "")
	commitPlan(t, repo, 4, "🔳", "Also waits on two", []int{2}, "")
	var doc report.PickDoc

	emit(t, &doc, "pick", "-n", "1", "--root", root)

	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(2), doc.Plans[0].ID,
		"the plan freeing the most downstream work ranks first")
}

// leaseRef parks refs/heads/plan/<id> on a fresh lease marker of the
// given kind, child of main's tip — the shape claim.Acquire and
// claim.Release leave the work ref in.
func leaseRef(t *testing.T, repo string, id int, kind string) {
	t.Helper()
	tree, err := gitCapture(t, repo, "rev-parse", "HEAD^{tree}")
	require.NoError(t, err)
	head, err := gitCapture(t, repo, "rev-parse", "HEAD")
	require.NoError(t, err)
	msg := fmt.Sprintf("plan %d: %s\n\nepoch:   1\nnonce:   cafe\n"+
		"holder:  box-a\nlane:    -\nsession: -", id, kind)
	sha, err := gitCapture(t, repo, "commit-tree", tree, "-p", head, "-m", msg)
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "update-ref",
		fmt.Sprintf("refs/heads/plan/%d", id), sha)
	require.NoError(t, err)
}

// TestReadyWithholdsALeaseRef: the id-only work ref the lease mints is
// a hold under the default patterns, so a plan with a live lease marker
// on its tip is not offered.
func TestReadyWithholdsALeaseRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Shader unit", nil, "")
	leaseRef(t, repo, 7, "claim")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing startable",
		"a live lease on the work ref withholds the plan")
}

// TestReadyOffersAReleasedLease: a work ref whose tip is a release
// marker is a lease that ended, not a hold — the plan is startable
// again without any human deleting the ref.
func TestReadyOffersAReleasedLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Shader unit", nil, "")
	leaseRef(t, repo, 7, "release")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Shader unit",
		"a released lease does not read as a live hold")
}

// TestPickListsAStaleLeaseAsTakeover: pick ranks a held plan whose
// takeover window matured as a candidate, carrying its observed age,
// while a live-tip hold stays hidden.
func TestPickListsAStaleLeaseAsTakeover(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Stale lane", nil, "")
	commitPlan(t, repo, 8, "🔲", "Live lane", nil, "")
	leaseRef(t, repo, 7, "claim")
	leaseRef(t, repo, 8, "claim")
	tip7, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, tip7, 3*time.Hour)
	var doc report.PickDoc

	emit(t, &doc, "pick", "--root", root)

	require.Len(t, doc.Plans, 1, "the live-tip hold stays hidden")
	card := doc.Plans[0]
	assert.Equal(t, int64(7), card.ID)
	assert.True(t, card.Held, "a takeover candidate is still a held plan")
	assert.True(t, card.Stale, "and marked stale, which is why it ranks")
	assert.GreaterOrEqual(t, card.StaleSeconds, int64(2*60*60),
		"the observed age rides the card")
}

// TestPickHonorsAConfiguredTakeoverWindow: a repository declaring its
// own clock sees staleness mature against it rather than the 2h
// default (F12) — the knobs travel with the repository, not one
// observer's machine.
func TestPickHonorsAConfiguredTakeoverWindow(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("takeover-window: 10m\nsample-gap: 3m\n"), 0o600))
	commitPlan(t, repo, 7, "🔲", "Shader unit", nil, "")
	leaseRef(t, repo, 7, "claim")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, tip, 15*time.Minute)
	var doc report.PickDoc

	emit(t, &doc, "pick", "--root", root)

	require.Len(t, doc.Plans, 1,
		"15m matures under the repo's own 10m window")
	assert.True(t, doc.Plans[0].Stale)
}

// TestPickBacksOffByTheTakeoverMarkersAlreadyInTheChain: a ref
// carrying one prior takeover marker matures at 2T instead of T, so
// oscillation between two quiet agents damps out (F3). The same
// window that matures a fresh hold at the default T does not yet
// mature one with a takeover already in its chain.
func TestPickBacksOffByTheTakeoverMarkersAlreadyInTheChain(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Backed-off lane", nil, "")
	commitPlan(t, repo, 8, "🔲", "Twice-matured lane", nil, "")
	leaseRef(t, repo, 7, "claim")
	claim7, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	tip7 := stackMarker(t, repo, 7, "takeover", claim7)

	leaseRef(t, repo, 8, "claim")
	claim8, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/8")
	require.NoError(t, err)
	tip8 := stackMarker(t, repo, 8, "takeover", claim8)

	// seedWindow overwrites the whole state file, so both windows are
	// seeded together rather than one clobbering the other.
	path, err := observe.Path()
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, observe.Save(path, observe.State{
		observe.Key("atlas", 7): discovery.Window{
			Tip: tip7, First: now.Add(-3 * time.Hour),
			Last: now.Add(-time.Minute), Samples: 9,
		},
		observe.Key("atlas", 8): discovery.Window{
			Tip: tip8, First: now.Add(-5 * time.Hour),
			Last: now.Add(-time.Minute), Samples: 9,
		},
	}))
	var doc report.PickDoc

	emit(t, &doc, "pick", "--root", root)

	require.Len(t, doc.Plans, 1,
		"3h has not matured a ref with one takeover marker already "+
			"in its chain — that needs 2T")
	assert.Equal(t, int64(8), doc.Plans[0].ID,
		"5h has matured past the backed-off 2T threshold")
}

// stackMarker mints a lease marker as a child of parent, reusing its
// tree the way every marker does, and moves the plan's work ref onto
// it — building a chain with more than one marker in it, the way a
// backoff test needs.
func stackMarker(t *testing.T, repo string, id int, kind, parent string) string {
	t.Helper()
	tree, err := gitCapture(t, repo, "rev-parse", parent+"^{tree}")
	require.NoError(t, err)
	msg := fmt.Sprintf("plan %d: %s\n\nepoch:   2\nnonce:   cafe2\n"+
		"holder:  box-a\nlane:    -\nsession: -", id, kind)
	sha, err := gitCapture(t, repo, "commit-tree", tree, "-p", parent, "-m", msg)
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "update-ref",
		fmt.Sprintf("refs/heads/plan/%d", id), sha)
	require.NoError(t, err)

	return sha
}
