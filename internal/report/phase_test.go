package report

import (
	"errors"
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
)

// TestNewPhaseCarriesTheBundle pins the field-by-field projection from
// a resume bundle into the wire shape.
func TestNewPhaseCarriesTheBundle(t *testing.T) {
	doc := NewPhase("/fleet", discovery.Plan{Repo: "atlas", ID: 100},
		planmeta.Bundle{
			N: "2", Spec: "Do the second thing.",
			HandoffIn: "Phase one landed cleanly.", Notes: "Parked.",
			Tier: "opus", Gate: "test two",
			ResultPath: "phase-2.result.md", HasPhase: true,
		})

	assert.Equal(t, "phase", doc.Command)
	assert.True(t, doc.HasPhase)
	assert.Equal(t, "2", doc.Phase.N)
	assert.Equal(t, "Do the second thing.", doc.Phase.Spec)
	assert.Equal(t, "Phase one landed cleanly.", doc.Phase.HandoffIn)
	assert.Equal(t, "Parked.", doc.Phase.Notes)
	assert.Equal(t, "opus", doc.Phase.Tier)
	assert.Equal(t, "test two", doc.Phase.Gate)
	assert.Equal(t, "phase-2.result.md", doc.Phase.ResultPath)
}

// TestNewPhaseLeavesTheCardEmptyWithNoOpenPhase mirrors NextDoc's own
// "nothing invented" rule: with no open phase, the bundle card stays
// its zero value rather than carrying a stale one.
func TestNewPhaseLeavesTheCardEmptyWithNoOpenPhase(t *testing.T) {
	doc := NewPhase("/fleet", discovery.Plan{Repo: "atlas", ID: 100},
		planmeta.Bundle{})

	assert.False(t, doc.HasPhase)
	assert.Equal(t, PhaseBundleCard{}, doc.Phase)
}

// TestPhaseDocAddProblemRecordsIt pins AddProblem the way every other
// document's own does.
func TestPhaseDocAddProblemRecordsIt(t *testing.T) {
	doc := NewPhase("/fleet", discovery.Plan{Repo: "atlas", ID: 100},
		planmeta.Bundle{})

	doc.AddProblem("atlas", errors.New("broken"))

	assert.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}
