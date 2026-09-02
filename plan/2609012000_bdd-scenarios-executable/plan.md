---
id: 2609012000
title: Every lease-protocol scenario has an executable BDD spec
status: "✅"
summary: >-
  The lease protocol commits to a numbered scenario matrix (S1..S87) in
  docs/research/lease-protocol.md, and the code cites those S-numbers in
  its comments as load-bearing references. But the scenarios are covered
  only by scattered Go unit and cmd tests, with no guarantee that every
  documented scenario has a test, and no Given/When/Then spec tying a
  test to the scenario it proves. A scenario can be added to the matrix —
  as the #122 rework adds to S76/S77 — with its test relationship left
  implicit. This plan makes the matrix executable with BDD: each scenario
  becomes a Gherkin scenario tagged with its id, run by godog over the
  origin-and-clone git fixtures and herdr fake the lease tests already
  build, and a coverage gate asserts the matrix's ids and the feature
  tags are in bijection. A scenario can then never be documented without
  a spec, or specced without a doc row. godog's pending steps let every
  scenario be declared from the start and fleshed out incrementally, so
  the gate is green from Phase 1 while the backfill proceeds in themed
  batches that mirror the matrix's own sections.
model: sonnet
depends-on: []
---
# Every lease-protocol scenario has an executable BDD spec

## Goal

Each scenario in the lease-protocol matrix is backed by an executable
Gherkin spec, run by godog over the fixtures the lease tests already
build. A coverage gate keeps the matrix ids and the feature tags in
bijection. So a scenario can never be documented without a test, or
tested without a doc row.

## Context

**The gap.** The lease protocol enumerates its guarantees as a scenario
matrix, S1..S87, and the code cites those ids in comments (`S76`, `F10`,
`A1`) as the reason a branch exists. The scenarios are covered today by
Go unit and cmd tests scattered across packages, keyed to the matrix
only by a comment. Nothing checks that every documented scenario has a
test, nor reads as a behavioral spec — a Given state, a When verb, a
Then outcome — against the matrix's own "state → outcome" shape. When a
scenario is added, as the #122 rework extends S76/S77, its test link is
left to convention.

**Reuse first.** The lease tests in
[internal/claim](../../internal/claim) and
[cmd/frit](../../cmd/frit) already build real origin-and-clone git
fixtures and a herdr fake. The BDD steps drive those, not new
scaffolding. The matrix table stays the scenario registry — the single
list — and the feature files the single spec; the gate keeps the two in
bijection rather than adding a third source to drift.

**Approach: godog.** godog is Cucumber for Go: Gherkin `.feature` files,
one scenario per matrix id tagged `@S1` … `@S87`, steps in Go over the
existing fixtures. A plain Go coverage test parses the matrix rows for
their ids and the feature files for their `@S` tags and asserts the two
sets are equal. A `@pending` tag lets a scenario be declared — its id
tag present, the gate satisfied — before its steps are written, so all
87 scenarios are registered in Phase 1 and converted to real steps
incrementally, the gate green throughout.

**Considered and not taken.** A Go-native table-driven registry without
Gherkin is simpler and adds no dependency, but it is not BDD — the ask
is the Given/When/Then spec, which reads directly against the matrix.
Duplicating the id list into a separate machine-readable file is
rejected: two lists drift; the matrix and the features stay the only
two, reconciled by the gate. godog is a test-only dependency; whether it
rides the main module's test graph or an isolated module is a Phase 1
call, kept from constraining any library consumer per the
[tools/go.mod](../../tools/go.mod) ethos.

**Out of scope.** The F liveness and A safety attacker analyses are not
in this pass — the S-matrix is "the scenarios"; a later plan can extend
the same harness to `@F`/`@A`. The BDD layer is the scenario-level spec
and the coverage gate, not a rewrite of the unit tests that already
assert the internals.

## Tasks

1. Phase 1 (proving slice): stand up the godog harness over the existing
   fixtures; express one stable scenario as a fully-implemented tagged
   Gherkin spec end to end; register every other matrix id as a pending
   tagged scenario; add the coverage gate asserting the matrix ids and
   the feature tags are in bijection. The gate is green with one real
   scenario and the rest pending.
2. Later phases: convert pending scenarios to real specs in themed
   batches that mirror the matrix's own sections — process death, host
   death, partitions, races, clocks, storage, identity, lifecycle,
   cross-layer.

## Execution

| Phase | Title                                                                                            | Tier   | Gate                                                                                                                                 |
| ----- | ------------------------------------------------------------------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | The godog harness runs one real scenario, and the coverage gate binds the matrix to the features | sonnet | godog runs green — one real scenario, the rest pending; the coverage test fails if a matrix id lacks a scenario or a tag lacks a row |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                                                          |
| --- | ------ | ---------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The godog harness runs one real scenario, and the coverage gate binds the matrix](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [x] godog runs from `go test ./...`, in the one package where the
      existing origin-and-clone fixtures, the herdr fake and the verbs
      are all reachable; the real scenario drives the git fixtures,
      and the herdr fake waits for the first session-liveness scenario
      a later batch converts
- [x] One stable matrix scenario is a fully-implemented Gherkin spec and
      passes
- [x] Every matrix id has a tagged Gherkin scenario, real or pending
- [x] A coverage test asserts the matrix ids and the feature tags are in
      bijection, and it fails red when a matrix row is added with no
      scenario, or a tag names no row
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
