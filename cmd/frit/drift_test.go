package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// driftRow finds the row for a plan id, so a test can assert on it
// without depending on row order.
func driftRow(t *testing.T, doc report.DriftDoc, id int64) report.DriftRow {
	t.Helper()
	for _, r := range doc.Rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no drift row for plan %d", id)

	return report.DriftRow{}
}

// mergedPlanRepo builds a repository holding a plan still marked in
// progress whose hold branch has already merged into main by an
// ordinary merge commit — the ancestor-merge signal drift's landed
// check reads, with the plan's own creation commit as the only
// evidence naming its id. It returns the repository path and that
// commit's subject, shared by the unit test below and by C2's own
// command-scenario fixture in bdd_commands_test.go.
func mergedPlanRepo(t *testing.T, root string, id int) (repo, subject string) {
	t.Helper()
	repo = initRepo(t, root, "atlas")
	commitPlan(t, repo, id, "🔳", "Underway", nil, "")
	branch := fmt.Sprintf("plan/%d-underway", id)
	git(t, repo, "checkout", "-q", "-b", branch)
	git(t, repo, "commit", "--allow-empty", "-q", "-m", "wip")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "merge", "--no-ff", "-q", "-m", "merge lane", branch)

	return repo, fmt.Sprintf("plan %d", id)
}

