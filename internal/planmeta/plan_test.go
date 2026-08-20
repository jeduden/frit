package planmeta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const realPlan = `---
id: 2608142306
title: The fleet index — discover every repo, worktree and branch
status: "🔳"
summary: >-
  Build the index frit is named for: walk a root for git
  repositories, enumerate every worktree and branch.
model: opus
depends-on: [2606122030, 2606131150]
---
# The fleet index

## Goal

Answer three questions.
`

func TestParseReadsEveryFrontMatterField(t *testing.T) {
	got, err := Parse([]byte(realPlan))

	require.NoError(t, err)
	assert.Equal(t, int64(2608142306), got.ID)
	assert.Equal(t,
		"The fleet index — discover every repo, worktree and branch",
		got.Title)
	assert.Equal(t, "🔳", got.Status)
	assert.Equal(t, "opus", got.Model)
	assert.Equal(t, []int64{2606122030, 2606131150}, got.DependsOn)
	assert.Contains(t, got.Summary, "walk a root for git")
}

func TestParseReadsTheGoalFromTheBody(t *testing.T) {
	got, err := Parse([]byte(realPlan))

	require.NoError(t, err)
	assert.Equal(t, "Answer three questions.", got.Goal)
}

func TestParseReadsAMultiLineGoalAsOneLine(t *testing.T) {
	src := "---\nid: 7\ntitle: T\nstatus: \"🔲\"\n---\n# T\n\n" +
		"## Goal\n\nA goal that wraps\nover two lines.\n\n## Context\n\nx\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	assert.Equal(t, "A goal that wraps over two lines.", got.Goal)
}

func TestParseReadsGoalWithInlineCode(t *testing.T) {
	src := "---\nid: 7\ntitle: T\nstatus: \"🔲\"\n---\n# T\n\n" +
		"## Goal\n\nMake `frit show` print it.\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	assert.Equal(t, "Make frit show print it.", got.Goal)
}

func TestParseLeavesGoalEmptyWhenAbsent(t *testing.T) {
	src := "---\nid: 7\ntitle: T\nstatus: \"🔲\"\n---\n# T\n\n## Tasks\n\n1. x\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	assert.Empty(t, got.Goal)
}

func TestParseFoldsTheBlockScalarSummaryIntoOneLine(t *testing.T) {
	got, err := Parse([]byte(realPlan))

	require.NoError(t, err)
	assert.NotContains(t, got.Summary, "\n",
		">- folds the block, so a summary is one line")
}

func TestParseDefaultsMissingOptionalFields(t *testing.T) {
	src := "---\nid: 100\ntitle: Small\nstatus: \"✅\"\n---\n# Small\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	assert.Equal(t, int64(100), got.ID)
	assert.Empty(t, got.Model)
	assert.Empty(t, got.Summary)
	assert.Empty(t, got.DependsOn)
}

func TestParseRejectsAFileWithNoFrontMatter(t *testing.T) {
	_, err := Parse([]byte("# Just a heading\n\nBody.\n"))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoFrontMatter)
}

func TestParseRejectsASchemaTemplate(t *testing.T) {
	// proto.md carries CUE type expressions where a plan carries
	// values. It must not be mistaken for a plan.
	proto := "---\nid: 'int & >=2601010000'\n" +
		"title: 'string & != \"\"'\n---\n# ?\n"

	_, err := Parse([]byte(proto))

	require.Error(t, err)
}

func TestParseRejectsMalformedYAML(t *testing.T) {
	_, err := Parse([]byte("---\nid: [unclosed\n---\n# x\n"))

	require.Error(t, err)
}

const phasedPlan = `---
id: 2607232057
title: Plans get first-class phases
status: "🔳"
phases:
  - { n: 1, title: 'Schema', status: "✅" }
  - { n: 2, title: 'The catalog', status: "✅" }
  - { n: 3, title: 'The three skills', status: "🔳" }
  - { n: 4, title: 'Docs', status: "🔲" }
---
# Plans get first-class phases

## Goal

First-class phases.
`

