package scenario

import (
	"os"
	"path/filepath"
	"testing"

	messages "github.com/cucumber/messages/go/v34"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFeature(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

// TestFeatureTagIDsCollectsEveryScenarioGodogWouldRun: a tag on its
// own line, one sharing a line with "@wip" and "@pending", one under a
// Rule, one in a subdirectory, and one on a scenario with no steps yet
// all count — the same set of scenarios godog itself walks.
func TestFeatureTagIDsCollectsEveryScenarioGodogWouldRun(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature", "Feature: a\n\n  @S1\n  Scenario: one\n    Given a\n\n"+
		"  Rule: r\n\n    @S3\n    Scenario: three\n      Given a\n")
	writeFeature(t, dir, "b.feature", "Feature: b\n\n  @wip @S2 @pending\n  Scenario: two\n")
	writeFeature(t, dir, filepath.Join("nested", "c.feature"), "Feature: c\n\n  @S4\n  Scenario: four\n    Given c\n")

	ids, err := FeatureTagIDs(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"S1": true, "S2": true, "S3": true, "S4": true}, ids)
}

// TestFeatureTagIDsCountsAnOutlineOnce: a Scenario Outline compiles to
// one pickle per Examples row, every one carrying the outline's tag;
// they are one scenario, so its id is recorded once rather than
// reported as repeated.
func TestFeatureTagIDsCountsAnOutlineOnce(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature", "Feature: a\n\n  @S1\n  Scenario Outline: one\n    Given <x>\n\n"+
		"    Examples:\n      | x |\n      | p |\n      | q |\n")

	ids, err := FeatureTagIDs(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"S1": true}, ids)
}

// TestScenariosListsEachScenarioOnceWithItsPlace: the runner's view —
// every scenario under dir in file order, placed by file and line,
// its id and whether it is pending, an outline listed once.
func TestScenariosListsEachScenarioOnceWithItsPlace(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature", "Feature: a\n\n  @S1 @pending\n  Scenario: one\n\n"+
		"  @S2\n  Scenario Outline: two\n    Given <x>\n\n    Examples:\n      | x |\n      | p |\n      | q |\n")

	got, err := Scenarios(dir)
	require.NoError(t, err)
	assert.Equal(t, []Scenario{
		{Path: filepath.Join(dir, "a.feature"), Line: 4, Name: "one", ID: "S1", Pending: true},
		{Path: filepath.Join(dir, "a.feature"), Line: 7, Name: "two", ID: "S2"},
	}, got)
}

// TestScenariosFailsOnAScenarioWithoutAnID: the runner and the gate
// read scenarios through the same walk, so a scenario with no id is an
// error naming its place rather than a scenario with an empty one.
func TestScenariosFailsOnAScenarioWithoutAnID(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature", "Feature: a\n\n  Scenario: untagged\n    Given a\n")

	_, err := Scenarios(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.feature:3:")
	assert.Contains(t, err.Error(), "want exactly one")
}

// TestFeatureTagIDsFailsOnATagRepeatedAcrossScenarios: two scenarios
// tagged for the same id would both count as that row's coverage, so
// the repeat is reported with the second scenario's line.
func TestFeatureTagIDsFailsOnATagRepeatedAcrossScenarios(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature",
		"Feature: a\n\n  @S1\n  Scenario: one\n    Given a\n\n  @S1\n  Scenario: dup\n    Given a\n")

	_, err := FeatureTagIDs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.feature:8:")
	assert.Contains(t, err.Error(), "@S1")
}

// TestFeatureTagIDsFailsOnAFeatureLevelTag: a tag above the Feature
// line is inherited by every scenario in the file, so it reads as the
// same id repeated, not as one scenario's coverage.
func TestFeatureTagIDsFailsOnAFeatureLevelTag(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature",
		"@S1\nFeature: a\n\n  Scenario: one\n    Given a\n\n  Scenario: two\n    Given a\n")

	_, err := FeatureTagIDs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "@S1")
}

