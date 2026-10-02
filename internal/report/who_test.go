package report

import (
	"errors"
	"testing"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWhoCarriesPresenceAndPlan checks the two facts a lane exists to
// join: the agent's honest state, and the plan it sits on.
func TestWhoCarriesPresenceAndPlan(t *testing.T) {
	doc := NewWho("/fleet")
	doc.AddLane(herdr.Lane{
		Pane: herdr.Pane{
			Agent: "claude", Status: herdr.StatusWorking,
			Workspace: "wC", Session: "sess-1", PaneID: "wC:p1",
			Title: "Land the join",
		},
		Root: "/fleet/atlas", Repo: "atlas",
		Branch: "plan/2608161808-herdr-join", PlanID: 2608161808,
	})

	require.Len(t, doc.Lanes, 1)
	lane := doc.Lanes[0]
	assert.Equal(t, "claude", lane.Agent)
	assert.Equal(t, herdr.StatusWorking, lane.Status)
	assert.Equal(t, "atlas", lane.Repo)
	assert.Equal(t, int64(2608161808), lane.PlanID)
	assert.Equal(t, "wC:p1", lane.Pane)
}

// TestWhoReportsUnknownNotIdle is the acceptance criterion in the
// report layer: a pane whose status frit could not read is never
// recorded as idle.
func TestWhoReportsUnknownNotIdle(t *testing.T) {
	doc := NewWho("/fleet")
	doc.AddLane(herdr.Lane{
		Pane: herdr.Pane{Agent: "claude", Status: ""},
	})

	assert.Equal(t, herdr.StatusUnknown, doc.Lanes[0].Status)
}

// TestWhoKeepsAnUnreachableSocket carries the failure in the document,
// so a consumer reading stdout alone can tell "no agents" from "never
// reached the server".
func TestWhoKeepsAnUnreachableSocket(t *testing.T) {
	doc := NewWho("/fleet")
	doc.AddProblem("herdr", errors.New("dial unix: no such file"))

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "herdr", doc.Problems[0].Repo)
}

// TestWhoAddLaneReportsNoAskByDefault: every lane carries ask_state and
// answer, "none" and empty until SetAsk says otherwise.
func TestWhoAddLaneReportsNoAskByDefault(t *testing.T) {
	doc := NewWho("/fleet")
	doc.AddLane(herdr.Lane{Pane: herdr.Pane{PaneID: "wC:p1"}, PlanID: 7})

	assert.Equal(t, "none", doc.Lanes[0].AskState)
	assert.Empty(t, doc.Lanes[0].Answer)
}

// TestWhoSetLastAskMarksTheLaneJustAdded: a pane id is unique only on
// its own host, so SetLastAsk marks the lane AddLane just recorded,
// never an earlier lane from another host that shares its pane id.
func TestWhoSetLastAskMarksTheLaneJustAdded(t *testing.T) {
	doc := NewWho("/fleet")
	doc.AddLane(herdr.Lane{Pane: herdr.Pane{Host: "box", PaneID: "wC:p1"}, PlanID: 5})
	doc.SetLastAsk("none", "")
	doc.AddLane(herdr.Lane{Pane: herdr.Pane{PaneID: "wC:p1"}, PlanID: 7})

	doc.SetLastAsk("answered", "in PR #9")

	assert.Equal(t, "none", doc.Lanes[0].AskState)
	assert.Equal(t, "answered", doc.Lanes[1].AskState)
	assert.Equal(t, "in PR #9", doc.Lanes[1].Answer)
}

// TestWhoSetLastAskWithNoLaneIsANoOp: with nothing added there is no
// lane to mark, and nothing panics.
func TestWhoSetLastAskWithNoLaneIsANoOp(t *testing.T) {
	doc := NewWho("/fleet")

	doc.SetLastAsk("pending", "")

	assert.Empty(t, doc.Lanes)
}
