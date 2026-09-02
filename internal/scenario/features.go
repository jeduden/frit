package scenario

import (
	"fmt"
	"regexp"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v34"
)

// featureTag is the shape of a scenario's id tag: "@S" and a number in
// the same form a matrix row's id takes, so "@S016" names nothing.
var featureTag = regexp.MustCompile(`^@(S` + idNumber + `)$`)

// Scenario is one scenario godog would run, as both the gate and the
// runner see it: where it sits, what it is called, the matrix id its
// tag names, and whether it is still declared rather than written.
type Scenario struct {
	Path    string
	Line    int64
	Name    string
	ID      string
	Pending bool
}

// Scenarios lists every scenario under dir in the order godog walks
// them — the same recursive walk, the same Gherkin — so the gate and
// the runner can never disagree about which scenarios exist: a feature
// in a subdirectory, a tag inherited from the Feature line, or a line
// inside a docstring reads the same to both. A Scenario Outline is one
// scenario however many Examples rows it has; godog compiles a pickle
// per row and every one carries the outline's tags, so the rows are
// folded back onto the outline they came from. Every scenario must
// carry exactly one S tag: one with none or several is reported with
// its place rather than listed with an empty or arbitrary id.
func Scenarios(dir string) ([]Scenario, error) {
	suite := godog.TestSuite{Options: &godog.Options{Paths: []string{dir}}}
	features, err := suite.RetrieveFeatures()
	if err != nil {
		return nil, fmt.Errorf("scenario: read features: %w", err)
	}

	var out []Scenario
	for _, f := range features {
		lines := scenarioLines(f.GherkinDocument)
		seen := map[string]bool{}
		for _, p := range f.Pickles {
			node := p.AstNodeIds[0]
			if seen[node] {
				continue
			}
			seen[node] = true
			sc, err := scenarioOf(f.Uri, lines[node], p)
			if err != nil {
				return nil, err
			}
			out = append(out, sc)
		}
	}

	return out, nil
}

// FeatureTagIDs reads the "@S<n>" tag off every scenario under dir,
// keyed by the id each names. A tag repeated across scenarios is
// reported rather than merged, since two scenarios sharing one id
// would otherwise both count as the matrix row's coverage.
func FeatureTagIDs(dir string) (map[string]bool, error) {
	scenarios, err := Scenarios(dir)
	if err != nil {
		return nil, err
	}

	ids := map[string]bool{}
	for _, sc := range scenarios {
		if err := recordID(sc, ids); err != nil {
			return nil, err
		}
	}

	return ids, nil
}

// scenarioOf reads one pickle's tags for the S id it names and whether
// it is pending, reporting a scenario with no id or several.
func scenarioOf(uri string, line int64, p *messages.Pickle) (Scenario, error) {
	sc := Scenario{Path: uri, Line: line, Name: p.Name}
	var found []string
	for _, tag := range p.Tags {
		if tag.Name == "@pending" {
			sc.Pending = true
		}
		if m := featureTag.FindStringSubmatch(tag.Name); m != nil {
			found = append(found, m[1])
		}
	}
	if len(found) != 1 {
		return Scenario{}, fmt.Errorf("scenario: %s:%d: scenario %q carries %d S tags, want exactly one",
			uri, line, p.Name, len(found))
	}
	sc.ID = found[0]

	return sc, nil
}

// recordID keeps a scenario's id, reporting one another scenario
// already carries.
func recordID(sc Scenario, ids map[string]bool) error {
	if ids[sc.ID] {
		return fmt.Errorf("scenario: %s:%d: tag %q repeated", sc.Path, sc.Line, "@"+sc.ID)
	}
	ids[sc.ID] = true

	return nil
}

// scenarioLines maps each scenario's AST id to the line it starts on,
// under the feature and under any rule, so an error can point at the
// scenario rather than the file. A .feature file with no Feature in it
// has no scenarios to place.
func scenarioLines(doc *messages.GherkinDocument) map[string]int64 {
	lines := map[string]int64{}
	if doc.Feature == nil {
		return lines
	}
	for _, child := range doc.Feature.Children {
		if child.Scenario != nil {
			lines[child.Scenario.Id] = child.Scenario.Location.Line
		}
		if child.Rule == nil {
			continue
		}
		for _, nested := range child.Rule.Children {
			if nested.Scenario != nil {
				lines[nested.Scenario.Id] = nested.Scenario.Location.Line
			}
		}
	}

	return lines
}