func TestParseReadsThePhaseLedger(t *testing.T) {
	got, err := Parse([]byte(phasedPlan))

	require.NoError(t, err)
	require.Len(t, got.Phases, 4)
	assert.Equal(t, PhaseNumber("1"), got.Phases[0].N)
	assert.Equal(t, "Schema", got.Phases[0].Title)
	assert.Equal(t, "✅", got.Phases[0].Status)
	assert.Equal(t, "🔲", got.Phases[3].Status)
}

// TestParseReadsAnAlphanumericPhaseNumber is the regression: a phase
// split into 3a/3b carries a non-integer number, and the whole plan
// must still parse rather than being dropped from the index over it.
func TestParseReadsAnAlphanumericPhaseNumber(t *testing.T) {
	src := "---\nid: 1\ntitle: Split phase\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 3, title: 'Third', status: \"✅\" }\n" +
		"  - { n: '3b', title: 'Third, part two', status: \"🔲\" }\n" +
		"---\n# Split\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	require.Len(t, got.Phases, 2)
	assert.Equal(t, PhaseNumber("3"), got.Phases[0].N)
	assert.Equal(t, PhaseNumber("3b"), got.Phases[1].N)

	phase, ok := got.FirstOpenPhase()
	require.True(t, ok)
	assert.Equal(t, PhaseNumber("3b"), phase.N)
}

func TestParseDefaultsAMissingPhaseLedgerToEmpty(t *testing.T) {
	got, err := Parse([]byte(realPlan))

	require.NoError(t, err)
	assert.Empty(t, got.Phases)
}

// TestFirstOpenPhaseSkipsDoneAndStopsAtTheFirstOpen is the rule frit
// next follows: the first phase not at ✅, whatever open status it
// carries.
func TestFirstOpenPhaseSkipsDoneAndStopsAtTheFirstOpen(t *testing.T) {
	got, err := Parse([]byte(phasedPlan))
	require.NoError(t, err)

	phase, ok := got.FirstOpenPhase()

	require.True(t, ok)
	assert.Equal(t, PhaseNumber("3"), phase.N)
	assert.Equal(t, "The three skills", phase.Title)
}

// TestFirstOpenPhaseSkipsASupersededPhase: a phase replaced by another
// is not work to pick up, so next steps over it to the first real open
// phase rather than pointing an executor at abandoned work.
func TestFirstOpenPhaseSkipsASupersededPhase(t *testing.T) {
	p := Plan{Phases: []Phase{
		{N: "1", Status: StatusDone},
		{N: "2", Status: StatusSuperseded},
		{N: "3", Status: StatusNotStarted},
	}}

	phase, ok := p.FirstOpenPhase()

	require.True(t, ok)
	assert.Equal(t, PhaseNumber("3"), phase.N,
		"a superseded phase is skipped, not returned")
}

func TestFirstOpenPhaseReportsNoneWhenEveryPhaseIsDone(t *testing.T) {
	p := Plan{Phases: []Phase{
		{N: "1", Status: StatusDone},
		{N: "2", Status: StatusDone},
	}}

	_, ok := p.FirstOpenPhase()

	assert.False(t, ok, "all phases done means no open phase")
}

func TestFirstOpenPhaseReportsNoneForaPlanWithNoLedger(t *testing.T) {
	_, ok := Plan{}.FirstOpenPhase()

	assert.False(t, ok)
}

func TestStatusHelpersNameTheLifecycle(t *testing.T) {
	assert.True(t, Plan{Status: "🔳"}.InProgress())
	assert.True(t, Plan{Status: "✅"}.Done())
	assert.True(t, Plan{Status: "🔲"}.NotStarted())
	assert.True(t, Plan{Status: "⛔"}.Superseded())

	open := Plan{Status: "🔲"}
	assert.False(t, open.Done())
	assert.False(t, open.InProgress())
}

const phasedPlanWithExecution = `---
id: 2607232057
title: Plans get first-class phases
status: "🔳"
phases:
  - { n: 1, title: 'Schema', status: "✅" }
  - { n: 2, title: 'The catalog', status: "🔳" }
  - { n: '3b', title: 'Split third', status: "🔲" }
---
# Plans get first-class phases

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase        | Design | Implement | Gate that catches a wrong answer   |
| ------------ | ------ | --------- | ----------------------------------- |
| 1 schema     | sonnet | sonnet    | schema unit tests                   |
| 2 the catalog | sonnet | opus     | golden catalog diff test            |
| 3b split third | opus | sonnet    | test the split phase runs alone     |

## Non-goals

Nothing yet.
`

