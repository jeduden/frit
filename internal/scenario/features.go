package scenario

import (
	"fmt"
	"regexp"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v34"
)

var featureTag = regexp.MustCompile(`^@(S[1-9][0-9]*)$`)

// FeatureTagIDs reads the "@S<n>" tag off every scenario godog would run
// from dir, keyed by the id each names. The features are parsed the way
// godog parses them — the same recursive walk, the same Gherkin — so
// the gate and the runner can never disagree about which scenarios
// exist: a feature in a subdirectory, a tag inherited from the Feature
// line, or a line inside a docstring reads the same to both. Every
// scenario must carry exactly one S tag, and a tag repeated across
// scenarios is reported rather than merged, since two scenarios sharing
// one id would otherwise both count as the matrix row's coverage.
func FeatureTagIDs(dir string) (map[string]bool, error) {
	suite := godog.TestSuite{Options: &godog.Options{Paths: []string{dir}}}
	features, err := suite.RetrieveFeatures()
	if err != nil {
		return nil, fmt.Errorf("scenario: read features: %w", err)
	}

	ids := map[string]bool{}
	for _, f := range features {
		lines := scenarioLines(f.GherkinDocument)
		for _, p := range f.Pickles {
			found := scenarioIDs(p.Tags)
			if err := recordID(f.Uri, lines[p.AstNodeIds[0]], p.Name, found, ids); err != nil {
				return nil, err
			}
		}
	}

	return ids, nil
}

// scenarioIDs lists the S ids a scenario's tags name, in tag order.
func scenarioIDs(tags []*messages.PickleTag) []string {
	var found []string
	for _, tag := range tags {
		if m := featureTag.FindStringSubmatch(tag.Name); m != nil {
			found = append(found, m[1])
		}
	}

	return found
}

// recordID keeps the one S id a scenario names, reporting a scenario
// with none or several, or an id another scenario already carries.
func recordID(uri string, line int64, name string, found []string, ids map[string]bool) error {
	if len(found) != 1 {
		return fmt.Errorf("scenario: %s:%d: scenario %q carries %d S tags, want exactly one",
			uri, line, name, len(found))
	}
	if ids[found[0]] {
		return fmt.Errorf("scenario: %s:%d: tag %q repeated", uri, line, "@"+found[0])
	}
	ids[found[0]] = true

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
