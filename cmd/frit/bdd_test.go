package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// featuresDir holds the executable lease-protocol scenarios: one
// tagged Gherkin scenario per matrix row.
const featuresDir = "../../features"

// TestFeatures runs every scenario declared under features/ as its own
// subtest named by its matrix id, so `-run TestFeatures/S16` picks one
// out. A scenario tagged @pending is declared but unwritten: it is
// skipped, and reported as such, rather than run as a pass that proves
// nothing. Every other scenario runs through godog in strict mode, so a
// step whose text matches no definition fails the build instead of
// passing as undefined. The suite lives in cmd/frit because this is the
// one package where everything a matrix row can name meets: the lease
// API, the repository fixtures, the herdr fake and the verbs.
func TestFeatures(t *testing.T) {
	features, err := godog.TestSuite{
		Options: &godog.Options{Paths: []string{featuresDir}},
	}.RetrieveFeatures()
	require.NoError(t, err)

	for _, f := range features {
		for _, p := range f.Pickles {
			names := make([]string, len(p.Tags))
			for i, tag := range p.Tags {
				names[i] = tag.Name
			}
			id, pending := scenarioTags(names)
			t.Run(id+" "+p.Name, func(t *testing.T) {
				if pending {
					t.Skip("pending: declared in the matrix, its steps not yet written")
				}
				runScenario(t, id)
			})
		}
	}
}

// scenarioTags reads a scenario's matrix id, and whether it is still
// pending, off its tags.
func scenarioTags(tags []string) (id string, pending bool) {
	for _, tag := range tags {
		switch {
		case tag == "@pending":
			pending = true
		case strings.HasPrefix(tag, "@S"):
			id = strings.TrimPrefix(tag, "@")
		}
	}

	return id, pending
}

// registrar binds one section's step texts to a fresh world built on
// the running subtest's *testing.T.
type registrar func(*testing.T, *godog.ScenarioContext)

// registrars is the step registry. Each section's step file appends its
// registrar from init — bdd_lease_test.go the lease vocabulary, a later
// bdd_<section>_test.go its own — so converting a section adds a file
// and never edits this one, and two sections can land in any order.
// godog runs strict, so a step text two sections both define is an
// ambiguity that fails at the second landing rather than a silent
// shadow.
var registrars []registrar

// bindAll registers every section's steps for one scenario run.
func bindAll(t *testing.T, sc *godog.ScenarioContext, regs []registrar) {
	t.Helper()
	for _, bind := range regs {
		bind(t, sc)
	}
}

// runScenario drives the one scenario tagged id through godog on this
// subtest's own goroutine: godog's per-scenario subtests are left off,
// so the world holds the real *testing.T the fixtures need and a
// failing step fails exactly this subtest. godog's report is kept and
// shown only when the scenario fails.
func runScenario(t *testing.T, id string) {
	t.Helper()
	var report bytes.Buffer
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) { bindAll(t, sc, registrars) },
		Options: &godog.Options{
			Paths:    []string{featuresDir},
			Tags:     "@" + id,
			Strict:   true,
			Format:   "progress",
			NoColors: true,
			Output:   &report,
		},
	}
	if status := suite.Run(); status != 0 {
		t.Errorf("scenario %s: godog exit status %d\n%s", id, status, report.String())
	}
}

func TestScenarioTagsReadsTheIDAndWhetherPending(t *testing.T) {
	id, pending := scenarioTags([]string{"@S16"})
	assert.Equal(t, "S16", id)
	assert.False(t, pending)

	id, pending = scenarioTags([]string{"@S17", "@pending"})
	assert.Equal(t, "S17", id)
	assert.True(t, pending)
}

// TestBindAllCallsEveryRegistrar: a section's step file appends its
// binder to the registry from init, and a scenario run binds every one
// of them on the subtest's own *testing.T — so a new section adds a
// file, never a line to this one.
func TestBindAllCallsEveryRegistrar(t *testing.T) {
	var seen []*testing.T
	probe := func(pt *testing.T, _ *godog.ScenarioContext) { seen = append(seen, pt) }

	bindAll(t, nil, []registrar{probe, probe})

	assert.Equal(t, []*testing.T{t, t}, seen)
	assert.NotEmpty(t, registrars, "the lease steps register themselves")
}