func TestParseReadsTierAndGateFromTheExecutionTable(t *testing.T) {
	got, err := Parse([]byte(phasedPlanWithExecution))

	require.NoError(t, err)
	require.Len(t, got.Phases, 3)
	assert.True(t, got.Phases[0].HasExecutionRow)
	assert.Equal(t, "sonnet", got.Phases[0].Tier)
	assert.Equal(t, "schema unit tests", got.Phases[0].Gate)
}

// TestParseTierIsTheMostDemandingColumn: phase 2's row names sonnet
// for Design and opus for Implement, so the phase's tier is opus, the
// more demanding of the two.
func TestParseTierIsTheMostDemandingColumn(t *testing.T) {
	got, err := Parse([]byte(phasedPlanWithExecution))

	require.NoError(t, err)
	assert.Equal(t, "opus", got.Phases[1].Tier)
}

// TestParseMatchesExecutionRowByLeadingPhaseNumber is the regression:
// a row's first cell carries a title after the number ("3b split
// third"), and an alphanumeric phase number still matches it.
func TestParseMatchesExecutionRowByLeadingPhaseNumber(t *testing.T) {
	got, err := Parse([]byte(phasedPlanWithExecution))

	require.NoError(t, err)
	assert.True(t, got.Phases[2].HasExecutionRow)
	assert.Equal(t, "opus", got.Phases[2].Tier)
	assert.Equal(t, "test the split phase runs alone", got.Phases[2].Gate)
}

// TestParseLeavesAPhaseWithNoExecutionRowUnflagged is the gap the
// report layer surfaces as a Problem rather than a blank tier: a
// phase whose number never appears in the Execution table gets no
// tier or gate invented for it.
func TestParseLeavesAPhaseWithNoExecutionRowUnflagged(t *testing.T) {
	src := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 1, title: 'Only phase', status: \"🔳\" }\n" +
		"---\n# T\n\n## Execution\n\n" +
		"| Phase | Design | Implement | Gate |\n" +
		"| --- | --- | --- | --- |\n" +
		"| 2 other phase | sonnet | sonnet | unrelated |\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	require.Len(t, got.Phases, 1)
	assert.False(t, got.Phases[0].HasExecutionRow)
	assert.Empty(t, got.Phases[0].Tier)
	assert.Empty(t, got.Phases[0].Gate)
}

// TestParseSkipsTheExecutionTableWhenThePlanCarriesNoLedger: a plan
// with no front-matter phases: has nothing to attach a row to, so the
// table is never even parsed.
func TestParseSkipsTheExecutionTableWhenThePlanCarriesNoLedger(t *testing.T) {
	got, err := Parse([]byte(realPlan))

	require.NoError(t, err)
	assert.Empty(t, got.Phases)
}

const sectionDerivedPlan = `---
id: 2608201200
title: A plan tracked by sections, not a ledger
status: "🔳"
---
# A plan tracked by sections, not a ledger

## Phase 1: First sitting

Do the first thing.

A second paragraph, still phase one.

## Phase 2: Second sitting

Do the second thing.

## Execution

| Phase          | Design | Implement | Gate           |
| -------------- | ------ | --------- | -------------- |
| 1 first sitting | sonnet | sonnet   | test one       |
| 2 second sitting | sonnet | opus    | test two       |
`

func TestParseDerivesPhasesFromHeadingsWhenNoLedger(t *testing.T) {
	got, err := Parse([]byte(sectionDerivedPlan))

	require.NoError(t, err)
	require.Len(t, got.Phases, 2)
	assert.Equal(t, PhaseNumber("1"), got.Phases[0].N)
	assert.Equal(t, "First sitting", got.Phases[0].Title)
	assert.Equal(t, PhaseNumber("2"), got.Phases[1].N)
	assert.Equal(t, "Second sitting", got.Phases[1].Title)
}

// TestParseLeavesADerivedPhaseWithNoStatus: section state carries no
// status, so a derived phase is left blank rather than an invented
// "not started" — see FirstOpenPhase for what that blank means for
// what next points at.
func TestParseLeavesADerivedPhaseWithNoStatus(t *testing.T) {
	got, err := Parse([]byte(sectionDerivedPlan))

	require.NoError(t, err)
	assert.Empty(t, got.Phases[0].Status)
	assert.Empty(t, got.Phases[1].Status)
}

