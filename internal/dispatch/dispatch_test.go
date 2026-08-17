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

// TestPhaseHasNoneWithoutLedgerOrOverride: a plan with no ledger and no
// override cannot name a phase, and says so rather than guessing.
func TestPhaseHasNoneWithoutLedgerOrOverride(t *testing.T) {
	_, ok := Phase(nil, "")
	assert.False(t, ok)
}
