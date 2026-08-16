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
}

// Phase is one entry in a plan's phase ledger: its number, title and
// its own status, tracked apart from the plan's so an executor can flip
// one phase without touching the rest.
type Phase struct {
	N      int    `yaml:"n"`
	Title  string `yaml:"title"`
	Status string `yaml:"status"`
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

// FirstOpenPhase returns the first phase not at ✅, which is the phase
// frit next points at and the one the plan-phase workflow defaults to.
// A plan with no ledger, or one whose every phase is done, has no open
// phase and reports false.
func (p Plan) FirstOpenPhase() (Phase, bool) {
	for _, phase := range p.Phases {
		if phase.Status != StatusDone {
			return phase, true
		}
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

	return p, nil
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
