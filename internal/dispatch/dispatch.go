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
// one named phase section. It is the only text frit sends.
func Command(planID int64, phase string) string {
	return fmt.Sprintf("/plan-phase %d %s", planID, phase)
}

// Phase resolves which phase to dispatch: the override when given,
// otherwise the first phase in the ledger not yet done — the phase an
// executor would pick up. A plan with no ledger and no override cannot
// name a phase and reports false, which the caller turns into a refusal
// rather than a guess.
func Phase(phases []planmeta.Phase, override string) (string, bool) {
	if override != "" {
		return override, true
	}

	open, ok := planmeta.Plan{Phases: phases}.FirstOpenPhase()
	if !ok {
		return "", false
	}

	return string(open.N), true
}
