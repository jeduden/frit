package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emit runs a command with --json and decodes stdout into doc. The
// decode is the assertion that matters most: a document that will not
// parse back into the shape frit publishes is a broken contract,
// whatever else the test goes on to check.
func emit(t *testing.T, doc any, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer

	code := run(append(args, "--json"), &out, &errb)

	require.Equal(t, 0, code, errb.String())
	require.NoError(t, json.Unmarshal(out.Bytes(), doc), out.String())

	return errb.String()
}

func TestVersionEmitsJSON(t *testing.T) {
	isolate(t)
	var doc report.VersionDoc

	emit(t, &doc, "version")

	assert.Equal(t, report.Schema, doc.Schema)
	assert.Equal(t, "version", doc.Command)
	assert.Equal(t, "dev", doc.Version)
}

func TestInitEmitsThePathsItWrote(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	var doc report.InitDoc

	emit(t, &doc, "init", repo)

	assert.Equal(t, "init", doc.Command)
	assert.Contains(t, doc.Paths, filepath.Join(repo, ".frit.yml"))
	assert.Contains(t, doc.Paths, filepath.Join(repo, "plan", "proto.md"))
}

func TestReposEmitsEveryWorktree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/2608142306",
		filepath.Join(root, "atlas-fleet-index"))
	var doc report.ReposDoc

	emit(t, &doc, "repos", "--root", root)

	assert.Equal(t, "repos", doc.Command)
	assert.Equal(t, root, doc.Root)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Worktrees, 2)
	assert.Equal(t, "atlas-fleet-index", doc.Repos[0].Worktrees[1].Name)
	assert.Equal(t, "plan/2608142306", doc.Repos[0].Worktrees[1].Branch)
	assert.True(t, doc.Repos[0].Worktrees[1].HasCommit)
}

// TestPlansEmitsEveryPlanWithoutDetail is where the two renderings
// part company on purpose: the table summarises unless asked, and the
// document never does.
func TestPlansEmitsEveryPlanWithoutDetail(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	writePlan(t, repo, "plan/a.md", 7, "Elsewhere")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "a plan")
	var doc report.PlansDoc

	emit(t, &doc, "plans", "--root", root)

	assert.Equal(t, "plans", doc.Command)
	assert.NotEmpty(t, doc.Host)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Plans, 1)

	p := doc.Repos[0].Plans[0]
	assert.Equal(t, int64(7), p.ID)
	assert.Equal(t, "Elsewhere", p.Title)
	assert.Equal(t, "plan/a.md", p.Path)
	assert.Equal(t, 1, p.RefCount)
	assert.Equal(t, doc.Host+":atlas:7", p.Key)
}

func TestOrphansEmitsTheClaimAndTheCleanRepository(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	claimBranch(t, repo, "plan/2608142306-fleet-index")
	initRepo(t, root, "clean")
	var doc report.OrphansDoc

	emit(t, &doc, "orphans", "--root", root)

	require.Len(t, doc.Repos, 2)
	assert.Equal(t, "atlas", doc.Repos[0].Name)
	require.Len(t, doc.Repos[0].Unstaffed, 1)
	assert.Equal(t, int64(2608142306), doc.Repos[0].Unstaffed[0].PlanID)
	assert.Equal(t, "refs/heads/plan/2608142306-fleet-index",
		doc.Repos[0].Unstaffed[0].Holds[0].Ref)

	// The table drops a clean repository; the document keeps it, so a
	// consumer can tell one that was walked from one that was not.
	assert.Equal(t, "clean", doc.Repos[1].Name)
	assert.False(t, doc.Repos[1].Any())
}

func TestStaleEmitsTheCutoffItMeasuredWith(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var doc report.StaleDoc

	// Everything committed just now is older than zero days.
	emit(t, &doc, "stale", "--root", root, "--days", "0")

	assert.Equal(t, "stale", doc.Command)
	assert.Equal(t, 0, doc.Days)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Stale, 1)
	assert.Equal(t, "atlas", doc.Repos[0].Stale[0].Worktree.Name)
	assert.Zero(t, doc.Repos[0].Stale[0].AgeDays)
}

// TestJSONCarriesAProblemInsteadOfPrintingIt is the rule that makes
// stdout the whole report: a repository frit could not read is in the
// document, and nothing is written beside it.
func TestJSONCarriesAProblemInsteadOfPrintingIt(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var doc report.OrphansDoc

	stderr := emit(t, &doc, "orphans", "--root", root)

	assert.Empty(t, stderr)
	assert.Empty(t, doc.Repos)
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
	assert.NotEmpty(t, doc.Problems[0].Message)
}

// TestTextPrintsAProblemToStderr is the same failure in the other
// rendering: the table stays on stdout and the failure goes beside it.
func TestTextPrintsAProblemToStderr(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"orphans", "--root", root}, &out, &errb)

	require.Equal(t, 0, code)
	assert.Contains(t, out.String(), "no orphaned lanes")
	assert.Contains(t, errb.String(), "frit: atlas:")
}

// writePlan commits nothing; it only puts a plan file in the tree.
func writePlan(t *testing.T, repo, rel string, id int, title string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	body := "---\nid: " + strconv.Itoa(id) + "\ntitle: " + title +
		"\nstatus: \"🔳\"\n---\n# " + title + "\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}
