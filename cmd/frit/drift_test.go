package main

import (
	"bytes"
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
