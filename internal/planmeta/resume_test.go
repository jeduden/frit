package planmeta

import (
	"os"
	"path/filepath"
	"strings"
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

// TestResumeCarriesNoHandoffWhenThePrecedingPhaseLeftNone: the open
// phase's incoming handoff is the phase immediately before it, and only
// that phase. When the preceding phase is done by its own status with no
// result file — and so no ## Handoff — the bundle carries no handoff,
// not the stale one an earlier phase left behind.
func TestResumeCarriesNoHandoffWhenThePrecedingPhaseLeftNone(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"✅\"\n---\nDo the first thing.\n")
	writePhaseFile(t, dir, "phase-1.result.md",
		"## Handoff\n\nPhase one landed cleanly.\n")
	writePhaseFile(t, dir, "phase-2.md",
		"---\nn: 2\ntitle: Second\nstatus: \"✅\"\n---\nDo the second thing.\n")
	writePhaseFile(t, dir, "phase-3.md",
		"---\nn: 3\ntitle: Third\nstatus: \"🔲\"\n---\nDo the third thing.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("3"), got.N)
	assert.Empty(t, got.HandoffIn)
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

// TestResumeFindsTheOpenPhaseFromPhaseFileStatus is Phase 2's RED: a
// ledger-free folder plan whose phase-1.md front matter says its status
// is done — with no phase-1.result.md and so no ## Handoff — resumes at
// phase 2. At HEAD Resume decides done by the ## Handoff marker alone,
// so phase 1 reads as open and the bundle returns it.
func TestResumeFindsTheOpenPhaseFromPhaseFileStatus(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"✅\"\n---\nDo the first thing.\n")
	writePhaseFile(t, dir, "phase-2.md",
		"---\nn: 2\ntitle: Second\nstatus: \"🔲\"\n---\nDo the second thing.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("2"), got.N)
	assert.Equal(t, "Do the second thing.", got.Spec)
	assert.Equal(t, "opus", got.Tier)
}

// TestResumeCarriesThePhaseFileTitle: a folder-plan phase file now
// carries its own title in front matter, so the bundle surfaces it — the
// way a ledger phase's title already travels — and the spec keeps only
// the phase's prose, not its front matter.
func TestResumeCarriesThePhaseFileTitle(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"🔲\"\n---\nDo the first thing.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.Equal(t, "First", got.Title)
	assert.Equal(t, "Do the first thing.", got.Spec)
}

// TestResumeOpenPhaseDoesNotBundleAHandoffAsNotes: a phase whose own
// status says it is still open, but whose result file already carries a
// ## Handoff draft, does not bundle that handoff as in-progress notes —
// a handoff never travels as the open phase's notes, as it never did.
func TestResumeOpenPhaseDoesNotBundleAHandoffAsNotes(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"🔳\"\n---\nDo the first thing.\n")
	writePhaseFile(t, dir, "phase-1.result.md",
		"## Handoff\n\nA draft, but the phase is still open.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.True(t, got.HasPhase)
	assert.Equal(t, PhaseNumber("1"), got.N)
	assert.Empty(t, got.Notes,
		"a result's ## Handoff is not bundled as in-progress notes")
}

// TestResumeKeepsAPreConventionSpecVerbatim: a phase file written before
// the front-matter convention, whose prose opens with a --- delimited
// block that is not valid phase front matter, keeps its whole content as
// the spec — the front-matter strip applies only when the file really
// carries front matter.
func TestResumeKeepsAPreConventionSpecVerbatim(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nnot a phase mapping, just prose\n---\nmore prose.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.Contains(t, got.Spec, "not a phase mapping, just prose")
}

// TestResumeReportsNoOpenPhaseWhenEveryPhaseFileStatusIsDone: a folder
// plan whose every phase-*.md carries a done status, with no result
// files at all, reports no open phase — the phase-file status is the
// done-signal in its own right.
func TestResumeReportsNoOpenPhaseWhenEveryPhaseFileStatusIsDone(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"✅\"\n---\nFirst.\n")
	writePhaseFile(t, dir, "phase-2.md",
		"---\nn: 2\ntitle: Second\nstatus: \"⛔\"\n---\nSecond.\n")

	got, err := Resume(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.False(t, got.HasPhase)
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
	assert.Equal(t, "Second", got.Title)
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

// TestResumeSurfacesAnUnreadableResultFile is CLAUDE.md's Defensive
// Code rule, driven for Resume's `case !os.IsNotExist(err)` branch: a
// phase-N.result.md that exists but cannot be read as a plain file —
// here, a directory sitting where the file belongs — must surface the
// read error rather than being silently treated as "not done yet".
func TestResumeSurfacesAnUnreadableResultFile(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md", "Do the first thing.")
	require.NoError(t, os.Mkdir(
		filepath.Join(dir, "phase-1.result.md"), 0o750))

	_, err := Resume(dir, []byte(folderPlanNoLedger))

	require.Error(t, err)
}

// TestPhaseSpecNumbersOrdersNumericallyThenBySplitToken is
// phaseSpecNumbers' own dedicated test, pinning the ordering rule
// Resume's callers rely on directly rather than only through Resume's
// end-to-end cases: leading digits first, then the full token breaks
// a tie within a split phase, and a companion result file or an
// unrelated name is never counted as a spec.
func TestPhaseSpecNumbersOrdersNumericallyThenBySplitToken(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-10.md", "")
	writePhaseFile(t, dir, "phase-2.md", "")
	writePhaseFile(t, dir, "phase-3b.md", "")
	writePhaseFile(t, dir, "phase-3a.md", "")
	writePhaseFile(t, dir, "phase-3.result.md", "")
	writePhaseFile(t, dir, "notaphase.txt", "")

	got, err := phaseSpecNumbers(dir)

	require.NoError(t, err)
	assert.Equal(t, []string{"2", "3a", "3b", "10"}, got)
}

// TestPhasesFromDirAssemblesFromFrontMatterAndExecutionRows: a
// ledger-free folder plan's phases come from its phase-*.md front
// matter, ordered numerically, each enriched with its Execution row —
// so a phase whose number the table names carries HasExecutionRow, and
// one it omits does not.
func TestPhasesFromDirAssemblesFromFrontMatterAndExecutionRows(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-2.md",
		"---\nn: 2\ntitle: Second\nstatus: \"🔲\"\n---\nBody two.\n")
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"✅\"\n---\nBody one.\n")
	writePhaseFile(t, dir, "phase-3.md",
		"---\nn: 3\ntitle: Third\nstatus: \"🔲\"\n---\nBody three.\n")

	got, err := PhasesFromDir(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, PhaseNumber("1"), got[0].N)
	assert.Equal(t, "First", got[0].Title)
	assert.Equal(t, "✅", got[0].Status)
	assert.True(t, got[0].HasExecutionRow, "phase 1 has an Execution row")
	assert.Equal(t, "sonnet", got[0].Tier)
	assert.True(t, got[1].HasExecutionRow, "phase 2 has an Execution row")
	assert.False(t, got[2].HasExecutionRow, "phase 3 has no Execution row")
}

// TestPhasesFromDirReturnsNilWhenNoPhaseFiles: a directory with no
// phase-N.md files yields no phases, so a caller leaves a flat or
// ledgered plan's own reading untouched.
func TestPhasesFromDirReturnsNilWhenNoPhaseFiles(t *testing.T) {
	dir := t.TempDir()

	got, err := PhasesFromDir(dir, []byte(folderPlanNoLedger))

	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestPhaseFilenameMismatchesFlagsOnlyADivergentN is Phase 3's RED:
// frit derives a phase's number from its phase-N.md filename, while
// the generated `## Phases` catalog renders from front-matter `n`. A
// synced phase-1.md (n: 1) reports no mismatch; a skewed phase-2.md
// (n: 5, filename still says 2) does. PhaseFilenameMismatches does not
// exist yet.
func TestPhaseFilenameMismatchesFlagsOnlyADivergentN(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"✅\"\n---\nBody one.\n")
	writePhaseFile(t, dir, "phase-2.md",
		"---\nn: 5\ntitle: Second\nstatus: \"🔲\"\n---\nBody two.\n")

	got, err := PhaseFilenameMismatches(dir)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "2", got[0].FileToken)
	assert.Equal(t, "5", got[0].FrontMatterN)
	assert.False(t, got[0].Result, "want the mismatch attributed to phase-2.md, not a result file")
}

