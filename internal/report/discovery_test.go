package report

import (
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/herdr"
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
		attendedLane)

	assert.False(t, doc.Plans[0].Dead,
		"a live pane on the lane disproves dead")
}

// TestReadySetPlansStillMarksAnUnattendedDeadLaneDead: a lane no pane
// attends reads dead exactly as before — the takeover candidate it
// always was.
func TestReadySetPlansStillMarksAnUnattendedDeadLaneDead(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		unattendedLane)

	assert.True(t, doc.Plans[0].Dead,
		"no live pane means the dead session still reads as a takeover candidate")
}

// TestPickSetPlansClearsDeadForAnAttendedLane confirms pick reads the
// same reconciled fact ready does, since both back onto cardOf.
func TestPickSetPlansClearsDeadForAnAttendedLane(t *testing.T) {
	doc := NewPick("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		attendedLane)

	assert.False(t, doc.Plans[0].Dead,
		"pick shares cardOf with ready; a live pane clears dead there too")
}

// TestFindSetPlansClearsDeadForAnAttendedLane confirms find reads the
// same reconciled fact ready and pick do, since all three back onto
// cardOf.
func TestFindSetPlansClearsDeadForAnAttendedLane(t *testing.T) {
	doc := NewFind("/fleet", "forge", "underway")
	doc.SetPlans([]discovery.Plan{deadHeldPlan},
		attendedLane)

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

// attendedLane is the presence callback a working pane on the lane
// answers with.
func attendedLane(discovery.Plan) string { return herdr.StatusWorking }

// unattendedLane is the presence callback no live pane answers with.
func unattendedLane(discovery.Plan) string { return "" }

// unvouchedLane is the presence callback a pane herdr cannot vouch for
// answers with: someone is there, but message would refuse them.
func unvouchedLane(discovery.Plan) string { return herdr.StatusUnknown }

// TestReadySetPlansNamesTheAskForAnAttendedDeadLane: a held lane whose
// bound session is gone but whose branch a live pane attends is the
// one git cannot classify — its work may be open as a PR and read
// unlanded all the same — so the card carries the verb that asks the
// agent, runnable verbatim. ready shares cardsOf with pick and find,
// so proving it here proves the pointer for all three.
func TestReadySetPlansNamesTheAskForAnAttendedDeadLane(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan}, attendedLane)

	assert.Equal(t, AskCommand(100), doc.Plans[0].Ask,
		"the ask names the real verb and selector")
}

// TestReadySetPlansLeavesAskEmptyForAnUnattendedDeadLane: no live
// pane means no agent to ask, so the deserted reading stands alone.
func TestReadySetPlansLeavesAskEmptyForAnUnattendedDeadLane(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan}, unattendedLane)

	assert.Empty(t, doc.Plans[0].Ask, "there is no agent to ask")
	assert.True(t, doc.Plans[0].Dead, "and the deserted reading stands")
}

// TestReadySetPlansLeavesAskEmptyForABoundLiveLane: a lane whose bound
// session is still live is not ambiguous — nobody read it deserted —
// so the pane attending it earns no ask pointer.
func TestReadySetPlansLeavesAskEmptyForABoundLiveLane(t *testing.T) {
	bound := deadHeldPlan
	bound.Dead = false
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{bound}, attendedLane)

	assert.Empty(t, doc.Plans[0].Ask, "a live bound lane is unchanged")
}

// TestReadySetPlansLeavesAskEmptyWithNoAttendedRead: a nil presence —
// the fact was never read — offers no ask, since an unread pane is not
// a live one.
func TestReadySetPlansLeavesAskEmptyWithNoAttendedRead(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan}, nil)

	assert.Empty(t, doc.Plans[0].Ask)
}

// TestReadySetPlansLeavesAskEmptyForAnUnvouchedPane: a pane whose
// status herdr cannot vouch for still clears dead — someone is there —
// but earns no ask, since message refuses exactly that pane and the
// card would otherwise hand the reader a command that refuses when
// run.
func TestReadySetPlansLeavesAskEmptyForAnUnvouchedPane(t *testing.T) {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan}, unvouchedLane)

	assert.False(t, doc.Plans[0].Dead, "a pane there still disproves dead")
	assert.Empty(t, doc.Plans[0].Ask, "but one message would refuse is not offered")
}

// TestPickSetPlansNamesTheAskForAnAttendedDeadLane confirms pick reads
// the same pointer ready does, since both back onto cardsOf.
func TestPickSetPlansNamesTheAskForAnAttendedDeadLane(t *testing.T) {
	doc := NewPick("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{deadHeldPlan}, attendedLane)

	assert.Equal(t, AskCommand(100), doc.Plans[0].Ask)
}

// TestFindSetPlansNamesTheAskForAnAttendedDeadLane confirms find reads
// the same pointer ready and pick do, since all three back onto cardsOf.
func TestFindSetPlansNamesTheAskForAnAttendedDeadLane(t *testing.T) {
	doc := NewFind("/fleet", "forge", "underway")
	doc.SetPlans([]discovery.Plan{deadHeldPlan}, attendedLane)

	assert.Equal(t, AskCommand(100), doc.Plans[0].Ask)
}

// TestAskOfIsGatedOnEveryDesertedInput pins askOf's own inputs: the
// deserted reading desertedRefusal fires on — held, confirmed dead,
// not matured — and a pane message would send to, and nothing short
// of all of them.
func TestAskOfIsGatedOnEveryDesertedInput(t *testing.T) {
	stale := deadHeldPlan
	stale.Stale = true
	unheld := deadHeldPlan
	unheld.Held = false

	assert.Equal(t, AskCommand(100), askOf(deadHeldPlan, herdr.StatusWorking))
	assert.Equal(t, AskCommand(100), askOf(deadHeldPlan, herdr.StatusIdle),
		"message reaches an idle pane too")
	assert.Empty(t, askOf(deadHeldPlan, ""), "unattended")
	assert.Empty(t, askOf(deadHeldPlan, herdr.StatusUnknown),
		"message refuses a pane herdr cannot vouch for")
	assert.Empty(t, askOf(stale, herdr.StatusWorking), "a matured window is staleHeld's own cell")
	assert.Empty(t, askOf(unheld, herdr.StatusWorking), "nobody holds it")
}

// TestAskableNamesTheStatusesMessageSendsTo pins the gate to message's
// own rule: working and idle are sent to; unknown and nothing are not.
func TestAskableNamesTheStatusesMessageSendsTo(t *testing.T) {
	assert.True(t, askable(herdr.StatusWorking))
	assert.True(t, askable(herdr.StatusIdle))
	assert.False(t, askable(herdr.StatusUnknown))
	assert.False(t, askable(""))
}
