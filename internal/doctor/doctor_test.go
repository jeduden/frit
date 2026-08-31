package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFixtureRoot lays down a temp directory with frit's own
// plan/proto.md and .mdsmith.yml, copied rather than hand-rolled: the
// `plan` kind's required-structure rule needs the `proto` kind
// declared too (mdsmith's own kind-assignment validation), which a
// trimmed-down config easily gets wrong. Copying the real files keeps
// the fixture honest about what a real plan must satisfy.
func newFixtureRoot(t *testing.T) string {
	t.Helper()
	root := newFixtureRootNoConfig(t)

	cfg, err := os.ReadFile(filepath.Join("..", "..", ".mdsmith.yml"))
	require.NoError(t, err)
	require.NoError(t,
		os.WriteFile(filepath.Join(root, ".mdsmith.yml"), cfg, 0o600))

	return root
}

// newFixtureRootNoConfig is newFixtureRoot without the .mdsmith.yml, so
// a test can drive the no-config case doctor's session open must
// handle.
func newFixtureRootNoConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	proto, err := os.ReadFile(filepath.Join("..", "..", "plan", "proto.md"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plan"), 0o750))
	require.NoError(t,
		os.WriteFile(filepath.Join(root, "plan", "proto.md"), proto, 0o600))

	return root
}

// writePlan lays one plan file into root/plan.
func writePlan(t *testing.T, root, filename, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "plan", filename), []byte(content), 0o600))
}

// writeFolderPlan lays a folder plan's fixed plan.md into
// root/plan/<folder>/plan.md.
func writeFolderPlan(t *testing.T, root, folder, content string) {
	t.Helper()
	dir := filepath.Join(root, "plan", folder)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t,
		os.WriteFile(filepath.Join(dir, "plan.md"), []byte(content), 0o600))
}

// writeFolderPhaseFile lays a folder plan's phase-N.md spec file, with
// its own {n, title, status} front matter, into root/plan/<folder>.
func writeFolderPhaseFile(t *testing.T, root, folder, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "plan", folder)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t,
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

// ledgerFreeFolderPlanMD renders a folder plan's plan.md carrying no
// `phases:` ledger — the shape plan-new now writes — with an
// `## Execution` table that names a row for phase 1 only.
func ledgerFreeFolderPlanMD(id int64) string {
	return fmt.Sprintf(`---
id: %d
title: A ledger-free folder plan
status: "🔲"
model: sonnet
---
# A ledger-free folder plan

## Goal

Ship the thing.

## Execution

| Phase | Design | Implement | Gate     |
| ----- | ------ | --------- | -------- |
| 1 one | sonnet | sonnet    | test one |

## Tasks

1. Do it.

## Acceptance Criteria

- [ ] It is done.
`, id)
}

// phaseFileMD renders a phase-N.md spec file's own {n, title, status}
// front matter and a line of body.
func phaseFileMD(n int, title, status string) string {
	return fmt.Sprintf(`---
n: %d
title: %s
status: "%s"
---
Do the %s thing.
`, n, title, status, title)
}

// cleanPlanWithID renders a plan with no semantic gaps, so a fixture
// built from it isolates whatever check the test adds beyond it.
func cleanPlanWithID(id int64) string {
	return fmt.Sprintf(`---
id: %d
title: A clean plan
status: "🔲"
model: sonnet
phases:
  - { n: 1, title: 'One', status: "🔲" }
---
# A clean plan

## Goal

Ship the thing.

## Phase 1: One

Do the one thing.

## Execution

| Phase | Design | Implement | Gate |
| ----- | ------ | --------- | ---- |
| 1 one | sonnet | sonnet    | test one |

## Tasks

1. Do it.

## Acceptance Criteria

- [ ] It is done.
`, id)
}

// TestScanOpensSessionOnDefaultsWithNoConfig: a repo with plan/proto.md
// but no .mdsmith.yml scans on mdsmith's own built-in defaults, rather
// than failing to open the session at all.
func TestScanOpensSessionOnDefaultsWithNoConfig(t *testing.T) {
	root := newFixtureRootNoConfig(t)
	writePlan(t, root, "110_clean.md", cleanPlanWithID(110))

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestScanFindsNothingOnACleanPlan(t *testing.T) {
	root := newFixtureRoot(t)
	writePlan(t, root, "100_a-clean-plan.md", cleanPlanWithID(100))

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestScanReportsErrNoSchemaWhenProtoIsMissing(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plan"), 0o750))

	_, err := Scan(root, "plan")

	require.ErrorIs(t, err, ErrNoSchema)
}

func TestScanFlagsAnEmptyGoal(t *testing.T) {
	root := newFixtureRoot(t)
	src := `---
id: 101
title: An empty goal
status: "🔲"
model: sonnet
---
# An empty goal

## Goal

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`
	writePlan(t, root, "101_an-empty-goal.md", src)

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(101), got[0].ID)
	assert.Equal(t, "goal", got[0].Check)
	assert.Contains(t, got[0].Message, "## Goal")
}

func TestScanFlagsAModelThatNamesNoKnownTier(t *testing.T) {
	root := newFixtureRoot(t)
	src := `---
id: 102
title: A bad model
status: "🔲"
model: sparkle
---
# A bad model

## Goal

Ship it.

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`
	writePlan(t, root, "102_a-bad-model.md", src)

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "schema", got[0].Check)
	assert.Contains(t, got[0].Message, "sparkle")
}

