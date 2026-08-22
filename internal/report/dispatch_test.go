package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStartHandoffTracksTheThreeTransitions pins the one axis a
// consumer keys on instead of re-deriving it from started/refused: a
// fresh escalation previews what --go would run, MarkStarted flips it
// to the prompt now running in a spawned agent's pane, and Refuse
// flips it to nothing running at all.
func TestStartHandoffTracksTheThreeTransitions(t *testing.T) {
	doc := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	assert.Equal(t, "preview", doc.Handoff, "a fresh doc previews a --go run")

	doc.MarkStarted("wZ:p1")
	assert.Equal(t, "running", doc.Handoff,
		"a started escalation is running in the spawned agent's pane")

	refused := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	refused.Refuse("already held")
	assert.Equal(t, "none", refused.Handoff,
		"a refusal runs nothing")
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