// TestDriftReportsLandedAndNamingCommits is the load-bearing slice:
// a plan whose hold branch merged into the default branch reads
// landed, with the commit that names its id as evidence; a plan with
// neither reads not landed, with an empty, never-null commits list.
func TestDriftReportsLandedAndNamingCommits(t *testing.T) {
	isolate(t)
	root := t.TempDir()

	// Plan 100: its creation commit names the id ("plan 100"), and its
	// hold branch merges into main without ever touching the id again —
	// the drift a ledger left behind.
	repo, subject := mergedPlanRepo(t, root, 100)

	// Plan 200: no commit ever names it, and no branch of it exists.
	writePlanFile(t, repo, 200, "🔲", "Untouched", nil, "", "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "add plan file")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	require.Len(t, doc.Rows, 2)

	row100 := driftRow(t, doc, 100)
	assert.True(t, row100.Landed)
	require.Len(t, row100.Commits, 1)
	assert.Equal(t, subject, row100.Commits[0].Subject)
	assert.NotEmpty(t, row100.Commits[0].SHA)

	row200 := driftRow(t, doc, 200)
	assert.False(t, row200.Landed)
	assert.Equal(t, []report.DriftCommit{}, row200.Commits)
}

// TestDriftIsQuietWhenNothingIsOutstanding: a repository with no
// not-done plan reports no rows and says so plainly in the table.
func TestDriftIsQuietWhenNothingIsOutstanding(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")

	var out, errb bytes.Buffer
	code := run([]string{"drift", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no not-done plans found")
}

// TestDriftIgnoresADonePlan: a plan already marked done is not the
// drift report's subject, whatever git shows for it.
func TestDriftIgnoresADonePlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 300, "✅", "Finished", nil, "")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	assert.Empty(t, doc.Rows)
}

// TestDriftReadsSquashMergedWorkAsLanded: a hold branch's content
// reaches main by a squash merge, so the branch itself is never an
// ancestor — the shape ordinary ancestry cannot see, and the reason
// landed also runs the content check. A branch carrying real work
// that never reached main at all still reads not landed.
func TestDriftReadsSquashMergedWorkAsLanded(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")

	// Plan 400: its hold branch's content is squashed onto main by a
	// second, unrelated commit — same tree, different history. The
	// branch is the bare, id-only shape the lease protocol writes, the
	// one HoldTip resolves off.
	commitPlan(t, repo, 400, "🔳", "Squashed", nil, "")
	git(t, repo, "checkout", "-q", "-b", "plan/400")
	writeFile(t, repo, "work.txt", "same content\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 400: work")
	git(t, repo, "checkout", "-q", "main")
	writeFile(t, repo, "work.txt", "same content\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "squash landed")

	// Plan 500: its hold branch carries real work that never reached
	// main by any route.
	commitPlan(t, repo, 500, "🔳", "Ongoing", nil, "")
	git(t, repo, "checkout", "-q", "-b", "plan/500")
	writeFile(t, repo, "ongoing.txt", "wip\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 500: wip")
	git(t, repo, "checkout", "-q", "main")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	assert.True(t, driftRow(t, doc, 400).Landed,
		"squash-merged content reads landed even off an unmerged branch")
	assert.False(t, driftRow(t, doc, 500).Landed,
		"real work never reaching main reads not landed")
}

// lastPhasePhases is the two-phase ledger shape lastPhasePlanRepo and
// TestDriftFlagsALastPhaseCommit's negative case both write: phase 2
// is the highest-numbered, still-open phase namesLastPhase looks for.
const lastPhasePhases = "phases:\n  - n: 1\n    title: setup\n    status: \"✅\"\n" +
	"  - n: 2\n    title: finish\n    status: \"🔳\"\n"

// lastPhasePlanRepo builds a repository holding a multi-phase plan
// still marked in progress whose last phase's commit already sits on
// main — the namesLastPhase signal drift's phase-level check reads.
// It returns the repository path and that commit's subject, shared by
// the unit test below and by C4's own command-scenario fixture in
// bdd_commands_test.go.
func lastPhasePlanRepo(t *testing.T, root string, id int) (repo, subject string) {
	t.Helper()
	repo = initRepo(t, root, "atlas")
	writePlanFile(t, repo, id, "🔳", "Ladder", nil, lastPhasePhases, "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", fmt.Sprintf("plan %d", id))
	writeFile(t, repo, "leg.txt", "done\n")
	git(t, repo, "add", "-A")
	subject = fmt.Sprintf("plan %d phase 2: GREEN — wire the last leg", id)
	git(t, repo, "commit", "-q", "-m", subject)

	return repo, subject
}

// TestDriftFlagsALastPhaseCommit: a plan with a phase ledger carries
// whether some naming commit also names its last phase — a plain
// mechanical flag, not a verdict that the phase actually closed.
func TestDriftFlagsALastPhaseCommit(t *testing.T) {
	isolate(t)
	root := t.TempDir()

	// Plan 600: a later commit names both the plan and its last phase.
	repo, _ := lastPhasePlanRepo(t, root, 600)

	// Plan 700: the same ledger shape, but no commit ever names phase 2.
	writePlanFile(t, repo, 700, "🔳", "NoGreenYet", nil, lastPhasePhases, "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 700")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	assert.True(t, driftRow(t, doc, 600).LastPhaseCommit)
	assert.False(t, driftRow(t, doc, 700).LastPhaseCommit)
}

// TestDriftDoesNotMatchIDAsSubstring: a commit naming an unrelated,
// longer number that merely contains this plan's id as a run of
// digits is not read as evidence for it.
func TestDriftDoesNotMatchIDAsSubstring(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")

	// Plan 20's own creation commit names it; a later, unrelated commit
	// merely contains "20" as a substring of "220" and must not count
	// as evidence for it.
	commitPlan(t, repo, 20, "🔲", "Short", nil, "")
	writeFile(t, repo, "unrelated.txt", "x\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "bump timeout to 220ms")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	row := driftRow(t, doc, 20)
	require.Len(t, row.Commits, 1)
	assert.Equal(t, "plan 20", row.Commits[0].Subject,
		"a commit naming 220 must not count as evidence for plan 20")
}

// TestDriftHonorsConfiguredBase: a repository that overrides `base:`
// in .frit.yml is judged landed against that ref, the same base every
// other verb (claim, reap, orphans) reads off the coordinate — not
// against whatever gitobj.DefaultRef would guess on its own.
func TestDriftHonorsConfiguredBase(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")

	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("base: release\n"), 0o600))
	git(t, repo, "add", ".frit.yml")
	git(t, repo, "commit", "-q", "-m", "configure base")
	git(t, repo, "branch", "-q", "release")

	// Plan 800's hold branch merges into "release", the configured
	// base — never into "main", the branch gitobj.DefaultRef would
	// pick left to its own cascade.
	commitPlan(t, repo, 800, "🔳", "OnRelease", nil, "")
	git(t, repo, "checkout", "-q", "-b", "plan/800")
	git(t, repo, "commit", "--allow-empty", "-q", "-m", "wip")
	git(t, repo, "checkout", "-q", "release")
	git(t, repo, "merge", "--no-ff", "-q", "-m", "merge lane", "plan/800")
	git(t, repo, "checkout", "-q", "main")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	assert.True(t, driftRow(t, doc, 800).Landed,
		"landed against the configured base, not main")
}

// TestDriftMatchesIDImmediatelyFollowedByANonDigit: a commit subject
// that quotes a plan's own filename convention (<id>_<slug>.md) names
// the plan even though the id is immediately followed by an
// underscore — a boundary Go's regexp \b would miss, since it treats
// '_' as a word character with no boundary before it.
func TestDriftMatchesIDImmediatelyFollowedByANonDigit(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")

	commitPlan(t, repo, 900, "🔲", "Filenamed", nil, "")
	writeFile(t, repo, "note.txt", "x\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m",
		"touch up plan/900_filenamed.md wording")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	row := driftRow(t, doc, 900)
	require.Len(t, row.Commits, 2,
		"the commit quoting the plan's own filename is evidence too")
}

// TestDriftSkipsPlansWhenRepoContextFails: a repo whose own drift
// context fails to build (here, a `base:` override gitobj.MergedRefs
// cannot resolve) reports the read failure as a Problem and emits no
// rows for that repo's not-done plans — never a fabricated
// "not landed, no commits" row indistinguishable from a genuinely
// untouched plan.
func TestDriftSkipsPlansWhenRepoContextFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "busted")

	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("base: does-not-exist\n"), 0o600))
	git(t, repo, "add", ".frit.yml")
	git(t, repo, "commit", "-q", "-m", "configure a base that never exists")
	commitPlan(t, repo, 950, "🔲", "Unreadable", nil, "")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	assert.Empty(t, doc.Rows,
		"a repo whose context failed to build reports no rows, not fabricated ones")
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "busted", doc.Problems[0].Repo)
}

