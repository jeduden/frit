package report

import (
	"testing"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
)

// TestOpenNextActionNamesStartOnlyWhenNoLaneIsLiveAndKnown pins the
// verb a --json consumer runs when open raised nothing. `frit start
// <id>` is named only when presence was read cleanly and no lane
// exists — nudge would refuse a laneless plan, so start is the rung. It
// is empty when a lane was focused (watch it, do not escalate) and
// empty when presence could not be read, because a lane may be running
// behind the socket open could not reach.
func TestOpenNextActionNamesStartOnlyWhenNoLaneIsLiveAndKnown(t *testing.T) {
	laneless := NewOpen("/fleet", "atlas", 7, "Shader unit")
	assert.Equal(t, "frit start 7", laneless.NextAction,
		"a plan with no live lane starts, not opens")

	focused := NewOpen("/fleet", "atlas", 7, "Shader unit")
	focused.Focus(herdr.Lane{Pane: herdr.Pane{PaneID: "wZ:p1"}})
	assert.Equal(t, "", focused.NextAction,
		"a focused lane is watched, not escalated")

	unknown := NewOpen("/fleet", "atlas", 7, "Shader unit")
	unknown.PresenceUnknown()
	assert.Equal(t, "", unknown.NextAction,
		"unread presence leaves a lane possible, so start is not named")
}

// TestOpenNextActionSurvivesANonPresenceProblem pins that a repo-read
// failure carried into the report — an unrelated broken checkout under
// --all — does not suppress the escalation. open still read the target
// plan's presence and found no lane, so start stays named; only an
// unread presence (PresenceUnknown) clears it.
func TestOpenNextActionSurvivesANonPresenceProblem(t *testing.T) {
	doc := NewOpen("/fleet", "atlas", 7, "Shader unit")
	doc.AddProblem("beacon", assert.AnError)
	assert.Equal(t, "frit start 7", doc.NextAction,
		"a repo-read problem does not make the target plan's presence unknown")
}

// TestStartNextActionIsAPureProjectionOfHandoff pins the derivation the
// handoff setter and both renderers share, independent of the transition
// methods: the running handoff yields frit open <id>, every other
// handoff yields "". next_action cannot disagree with handoff because it
// is this function of it.
func TestStartNextActionIsAPureProjectionOfHandoff(t *testing.T) {
	assert.Equal(t, "frit open 7", startNextAction(HandoffRunning, 7))
	assert.Equal(t, "", startNextAction(HandoffPreview, 7))
	assert.Equal(t, "", startNextAction(HandoffNone, 7))
}

// TestOpenNextActionIsAPureProjection pins open's derivation: the start
// rung is named only for a plan with no focused lane whose presence was
// read; a focused lane or unread presence yields "".
func TestOpenNextActionIsAPureProjection(t *testing.T) {
	assert.Equal(t, "frit start 7", openNextAction(false, false, 7))
	assert.Equal(t, "", openNextAction(true, false, 7))
	assert.Equal(t, "", openNextAction(false, true, 7))
	assert.Equal(t, "", openNextAction(true, true, 7))
}

// TestStartHandoffTracksTheThreeTransitions pins the one axis a
// consumer keys on instead of re-deriving it from started/refused: a
// fresh escalation previews what --go would run, MarkStarted flips it
// to the prompt now running in a spawned agent's pane, and Refuse
// flips it to nothing running at all.
func TestStartHandoffTracksTheThreeTransitions(t *testing.T) {
	doc := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	assert.Equal(t, HandoffPreview, doc.Handoff, "a fresh doc previews a --go run")

	doc.MarkStarted("wZ:p1")
	assert.Equal(t, HandoffRunning, doc.Handoff,
		"a started escalation is running in the spawned agent's pane")

	refused := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	refused.Refuse("already held")
	assert.Equal(t, HandoffNone, refused.Handoff,
		"a refusal runs nothing")
}

// TestStartNextActionTracksTheThreeTransitions pins the verb a
// consumer runs instead of the already-dispatched prompt: empty on a
// preview, `frit open <id>` once MarkStarted flips the handoff to
// running, and empty again on a refusal, where prompt is still the
// recipe.
func TestStartNextActionTracksTheThreeTransitions(t *testing.T) {
	doc := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	assert.Equal(t, "", doc.NextAction, "a fresh doc names no next action")

	doc.MarkStarted("wZ:p1")
	assert.Equal(t, "frit open 7", doc.NextAction,
		"a started escalation hands the caller the verb to watch it with")

	refused := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	refused.Refuse("already held")
	assert.Equal(t, "", refused.NextAction, "a refusal names no next action")
}

// TestStartPromptDispatchedTracksTheThreeTransitions pins the boolean a
// consumer checks instead of re-deriving "is this prompt mine to run"
// from handoff: false on a fresh preview, true once MarkStarted sends
// it into a spawned agent's pane, and false again on a refusal, where
// Prompt is still the caller's recipe.
func TestStartPromptDispatchedTracksTheThreeTransitions(t *testing.T) {
	doc := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	assert.False(t, doc.PromptDispatched, "a fresh doc has not dispatched its prompt")

	doc.MarkStarted("wZ:p1")
	assert.True(t, doc.PromptDispatched,
		"a started escalation has sent its prompt into the pane")

	refused := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	refused.Refuse("already held")
	assert.False(t, refused.PromptDispatched,
		"a refusal never dispatched its prompt")
}

// TestNewStartRendersAnEmptyPhaseAsWholePlan: a phase-less plan is
// dispatched as one whole-plan prompt, so its doc reports that rather
// than a blank phase cell — blank reads as a missing field, not a
// deliberate whole-plan dispatch.
func TestNewStartRendersAnEmptyPhaseAsWholePlan(t *testing.T) {
	doc := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Prompt: "/plan-phase 7"}, true)
	assert.Equal(t, WholePlanPhase, doc.Phase)
}

// TestNewNudgeRendersAnEmptyPhaseAsWholePlan is NewStart's rule for
// nudge's document.
func TestNewNudgeRendersAnEmptyPhaseAsWholePlan(t *testing.T) {
	doc := NewNudge("/fleet", "atlas", 7, "Shader unit",
		"", "sonnet", "/plan-phase 7", true)
	assert.Equal(t, WholePlanPhase, doc.Phase)
}
