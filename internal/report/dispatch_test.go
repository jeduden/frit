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
// handoff yields "" unless the hold is unproven. next_action cannot
// disagree with handoff and kind because it is this function of them.
func TestStartNextActionIsAPureProjectionOfHandoff(t *testing.T) {
	assert.Equal(t, "frit open 7", startNextAction(HandoffRunning, HoldNone, 7))
	assert.Equal(t, "", startNextAction(HandoffPreview, HoldNone, 7))
	assert.Equal(t, "", startNextAction(HandoffNone, HoldNone, 7))
}

// TestStartNextActionNamesTheWaitForAnUnprovenRefusal pins phase 2's
// own addition: a refusal (HandoffNone) whose hold reads HoldUnproven
// carries the same wait-or-take-over wording open already gives the
// identical hold, so an agent branches on next_action rather than
// parsing the refusal's own sentence. Every other kind leaves it
// empty — a live or unparked hold already names its own way out
// elsewhere, and naming this wording there would be dishonest.
func TestStartNextActionNamesTheWaitForAnUnprovenRefusal(t *testing.T) {
	got := startNextAction(HandoffNone, HoldUnproven, 7)
	assert.Equal(t, openNextAction(false, false, HoldUnproven, 7), got,
		"start's refusal names the same wording open already gives")

	assert.Equal(t, "", startNextAction(HandoffNone, HoldLive, 7),
		"a live hold names its own way out elsewhere, not this wording")
}

// TestOpenNextActionIsAPureProjection pins open's derivation: the start
// rung is named only for a plan with no focused lane whose presence was
// read; a focused lane or unread presence yields "".
func TestOpenNextActionIsAPureProjection(t *testing.T) {
	assert.Equal(t, "frit start 7", openNextAction(false, false, HoldNone, 7))
	assert.Equal(t, "", openNextAction(true, false, HoldNone, 7))
	assert.Equal(t, "", openNextAction(false, true, HoldNone, 7))
	assert.Equal(t, "", openNextAction(true, true, HoldNone, 7))
}

// TestOpenNextActionResumesALaneThisMachineHolds pins the ladder's
// entry rung honest for #122's own case: a held lane whose token this
// machine holds, with no agent attending it, is HoldResumable, and the
// projection names the very verb that resumes it rather than a bare
// recommendation that would refuse — the same frit start <id> a
// laneless plan gets, now honest for a held one too because start
// itself resumes it (plan 2609011836).
func TestOpenNextActionResumesALaneThisMachineHolds(t *testing.T) {
	assert.Equal(t, "frit start 7",
		openNextAction(false, false, HoldResumable, 7),
		"a token this machine holds makes frit start an honest resume")
}

// TestOpenNextActionNamesTheWaitForALaneItCannotProve pins the honest
// answer for a hold with no token on disk here: frit start would still
// refuse until the takeover window matures, so the projection never
// names it — it names the wait, or the take-over once matured, instead.
func TestOpenNextActionNamesTheWaitForALaneItCannotProve(t *testing.T) {
	got := openNextAction(false, false, HoldUnproven, 7)
	assert.NotEqual(t, "frit start 7", got,
		"a hold this machine cannot prove would still have frit start refuse")
	assert.Contains(t, got, "takeover window",
		"the honest next step for an unprovable hold is to wait it out")
}

// TestOpenNextActionNamesTheLiveAgent pins the honest answer for a hold
// a live agent already attends: the projection says so plainly, never
// naming a start that would either refuse or interrupt a working agent.
func TestOpenNextActionNamesTheLiveAgent(t *testing.T) {
	got := openNextAction(false, false, HoldLive, 7)
	assert.NotEqual(t, "frit start 7", got)
	assert.Contains(t, got, "live agent")
}