func TestScanFlagsAPhaseWithNoExecutionRow(t *testing.T) {
	root := newFixtureRoot(t)
	src := `---
id: 103
title: A phase with no row
status: "🔲"
model: sonnet
phases:
  - { n: 1, title: 'One', status: "🔲" }
---
# A phase with no row

## Goal

Ship it.

## Phase 1: One

Do the one thing.

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`
	writePlan(t, root, "103_a-phase-with-no-row.md", src)

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "execution-row", got[0].Check)
	assert.Contains(t, got[0].Message, "phase 1")
}

func TestScanFlagsAPhaseTierThatNamesNoKnownModel(t *testing.T) {
	root := newFixtureRoot(t)
	src := `---
id: 104
title: A phase with a bad tier
status: "🔲"
model: sonnet
phases:
  - { n: 1, title: 'One', status: "🔲" }
---
# A phase with a bad tier

## Goal

Ship it.

## Phase 1: One

Do the one thing.

## Execution

| Phase | Design  | Implement | Gate     |
| ----- | ------- | --------- | -------- |
| 1 one | sparkle | sonnet    | test one |

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`
	writePlan(t, root, "104_a-phase-with-a-bad-tier.md", src)

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "tier", got[0].Check)
	assert.Contains(t, got[0].Message, "sparkle")
}

// TestScanFlagsAMissingExecutionRowForALedgerFreeFolderPlan is Phase
// 1's RED: a folder plan carrying no `phases:` ledger has its phases
// assembled from phase-*.md front matter, so doctor validates its
// Execution rows. A phase whose number has no Execution row is flagged,
// the same as for a ledgered plan. At HEAD Plan.Phases is empty for such
// a plan, so doctor walks nothing and the finding is absent.
func TestScanFlagsAMissingExecutionRowForALedgerFreeFolderPlan(t *testing.T) {
	root := newFixtureRoot(t)
	const folder = "2601030000_ledger-free"
	writeFolderPlan(t, root, folder, ledgerFreeFolderPlanMD(2601030000))
	writeFolderPhaseFile(t, root, folder, "phase-1.md", phaseFileMD(1, "one", "🔲"))
	writeFolderPhaseFile(t, root, folder, "phase-2.md", phaseFileMD(2, "two", "🔲"))

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(2601030000), got[0].ID)
	assert.Equal(t, "execution-row", got[0].Check)
	assert.Contains(t, got[0].Message, "phase 2")
}

// TestScanToleratesAPhaseFileWithoutFrontMatter: a ledger-free folder
// plan whose phase file predates the {n, title, status} convention —
// pure prose, no front matter — must not abort the whole scan and lose
// every plan's findings. doctor keys it by its file-name number and
// still validates its Execution row, the same leniency Resume applies.
func TestScanToleratesAPhaseFileWithoutFrontMatter(t *testing.T) {
	root := newFixtureRoot(t)
	const folder = "2601030000_ledger-free"
	writeFolderPlan(t, root, folder, ledgerFreeFolderPlanMD(2601030000))
	writeFolderPhaseFile(t, root, folder, "phase-1.md",
		"Just prose, no front matter.\n")
	writeFolderPhaseFile(t, root, folder, "phase-2.md", "Also prose.\n")

	got, err := Scan(root, "plan")

	require.NoError(t, err,
		"a phase file with no front matter must not abort the scan")
	require.Len(t, got, 1)
	assert.Equal(t, "execution-row", got[0].Check)
	assert.Contains(t, got[0].Message, "phase 2")
}

