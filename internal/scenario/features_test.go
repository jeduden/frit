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

// TestScenarioIDsListsOnlyCleanSTags: "@S16" names an id, while
// "@pending", "@S016" and "@s16" do not.
func TestScenarioIDsListsOnlyCleanSTags(t *testing.T) {
	tags := []*messages.PickleTag{
		{Name: "@pending"}, {Name: "@S16"}, {Name: "@S016"}, {Name: "@s16"}, {Name: "@S2"},
	}
	assert.Equal(t, []string{"S16", "S2"}, scenarioIDs(tags))
}

// TestRecordIDKeepsOneIDPerScenario pins the three outcomes: one id is
// recorded, a second sighting of it is a repeat, and none or several
// is not exactly one.
func TestRecordIDKeepsOneIDPerScenario(t *testing.T) {
	ids := map[string]bool{}
	require.NoError(t, recordID("a.feature", 4, "one", []string{"S1"}, ids))
	assert.Equal(t, map[string]bool{"S1": true}, ids)

	err := recordID("a.feature", 9, "dup", []string{"S1"}, ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.feature:9: tag \"@S1\" repeated")

	err = recordID("a.feature", 12, "none", nil, ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carries 0 S tags")
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
