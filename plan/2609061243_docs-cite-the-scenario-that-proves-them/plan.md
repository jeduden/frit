---
id: 2609061243
title: A behavioral doc cites the scenario that proves it
status: "✅"
summary: >-
  frit's behavior is catalogued twice: once as prose in the docs, and
  once as an executable scenario matrix — S-rows in lease-protocol.md
  and C-rows in command-scenarios.md, each with a tagged scenario under
  features/, held in bijection by a test. The two never point at each
  other. claiming.md narrates the very lease behaviors the S-matrix
  catalogs and cites not one id; ux-principles.md, commands.md,
  architecture.md and reaping.md cite none either; and the matrix does
  not link its own feature files. So a reader cannot cross from a prose
  claim to the running test that proves it, and prose can drift from
  the scenarios with nothing flagging it. Adopt a convention: where a
  doc asserts a concrete behavior a scenario proves, it names the id
  and links the feature. The claim becomes traceable to a test, and a
  scenario that changes shows which prose to revisit. Conceptual "why"
  prose and the dated research notes stay as they are.
model: sonnet
depends-on: []
---
# A behavioral doc cites the scenario that proves it

## Goal

Where a doc asserts a concrete behavior an executable scenario proves,
it names that scenario's id and links its feature. A reader crosses
from the prose to the running test in one hop, and a scenario that
changes names the prose to revisit.

## Context

**The gap, measured.** frit catalogs its behavior twice. The prose
docs describe it; the scenario matrix runs it — S-rows in
[lease-protocol.md](../../docs/research/lease-protocol.md) and C-rows
in [command-scenarios.md](../../docs/research/command-scenarios.md),
each with a `@S<n>`/`@C<n>` scenario under
[features/](../../features), held in bijection by
`go test ./internal/scenario`. The two never reference each other. A
grep for scenario ids finds them only in
[development.md](../../docs/development.md), which documents the matrix
mechanics. [claiming.md](../../docs/claiming.md),
[ux-principles.md](../../docs/ux-principles.md),
[commands.md](../../docs/commands.md),
[architecture.md](../../docs/architecture.md) and
[reaping.md](../../docs/reaping.md) cite none, and the matrix does not
link its own feature files.

**Why it matters.** claiming.md's own sections — two machines at once,
staleness and takeover, liveness veto, self-resume, fencing and yield,
when a claim is refused — are the behaviors the S-matrix exists to
prove. Today a reader has no way from that prose to the scenario, and
a change to a scenario leaves no trail to the sentence it dates. The
executable matrix is the source of truth for behavior; the prose
should point at it, not restate it and drift.

**Reuse first.** No new machinery. The matrix, the tagged scenarios
and the bijection gate already exist. The ids are stable names a doc
can cite in prose (`S16`, `C1`) and the feature files are linkable
paths. This plan adds citations and links, not a catalog or a test.

**Scope — behavioral claims only.** A citation belongs where a doc
asserts a concrete behavior a scenario proves: a refusal's wording, a
takeover maturing, a fenced lane parking. It does not belong on the
conceptual "why" — the boundaries, the one-mutation rule — which no
single scenario proves, nor on the
[research notes](../../docs/research), which CLAUDE.md keeps dated
rather than current. Forcing links there would be noise, not
traceability.

**Out of scope.** No change to any behavior, scenario, or the matrix
rows themselves. No new lint requiring a citation — this plan sets the
convention and applies it; a gate that enforces it is later work if
wanted at all.

## Tasks

1. Phase 1 (proving slice): claiming.md cites the S-id for each lease
   behavior it describes, and lease-protocol.md links each matrix
   section to its feature file. Establishes the citation shape the
   other docs copy.
2. Later phases: ux-principles.md and commands.md cite the C-ids and
   S-ids for the verb and dry-run behaviors they describe;
   architecture.md and reaping.md cite where a scenario proves a claim.

## Execution

| Phase | Title                                          | Tier   | Gate                                                                                                                                             |
| ----- | ---------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | claiming.md and the matrix point at each other | sonnet | every id claiming.md cites exists as a matrix row (bijection gate green); each matrix section links a real feature file; `mdsmith check .` clean |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                      |
| --- | ------ | -------------------------------------------------------------------------- |
| 1   | ✅     | [claiming.md and the matrix point at each other](phase-1.md)               |
|     | ↳      | claiming.md cites its scenarios; lease-protocol.md links its feature files |
<?/catalog?>

## Acceptance Criteria

- [x] claiming.md cites the scenario id for each concrete lease
      behavior it describes, and the id exists as a matrix row
- [x] lease-protocol.md links each of its matrix sections to the
      feature file that holds its scenarios
- [x] No citation is added to conceptual "why" prose or to the dated
      research notes
- [x] The bijection gate `go test ./internal/scenario` stays green —
      no cited id names a missing row
- [x] `mdsmith check .` is clean and every added link resolves
