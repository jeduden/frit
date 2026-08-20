// Package planmeta reads a plan file's front matter.
//
// Markdown is split by mdsmith's own public parser rather than by a
// regular expression here, so frit and mdsmith always agree on where
// front matter ends — including the awkward cases, like a block
// scalar whose body contains a line of three dashes.
package planmeta

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/jeduden/mdsmith/pkg/goldmark/ast"
	"github.com/jeduden/mdsmith/pkg/goldmark/extension"
	extast "github.com/jeduden/mdsmith/pkg/goldmark/extension/ast"
	"github.com/jeduden/mdsmith/pkg/goldmark/text"
	"github.com/jeduden/mdsmith/pkg/markdown"
	"github.com/jeduden/mdsmith/pkg/markdown/flavor"
	"gopkg.in/yaml.v3"
)

// ErrNoFrontMatter reports a markdown file with no front-matter
// block. Every plan has one, so this marks a file that is not a plan.
var ErrNoFrontMatter = errors.New("no front matter")

// ProtoName is the schema template that lives beside real plans. It
// carries CUE type expressions where a plan carries values, so it is
// excluded by name rather than by failing to parse.
const ProtoName = "proto.md"

// Plan is a plan file's front matter.
//
// Only the fields frit indexes are decoded. Unknown keys are ignored
// rather than rejected, so a repository may carry extra front matter
// without frit refusing to read its plans.
type Plan struct {
	ID        int64   `yaml:"id"`
	Title     string  `yaml:"title"`
	Status    string  `yaml:"status"`
	Summary   string  `yaml:"summary"`
	Model     string  `yaml:"model"`
	DependsOn []int64 `yaml:"depends-on"`
	Phases    []Phase `yaml:"phases"`

	// Goal is the prose of the plan's `## Goal` section, folded to one
	// line. It lives in the body, not the front matter, so it is read
	// by walking the parsed AST rather than decoded from YAML. Empty
	// when the plan carries no such section.
	Goal string `yaml:"-"`
}

// Phase is one entry in a plan's phase ledger: its number, title and
// its own status, tracked apart from the plan's so an executor can flip
// one phase without touching the rest.
type Phase struct {
	N      PhaseNumber `yaml:"n"`
	Title  string      `yaml:"title"`
	Status string      `yaml:"status"`

	// Tier is the model tier the plan's `## Execution` table names for
	// this phase: the more demanding of its Design and Implement
	// columns. It lives in the body, not the front matter, so it is
	// read by walking the parsed AST rather than decoded from YAML.
	// Empty when the phase carries no Execution row.
	Tier string `yaml:"-"`
	// Design and Implement are that same row's own two columns,
	// before mostDemandingTier collapses them into Tier. An unknown
	// value in one column is invisible in Tier when the other column
	// is a real model — frit doctor checks each of these on its own,
	// so a typo does not hide behind a valid neighbor.
	Design    string `yaml:"-"`
	Implement string `yaml:"-"`
	// Gate is the check the same row names as what catches a wrong
	// answer for this phase. Empty when the phase carries no
	// Execution row.
	Gate string `yaml:"-"`
	// HasExecutionRow reports whether the `## Execution` table
	// carried a row for this phase's number, so a caller can surface
	// a missing row as a gap rather than render a blank tier as if it
	// meant something.
	HasExecutionRow bool `yaml:"-"`

	// Body is the prose of this phase's `## Phase N` section — the
	// text an executor reads to do the work, not just its title.
	// Paragraphs join on a blank line; each one is folded to a single
	// line the way Goal is. Empty when the plan carries no matching
	// section.
	Body string `yaml:"-"`
}

// PhaseNumber is a phase's position in the ledger, kept as a string
// rather than an int because phases split: a plan may carry 3a and 3b
// beside 1 and 2 when one sitting grows into two. A bare YAML integer
// and a quoted token both decode to their literal text, so `n: 3` and
// `n: '3b'` sit in the same ledger without the whole plan failing to
// parse over the one that is not a number.
type PhaseNumber string

// UnmarshalYAML reads a phase number from whatever scalar carries it,
// integer or string, taking the node's literal text either way.
func (n *PhaseNumber) UnmarshalYAML(node *yaml.Node) error {
	*n = PhaseNumber(node.Value)

	return nil
}

