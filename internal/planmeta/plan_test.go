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
	assert.Equal(t, 1, got.Phases[0].N)
	assert.Equal(t, "Schema", got.Phases[0].Title)
	assert.Equal(t, "✅", got.Phases[0].Status)
	assert.Equal(t, "🔲", got.Phases[3].Status)
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
	assert.Equal(t, 3, phase.N)
	assert.Equal(t, "The three skills", phase.Title)
}

func TestFirstOpenPhaseReportsNoneWhenEveryPhaseIsDone(t *testing.T) {
	p := Plan{Phases: []Phase{
		{N: 1, Status: StatusDone},
		{N: 2, Status: StatusDone},
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

func TestIsProtoRecognisesTheTemplateByName(t *testing.T) {
	assert.True(t, IsProto("plan/proto.md"))
	assert.True(t, IsProto("proto.md"))
	assert.False(t, IsProto("plan/2608142306_fleet-index.md"))
	assert.False(t, IsProto("plan/prototype-notes.md"))
}