// TestFirstOpenPhaseOnADerivedLedgerPointsAtTheFirst: with no status
// to skip by, the first derived phase is the most next can promise,
// whatever a person or an agent has actually finished.
func TestFirstOpenPhaseOnADerivedLedgerPointsAtTheFirst(t *testing.T) {
	got, err := Parse([]byte(sectionDerivedPlan))
	require.NoError(t, err)

	phase, ok := got.FirstOpenPhase()

	require.True(t, ok)
	assert.Equal(t, PhaseNumber("1"), phase.N)
}

func TestParseReadsThePhaseBody(t *testing.T) {
	got, err := Parse([]byte(sectionDerivedPlan))

	require.NoError(t, err)
	assert.Equal(t, "Do the first thing.\n\nA second paragraph, still phase one.",
		got.Phases[0].Body)
	assert.Equal(t, "Do the second thing.", got.Phases[1].Body)
}

// TestParseAlsoReadsThePhaseBodyWithAFrontMatterLedger: the body walk
// runs for an explicit ledger too, not only a derived one — the two
// are independent seams over the same headings.
func TestParseAlsoReadsThePhaseBodyWithAFrontMatterLedger(t *testing.T) {
	src := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 1, title: 'One', status: \"🔳\" }\n" +
		"---\n# T\n\n## Phase 1: One\n\nThe body of phase one.\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	require.Len(t, got.Phases, 1)
	assert.Equal(t, "The body of phase one.", got.Phases[0].Body)
}

// TestParseLeavesPhaseBodyEmptyWhenSectionAbsent: a ledger entry with
// no matching `## Phase N` section gets no body invented for it.
func TestParseLeavesPhaseBodyEmptyWhenSectionAbsent(t *testing.T) {
	src := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 1, title: 'One', status: \"🔳\" }\n" +
		"---\n# T\n\nNo phase sections here.\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	assert.Empty(t, got.Phases[0].Body)
}

// TestParseDerivesAnAlphanumericPhaseNumberFromAHeading is the same
// regression as the front-matter ledger, for the derived path: a
// heading titled "Phase 3b: Split" must not swallow the colon or the
// "Split" word into the number.
func TestParseDerivesAnAlphanumericPhaseNumberFromAHeading(t *testing.T) {
	src := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\n---\n# T\n\n" +
		"## Phase 3b: Split third\n\nThe split half.\n"

	got, err := Parse([]byte(src))

	require.NoError(t, err)
	require.Len(t, got.Phases, 1)
	assert.Equal(t, PhaseNumber("3b"), got.Phases[0].N)
	assert.Equal(t, "Split third", got.Phases[0].Title)
}

// TestParseKeepsTheFrontMatterLedgerOverDerivingOne: a plan that
// carries both an explicit ledger and `## Phase N` headings trusts
// the ledger — its statuses are load-bearing, a derived one has none.
func TestParseKeepsTheFrontMatterLedgerOverDerivingOne(t *testing.T) {
	src := "---\nid: 1\ntitle: T\nstatus: \"🔳\"\nphases:\n" +
		"  - { n: 1, title: 'One', status: \"✅\" }\n" +
		"  - { n: 2, title: 'Two', status: \"🔲\" }\n" +
		"---\n# T\n\n## Phase 1: One\n\nBody one.\n\n" +
		"## Phase 2: Two\n\nBody two.\n"

	got, err := Parse([]byte(src))
	require.NoError(t, err)
	require.Len(t, got.Phases, 2)

	phase, ok := got.FirstOpenPhase()
	require.True(t, ok)
	assert.Equal(t, PhaseNumber("2"), phase.N,
		"the ledger's own ✅ on phase one is trusted, not overwritten")
}

func TestIsProtoRecognisesTheTemplateByName(t *testing.T) {
	assert.True(t, IsProto("plan/proto.md"))
	assert.True(t, IsProto("proto.md"))
	assert.False(t, IsProto("plan/2608142306_fleet-index.md"))
	assert.False(t, IsProto("plan/prototype-notes.md"))
}