// TestDriftPicksHighestPhaseNumberRegardlessOfOrder: a plan's last
// phase is the highest-numbered one, not merely the last entry in the
// front matter's own phases: list order — nothing enforces that a
// plan's phases are written in ascending order.
func TestDriftPicksHighestPhaseNumberRegardlessOfOrder(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	phases := "phases:\n  - n: 2\n    title: finish\n    status: \"🔳\"\n" +
		"  - n: 1\n    title: setup\n    status: \"✅\"\n"

	writePlanFile(t, repo, 960, "🔳", "OutOfOrder", nil, phases, "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 960")
	writeFile(t, repo, "leg.txt", "done\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 960 phase 2: GREEN — finish")

	var doc report.DriftDoc
	emit(t, &doc, "drift", "--root", root)

	assert.True(t, driftRow(t, doc, 960).LastPhaseCommit,
		"phase 2 is the highest-numbered phase, even listed first")
}

// writeFile writes a file's content within a repository checkout,
// without staging or committing it.
func writeFile(t *testing.T, repo, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, name), []byte(content), 0o600))
}

// TestDriftFailsWhenTheRootCannotBeWalked: a root that cannot be
// walked fails before drift ever gathers the fleet.
func TestDriftFailsWhenTheRootCannotBeWalked(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"drift", "--root",
		filepath.Join(t.TempDir(), "missing")}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// TestDriftSkipsAPlanWithNoCoordinate: two checkouts sharing a
// repository name leave the fleet unable to place a not-done plan's
// evidence, so drift reports no row for it rather than guessing a
// repository.
func TestDriftSkipsAPlanWithNoCoordinate(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repoA := initRepo(t, filepath.Join(root, "a"), "frontend")
	commitPlan(t, repoA, 7, "🔲", "Shader unit", nil, "")
	repoB := initRepo(t, filepath.Join(root, "b"), "frontend")
	commitPlan(t, repoB, 9, "🔲", "Other work", nil, "")
	var doc report.DriftDoc

	emit(t, &doc, "drift", "--root", root)

	assert.Empty(t, doc.Rows,
		"an ambiguous repo name has nowhere to read drift evidence from")
}

// TestNewDriftRepoContextSurfacesAnUnreadableConfig:
// newDriftRepoContext's own repocfg.Load error, called directly.
func TestNewDriftRepoContextSurfacesAnUnreadableConfig(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))

	_, err := newDriftRepoContext(repo, "main", gitwt.Exec)

	assert.Error(t, err)
}

