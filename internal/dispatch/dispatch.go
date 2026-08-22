// Package dispatch composes the one thing the ladder ever sends: a
// typed slash command naming a plan and a phase.
//
// The whole prompt is about twenty characters — `/plan-phase <id>
// <phase>` — because the plan already contains the prompt. The tool
// composes it; the user never writes prose. This package is that
// composition, kept pure so the rule "sent text is always a slash
// command" is provable in a unit test with no herdr and no git.
package dispatch

import (
	"fmt"

	"github.com/jeduden/frit/internal/planmeta"
)

// Command is the seed a dispatch verb types for a phase: a slash command
// the plan-phase skill expands by loading only the front matter and the
// one named phase section. It is the only text frit sends. An empty
// phase seeds a whole-plan dispatch, and is trimmed rather than sent as
// a trailing space — the plan-phase skill defaults the phase itself.
func Command(planID int64, phase string) string {
	if phase == "" {
		return fmt.Sprintf("/plan-phase %d", planID)
	}

	return fmt.Sprintf("/plan-phase %d %s", planID, phase)
}

// Phase resolves which phase to dispatch: the override when given,
// otherwise the first phase in the ledger not yet done — the phase an
// executor would pick up. A plan with no ledger and no override has no
// slice to name it by, so it is dispatched whole: an empty phase,
// dispatchable. A phased ledger with no phase left open is genuinely
// nothing to send and reports false, which the caller turns into a
// refusal rather than a guess.
func Phase(phases []planmeta.Phase, override string) (string, bool) {
	if override != "" {
		return override, true
	}
	if len(phases) == 0 {
		return "", true
	}

	open, ok := planmeta.Plan{Phases: phases}.FirstOpenPhase()
	if !ok {
		return "", false
	}

	return string(open.N), true
}
