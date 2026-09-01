---
n: 1
title: The godog harness runs one real scenario, and the coverage gate binds the matrix
status: "🔲"
result: false
---
Stand the BDD harness up and make it self-checking. One matrix scenario
becomes a real Gherkin spec running over the existing fixtures; every
other id is registered as a pending scenario; a coverage gate binds the
matrix to the features. This fixes the shape later phases copy when they
convert pending scenarios to real ones.

**Assumes.** The matrix in
[lease-protocol.md](../../docs/research/lease-protocol.md) is a Markdown
table whose scenario rows lead with an id cell, `| S3 | … |`. The lease
tests in [internal/claim](../../internal/claim) and
[cmd/frit](../../cmd/frit) already build real origin-and-clone git
fixtures and a herdr fake that a step definition can drive. godog runs
as a `*_test.go` `TestMain` that hands its scenarios to `go test`, and
its steps may return a pending result so a declared scenario need not be
implemented yet.

**Value.** The scenario matrix stops being prose a comment points at and
becomes an executable, self-checking registry. A scenario added to the
doc without a spec, or a spec tag naming no scenario, fails the build
rather than passing silently. The one real scenario proves the fixture
wiring end to end.

**RED.** Two failing tests first.

- `TestMatrixAndFeaturesAreInBijection` in a new `internal/scenario`
  package: parse the matrix rows for their `S<n>` ids, parse the
  `.feature` files for their `@S<n>` tags, assert the two sets are
  equal. Red now — no features exist. Anchor the matrix parse to table
  rows, so a bare "S76" in prose elsewhere is not counted.
- A godog scenario for one stable, self-contained matrix id — a fence
  case such as S16 (host resurrected → FENCE), chosen because it does
  not touch the resume path plan 2609011836 is reworking — written
  Given/When/Then against the fixtures, red until its steps exist.

**GREEN.** Build the harness in three moves.

- Add `features/lease.feature` (or per-section files) declaring all 86
  ids as `@S<n>` scenarios: the chosen one fully written, every other a
  single pending step, so the bijection holds while the work is
  incremental.
- Add the step definitions for the chosen scenario over the existing
  origin-and-clone fixtures and herdr fake, and the godog `TestMain`
  runner wired into `go test`.
- Decide godog's placement: a test-only require on the main module, or
  an isolated module, whichever keeps godog's graph from constraining a
  library consumer. Record the choice in the handoff.

**Guard the edges.** A pending scenario counts as present — the gate
checks the tag exists, not that its steps are written. A tag naming no
matrix row fails, as does a row with no tag. The matrix parse reads only
the scenario tables, not the header prose or the F/A attacker tables, so
the id set is exactly the S-scenarios. A malformed table row fails loud
rather than silently dropping an id.

**Gate.** `go test ./...` runs the godog suite: the one real scenario
passes, the rest report pending, and `TestMatrixAndFeaturesAreInBijection`
is green. Add a matrix row with no scenario in a scratch check and watch
the bijection test go red, then revert. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
