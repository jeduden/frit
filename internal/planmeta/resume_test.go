package planmeta

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const folderPlanNoLedger = `---
id: 1
title: Folder plan
status: "🔳"
---
# Folder plan

## Execution

| Phase | Design | Implement | Gate |
| --- | --- | --- | --- |
| 1 first | sonnet | sonnet | test one |
| 2 second | sonnet | opus | test two |
`

func writePhaseFile(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name),
		[]byte(body), 0o600))
}

// TestResumeFindsTheOpenPhaseFile is Phase 1's RED: the first
// phase-N.md whose result carries no Handoff is the open phase, and
// its bundle carries the previous phase's handoff, its own spec, and
// the result file to write.
func TestResumeFindsTheOpenPhaseFile(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md", "Do the first thing.")
	writePhaseFile(t, dir, "phase-1.result.md",
		"## Handoff\n\nPhase one landed cleanly.\n")
	writePhaseFile(t, dir, "phase-2.md", "Do the second thing.")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("2"), got.N)
	assert.Equal(t, "Do the second thing.", got.Spec)
	assert.Equal(t, "Phase one landed cleanly.", got.HandoffIn)
	assert.Equal(t, "phase-2.result.md", got.ResultPath)
	assert.Equal(t, "opus", got.Tier)
	assert.Equal(t, "test two", got.Gate)
	assert.Empty(t, got.Notes)
}

// TestResumeDoneTestParsesHeadingsNotSubstrings: a result file whose
// own notes quote a "## Handoff" line inside a fenced code block, but
// carry no Handoff heading of their own, still reads as open — and
// those notes travel in the bundle.
func TestResumeDoneTestParsesHeadingsNotSubstrings(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md", "Do the first thing.")
	writePhaseFile(t, dir, "phase-1.result.md",
		"## Handoff\n\nPhase one landed cleanly.\n")
	writePhaseFile(t, dir, "phase-2.md", "Do the second thing.")
	writePhaseFile(t, dir, "phase-2.result.md",
		"## Follow-ups\n\nSaw this pattern once:\n\n"+
			"```\n## Handoff\n```\n\nParked, not done.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("2"), got.N)
	assert.Contains(t, got.Notes, "Parked, not done.")
	assert.Contains(t, got.Notes, "## Handoff")
}

// TestResumeOrdersPhasesNumerically: phase-2 precedes phase-10, which
// a lexical sort would get backwards.
func TestResumeOrdersPhasesNumerically(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-2.md", "Second.")
	writePhaseFile(t, dir, "phase-2.result.md", "## Handoff\n\nDone.\n")
	writePhaseFile(t, dir, "phase-10.md", "Tenth.")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.Equal(t, PhaseNumber("10"), got.N)
}

// TestResumeRecognisesASplitPhaseFile: PhaseNumber is alphanumeric
// elsewhere in this package (a sitting that grows into two splits as
// "3a"/"3b", per TestParseReadsAnAlphanumericPhaseNumber) — a
// phase-file plan gets the same split, not a numeric-only one.
func TestResumeRecognisesASplitPhaseFile(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md", "First.")
	writePhaseFile(t, dir, "phase-1.result.md", "## Handoff\n\nDone.\n")
	writePhaseFile(t, dir, "phase-3a.md", "Split, part a.")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("3a"), got.N)
	assert.Equal(t, "Split, part a.", got.Spec)
	assert.Equal(t, "phase-3a.result.md", got.ResultPath)
}

// TestResumeOrdersASplitPhaseByItsFullToken: phase-3a precedes
// phase-3b, and both precede phase-10 — numeric first, then the
// letter suffix breaks the tie within the same leading number.
func TestResumeOrdersASplitPhaseByItsFullToken(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-3b.md", "Split, part b.")
	writePhaseFile(t, dir, "phase-3b.result.md", "## Handoff\n\nDone.\n")
	writePhaseFile(t, dir, "phase-3a.md", "Split, part a.")
	writePhaseFile(t, dir, "phase-3a.result.md", "## Handoff\n\nDone.\n")
	writePhaseFile(t, dir, "phase-10.md", "Tenth.")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.Equal(t, PhaseNumber("10"), got.N)
}

// TestResumeFallsBackToTheLedgerWhenNoPhaseFiles: a plan with no
// phase-N.md files at all reports its open phase from the plan.md
// ledger and sections, unchanged.
func TestResumeFallsBackToTheLedgerWhenNoPhaseFiles(t *testing.T) {
	dir := t.TempDir()
	body := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 1, title: 'First', status: \"✅\" }\n" +
		"  - { n: 2, title: 'Second', status: \"🔲\" }\n" +
		"---\n# T\n\n## Phase 2: Second\n\nDo the second thing.\n\n" +
		"## Execution\n\n| Phase | Design | Implement | Gate |\n" +
		"| --- | --- | --- | --- |\n| 2 second | sonnet | opus | test two |\n"

	got, err := Resume(dir, []byte(body))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("2"), got.N)
	assert.Equal(t, "Do the second thing.", got.Spec)
	assert.Equal(t, "opus", got.Tier)
	assert.Empty(t, got.ResultPath)
	assert.Empty(t, got.HandoffIn)
}

// TestResumeWithNoDirectoryUsesTheLedger: an empty dir — a flat
// plan's own signal that it has no directory of its own to glob —
// resumes from the ledger without ever touching the filesystem, the
// same answer a folder plan with no phase-N.md files gives.
func TestResumeWithNoDirectoryUsesTheLedger(t *testing.T) {
	body := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 1, title: 'First', status: \"✅\" }\n" +
		"  - { n: 2, title: 'Second', status: \"🔲\" }\n" +
		"---\n# T\n\n## Phase 2: Second\n\nDo the second thing.\n"

	got, err := Resume("", []byte(body))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("2"), got.N)
	assert.Equal(t, "Do the second thing.", got.Spec)
}

// TestResumeReportsNoOpenPhaseWhenEveryPhaseFileIsDone mirrors
// FirstOpenPhase's own "none left" case for a phase-file plan.
func TestResumeReportsNoOpenPhaseWhenEveryPhaseFileIsDone(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md", "Do the first thing.")
	writePhaseFile(t, dir, "phase-1.result.md",
		"## Handoff\n\nDone.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.False(t, got.HasPhase)
}