// TestOpenNextActionNamesTheParkFirstStepForAnUnparkedHold pins the
// honest answer for a hold whose local lane carries commits past its
// persisted token: `frit start <id>` would still refuse there — S77's
// park-first guard — so the projection never names it; it names
// `frit yield <id>` instead (code review, plan 2609011941: `open` used
// to call this HoldResumable and recommend a `frit start` that would
// refuse the same way an unproven or live-attended hold does).
func TestOpenNextActionNamesTheParkFirstStepForAnUnparkedHold(t *testing.T) {
	got := openNextAction(false, false, HoldUnparked, 7)
	assert.NotEqual(t, "frit start 7", got,
		"a hold with unparked local work would still have frit start refuse")
	assert.Contains(t, got, "frit yield 7",
		"the honest next step for unparked local work is to park it first")
}

// TestOpenNextActionStillStartsAnUnheldLanelessPlan pins the common
// path untouched: a plan with no hold at all is HoldNone, and the
// projection still names frit start <id>, exactly as before this
// plan's held-lane kinds existed.
func TestOpenNextActionStillStartsAnUnheldLanelessPlan(t *testing.T) {
	assert.Equal(t, "frit start 7", openNextAction(false, false, HoldNone, 7))
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

// TestStartSetHoldKindNamesTheWaitOnAnUnprovenRefusal pins the setter a
// refused StartDoc's caller uses once it has read the hold's true kind
// off the same marker and token reads open runs (#122): NextAction
// reprojects to the wait-or-take-over wording for HoldUnproven, and
// stays empty for a kind that already names its own way out (HoldLive).
func TestStartSetHoldKindNamesTheWaitOnAnUnprovenRefusal(t *testing.T) {
	doc := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	doc.Refuse("already held")
	assert.Equal(t, "", doc.NextAction, "unset until the kind is known")

	doc.SetHoldKind(HoldUnproven)
	assert.Equal(t, openNextAction(false, false, HoldUnproven, 7), doc.NextAction)

	live := NewStart("/fleet", "atlas", 7, "Shader unit",
		StartPlan{Phase: "3", Prompt: "/plan-phase 7 3"}, true)
	live.Refuse("already held")
	live.SetHoldKind(HoldLive)
	assert.Equal(t, "", live.NextAction,
		"a live hold names its own way out elsewhere")
}

// TestReleaseRefuseUnprovenNamesTheWaitForATokenlessOwnLane pins
// release's own new way out: a refusal for a hold this lane cannot
// prove — its own checkout, no token — carries the same wording open
// already gives HoldUnproven, in one call alongside the refusal
// itself.
func TestReleaseRefuseUnprovenNamesTheWaitForATokenlessOwnLane(t *testing.T) {
	doc := NewRelease("/fleet", "atlas", 7, "Shader unit", "plan/7")
	doc.RefuseUnproven("carries no token to prove its own lease", 7)

	assert.Equal(t, "carries no token to prove its own lease", doc.Refused)
	assert.Equal(t, openNextAction(false, false, HoldUnproven, 7), doc.NextAction)
}

// TestYieldRefuseUnprovenNamesTheWaitForAForeignHold pins yield's own
// way out: a refusal for a foreign hold with nothing local to park
// carries the same wait-or-take-over wording release's own
// RefuseUnproven gives, in one call alongside the refusal itself.
func TestYieldRefuseUnprovenNamesTheWaitForAForeignHold(t *testing.T) {
	doc := NewYield("/fleet", "atlas", 7, "Shader unit", "plan/7")
	doc.RefuseUnproven("is held live by another lane", 7)

	assert.Equal(t, "is held live by another lane", doc.Refused)
	assert.Equal(t, unprovenNextAction(7), doc.NextAction)
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

// TestAskCommandNamesTheVerbAndSelector pins the one remedy text every
// site that points a reader at the agent shares: the real verb, the
// plan's own selector, and a question, so it runs verbatim — message
// takes its text as a required positional, so a bare `frit message 7`
// would refuse.
func TestAskCommandNamesTheVerbAndSelector(t *testing.T) {
	assert.Equal(t, `frit message 7 "what is your status?"`, AskCommand(7))
	assert.Equal(t, "what is your status?", AskText)
}
