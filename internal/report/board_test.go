package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBoardAddPlanNamesTheAskForAnAttendedDeadLane: the board is the
// survey a reader consults before touching a lane, so a held lane
// whose bound session is gone but whose branch a live agent still
// works carries the verb that asks that agent — the one source that
// can tell a PR-in-flight from an abandoned lane.
func TestBoardAddPlanNamesTheAskForAnAttendedDeadLane(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "claude", "working", false)

	assert.Equal(t, AskCommand(100), doc.Plans[0].Ask)
	assert.False(t, doc.Plans[0].Dead, "the live agent still clears dead")
}

// TestBoardAddPlanLeavesAskEmptyWhenNoAgentIsLive: no agent on the
// lane means nobody to ask; the dead reading stands as before.
func TestBoardAddPlanLeavesAskEmptyWhenNoAgentIsLive(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "", "", false)

	assert.Empty(t, doc.Plans[0].Ask)
	assert.True(t, doc.Plans[0].Dead)
}

// TestBoardAddPlanLeavesAskEmptyForAnUnvouchedAgent: an agent whose
// status herdr cannot vouch for still clears dead — someone is there —
// but earns no ask, since message refuses exactly that pane.
func TestBoardAddPlanLeavesAskEmptyForAnUnvouchedAgent(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "claude", "unknown", false)

	assert.False(t, doc.Plans[0].Dead, "a pane there still disproves dead")
	assert.Empty(t, doc.Plans[0].Ask, "but one message would refuse is not offered")
}

// TestBoardAddPlanLeavesAskEmptyForABoundLiveLane: a lane whose bound
// session is live is unambiguous, so its agent earns no ask pointer.
func TestBoardAddPlanLeavesAskEmptyForABoundLiveLane(t *testing.T) {
	bound := deadHeldPlan
	bound.Dead = false
	doc := NewBoard("/fleet", true)
	doc.AddPlan(bound, "claude", "working", false)

	assert.Empty(t, doc.Plans[0].Ask)
}

// TestBoardAddPlanWithholdsAskOnIncompletePresenceWithoutRewritingStatus:
// unknown withholds the ask the same way an unvouched status does, but
// AgentStatus still carries the pane's real status — the fleet's own
// presence read being incomplete is not a reason to misreport what
// herdr actually saw a working pane doing.
func TestBoardAddPlanWithholdsAskOnIncompletePresenceWithoutRewritingStatus(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "claude", "working", true)

	assert.False(t, doc.Plans[0].Dead, "the live agent still clears dead")
	assert.Equal(t, "working", doc.Plans[0].AgentStatus,
		"agent_status reports what herdr saw, not what the ask gate withheld")
	assert.Empty(t, doc.Plans[0].Ask,
		"a configured host went unread, so no ask is offered off this read")
}