// TestScanSortsFindingsByPlanIDThenCheck: a fleet-wide report reads
// the same way on every run, not in filesystem-glob order.
func TestScanSortsFindingsByPlanIDThenCheck(t *testing.T) {
	root := newFixtureRoot(t)
	writePlan(t, root, "200_second.md", `---
id: 200
title: Second
status: "🔲"
model: sonnet
---
# Second

## Goal

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`)
	writePlan(t, root, "100_first.md", `---
id: 100
title: First
status: "🔲"
model: bogus
---
# First

## Goal

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`)

	got, err := Scan(root, "plan")

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, int64(100), got[0].ID)
	assert.Equal(t, int64(100), got[1].ID)
	assert.Equal(t, int64(200), got[2].ID)
	assert.Less(t, got[0].Check, got[1].Check,
		"same plan, checks sort by name: goal before schema")
}

// TestScanSeesFolderPlansAndProvesIDSync is Phase 3's RED: a folder
// plan is scanned for the same gaps a flat plan is, and a plan of
// either shape whose on-disk name disagrees with its front-matter id
// is reported — never crashed on, never silently skipped.
func TestScanSeesFolderPlansAndProvesIDSync(t *testing.T) {
	root := newFixtureRoot(t)
	writeFolderPlan(t, root, "2601030000_synced", cleanPlanWithID(2601030000))
	writeFolderPlan(t, root, "2601040000_skewed", cleanPlanWithID(2601999999))
	writeFolderPlan(t, root, "notanid_x", cleanPlanWithID(999))
	writePlan(t, root, "2601060000_flatskew.md", cleanPlanWithID(2601999999))

	got, err := Scan(root, "plan")
	require.NoError(t, err)

	byPath := map[string][]Finding{}
	for _, f := range got {
		byPath[f.Path] = append(byPath[f.Path], f)
	}

	assert.Empty(t,
		byPath[filepath.Join("plan", "2601030000_synced", "plan.md")],
		"a folder plan whose name agrees with its id is scanned clean, "+
			"like a flat plan")

	skewed := byPath[filepath.Join("plan", "2601040000_skewed", "plan.md")]
	require.Len(t, skewed, 1)
	assert.Equal(t, "id-sync", skewed[0].Check)

	notAnID := byPath[filepath.Join("plan", "notanid_x", "plan.md")]
	require.Len(t, notAnID, 1, "a non-numeric prefix is a mismatch, not a crash")
	assert.Equal(t, "id-sync", notAnID[0].Check)

	flatSkew := byPath["plan/2601060000_flatskew.md"]
	require.Len(t, flatSkew, 1, "the id-sync check is not folder-only")
	assert.Equal(t, "id-sync", flatSkew[0].Check)
}

// TestScanSkipsADirectoryThatMatchesAPlanGlob: filepath.Glob matches
// directories as well as files, and a folder plan makes "plan.md" a
// name a directory can plausibly collide with (a typo'd folder-plan
// path, or a flat-glob match on a directory literally named like
// "<id>_<slug>.md"). One such entry must not make Scan fail outright
// and lose every other plan's findings — it is skipped like any other
// non-plan path.
func TestScanSkipsADirectoryThatMatchesAPlanGlob(t *testing.T) {
	root := newFixtureRoot(t)
	writePlan(t, root, "2601030000_ok.md", cleanPlanWithID(2601030000))
	require.NoError(t, os.MkdirAll(
		filepath.Join(root, "plan", "2601020000_oops.md"), 0o750))

	got, err := Scan(root, "plan")

	require.NoError(t, err, "one bad path must not blind the whole scan")
	assert.Empty(t, got, "the one real plan is clean")
}

func TestPlanPathsListsFlatAndFolderPlansTogetherSorted(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plan"), 0o750))
	writePlan(t, root, "2601020000_b.md", "")
	writePlan(t, root, "2601010000_a.md", "")
	writeFolderPlan(t, root, "2601015000_mid", "")

	got, err := planPaths(root, "plan")

	require.NoError(t, err)
	want := []string{
		filepath.Join(root, "plan", "2601010000_a.md"),
		filepath.Join(root, "plan", "2601015000_mid", "plan.md"),
		filepath.Join(root, "plan", "2601020000_b.md"),
	}
	assert.Equal(t, want, got)
}

func TestCheckIDSyncFlagsOnlyAMismatch(t *testing.T) {
	assert.Nil(t, checkIDSync(2601010000, "plan/2601010000_x.md"))

	got := checkIDSync(2601010000, "plan/2601999999_x.md")
	require.NotNil(t, got)
	assert.Equal(t, "id-sync", got.Check)
}

func TestLeadingIDTokenReadsTheFolderNameForAFolderPlan(t *testing.T) {
	assert.Equal(t, "2601010000",
		leadingIDToken("plan/2601010000_x/plan.md"))
	assert.Equal(t, "2601010000",
		leadingIDToken("plan/2601010000_x.md"))
	assert.Equal(t, "noid",
		leadingIDToken("plan/noid/plan.md"), "no underscore, name as-is")
}
