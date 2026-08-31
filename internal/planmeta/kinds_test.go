package planmeta

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/mdsmith/pkg/mdsmith"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoFile reads a file relative to the repository root, resolved from
// this file's own path so the test does not depend on the working
// directory.
func repoFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	elems := append([]string{"..", ".."}, parts...)
	b, err := os.ReadFile(filepath.Join(elems...))
	require.NoError(t, err)

	return b
}

// phaseKindsSession opens a session against frit's own real
// .mdsmith.yml, the config `frit phase`'s companion files must lint
// under, with an in-memory copy of plan/proto.md — the plan kind's
// own required-structure schema — so the config loads the way it does
// at the repository root.
func phaseKindsSession(t *testing.T) *mdsmith.Session {
	t.Helper()
	cfg := repoFile(t, ".mdsmith.yml")
	proto := repoFile(t, "plan", "proto.md")

	sess, err := mdsmith.NewSession(mdsmith.SessionOptions{
		Workspace: mdsmith.NewMemWorkspace(map[string][]byte{
			"plan/proto.md": proto,
		}),
		Config: mdsmith.ConfigYAML(string(cfg)),
	})
	require.NoError(t, err)

	return sess
}

// catalogFixtureSession mirrors phaseKindsSession but adds a synthetic
// folder plan's companion files to the workspace, so a `## Phases`
// catalog directive in the fixture plan.md can glob its own phase-1.md
// and phase-1.result.md. Keyed by workspace-relative path.
func catalogFixtureSession(t *testing.T, files map[string][]byte) *mdsmith.Session {
	t.Helper()
	cfg := repoFile(t, ".mdsmith.yml")
	proto := repoFile(t, "plan", "proto.md")

	ws := map[string][]byte{"plan/proto.md": proto}
	maps.Copy(ws, files)

	sess, err := mdsmith.NewSession(mdsmith.SessionOptions{
		Workspace: mdsmith.NewMemWorkspace(ws),
		Config:    mdsmith.ConfigYAML(string(cfg)),
	})
	require.NoError(t, err)

	return sess
}

const catalogFixturePhaseSpec = "---\n" +
	"n: 1\n" +
	"title: A proving-slice phase\n" +
	"status: \"\U0001F532\"\n" +
	"result: false\n" +
	"---\n" +
	"Prove the thing works end to end.\n"

const catalogFixturePhaseRecord = "---\n" +
	"n: 1\n" +
	"title: A proving-slice phase\n" +
	"status: \"✅\"\n" +
	"result: true\n" +
	"summary: Proved the thing works end to end.\n" +
	"---\n" +
	"## Handoff\n\nDone.\n"

// catalogFixturePlan carries a `## Phases` catalog whose row-expr
// branches on the `result` discriminator: a spec row for phase-1.md,
// an indented `↳` summary row for phase-1.result.md, in that order —
// the interleaved table Phase 1 proves.
const catalogFixturePlanHeader = "---\n" +
	"id: 2601010000\n" +
	"title: X\n" +
	"status: \"\U0001F532\"\n" +
	"summary: fixture\n" +
	"model: sonnet\n" +
	"depends-on: []\n" +
	"---\n" +
	"# X\n\n" +
	"## Goal\n\nFixture goal.\n\n" +
	"## Tasks\n\n1. Do the thing.\n\n" +
	"## Phases\n\n" +
	"<?catalog\n" +
	"glob:\n" +
	// A MemWorkspace-backed Session lints via RunSource, which wires
	// lint.File.FS to the whole workspace's root FS rather than the
	// on-disk CLI's per-directory os.Root scoping — see catalogFixtureSession.
	// A bare "phase-*.md" pattern (workspace root) would match zero
	// files, so the fixture globs workspace-relative from plan/.
	"  - \"plan/2601010000_x/phase-*.md\"\n" +
	"  - \"plan/2601010000_x/phase-*.result.md\"\n" +
	"sort: numeric:n\n" +
	"header: |\n\n" +
	"  | # | Status | Phase |\n" +
	"  |---|--------|-------|\n" +
	"row-expr: |\n" +
	"  [if result {\n" +
	"    \"|  | ↳ | \\(summary) |\"\n" +
	"  }, if !result {\n" +
	"    \"| \\(n) | \\(status) | [\\(title)](phase-\\(n).md) |\"\n" +
	"  }][0]\n" +
	"footer: |\n\n" +
	"?>\n\n"

