package planmeta

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jeduden/mdsmith/pkg/goldmark/ast"
	"github.com/jeduden/mdsmith/pkg/markdown"
)

// Bundle is the minimal working-session bundle for a plan's open
// phase: the spec an executor reads, the previous phase's handoff,
// its own in-progress notes, the tier and gate its Execution row
// names, and the result file to write. HasPhase is false when every
// phase is done, the way Plan.FirstOpenPhase reports none left.
type Bundle struct {
	N          PhaseNumber
	Spec       string
	HandoffIn  string
	Notes      string
	Tier       string
	Gate       string
	ResultPath string
	HasPhase   bool
}

// specFileRE matches a phase's own spec file, phase-N.md — not its
// companion phase-N.result.md, which shares the "phase-" prefix and
// ".md" suffix and would otherwise also match a naive glob.
var specFileRE = regexp.MustCompile(`^phase-([0-9]+)\.md$`)

// Resume finds a plan's open phase and assembles its working bundle.
//
// dir is the plan's own directory — where a folder plan's phase-N.md
// and phase-N.result.md files live — and planBody is the plan.md
// bytes already read from there. When dir carries no phase-N.md at
// all, Resume falls back to planBody's own `phases:` ledger and
// `## Phase N` sections, so a flat or inline-section plan resumes
// unchanged.
func Resume(dir string, planBody []byte) (Bundle, error) {
	specs, err := phaseSpecNumbers(dir)
	if err != nil {
		return Bundle{}, err
	}
	if len(specs) == 0 {
		return resumeFromLedger(planBody)
	}

	body := markdown.Parse(planBody).Body

	var handoffIn string
	for _, n := range specs {
		result, err := os.ReadFile(filepath.Join(dir, resultFileName(n)))
		notes := ""
		handoff, done := "", false
		switch {
		case err == nil:
			handoff, done = handoffOf(result)
			if !done {
				notes = strings.TrimSpace(string(result))
			}
		case !os.IsNotExist(err):
			return Bundle{}, err
		}

		if done {
			handoffIn = handoff

			continue
		}

		spec, err := os.ReadFile(filepath.Join(dir, specFileName(n)))
		if err != nil {
			return Bundle{}, err
		}
		tier, gate, _ := executionRowFor(body, PhaseNumber(n))

		return Bundle{
			N:          PhaseNumber(n),
			Spec:       strings.TrimSpace(string(spec)),
			HandoffIn:  handoffIn,
			Notes:      notes,
			Tier:       tier,
			Gate:       gate,
			ResultPath: resultFileName(n),
			HasPhase:   true,
		}, nil
	}

	return Bundle{}, nil
}

// resumeFromLedger builds a bundle from a plan's own `phases:` ledger
// and `## Phase N` sections — the path a plan with no phase-N.md
// files takes, unchanged from how next already reads it. It carries
// no handoff, notes or result path: those are a phase-file plan's own
// convention, and a ledger plan keeps writing its handoff into
// plan.md the way plan-phase already does.
func resumeFromLedger(planBody []byte) (Bundle, error) {
	plan, err := Parse(planBody)
	if err != nil {
		return Bundle{}, err
	}
	phase, ok := plan.FirstOpenPhase()
	if !ok {
		return Bundle{}, nil
	}

	return Bundle{
		N:        phase.N,
		Spec:     phase.Body,
		Tier:     phase.Tier,
		Gate:     phase.Gate,
		HasPhase: true,
	}, nil
}

// phaseSpecNumbers lists a plan directory's own phase-N.md files, as
// their bare N tokens, ordered numerically so phase-2 precedes
// phase-10 — a lexical sort would not.
func phaseSpecNumbers(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type numbered struct {
		n    int
		text string
	}
	var found []numbered
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := specFileRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		found = append(found, numbered{n, m[1]})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].n < found[j].n })

	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.text
	}

	return out, nil
}

func specFileName(n string) string   { return "phase-" + n + ".md" }
func resultFileName(n string) string { return "phase-" + n + ".result.md" }

// handoffOf reports a phase result file's `## Handoff` section, found
// by walking its parsed AST for a level-2 heading with that exact
// title — not a substring match — so a `## Handoff` quoted or fenced
// inside a parked note does not read as the phase's own closing
// marker.
func handoffOf(source []byte) (text string, ok bool) {
	doc := markdown.Parse(source)
	for n := doc.AST.FirstChild(); n != nil; n = n.NextSibling() {
		h, isHeading := n.(*ast.Heading)
		if !isHeading || h.Level != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(inlineText(h, doc.Body)),
			"Handoff") {
			return sectionText(doc, "Handoff"), true
		}
	}

	return "", false
}

// executionRowFor reads a plan body's `## Execution` table for the
// row a phase number leads with, independent of whether the plan
// carries a `phases:` ledger — a phase-file plan has moved its phase
// prose and status out of plan.md, but keeps its tier and gate in the
// one shared table.
func executionRowFor(body []byte, n PhaseNumber) (tier, gate string, ok bool) {
	row, ok := executionTable(body)[n]

	return row.tier, row.gate, ok
}