// TestPhaseFilenameMismatchesFlagsADivergentResultN is the result-file
// half of the RED above: the `## Phases` catalog globs both
// phase-*.md and phase-*.result.md and sorts on each file's own
// front-matter `n`, so a phase-N.result.md whose `n` disagrees with
// its filename is just as load-bearing as its spec — a split-phase
// renumbering that touches the spec but not its already-closed record
// is exactly this drift. A synced result file (n matches) reports
// nothing; a skewed one is flagged with Result: true so a caller can
// tell it apart from a spec mismatch.
func TestPhaseFilenameMismatchesFlagsADivergentResultN(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"✅\"\n---\nBody one.\n")
	writePhaseFile(t, dir, "phase-1.result.md",
		"---\nn: 5\ntitle: First\nstatus: \"✅\"\nresult: true\nsummary: Done.\n---\n## Handoff\n\nDone.\n")

	got, err := PhaseFilenameMismatches(dir)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "1", got[0].FileToken)
	assert.Equal(t, "5", got[0].FrontMatterN)
	assert.True(t, got[0].Result, "want the mismatch attributed to phase-1.result.md")
}

// TestPhaseFilenameMismatchesSkipsAnAbsentResultFile: an open phase's
// phase-N.md has no phase-N.result.md yet, and that must not be
// reported as a mismatch or error the scan.
func TestPhaseFilenameMismatchesSkipsAnAbsentResultFile(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md",
		"---\nn: 1\ntitle: First\nstatus: \"🔲\"\n---\nBody one.\n")

	got, err := PhaseFilenameMismatches(dir)

	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestPhaseFilenameMismatchesSkipsAPreConventionFile mirrors the
