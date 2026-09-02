package main

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// featuresDir holds the executable lease-protocol scenarios: one
// tagged Gherkin scenario per matrix row.
const featuresDir = "../../features"

// TestFeatures runs every scenario declared under features/ as its own
// subtest, named "<id>: <title>" so `-run 'TestFeatures/^S16:'` picks
// exactly one out — an unanchored `S1` would also match S10 to S19.
// The scenarios are listed through the same walk the bijection gate
// reads, so a scenario with no id, or several, fails here naming its
// place instead of running under an empty tag. A scenario tagged
// @pending is declared but unwritten: it is skipped, and reported as
// such, rather than run as a pass that proves nothing. Every other
// scenario runs through godog in strict mode, so a step whose text
// matches no definition fails the build instead of passing as
// undefined. The suite lives in cmd/frit because this is the one
// package where everything a matrix row can name meets: the lease API,
// the repository fixtures, the herdr fake and the verbs.
func TestFeatures(t *testing.T) {
	scenarios, err := scenario.Scenarios(featuresDir)
	require.NoError(t, err)

	for _, sc := range scenarios {
		t.Run(sc.ID+": "+sc.Name, func(t *testing.T) {
			if sc.Pending {
				t.Skip("pending: declared in the matrix, its steps not yet written")
			}
			runScenario(t, sc.ID, sc.Path)
		})
	}
}

// registrar binds one section's step texts to the scenario's world.
type registrar func(*world, *godog.ScenarioContext)

// registrars is the step registry. Each section's step file appends its
// registrar from init — bdd_lease_test.go the lease vocabulary, a later
// bdd_<section>_test.go its own — so converting a section adds a file
// and never edits this one, and two sections can land in any order.
// Every registrar binds on the one world a scenario threads, so a
// section's step reads what a reused lease step set up. godog runs
// strict, so a step text two sections both define is an ambiguity that
// fails at the second landing rather than a silent shadow.
var registrars []registrar

// bindAll registers every section's steps on w for one scenario run.
func bindAll(w *world, sc *godog.ScenarioContext, regs []registrar) {
	w.t.Helper()
	for _, bind := range regs {
		bind(w, sc)
	}
}

// section is a section's own state beside the shared world, keyed by
// its type: created on first use, the same value for the rest of the
// scenario, and gone with the world. A section declares a struct for
// what its rows track and never adds a field to world.
func section[T any](w *world) *T {
	key := reflect.TypeFor[T]()
	if v, ok := w.sections[key]; ok {
		return v.(*T)
	}
	v := new(T)
	w.sections[key] = v

	return v
}

// runScenario drives the one scenario tagged id, in the feature file
// at path, through godog on this subtest's own goroutine: godog's
// per-scenario subtests are left off, so the world holds the real
// *testing.T the fixtures need and a failing step fails exactly this
// subtest. Only the scenario's own file is parsed, not the whole
// directory again. godog's report is kept and shown only when the
// scenario fails.
func runScenario(t *testing.T, id, path string) {
	t.Helper()
	var report bytes.Buffer
	w := newWorld(t)
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) { bindAll(w, sc, registrars) },
		Options: &godog.Options{
			Paths:    []string{path},
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

// TestBindAllBindsEveryRegistrarOnOneWorld: a section's step file
// appends its binder to the registry from init, and a scenario run
// hands every one of them the same world, built on the subtest's own
// *testing.T — so a section's step can read what a reused lease step
// set up, and a new section adds a file, never a line to this one.
func TestBindAllBindsEveryRegistrarOnOneWorld(t *testing.T) {
	var seen []*world
	probe := func(w *world, _ *godog.ScenarioContext) { seen = append(seen, w) }
	w := newWorld(t)

	bindAll(w, nil, []registrar{probe, probe})

	assert.Equal(t, []*world{w, w}, seen)
	assert.Same(t, t, w.t)
	assert.NotEmpty(t, registrars, "the lease steps register themselves")
}

// TestSectionStateIsOnePerTypePerWorld: a section keeps its own state
// beside the shared world, keyed by its type — one value per scenario,
// created on first use, the same pointer on every later call.
func TestSectionStateIsOnePerTypePerWorld(t *testing.T) {
	type storage struct{ origin string }
	w := newWorld(t)

	first := section[storage](w)
	first.origin = "/o"

	assert.Same(t, first, section[storage](w))
	assert.Equal(t, "/o", section[storage](w).origin)
	assert.NotSame(t, first, section[storage](newWorld(t)), "another scenario, another value")
}
