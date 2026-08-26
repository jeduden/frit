package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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

// TestDriftReportsLandedAndNamingCommits is the load-bearing slice:
// a plan whose hold branch merged into the default branch reads
// landed, with the commit that names its id as evidence; a plan with
// neither reads not landed, with an empty, never-null commits list.
func TestDriftReportsLandedAndNamingCommits(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")

	// Plan 100: its creation commit names the id ("plan 100"), and its
	// hold branch merges into main without ever touching the id again —
	// the drift a ledger left behind.
	commitPlan(t, repo, 100, "🔳", "Underway", nil, "")
	git(t, repo, "checkout", "-q", "-b", "plan/100-underway")
	git(t, repo, "commit", "--allow-empty", "-q", "-m", "wip")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "merge", "--no-ff", "-q", "-m", "merge lane",
		"plan/100-underway")

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
	assert.Equal(t, "plan 100", row100.Commits[0].Subject)
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

// TestDriftFlagsALastPhaseCommit: a plan with a phase ledger carries
// whether some naming commit also names its last phase — a plain
// mechanical flag, not a verdict that the phase actually closed.
func TestDriftFlagsALastPhaseCommit(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	phases := "phases:\n  - n: 1\n    title: setup\n    status: \"✅\"\n" +
		"  - n: 2\n    title: finish\n    status: \"🔳\"\n"

	// Plan 600: a later commit names both the plan and its last phase.
	writePlanFile(t, repo, 600, "🔳", "Ladder", nil, phases, "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 600")
	writeFile(t, repo, "leg.txt", "done\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 600 phase 2: GREEN — wire the last leg")

	// Plan 700: the same ledger shape, but no commit ever names phase 2.
	writePlanFile(t, repo, 700, "🔳", "NoGreenYet", nil, phases, "")
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

// writeFile writes a file's content within a repository checkout,
// without staging or committing it.
func writeFile(t *testing.T, repo, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, name), []byte(content), 0o600))
}
