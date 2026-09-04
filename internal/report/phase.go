package report

import (
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/planmeta"
)

// PhaseBundleCard is the open phase's working bundle, in wire shape:
// the spec an executor reads, the previous phase's handoff, its own
// in-progress notes, the tier and gate its Execution row names, and
// the result file to write. Every field is empty rather than absent
// when the plan fell back to its plan.md ledger, which carries no
// per-phase files of its own. Title is likewise empty for a
// phase-file plan, which has no title convention yet for its own
// phase-N.md.
type PhaseBundleCard struct {
	N          string `json:"n"`
	Title      string `json:"title"`
	Spec       string `json:"spec"`
	HandoffIn  string `json:"handoff_in"`
	Notes      string `json:"notes"`
	Tier       string `json:"tier"`
	Gate       string `json:"gate"`
	ResultPath string `json:"result_path"`
}

// PhaseDoc is what `frit phase` found: a plan's open phase, bundled
// with everything a working session needs to resume it without
// reading the rest of the plan.
//
// HasPhase distinguishes "the open phase is this" from "there is no
// open phase left", the same split NextDoc makes.
type PhaseDoc struct {
	header
	gathered
	Root     string          `json:"root"`
	Plan     PlanCard        `json:"plan"`
	Phase    PhaseBundleCard `json:"phase"`
	HasPhase bool            `json:"has_phase"`
	Problems []Problem       `json:"problems"`
}

// NewPhase opens a phase-bundle report for one resolved plan and its
// assembled bundle.
func NewPhase(root string, plan discovery.Plan, bundle planmeta.Bundle) *PhaseDoc {
	doc := &PhaseDoc{
		header:   newHeader("phase"),
		Root:     root,
		Plan:     cardOf(plan),
		Problems: []Problem{},
	}
	if bundle.HasPhase {
		doc.HasPhase = true
		doc.Phase = phaseBundleCard(bundle)
	}

	return doc
}

// AddProblem records a repository whose plans could not be read.
func (d *PhaseDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// phaseBundleCard projects a resume bundle into its wire shape.
func phaseBundleCard(b planmeta.Bundle) PhaseBundleCard {
	return PhaseBundleCard{
		N: string(b.N), Title: b.Title, Spec: b.Spec, HandoffIn: b.HandoffIn,
		Notes: b.Notes, Tier: b.Tier, Gate: b.Gate,
		ResultPath: b.ResultPath,
	}
}
