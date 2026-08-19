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
	"strings"

	"github.com/jeduden/mdsmith/pkg/goldmark/ast"
	"github.com/jeduden/mdsmith/pkg/markdown"
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

	return p, nil
}

// sectionText returns the prose of the level-2 section with the given
// title, folded to one line, or "" when the plan has no such section.
//
// The body was already parsed by mdsmith, so its AST is walked rather
// than the source re-scanned: frit and mdsmith agree on the document's
// shape, and this reuses that agreement instead of a second parser.
func sectionText(doc *markdown.Document, title string) string {
	var out []string
	inSection := false

	for n := doc.AST.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			if inSection {
				break
			}
			if h.Level == 2 &&
				strings.EqualFold(strings.TrimSpace(inlineText(h, doc.Body)),
					title) {
				inSection = true
			}

			continue
		}
		if inSection {
			if text := strings.TrimSpace(inlineText(n, doc.Body)); text != "" {
				out = append(out, text)
			}
		}
	}

	return strings.Join(out, " ")
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
