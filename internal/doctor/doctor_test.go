package doctor

import (
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
	root := t.TempDir()

	proto, err := os.ReadFile(filepath.Join("..", "..", "plan", "proto.md"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plan"), 0o750))
	require.NoError(t,
		os.WriteFile(filepath.Join(root, "plan", "proto.md"), proto, 0o600))

	cfg, err := os.ReadFile(filepath.Join("..", "..", ".mdsmith.yml"))
	require.NoError(t, err)
	require.NoError(t,
		os.WriteFile(filepath.Join(root, ".mdsmith.yml"), cfg, 0o600))

	return root
}

// writePlan lays one plan file into root/plan.
func writePlan(t *testing.T, root, filename, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "plan", filename), []byte(content), 0o600))
}

const cleanPlan = `---
id: 100
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
`

func TestScanFindsNothingOnACleanPlan(t *testing.T) {
	root := newFixtureRoot(t)
	writePlan(t, root, "100_a-clean-plan.md", cleanPlan)

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
