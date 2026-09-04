package report

import (
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deadHeldPlan is a held plan whose bound session herdr confirms
// gone — the identity fact ready, pick and find each carry through
// cardOf, which this phase reconciles with a live pane.
var deadHeldPlan = discovery.Plan{
	Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
	Held: true, Holds: []string{"plan/100"}, Dead: true,
}

// TestReadySetPlansClearsDeadForAnAttendedLane: ready shares cardOf
// with pick and find, so proving it here proves the reconciliation for
// all three at their one shared site.
func TestReadySetPlansClearsDeadForAnAttendedLane(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		func(discovery.Plan) bool { return true })

	assert.False(t, doc.Plans[0].Dead,
		"a live pane on the lane disproves dead")
}

// TestReadySetPlansStillMarksAnUnattendedDeadLaneDead: a lane no pane
// attends reads dead exactly as before — the takeover candidate it
// always was.
func TestReadySetPlansStillMarksAnUnattendedDeadLaneDead(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		func(discovery.Plan) bool { return false })

	assert.True(t, doc.Plans[0].Dead,
		"no live pane means the dead session still reads as a takeover candidate")
}

// TestPickSetPlansClearsDeadForAnAttendedLane confirms pick reads the
// same reconciled fact ready does, since both back onto cardOf.
func TestPickSetPlansClearsDeadForAnAttendedLane(t *testing.T) {
	doc := NewPick("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		func(discovery.Plan) bool { return true })

	assert.False(t, doc.Plans[0].Dead,
		"pick shares cardOf with ready; a live pane clears dead there too")
}

// TestFindSetPlansClearsDeadForAnAttendedLane confirms find reads the
// same reconciled fact ready and pick do, since all three back onto
// cardOf.
func TestFindSetPlansClearsDeadForAnAttendedLane(t *testing.T) {
	doc := NewFind("/fleet", "forge", "underway")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		func(discovery.Plan) bool { return true })

	assert.False(t, doc.Plans[0].Dead,
		"find shares cardOf with ready; a live pane clears dead there too")
}

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
