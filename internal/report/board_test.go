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
	doc.AddPlan(deadHeldPlan, "claude", "working")

	assert.Equal(t, AskCommand(100), doc.Plans[0].Ask)
	assert.False(t, doc.Plans[0].Dead, "the live agent still clears dead")
}

// TestBoardAddPlanLeavesAskEmptyWhenNoAgentIsLive: no agent on the
// lane means nobody to ask; the dead reading stands as before.
func TestBoardAddPlanLeavesAskEmptyWhenNoAgentIsLive(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "", "")

	assert.Empty(t, doc.Plans[0].Ask)
	assert.True(t, doc.Plans[0].Dead)
}

// TestBoardAddPlanLeavesAskEmptyForABoundLiveLane: a lane whose bound
// session is live is unambiguous, so its agent earns no ask pointer.
func TestBoardAddPlanLeavesAskEmptyForABoundLiveLane(t *testing.T) {
	bound := deadHeldPlan
	bound.Dead = false
	doc := NewBoard("/fleet", true)
	doc.AddPlan(bound, "claude", "working")

	assert.Empty(t, doc.Plans[0].Ask)
}