// leniency PhasesFromDir applies: a phase file written before the
// {n, title, status} convention carries no front matter to compare, so
// it is skipped rather than erroring the whole scan.
func TestPhaseFilenameMismatchesSkipsAPreConventionFile(t *testing.T) {
	dir := t.TempDir()
	writePhaseFile(t, dir, "phase-1.md", "No front matter here.\n")

	got, err := PhaseFilenameMismatches(dir)

	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestParsePhaseFileDecodesFrontMatter: a phase-N.md's own
// {n, title, status} front matter decodes into a Phase, and its prose
// comes back as the body so a caller need not parse the source again.
func TestParsePhaseFileDecodesFrontMatter(t *testing.T) {
	got, body, err := parsePhaseFile(
		[]byte("---\nn: 3b\ntitle: Split\nstatus: \"🔳\"\n---\nBody.\n"))

	require.NoError(t, err)
	assert.Equal(t, PhaseNumber("3b"), got.N)
	assert.Equal(t, "Split", got.Title)
	assert.Equal(t, "🔳", got.Status)
	assert.Equal(t, "Body.", strings.TrimSpace(string(body)))
}

// TestParsePhaseFileReportsErrNoFrontMatter: a file with no front-matter
// block is not a phase file, the way Parse reports a non-plan.
func TestParsePhaseFileReportsErrNoFrontMatter(t *testing.T) {
	_, _, err := parsePhaseFile([]byte("Just a body, no front matter.\n"))

	require.ErrorIs(t, err, ErrNoFrontMatter)
}

// TestExecutionRowForReadsTheNamedPhasesTierAndGate is
// executionRowFor's own dedicated test.
func TestExecutionRowForReadsTheNamedPhasesTierAndGate(t *testing.T) {
	body := []byte("## Execution\n\n" +
		"| Phase | Design | Implement | Gate |\n" +
		"| --- | --- | --- | --- |\n" +
		"| 2 second | sonnet | opus | test two |\n")

	tier, gate, ok := executionRowFor(body, PhaseNumber("2"))

	assert.True(t, ok)
	assert.Equal(t, "opus", tier)
	assert.Equal(t, "test two", gate)
}

// TestExecutionRowForMissingPhaseReportsNotOK pins the other side: a
// phase number the table carries no row for reports ok=false rather
// than a zero-value row indistinguishable from an empty tier and gate.
func TestExecutionRowForMissingPhaseReportsNotOK(t *testing.T) {
	body := []byte("## Execution\n\n" +
		"| Phase | Design | Implement | Gate |\n" +
		"| --- | --- | --- | --- |\n" +
		"| 2 second | sonnet | opus | test two |\n")

	_, _, ok := executionRowFor(body, PhaseNumber("9"))

	assert.False(t, ok)
}