const catalogFixturePlanBodyClose = "<?/catalog?>\n\n" +
	"## Acceptance Criteria\n\n" +
	"- [ ] Fixture criterion\n"

// catalogFixtureInterleavedRows is the regenerated body Phase 1's
// gate requires: the spec row immediately followed by its
// result-summary row.
const catalogFixtureInterleavedRows = "" +
	"| #   | Status | Phase                               |\n" +
	"| --- | ------ | ----------------------------------- |\n" +
	"| 1   | \U0001F532     | [A proving-slice phase](phase-1.md) |\n" +
	"|     | ↳      | Proved the thing works end to end.  |\n"

// TestPhaseCatalogInterleavesSpecAndResultRow is Phase 1's RED: at
// HEAD the fixture's `result` and `summary` keys are undeclared in the
// closed phase-spec/phase-record frontmatter maps, so phase-1.md and
// phase-1.result.md each trip required-structure (MDS020) on their own
// Check — the catalog directive itself reads front matter directly and
// keeps rendering regardless, so the fixture's schema violations, not
// its rendered rows, are what gates RED here. Once GREEN, both phase
// files lint clean and the interleaved table still renders: phase-1.md's
// row directly followed by phase-1.result.md's summary row.
func TestPhaseCatalogInterleavesSpecAndResultRow(t *testing.T) {
	staleBody := catalogFixturePlanHeader +
		"| #   | Status | Phase                               |\n" +
		"| --- | ------ | ------------------------------------ |\n" +
		"| 1   | \U0001F532     | [A proving-slice phase](phase-1.md) |\n" +
		catalogFixturePlanBodyClose

	sess := catalogFixtureSession(t, map[string][]byte{
		"plan/2601010000_x/plan.md":           []byte(staleBody),
		"plan/2601010000_x/phase-1.md":        []byte(catalogFixturePhaseSpec),
		"plan/2601010000_x/phase-1.result.md": []byte(catalogFixturePhaseRecord),
	})

	specDiags, err := sess.Check("plan/2601010000_x/phase-1.md", []byte(catalogFixturePhaseSpec))
	require.NoError(t, err)
	assert.Empty(t, specDiags,
		"phase-1.md with a result discriminator: want no diagnostics, got %+v", specDiags)

	recordDiags, err := sess.Check("plan/2601010000_x/phase-1.result.md", []byte(catalogFixturePhaseRecord))
	require.NoError(t, err)
	assert.Empty(t, recordDiags,
		"phase-1.result.md with a result discriminator and summary: want no diagnostics, got %+v",
		recordDiags)

	result, err := sess.Fix("plan/2601010000_x/plan.md", []byte(staleBody))

	require.NoError(t, err)
	assert.True(t, result.Changed, "want the stale spec-only body rewritten")
	assert.Contains(t, result.Source, catalogFixtureInterleavedRows,
		"want the spec row directly followed by its result-summary row, got:\n%s",
		result.Source)
	assert.Empty(t, result.Diagnostics,
		"want the regenerated fixture clean, got %+v", result.Diagnostics)
}

// TestPhaseCatalogStaleBodyTripsMDS019 is Phase 1's other RED half: a
// folder plan whose `## Phases` body carries only the spec row —
// missing its result file's summary row — must trip MDS019 rather
// than lint clean, so a reader can trust the catalog reflects the
// result files on disk.
func TestPhaseCatalogStaleBodyTripsMDS019(t *testing.T) {
	staleBody := catalogFixturePlanHeader +
		"| #   | Status | Phase                               |\n" +
		"| --- | ------ | ------------------------------------ |\n" +
		"| 1   | \U0001F532     | [A proving-slice phase](phase-1.md) |\n" +
		catalogFixturePlanBodyClose

	sess := catalogFixtureSession(t, map[string][]byte{
		"plan/2601010000_x/plan.md":           []byte(staleBody),
		"plan/2601010000_x/phase-1.md":        []byte(catalogFixturePhaseSpec),
		"plan/2601010000_x/phase-1.result.md": []byte(catalogFixturePhaseRecord),
	})

	diags, err := sess.Check("plan/2601010000_x/plan.md", []byte(staleBody))

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "catalog"),
		"stale interleaved catalog: want a catalog (MDS019) diagnostic, got %+v", diags)
}