// The status vocabulary, as it appears in the files themselves.
const (
	StatusNotStarted = "🔲"
	StatusInProgress = "🔳"
	StatusDone       = "✅"
	StatusSuperseded = "⛔"
)

// NotStarted reports a plan nobody has begun.
func (p Plan) NotStarted() bool { return p.Status == StatusNotStarted }

// InProgress reports a plan currently being worked.
func (p Plan) InProgress() bool { return p.Status == StatusInProgress }

// Done reports a completed plan.
func (p Plan) Done() bool { return p.Status == StatusDone }

// Superseded reports a plan replaced by another.
func (p Plan) Superseded() bool { return p.Status == StatusSuperseded }

// FirstOpenPhase returns the first phase still to do, which is the
// phase frit next points at and the one the plan-phase workflow
// defaults to. Done and superseded phases are stepped over, since
// neither is work to pick up. A plan with no ledger, or none left open,
// reports false.
func (p Plan) FirstOpenPhase() (Phase, bool) {
	for _, phase := range p.Phases {
		if phase.Status == StatusDone || phase.Status == StatusSuperseded {
			continue
		}

		return phase, true
	}

	return Phase{}, false
}

// IsProto reports whether a path is the schema template rather than a
// plan.
func IsProto(p string) bool {
	return path.Base(p) == ProtoName
}

// Parse reads a plan file's front matter.
//
// The front-matter block is located by mdsmith and decoded here: the
// public parser hands back the raw prefix including its delimiters,
// and turning that into typed fields is a plain YAML decode.
func Parse(source []byte) (Plan, error) {
	doc := markdown.Parse(source)
	body := insideDelimiters(doc.FrontMatter)
	if len(bytes.TrimSpace(body)) == 0 {
		return Plan{}, ErrNoFrontMatter
	}

	var p Plan
	if err := yaml.Unmarshal(body, &p); err != nil {
		return Plan{}, fmt.Errorf("front matter: %w", err)
	}
	p.Goal = sectionText(doc, "Goal")
	if len(p.Phases) == 0 {
		p.Phases = derivePhasesFromHeadings(doc)
	}
	attachPhaseBodies(&p, doc)
	attachExecutionRows(&p, doc.Body)

	return p, nil
}

// phaseHeadingRE matches a `## Phase N: Title` heading's text, taking
// the phase number as whatever token follows "Phase" and the rest of
// the line, past an optional colon, as the title. The number is
// captured non-greedily so "3b: Split" and "3b Split" both yield "3b"
// rather than swallowing the colon or the title's first word.
var phaseHeadingRE = regexp.MustCompile(`(?i)^Phase\s+(\S+?)(?::\s*|\s+|$)(.*)$`)

// derivePhasesFromHeadings builds a phase ledger from `## Phase N`
// headings for a plan that carries no front-matter `phases:` list —
// the convention frit's own plans use. Section state carries no
// status, so every derived phase is left with an empty one rather
// than an invented "not started": FirstOpenPhase's skip-done rule
// then never skips a derived phase, so next still points at the
// first one, which is the most a ledger with no status can promise.
func derivePhasesFromHeadings(doc *markdown.Document) []Phase {
	var out []Phase
	for n := doc.AST.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok || h.Level != 2 {
			continue
		}
		m := phaseHeadingRE.FindStringSubmatch(
			strings.TrimSpace(inlineText(h, doc.Body)))
		if m == nil {
			continue
		}
		out = append(out, Phase{N: PhaseNumber(m[1]), Title: strings.TrimSpace(m[2])})
	}

	return out
}

// attachPhaseBodies enriches the phase ledger — front-matter or
// derived — with the prose of each phase's own `## Phase N` section.
func attachPhaseBodies(p *Plan, doc *markdown.Document) {
	if len(p.Phases) == 0 {
		return
	}

	bodies := phaseSectionBodies(doc)
	for i := range p.Phases {
		p.Phases[i].Body = bodies[p.Phases[i].N]
	}
}

