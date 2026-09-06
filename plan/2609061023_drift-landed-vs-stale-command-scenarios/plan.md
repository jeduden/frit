---
id: 2609061023
title: frit drift proves its landed-vs-stale reconciliation with command scenarios
status: "✅"
summary: >-
  A BDD-only run leaves frit drift entirely dark — the whole verb at
  zero coverage — while it is the verb a developer trusts to tell
  landed work from a status that never flipped. Plan 2609052303 just
  landed the home for exactly this: a command-scenario catalog,
  command-scenarios.md, numbered C<n> and kept in bijection with
  features/ by the same gate as the lease matrix. This plan gives
  drift's core signal its first command scenarios there. A plan whose
  work merged into main, but whose status still says in progress,
  drift reports as landed and names the commit carrying the plan's id.
  The proving slice is one C-scenario end to end; the rest follow the
  same shape — squash-merged work still read as landed, a bare claim's
  marker-only commits never mistaken for finished work, and the --json
  a consumer branches on.
model: sonnet
depends-on: []
---
# frit drift proves its landed-vs-stale reconciliation with command scenarios

## Goal

`frit drift`'s core reconciliation — a plan whose work reached `main`
while its status still lags — is proven end to end by command
scenarios, not unit tests alone. A developer reading `drift` trusts it
because a scenario drives the real verb over real landed evidence.

## Context

**The gap.** A BDD-only coverage run — every scenario, nothing else —
leaves `frit drift` at zero. The reconciliation verb a developer
leans on ("trust `frit drift`, not commit subjects") has no scenario
driving it. Its logic in [cmd/frit/drift.go](../../cmd/frit/drift.go)
— `landed`, `allCommits`, `commitsNaming`, `printDrift` — is exercised
only by unit tests, if at all.

**The home now exists.** Plan 2609052303 landed
[command-scenarios.md](../../docs/research/command-scenarios.md): a
catalog for behavior that is not the lease protocol, numbered `C<n>`
so a reader tells it from an `S<n>` at a glance. It is kept in
bijection with `features/` by the same gate, `MatrixIDsAll` in
[internal/scenario](../../internal/scenario/matrix.go). drift is
exactly its case: a single host reads landed evidence, with no lease
race in sight. This plan writes `C<n>` rows there, not `S<n>` rows in
the lease matrix.

**Reuse first.** The catalog and its feature file already stand. `C1`
in [features/commands.feature](../../features/commands.feature) is the
worked example, its steps bound in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
This plan appends `C<n>` rows and drift steps to those same files,
reusing the world a command scenario threads rather than a new one.
drift's own detectors are reused unchanged: no drift behavior is added
here, only a scenario over code that already passes.

**C-id allocation.** The next free id is `C2` today. A concurrent lane
may take it first, so the implementing lane merges `main`, reads the
catalog, and allocates the next free `C<n>` — never a hardcoded id
across lanes.

**Out of scope.** No change to what drift reports. Phase-level drift
and drift's no-false-positive cases are a sibling plan that runs after
this one, since both edit the same catalog and step file.

## Tasks

1. Phase 1 (proving slice): a `C<n>` scenario for the core signal — a
   plan whose merged work drift reports as landed, naming the commit
   that carries the plan's id. Drives the real `frit drift` command
   and establishes the drift step vocabulary in the command world.
2. Later phases: squash-merged work still read as landed; a bare
   claim's marker-only commits never called landed; the `--json`
   drift a consumer branches on.

## Execution

| Phase | Title                                                      | Tier   | Gate                                                                                                                                                 |
| ----- | ---------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | A merged plan drift reports as landed, named by its commit | sonnet | the new `C<n>` scenario runs against the built frit, drift reports the plan landed and names the commit; bijection gate green; `go test ./...` green |

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

| #   | Status | Phase                                                                    |
| --- | ------ | ------------------------------------------------------------------------ |
| 1   | ✅     | [A merged plan drift reports as landed, named by its commit](phase-1.md) |
|     | ↳      | C2 proves drift's core signal end to end — landed, named by its commit.  |
<?/catalog?>

## Acceptance Criteria

- [x] A `C<n>` row in
      [command-scenarios.md](../../docs/research/command-scenarios.md)
      names the landed-but-not-flipped case, with a tagged scenario in
      [features/commands.feature](../../features/commands.feature)
- [x] The scenario drives the real `frit drift` command, and drift
      reports the plan's work landed and names the commit
- [x] The bijection gate `go test ./internal/scenario` is green — the
      new `C<n>` maps to its row and back
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