// TestNewDriftRepoContextSurfacesAnUncompilableHoldPattern:
// newDriftRepoContext's own cfg.Compiled error, called directly.
func TestNewDriftRepoContextSurfacesAnUncompilableHoldPattern(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\"plan/[\"]\n"), 0o600))

	_, err := newDriftRepoContext(repo, "main", gitwt.Exec)

	assert.Error(t, err)
}

// TestNewDriftRepoContextSurfacesAnUnreadableRefList:
// newDriftRepoContext's own gitobj.Refs error, called directly
// against a path with no git dir at all.
func TestNewDriftRepoContextSurfacesAnUnreadableRefList(t *testing.T) {
	_, err := newDriftRepoContext(
		filepath.Join(t.TempDir(), "missing"), "main", gitwt.Exec)

	assert.Error(t, err)
}

// TestNewDriftRepoContextSurfacesAnUnreadableCommitLog:
// newDriftRepoContext's own allCommits error, called directly with a
// stub runner that fails only the log call.
func TestNewDriftRepoContextSurfacesAnUnreadableCommitLog(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	failLog := func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "log" {
			return nil, errors.New("boom")
		}

		return gitwt.Exec(dir, args...)
	}

	_, err := newDriftRepoContext(repo, "main", failLog)

	assert.Error(t, err)
}

// TestNewDriftRepoContextSkipsARefWithNoBranchName: a merged ref that
// is not a branch — a tag, say — carries no plan id to read, so it is
// skipped rather than misread.
func TestNewDriftRepoContextSkipsARefWithNoBranchName(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "tag", "v1.0.0")

	ctx, err := newDriftRepoContext(repo, "main", gitwt.Exec)

	require.NoError(t, err)
	assert.Empty(t, ctx.ancestorLanded)
}

// TestBucketByIDSkipsADigitRunThatOverflows: bucketByID's own
// strconv.ParseInt error, a digit run too long to fit an int64,
// called directly.
func TestBucketByIDSkipsADigitRunThatOverflows(t *testing.T) {
	commits := []report.DriftCommit{
		{SHA: "abc", Subject: "plan 99999999999999999999999: overflow"},
	}

	got := bucketByID(commits)

	assert.Empty(t, got)
}

// TestAllCommitsSkipsALineWithNoUnitSeparator: allCommits' own guard
// against a malformed log line, called directly with a stub runner
// answering plain, un-delimited text.
func TestAllCommitsSkipsALineWithNoUnitSeparator(t *testing.T) {
	rt := func(string, ...string) ([]byte, error) {
		return []byte("not a delimited line\n"), nil
	}

	got, err := allCommits("/repo", rt)

	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestLastPhaseNumberSkipsANonNumericPhase: lastPhaseNumber's own
// strconv.Atoi error, called directly.
func TestLastPhaseNumberSkipsANonNumericPhase(t *testing.T) {
	got := lastPhaseNumber([]planmeta.Phase{{N: "abc"}, {N: "2"}})

	assert.Equal(t, "2", got)
}

// TestLastPhaseNumberFallsBackToTheLastEntryWhenNoneParse:
// lastPhaseNumber's own fallback, when no phase number parses as a
// plain integer at all.
func TestLastPhaseNumberFallsBackToTheLastEntryWhenNoneParse(t *testing.T) {
	got := lastPhaseNumber([]planmeta.Phase{{N: "3a"}, {N: "3b"}})

	assert.Equal(t, "3b", got)
}

// TestPrintDriftRendersLandedAndLastPhaseColumns: printDrift's own
// rendering, direct-called against hand-built rows covering every
// landed/last-phase combination — every existing drift test reads
// --json, never the table.
func TestPrintDriftRendersLandedAndLastPhaseColumns(t *testing.T) {
	doc := &report.DriftDoc{Rows: []report.DriftRow{
		{Repo: "atlas", ID: 7, Landed: true, LastPhaseCommit: true,
			Commits: []report.DriftCommit{{SHA: "abc", Subject: "plan 7"}}},
		{Repo: "atlas", ID: 9, Landed: false, LastPhaseCommit: false},
	}}
	var out bytes.Buffer

	printDrift(&out, doc)

	got := out.String()
	assert.Contains(t, got, "landed")
	assert.Contains(t, got, "last phase named")
	assert.Contains(t, got, "not landed")
}