// phaseSectionBodies walks the body for every `## Phase N` section and
// returns its prose, keyed by phase number. A phase body commonly
// spans several paragraphs, so blocks join on a blank line rather
// than folding the whole section to one line the way Goal does; each
// block itself still folds its own soft-wrapped lines to one, via
// inlineText.
func phaseSectionBodies(doc *markdown.Document) map[PhaseNumber]string {
	blocks := map[PhaseNumber][]string{}
	var current PhaseNumber
	inPhase := false

	for n := doc.AST.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			inPhase = false
			if h.Level == 2 {
				m := phaseHeadingRE.FindStringSubmatch(
					strings.TrimSpace(inlineText(h, doc.Body)))
				if m != nil {
					current = PhaseNumber(m[1])
					inPhase = true
				}
			}

			continue
		}
		if inPhase {
			if text := strings.TrimSpace(inlineText(n, doc.Body)); text != "" {
				blocks[current] = append(blocks[current], text)
			}
		}
	}

	out := make(map[PhaseNumber]string, len(blocks))
	for n, bs := range blocks {
		out[n] = strings.Join(bs, "\n\n")
	}

	return out
}

// attachExecutionRows enriches the front-matter phase ledger with the
// tier and gate its `## Execution` table row names, matched by
// leading phase number. A phase whose number has no row is left with
// HasExecutionRow false, so a caller reports the gap rather than
// rendering a blank tier as if it meant something. A plan with no
// ledger has nothing to attach to, so the table is left unparsed.
func attachExecutionRows(p *Plan, body []byte) {
	if len(p.Phases) == 0 {
		return
	}

	rows := executionTable(body)
	for i := range p.Phases {
		row, ok := rows[p.Phases[i].N]
		if !ok {
			continue
		}
		p.Phases[i].Tier = row.tier
		p.Phases[i].Gate = row.gate
		p.Phases[i].Design = row.design
		p.Phases[i].Implement = row.implement
		p.Phases[i].HasExecutionRow = true
	}
}

// executionRow is what a plan's `## Execution` table says about one
// phase.
type executionRow struct {
	tier              string
	gate              string
	design, implement string
}

// executionTable parses a plan's `## Execution` section into a row per
// phase, keyed by the phase number its first column leads with — "2
// tier & gate" keys "2".
//
// The shared body AST (markdown.Parse) is CommonMark-only, so a GFM
// table there is a Paragraph of literal pipe text, not a table node.
// This re-parses the body with mdsmith's exported
// flavor.NewPooledParserWith(extension.Table) — the same seam
// mdsmith's own schema validator reaches for when it needs to see a
// GFM table (internal/schema.parseWithTableExt, "lint.NewFile's
// parser is CommonMark-only, so GFM tables would otherwise appear as
// paragraphs") — rather than hand-rolling a second table parser.
func executionTable(body []byte) map[PhaseNumber]executionRow {
	p, reset := flavor.NewPooledParserWith(extension.Table)
	defer reset()
	root := p.Parse(text.NewReader(body))

	out := map[PhaseNumber]executionRow{}
	for _, n := range sectionNodes(root, body, "Execution") {
		tbl, ok := n.(*extast.Table)
		if !ok {
			continue
		}
		collectExecutionRows(tbl, body, out)

		break
	}

	return out
}

// collectExecutionRows reads an Execution table's header to find its
// Design, Implement and Gate columns by name, then fills one row per
// data row, keyed by the phase number the first column leads with.
func collectExecutionRows(
	tbl *extast.Table, body []byte, out map[PhaseNumber]executionRow,
) {
	header, ok := tbl.FirstChild().(*extast.TableHeader)
	if !ok {
		return
	}

	designCol, implCol, gateCol := -1, -1, -1
	for i, c := 0, header.FirstChild(); c != nil; i, c = i+1, c.NextSibling() {
		switch label := strings.ToLower(inlineText(c, body)); {
		case strings.Contains(label, "design"):
			designCol = i
		case strings.Contains(label, "implement"):
			implCol = i
		case strings.Contains(label, "gate"):
			gateCol = i
		}
	}

	for n := header.NextSibling(); n != nil; n = n.NextSibling() {
		row, ok := n.(*extast.TableRow)
		if !ok {
			continue
		}
		cells := cellTexts(row, body)
		if len(cells) == 0 {
			continue
		}
		fields := strings.Fields(cells[0])
		if len(fields) == 0 {
			continue
		}

		var er executionRow
		if designCol >= 0 && designCol < len(cells) {
			er.design = cells[designCol]
			er.tier = er.design
		}
		if implCol >= 0 && implCol < len(cells) {
			er.implement = cells[implCol]
			er.tier = mostDemandingTier(er.tier, er.implement)
		}
		if gateCol >= 0 && gateCol < len(cells) {
			er.gate = cells[gateCol]
		}
		out[PhaseNumber(fields[0])] = er
	}
}