func hasDiagnostic(diags []mdsmith.Diagnostic, rule string) bool {
	for _, d := range diags {
		if d.Name == rule {
			return true
		}
	}

	return false
}

// TestPhaseSpecKindEnforcesTokenBudget is Phase 2's RED: today's
// config still runs the freeform companion override over
// plan/*/phase-N.md, so an oversized phase-N.md trips no
// token-budget diagnostic. Once the phase-spec kind lands, it does.
func TestPhaseSpecKindEnforcesTokenBudget(t *testing.T) {
	sess := phaseKindsSession(t)
	// 1500 words is ~2000 heuristic tokens, above the phase-spec
	// budget of 1800; the fixture must exceed the raised ceiling to
	// keep proving the kind enforces the budget at all.
	oversized := []byte(strings.Repeat("word ", 1500))

	diags, err := sess.Check("plan/2601010000_x/phase-1.md", oversized)

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "token-budget"),
		"phase-1.md: want a token-budget diagnostic, got %+v", diags)
}

// TestPhaseRecordKindEnforcesTokenBudget mirrors the spec case for
// phase-N.result.md.
func TestPhaseRecordKindEnforcesTokenBudget(t *testing.T) {
	sess := phaseKindsSession(t)
	// 1500 words is ~2000 heuristic tokens, above the phase-record
	// budget of 1800; the fixture must exceed the raised ceiling to
	// keep proving the kind enforces the budget at all.
	oversized := []byte(strings.Repeat("word ", 1500))

	diags, err := sess.Check("plan/2601010000_x/phase-1.result.md", oversized)

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "token-budget"),
		"phase-1.result.md: want a token-budget diagnostic, got %+v", diags)
}

// TestPhaseRecordKindAllowsAHandoffHeading proves the phase-record
// kind's schema stays open: a result file that is just a `## Handoff`
// section, the shape every phase-N.result.md in this repo already
// takes, lints clean under the new kind rather than tripping
// first-line-heading or paragraph-readability the way the repo's
// default rules otherwise would.
func TestPhaseRecordKindAllowsAHandoffHeading(t *testing.T) {
	sess := phaseKindsSession(t)
	body := repoFile(t, "plan",
		"2608300937_per-phase-files-token-cheap-resume",
		"phase-1.result.md")

	diags, err := sess.Check(
		"plan/2601010000_x/phase-1.result.md", body)

	require.NoError(t, err)
	assert.Empty(t, diags)
}

// TestPhaseSpecKindAcceptsIntPhaseNumber proves the typed schema does
// not flag the shape every phase file in this repo already carries: an
// otherwise-clean phase-N.md with an integer `n` trips no
// required-structure diagnostic.
func TestPhaseSpecKindAcceptsIntPhaseNumber(t *testing.T) {
	sess := phaseKindsSession(t)
	intN := []byte("---\nn: 1\n" +
		"title: A phase whose number is an integer\n" +
		"status: \"\U0001F532\"\nresult: false\n---\nBody.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-1.md", intN)

	require.NoError(t, err)
	assert.False(t, hasDiagnostic(diags, "required-structure"),
		"phase-1.md with int n: want no required-structure diagnostic, got %+v", diags)
}

// TestPhaseSpecKindAcceptsSplitPhaseNumber guards the split-phase case:
// a sitting that grows into two carries phase-3a.md and phase-3b.md with
// a `n` like `3b` — digits then a letter suffix, which PhaseNumber keeps
// as a string. The schema types `n` as int OR that token, so a split
// phase still lints; a bare `int` would reject it.
func TestPhaseSpecKindAcceptsSplitPhaseNumber(t *testing.T) {
	sess := phaseKindsSession(t)
	splitN := []byte("---\nn: 3b\n" +
		"title: The second half of a split phase\n" +
		"status: \"\U0001F532\"\nresult: false\n---\nBody.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-3b.md", splitN)

	require.NoError(t, err)
	assert.False(t, hasDiagnostic(diags, "required-structure"),
		"phase-3b.md with split n: want no required-structure diagnostic, got %+v", diags)
}

