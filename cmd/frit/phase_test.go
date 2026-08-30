package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFolderPlan lays a folder plan's plan.md at
// plan/<id>_<slug>/plan.md, carrying an Execution table but no
// phases: ledger — the shape a plan moving its phase content out to
// its own phase-N.md files takes. It returns the folder's path.
func writeFolderPlan(
	t *testing.T, repo string, id int, status, title string,
	execRows string,
) string {
	t.Helper()
	dir := filepath.Join(repo, "plan", fmt.Sprintf("%d_%s", id,
		slugify(title)))
	require.NoError(t, os.MkdirAll(dir, 0o750))

	body := fmt.Sprintf("---\nid: %d\ntitle: %s\nstatus: %q\n---\n# %s\n",
		id, title, status, title)
	if execRows != "" {
		body += "\n## Execution\n\n" +
			"| Phase | Design | Implement | Gate |\n" +
			"| --- | --- | --- | --- |\n" + execRows
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.md"),
		[]byte(body), 0o600))

	return dir
}

// writePhaseCompanion writes one phase-N.md or phase-N.result.md file
// beside a folder plan's plan.md.
func writePhaseCompanion(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name),
		[]byte(body), 0o600))
}

// TestPhaseNamesTheOpenPhaseFileAndBundlesItsHandoff is Phase 1's RED,
// case one: a folder plan whose phase-1 has already closed with a
// Handoff, and whose phase-2 carries no result file yet, reports
// phase 2 as open, phase-2.md's own body as the spec, phase-1's
// handoff, and phase-2.result.md as the file to write.
func TestPhaseNamesTheOpenPhaseFileAndBundlesItsHandoff(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	dir := writeFolderPlan(t, repo, 100, "🔳", "Layered work",
		"1 first | sonnet | sonnet | test one\n"+
			"2 second | sonnet | opus | test two\n")
	writePhaseCompanion(t, dir, "phase-1.md", "Do the first thing.")
	writePhaseCompanion(t, dir, "phase-1.result.md",
		"## Handoff\n\nPhase one landed cleanly.\n")
	writePhaseCompanion(t, dir, "phase-2.md", "Do the second thing.")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	t.Chdir(wt)
	var out, errb bytes.Buffer

	code := run([]string{"phase", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "phase 2")
	assert.Contains(t, got, "Do the second thing.")
	assert.Contains(t, got, "Phase one landed cleanly.")
	assert.Contains(t, got, "phase-2.result.md")
	assert.Contains(t, got, "opus")
	assert.Contains(t, got, "test two")
}

// TestPhaseDoneTestParsesTheHandoffHeadingNotASubstring is Phase 1's
// RED, case two: phase-2's own result file parks Follow-ups that
// quote a "## Handoff" line inside a fenced code block, but carries
// no Handoff heading of its own — phase 2 still reads as open, and
// those parked notes travel in the bundle.
func TestPhaseDoneTestParsesTheHandoffHeadingNotASubstring(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	dir := writeFolderPlan(t, repo, 100, "🔳", "Layered work",
		"1 first | sonnet | sonnet | test one\n"+
			"2 second | sonnet | opus | test two\n")
	writePhaseCompanion(t, dir, "phase-1.md", "Do the first thing.")
	writePhaseCompanion(t, dir, "phase-1.result.md",
		"## Handoff\n\nPhase one landed cleanly.\n")
	writePhaseCompanion(t, dir, "phase-2.md", "Do the second thing.")
	writePhaseCompanion(t, dir, "phase-2.result.md",
		"## Follow-ups\n\nSaw this shape once:\n\n"+
			"```\n## Handoff\n```\n\nParked, not done.\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	t.Chdir(wt)
	var out, errb bytes.Buffer

	code := run([]string{"phase", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "phase 2")
	assert.Contains(t, got, "Parked, not done.")
}

// TestPhaseFallsBackToThePlanLedgerWithNoPhaseFiles is Phase 1's RED,
// case three: a folder plan carrying no phase-N.md files at all
// reports its open phase from the plan.md ledger and its "## Phase N"
// section, proving the fallback plan-phase already relies on.
func TestPhaseFallsBackToThePlanLedgerWithNoPhaseFiles(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	dir := filepath.Join(repo, "plan", "100_layered-work")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	planBody := "---\nid: 100\ntitle: Layered work\nstatus: \"🔳\"\n" +
		"phases:\n" +
		"  - { n: 1, title: 'First', status: \"✅\" }\n" +
		"  - { n: 2, title: 'Second', status: \"🔲\" }\n" +
		"---\n# Layered work\n\n## Phase 2: Second\n\n" +
		"Do the second thing.\n\n## Execution\n\n" +
		"| Phase | Design | Implement | Gate |\n" +
		"| --- | --- | --- | --- |\n" +
		"| 2 second | sonnet | opus | test two |\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.md"),
		[]byte(planBody), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	t.Chdir(wt)
	var out, errb bytes.Buffer

	code := run([]string{"phase", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "phase 2")
	assert.Contains(t, got, "Do the second thing.")
	assert.False(t, strings.Contains(got, "phase-2.result.md"),
		"a ledger plan names no per-phase result file")
}

// TestPhaseEmitsJSON pins the wire shape: the open phase's number,
// spec, and the plan it belongs to.
func TestPhaseEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	dir := writeFolderPlan(t, repo, 100, "🔳", "Layered work",
		"1 first | sonnet | sonnet | test one\n")
	writePhaseCompanion(t, dir, "phase-1.md", "Do the first thing.")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-layered", wt)
	t.Chdir(wt)
	var doc report.PhaseDoc

	emit(t, &doc, "phase", "--root", root)

	assert.Equal(t, "phase", doc.Command)
	assert.True(t, doc.HasPhase)
	assert.Equal(t, "1", doc.Phase.N)
	assert.Equal(t, "Do the first thing.", doc.Phase.Spec)
	assert.Equal(t, int64(100), doc.Plan.ID)
}

// TestPhaseRefusesOutsideThePlansOwnLane: a folder plan's phase-N.md
// and phase-N.result.md files live only in a worktree, so phase
// refuses rather than silently bundling nothing from the default
// branch.
func TestPhaseRefusesOutsideThePlansOwnLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	dir := writeFolderPlan(t, repo, 100, "🔳", "Layered work",
		"1 first | sonnet | sonnet | test one\n")
	writePhaseCompanion(t, dir, "phase-1.md", "Do the first thing.")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 100")
	var out, errb bytes.Buffer

	code := run([]string{"phase", "100", "--root", root}, &out, &errb)

	require.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "own lane")
}