// TestFeatureTagIDsFailsOnAScenarioWithoutExactlyOneID: a scenario
// tagged for two rows would cover both with one spec, and a scenario
// tagged for none is a spec the matrix does not know; each is reported.
func TestFeatureTagIDsFailsOnAScenarioWithoutExactlyOneID(t *testing.T) {
	for name, body := range map[string]string{
		"two ids": "Feature: a\n\n  @S1 @S2\n  Scenario: both\n    Given a\n",
		"no id":   "Feature: a\n\n  @wip\n  Scenario: untagged\n    Given a\n",
	} {
		dir := t.TempDir()
		writeFeature(t, dir, "a.feature", body)

		_, err := FeatureTagIDs(dir)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "want exactly one", name)
	}
}

// TestFeatureTagIDsIgnoresATagShapedLineInsideADocstring: only a tag
// the Gherkin parser attaches to a scenario counts; text that merely
// starts with "@S" is data.
func TestFeatureTagIDsIgnoresATagShapedLineInsideADocstring(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature",
		"Feature: a\n\n  @S1\n  Scenario: one\n    Given a note\n      \"\"\"\n      @S9\n      \"\"\"\n")

	ids, err := FeatureTagIDs(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"S1": true}, ids)
}

// TestFeatureTagIDsRejectsAMissingDirectory surfaces the path error
// rather than an empty set that would read as "no scenarios".
func TestFeatureTagIDsRejectsAMissingDirectory(t *testing.T) {
	_, err := FeatureTagIDs(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

// TestFeatureTagIDsIgnoresAnEmptyDirectory reports no ids rather than
// an error, so the bijection test's failure names the missing tags.
func TestFeatureTagIDsIgnoresAnEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "empty.feature", "")

	ids, err := FeatureTagIDs(dir)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// TestScenarioOfReadsTheOneIDAndWhetherPending: "@S16" names the id
// and "@pending" marks the scenario unwritten, while "@S016" and
// "@s16" name nothing; two ids, or none, is not exactly one.
func TestScenarioOfReadsTheOneIDAndWhetherPending(t *testing.T) {
	tagged := func(names ...string) *messages.Pickle {
		p := &messages.Pickle{Name: "one"}
		for _, name := range names {
			p.Tags = append(p.Tags, &messages.PickleTag{Name: name})
		}

		return p
	}

	got, err := scenarioOf("a.feature", 4, tagged("@pending", "@S16", "@S016", "@s16"))
	require.NoError(t, err)
	assert.Equal(t, Scenario{Path: "a.feature", Line: 4, Name: "one", ID: "S16", Pending: true}, got)

	_, err = scenarioOf("a.feature", 9, tagged("@S1", "@S2"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.feature:9: scenario \"one\" carries 2 S tags")

	_, err = scenarioOf("a.feature", 12, tagged("@wip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carries 0 S tags")
}

// TestRecordIDReportsASecondSighting pins the two outcomes: a first
// sighting of an id is recorded, a second is a repeat.
func TestRecordIDReportsASecondSighting(t *testing.T) {
	ids := map[string]bool{}
	require.NoError(t, recordID(Scenario{Path: "a.feature", Line: 4, ID: "S1"}, ids))
	assert.Equal(t, map[string]bool{"S1": true}, ids)

	err := recordID(Scenario{Path: "a.feature", Line: 9, ID: "S1"}, ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.feature:9: tag \"@S1\" repeated")
}

// TestScenarioLinesPlacesFeatureAndRuleScenarios maps a scenario's AST
// id to its line whether it sits under the feature or under a rule,
// and an empty document places nothing.
func TestScenarioLinesPlacesFeatureAndRuleScenarios(t *testing.T) {
	doc := &messages.GherkinDocument{Feature: &messages.Feature{Children: []*messages.FeatureChild{
		{Scenario: &messages.Scenario{Id: "s1", Location: &messages.Location{Line: 4}}},
		{Rule: &messages.Rule{Children: []*messages.RuleChild{
			{Scenario: &messages.Scenario{Id: "s2", Location: &messages.Location{Line: 9}}},
		}}},
	}}}
	assert.Equal(t, map[string]int64{"s1": 4, "s2": 9}, scenarioLines(doc))
	assert.Empty(t, scenarioLines(&messages.GherkinDocument{}))
}