// TestPhaseSpecKindRejectsMalformedPhaseNumber proves the type still
// bites: a `n` that is neither an integer nor a digits-then-letters
// token — here a word — trips required-structure.
func TestPhaseSpecKindRejectsMalformedPhaseNumber(t *testing.T) {
	sess := phaseKindsSession(t)
	badN := []byte("---\nn: \"foo\"\n" +
		"title: A phase whose number is a word\n" +
		"status: \"\U0001F532\"\n---\nBody.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-1.md", badN)

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "required-structure"),
		"phase-1.md with word n: want a required-structure diagnostic, got %+v", diags)
}

// TestPhaseRecordKindAcceptsIntPhaseNumber mirrors the spec case for
// phase-N.result.md: the record carries the same `n`, typed the same way.
func TestPhaseRecordKindAcceptsIntPhaseNumber(t *testing.T) {
	sess := phaseKindsSession(t)
	intN := []byte("---\nn: 1\n" +
		"title: A phase whose number is an integer\n" +
		"status: \"\U0001F532\"\nresult: true\n" +
		"summary: Landed the integer case.\n---\n## Handoff\n\nDone.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-1.result.md", intN)

	require.NoError(t, err)
	assert.False(t, hasDiagnostic(diags, "required-structure"),
		"phase-1.result.md with int n: want no required-structure diagnostic, got %+v", diags)
}

// TestPhaseRecordKindAcceptsSplitPhaseNumber mirrors the split-phase
// guard for the record file, which shares its phase's `n`.
func TestPhaseRecordKindAcceptsSplitPhaseNumber(t *testing.T) {
	sess := phaseKindsSession(t)
	splitN := []byte("---\nn: 3b\n" +
		"title: The second half of a split phase\n" +
		"status: \"\U0001F532\"\nresult: true\n" +
		"summary: Landed the split-phase case.\n---\n## Handoff\n\nDone.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-3b.result.md", splitN)

	require.NoError(t, err)
	assert.False(t, hasDiagnostic(diags, "required-structure"),
		"phase-3b.result.md with split n: want no required-structure diagnostic, got %+v", diags)
}

// TestPhaseRecordKindRejectsMalformedPhaseNumber mirrors the reject case
// for the record file.
func TestPhaseRecordKindRejectsMalformedPhaseNumber(t *testing.T) {
	sess := phaseKindsSession(t)
	badN := []byte("---\nn: \"foo\"\n" +
		"title: A phase whose number is a word\n" +
		"status: \"\U0001F532\"\n---\n## Handoff\n\nDone.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-1.result.md", badN)

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "required-structure"),
		"phase-1.result.md with word n: want a required-structure diagnostic, got %+v", diags)
}

// TestPhaseSpecKindRequiresResultField is Phase 2's RED: `result` lands
// optional in Phase 1 so every existing phase-N.md still lints without
// it, but a folder plan's catalog needs it on every file to render the
// interleaved table. A `phase-1.md` missing `result` must trip
// required-structure once the field is tightened to required.
func TestPhaseSpecKindRequiresResultField(t *testing.T) {
	sess := phaseKindsSession(t)
	noResult := []byte("---\nn: 1\n" +
		"title: A phase missing its discriminator\n" +
		"status: \"\U0001F532\"\n---\nBody.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-1.md", noResult)

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "required-structure"),
		"phase-1.md with no result: want a required-structure diagnostic, got %+v", diags)
}

// TestPhaseRecordKindRequiresResultAndSummary mirrors the spec case for
// phase-N.result.md, which also needs `summary` for its catalog row.
func TestPhaseRecordKindRequiresResultAndSummary(t *testing.T) {
	sess := phaseKindsSession(t)
	noResultOrSummary := []byte("---\nn: 1\n" +
		"title: A record missing its discriminator and summary\n" +
		"status: \"✅\"\n---\n## Handoff\n\nDone.\n")

	diags, err := sess.Check("plan/2601010000_x/phase-1.result.md", noResultOrSummary)

	require.NoError(t, err)
	assert.True(t, hasDiagnostic(diags, "required-structure"),
		"phase-1.result.md with no result or summary: want a required-structure diagnostic, got %+v",
		diags)
}
