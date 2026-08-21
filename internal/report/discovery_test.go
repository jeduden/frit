package report

import (
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewNextCarriesThePhasesTierAndGate is the seam phase 2 opens:
// what the Execution table names for the target phase rides in the
// wire shape, not just its number, title and status.
func TestNewNextCarriesThePhasesTierAndGate(t *testing.T) {
	doc := NewNext("/fleet", discovery.Plan{
		Repo: "atlas", ID: 100,
		Phases: []planmeta.Phase{
			{
				N: "2", Title: "the tier", Status: "🔳",
				Tier: "opus", Gate: "the gate", HasExecutionRow: true,
			},
		},
	})

	assert.Equal(t, "opus", doc.Phase.Tier)
	assert.Equal(t, "the gate", doc.Phase.Gate)
	assert.Empty(t, doc.Problems, "a phase carrying a row is not a gap")
}

// TestNewNextReportsAMissingExecutionRowAsAProblem: a phase with no
// row is surfaced through Problems, never rendered with a blank tier
// as if the plan had asked for nothing.
func TestNewNextReportsAMissingExecutionRowAsAProblem(t *testing.T) {
	doc := NewNext("/fleet", discovery.Plan{
		Repo: "atlas", ID: 100,
		Phases: []planmeta.Phase{
			{N: "2", Title: "the gap", Status: "🔳"},
		},
	})

	assert.Empty(t, doc.Phase.Tier)
	assert.Empty(t, doc.Phase.Gate)
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
	assert.Contains(t, doc.Problems[0].Message, "phase 2")
	assert.Contains(t, doc.Problems[0].Message, "no Execution row")
}

// TestNextCarriesRescueRefsForStrandedCommits: next lists a plan's
// rescue refs, so commits a scavenge or a yield parked are found
// again — and a plan with none reads as an empty list, never null.
func TestNextCarriesRescueRefsForStrandedCommits(t *testing.T) {
	doc := NewNext("/fleet", discovery.Plan{Repo: "atlas", ID: 7})
	assert.Equal(t, []string{}, doc.Rescue, "no rescue refs by default")

	doc.SetRescue([]string{"refs/frit/rescue/7/box-a"})
	assert.Equal(t, []string{"refs/frit/rescue/7/box-a"}, doc.Rescue)
}

// TestShowCarriesRescueRefsForStrandedCommits: show carries the same
// list for the plan it resolved, so "what blocks this" and "what is
// stranded on this" are answered from the one document.
func TestShowCarriesRescueRefsForStrandedCommits(t *testing.T) {
	doc := NewShow("/fleet", discovery.DepNode{
		Plan: discovery.Plan{Repo: "atlas", ID: 7}, Found: true,
	})
	assert.Equal(t, []string{}, doc.Rescue, "no rescue refs by default")

	doc.SetRescue([]string{"refs/frit/rescue/7/box-a"})
	assert.Equal(t, []string{"refs/frit/rescue/7/box-a"}, doc.Rescue)
}
