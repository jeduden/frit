package report

import (
	"github.com/jeduden/frit/internal/discovery"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestBoardAddProblemRecordsAnUnreadRepository: a repository whose
// plans could not be read still surfaces on the board, alongside the
// rows that were.
func TestBoardAddProblemRecordsAnUnreadRepository(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddProblem("broken", assert.AnError)

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "broken", doc.Problems[0].Repo)
}

// TestHostOfReturnsEmptyForAKeyWithNoColon: hostOf pulls the machine
// out of a host:repo:id key; a key that never carries one names no
// host rather than panicking on the missing separator.
func TestHostOfReturnsEmptyForAKeyWithNoColon(t *testing.T) {
	assert.Empty(t, hostOf("no-colon-here"))
}

// TestBoardMarkUnprovenNamesTheWayOut: MarkUnproven reprojects a
// row's NextAction to the same wait-or-take-over wording open already
// gives an unprovable hold, matched on (repo, id) since two
// repositories can share a plan id (S74) — a mismatch is a no-op,
// leaving every other row untouched.
func TestBoardMarkUnprovenNamesTheWayOut(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "", "", false)
	assert.Empty(t, doc.Plans[0].NextAction)

	doc.MarkUnproven("wrong-repo", deadHeldPlan.ID)
	assert.Empty(t, doc.Plans[0].NextAction, "a repo mismatch is a no-op")

	doc.MarkUnproven(deadHeldPlan.Repo, deadHeldPlan.ID)
	assert.Equal(t, unprovenNextAction(deadHeldPlan.ID), doc.Plans[0].NextAction)
}

// TestBoardAddPlanReportsNoAskByDefault: a row nobody has asked
// carries ask_state "none" and an empty answer — every key present,
// per the JSON contract, so a consumer branches on the field.
func TestBoardAddPlanReportsNoAskByDefault(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "", "", false)

	assert.Equal(t, "none", doc.Plans[0].AskState)
	assert.Empty(t, doc.Plans[0].Answer)
}

// TestBoardSetAskMarksTheMatchingRowOnly: SetAsk carries a lane's ask
// state and answer onto its row, matched on (repo, id) like
// MarkUnproven, so another repository's same id is left alone.
func TestBoardSetAskMarksTheMatchingRowOnly(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "", "", false)

	doc.SetAsk("wrong-repo", deadHeldPlan.ID, "answered", "in PR #9")
	assert.Equal(t, "none", doc.Plans[0].AskState, "a repo mismatch is a no-op")

	doc.SetAsk(deadHeldPlan.Repo, deadHeldPlan.ID, "answered", "in PR #9")
	assert.Equal(t, "answered", doc.Plans[0].AskState)
	assert.Equal(t, "in PR #9", doc.Plans[0].Answer)
}

// TestBoardAskRemoteSwapsOnlyAnAskedRow: a row carrying an ask whose
// lane runs on another host gets the plain message; a row with no ask
// gains none, and another repository's same id is left alone.
func TestBoardAskRemoteSwapsOnlyAnAskedRow(t *testing.T) {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(deadHeldPlan, "claude", "working", false)
	doc.AddPlan(discovery.Plan{Repo: "atlas", ID: 9}, "", "", false)

	doc.AskRemote("wrong-repo", deadHeldPlan.ID)
	assert.Equal(t, AskCommand(100), doc.Plans[0].Ask, "a repo mismatch is a no-op")
	doc.AskRemote("atlas", 9)
	assert.Empty(t, doc.Plans[1].Ask, "no ask, none gained")

	doc.AskRemote(deadHeldPlan.Repo, deadHeldPlan.ID)
	assert.Equal(t, AskCommandFor(100, true), doc.Plans[0].Ask)
}
