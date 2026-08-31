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
//
// Title is carried only for a ledger phase, whose `phases:` entry
// already names one; a phase-file plan has no title convention yet
// for its own phase-N.md, so Title is empty there.
type Bundle struct {
	N          PhaseNumber
	Title      string
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
// ".md" suffix and would otherwise also match a naive glob. N may
// carry a trailing letter, the same split-phase convention the
// ledger's own PhaseNumber allows — a sitting that grows into two
// becomes "3a" and "3b" beside "1" and "2" — captured apart from its
// leading digits so phaseSpecNumbers can sort on the digits alone.
var specFileRE = regexp.MustCompile(`^phase-([0-9]+)([A-Za-z]*)\.md$`)

// Resume finds a plan's open phase and assembles its working bundle.
//
// dir is a folder plan's own directory — where its phase-N.md and
// phase-N.result.md files live — and planBody is the plan.md bytes
// already read from there. dir is "" for a flat plan, which has no
// directory of its own: its path's parent is plan/, shared by every
// flat plan in the repository, and a phase-N.md glob-matched there
// could belong to any of them. An empty dir, like a folder plan's
// directory carrying no phase-N.md, falls back to planBody's own
// `phases:` ledger and `## Phase N` sections, so a flat or
// inline-section plan resumes unchanged.
func Resume(dir string, planBody []byte) (Bundle, error) {
	if dir == "" {
		return resumeFromLedger(planBody)
	}

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
		st, err := readPhaseState(dir, n)
		if err != nil {
			return Bundle{}, err
		}
		if st.done {
			if st.hasHandoff {
				handoffIn = st.handoff
			}

			continue
		}

		notes := ""
		if st.hasResult {
			notes = strings.TrimSpace(string(st.result))
		}
		tier, gate, _ := executionRowFor(body, PhaseNumber(n))

		return Bundle{
			N: PhaseNumber(n),
			// A phase file that carries {n, title, status} front matter
			// keeps only its prose in the bundle: the front matter is the
			// ledger, not the working brief an executor reads. A phase file
			// with none yields its whole content, unchanged.
			Spec:       strings.TrimSpace(string(markdown.Parse(st.spec).Body)),
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

// phaseState is what one phase-N.md and its result file say about that
// phase: whether it is done and the handoff it hands its successor, plus
// the raw material Resume bundles when it is the open phase.
type phaseState struct {
	done       bool
	handoff    string
	hasHandoff bool
	spec       []byte
	result     []byte
	hasResult  bool
}

// readPhaseState reads one phase's spec and result files and decides
// whether it is done. The phase file's own status is the done-signal
// where it carries one — a phase closes by flipping its own phase-N.md.
// A phase file written before the status convention carries none, and
// falls back to the result file's ## Handoff marker so it still resumes
// as before.
func readPhaseState(dir, n string) (phaseState, error) {
	spec, err := os.ReadFile(filepath.Join(dir, specFileName(n)))
	if err != nil {
		return phaseState{}, err
	}
	status := phaseFileStatus(spec)

	result, rerr := os.ReadFile(filepath.Join(dir, resultFileName(n)))
	handoff, hasHandoff := "", false
	switch {
	case rerr == nil:
		handoff, hasHandoff = handoffOf(result)
	case !os.IsNotExist(rerr):
		return phaseState{}, rerr
	}

	done := status == StatusDone || status == StatusSuperseded
	if status == "" {
		done = hasHandoff
	}

	return phaseState{
		done:       done,
		handoff:    handoff,
		hasHandoff: hasHandoff,
		spec:       spec,
		result:     result,
		hasResult:  rerr == nil,
	}, nil
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
		Title:    phase.Title,
		Spec:     phase.Body,
		Tier:     phase.Tier,
		Gate:     phase.Gate,
		HasPhase: true,
	}, nil
}

// phaseSpecNumbers lists a plan directory's own phase-N.md files, as
// their bare N tokens, ordered by leading digits so phase-2 precedes
// phase-10 — a lexical sort would not — and, within a split phase
// sharing those digits, by the full token, so phase-3a precedes
// phase-3b.
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
		// specFileRE's first group is `[0-9]+`, so it always parses;
		// nothing here can make Atoi fail.
		n, _ := strconv.Atoi(m[1])
		found = append(found, numbered{n, m[1] + m[2]})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].n != found[j].n {
			return found[i].n < found[j].n
		}

		return found[i].text < found[j].text
	})

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

// phaseFileStatus reports a phase-N.md's own front-matter status, or ""
// when the file carries no front matter — the shape a phase file written
// before the status convention takes, which Resume then reads through the
// result file's ## Handoff marker instead. It reuses parsePhaseFile, the
// same decode PhasesFromDir uses to build a folder plan's ledger.
func phaseFileStatus(spec []byte) string {
	phase, err := parsePhaseFile(spec)
	if err != nil {
		return ""
	}

	return phase.Status
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