// cellTexts reads a table row's cells as prose, in column order.
func cellTexts(row *extast.TableRow, body []byte) []string {
	var out []string
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, strings.TrimSpace(inlineText(c, body)))
	}

	return out
}

// tierRank orders the model tiers by how demanding they are, so
// mostDemandingTier can pick the higher of a phase's Design and
// Implement columns.
var tierRank = map[string]int{"haiku": 0, "sonnet": 1, "opus": 2}

// KnownTier reports whether s names a model tier frit recognizes — the
// same vocabulary mostDemandingTier ranks by. frit doctor uses this to
// flag a phase whose Execution row names something else.
func KnownTier(s string) bool {
	_, ok := tierRank[s]

	return ok
}

// mostDemandingTier returns whichever of a and b ranks higher. An
// unrecognized tier ranks below any recognized one rather than
// panicking or erroring — frit doctor (phase 4) is where a tier that
// names no known model becomes a reported gap, not this parse.
func mostDemandingTier(a, b string) string {
	ra, oka := tierRank[a]
	rb, okb := tierRank[b]

	switch {
	case oka && okb:
		if rb > ra {
			return b
		}

		return a
	case okb:
		return b
	default:
		return a
	}
}

// sectionText returns the prose of the level-2 section with the given
// title, folded to one line, or "" when the plan has no such section.
//
// The body was already parsed by mdsmith, so its AST is walked rather
// than the source re-scanned: frit and mdsmith agree on the document's
// shape, and this reuses that agreement instead of a second parser.
func sectionText(doc *markdown.Document, title string) string {
	var out []string
	for _, n := range sectionNodes(doc.AST, doc.Body, title) {
		if text := strings.TrimSpace(inlineText(n, doc.Body)); text != "" {
			out = append(out, text)
		}
	}

	return strings.Join(out, " ")
}

// sectionNodes returns the block-level nodes inside the level-2
// section with the given title, walking root directly rather than a
// *markdown.Document so a differently-configured re-parse (e.g.
// executionTable's table-aware AST) can share this walk.
func sectionNodes(root ast.Node, body []byte, title string) []ast.Node {
	var out []ast.Node
	inSection := false

	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			if inSection {
				break
			}
			if h.Level == 2 &&
				strings.EqualFold(strings.TrimSpace(inlineText(h, body)),
					title) {
				inSection = true
			}

			continue
		}
		if inSection {
			out = append(out, n)
		}
	}

	return out
}

// inlineText collects a node's visible text, joining soft-wrapped
// lines with a single space and taking the literal content of inline
// code, so a goal carrying `frit show` reads as prose.
func inlineText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
			if t.SoftLineBreak() {
				b.WriteByte(' ')
			}
		}

		return ast.WalkContinue, nil
	})

	return b.String()
}

// insideDelimiters strips the leading and trailing `---` fences from
// a raw front-matter prefix, leaving the YAML document.
func insideDelimiters(fm []byte) []byte {
	trimmed := bytes.TrimSpace(fm)
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil
	}

	// Drop the opening fence line.
	if nl := bytes.IndexByte(trimmed, '\n'); nl >= 0 {
		trimmed = trimmed[nl+1:]
	} else {
		return nil
	}

	// Drop the closing fence, which is the final line.
	if idx := bytes.LastIndex(trimmed, []byte("\n---")); idx >= 0 {
		trimmed = trimmed[:idx]
	} else if bytes.HasPrefix(trimmed, []byte("---")) {
		return nil
	}

	return trimmed
}
