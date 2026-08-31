package planmeta

import (
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
		"status: \"\U0001F532\"\n---\nBody.\n")

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
		"status: \"\U0001F532\"\n---\nBody.\n")

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
		"status: \"\U0001F532\"\n---\n## Handoff\n\nDone.\n")

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
		"status: \"\U0001F532\"\n---\n## Handoff\n\nDone.\n")

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
