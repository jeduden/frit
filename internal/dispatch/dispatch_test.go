package dispatch

import (
	"testing"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
)

// TestCommandNamesThePlanAndPhase: the whole prompt frit ever sends is a
// slash command naming a plan and a phase, about twenty characters. No
// prose, ever.
func TestCommandNamesThePlanAndPhase(t *testing.T) {
	assert.Equal(t, "/plan-phase 2607191320 8",
		Command(2607191320, "8"))
	assert.Equal(t, "/plan-phase 100 3b",
		Command(100, "3b"), "a split phase keeps its literal token")
}

// TestCommandTrimsAnEmptyPhase: a phase-less plan's seed names only the
// plan, with no trailing space — the plan-phase skill defaults the
// phase itself.
func TestCommandTrimsAnEmptyPhase(t *testing.T) {
	assert.Equal(t, "/plan-phase 2608220941", Command(2608220941, ""))
}

// TestPhasePrefersTheOverride: an explicit --phase wins, so a person can
// aim at a phase the ledger has not reached.
func TestPhasePrefersTheOverride(t *testing.T) {
	phases := []planmeta.Phase{{N: "1", Status: planmeta.StatusNotStarted}}
	got, ok := Phase(phases, "3")
	assert.True(t, ok)
	assert.Equal(t, "3", got)
}

// TestPhaseFallsToTheLedger: with no override, the phase is the first
// one not yet done — the phase an executor would pick up.
func TestPhaseFallsToTheLedger(t *testing.T) {
	phases := []planmeta.Phase{
		{N: "1", Status: planmeta.StatusDone},
		{N: "2", Status: planmeta.StatusNotStarted},
	}
	got, ok := Phase(phases, "")
	assert.True(t, ok)
	assert.Equal(t, "2", got, "the first open phase")
}

// TestPhaseWithNoLedgerDispatchesTheWholePlan: a plan with no ledger
// and no override has no slice to name — it is dispatched whole, with
// an empty phase, rather than refused.
func TestPhaseWithNoLedgerDispatchesTheWholePlan(t *testing.T) {
	got, ok := Phase(nil, "")
	assert.True(t, ok)
	assert.Empty(t, got)
}

// TestPhaseWithEveryPhaseDoneReportsNoneOpen: a phased ledger whose
// every phase is done has genuinely nothing left to send, unlike a
// plan with no ledger at all.
func TestPhaseWithEveryPhaseDoneReportsNoneOpen(t *testing.T) {
	phases := []planmeta.Phase{
		{N: "1", Status: planmeta.StatusDone},
		{N: "2", Status: planmeta.StatusDone},
	}
	_, ok := Phase(phases, "")
	assert.False(t, ok)
}
