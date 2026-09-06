---
id: 2609061024
title: frit drift proves its phase-level drift and its silence
status: "🔳"
summary: >-
  frit drift also reads a plan's final phase: a commit naming the
  last-numbered phase is the drift signal that a multi-phase plan's
  work closed while its status still lags. Just as important from a
  developer's seat is where drift must stay quiet — a plan mid-flight
  whose work has not merged, and a plan already done, must raise no
  drift at all, or the verb is not trusted. This plan gives those the
  command scenarios they lack, as C<n> rows in command-scenarios.md.
  It runs after the sibling drift plan, since both edit the same
  catalog, feature file and step file, and the shared drift step
  vocabulary is established there first.
model: sonnet
depends-on: [2609061023]
---
# frit drift proves its phase-level drift and its silence

## Goal

`frit drift`'s phase-level signal and its restraint are both proven by
command scenarios. A commit naming a plan's final phase surfaces as
drift. A plan mid-flight, or one already done, raises none — so a
developer trusts the verb not to cry wolf.

## Context

**The gap.** The sibling plan proves drift's core landed signal. This
plan proves the rest of what the BDD-only run left dark: the
phase-level detectors `lastPhaseNumber`, `namesLastPhase` and
`bucketByID` in [cmd/frit/drift.go](../../cmd/frit/drift.go), and the
`Unfinished()` filter that skips a done plan. None is driven by a
scenario today.

**The home.** Like its sibling, this plan writes `C<n>` rows in
[command-scenarios.md](../../docs/research/command-scenarios.md), kept
in bijection with [features/](../../features) by the same gate. drift
is a single-host read, not a lease race, so the command catalog is its
home.

**Reuse first.** The sibling plan establishes the drift step
vocabulary in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
This plan reuses those steps and adds only what phase-drift and the
silent cases need. `depends-on` names the sibling so the shared step
file is written once, not raced.

**C-id allocation.** Merge `main`, read the catalog, allocate the next
free `C<n>`. Never a hardcoded id across lanes.

**Out of scope.** No change to drift's behavior. The core landed
signal is the sibling plan's.

## Tasks

1. Phase 1 (proving slice): a `C<n>` scenario for phase-level drift —
   a multi-phase plan whose last phase's commit is on `main` while its
   status still lags, which drift reports as naming the final phase.
2. Later phases: the silent cases — a plan mid-flight whose work has
   not merged raises no drift; a done plan is not listed; a commit
   naming two ids is attributed to both.

## Execution

| Phase | Title                                             | Tier   | Gate                                                                                                                             |
| ----- | ------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------- |
| 1     | A plan whose final phase landed surfaces as drift | sonnet | the new `C<n>` scenario runs against the built frit, drift names the final phase; bijection gate green; `go test ./...` green    |
| 2     | A plan mid-flight or already done raises no drift | sonnet | the new `C<n>` scenario runs against the built frit, drift is quiet for both shapes; bijection gate green; `go test ./...` green |

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

| #   | Status | Phase                                                                                     |
| --- | ------ | ----------------------------------------------------------------------------------------- |
| 1   | ✅     | [A plan whose final phase landed surfaces as drift](phase-1.md)                           |
|     | ↳      | C4 proves drift's phase-level signal end to end — a commit naming the plan's final phase. |
| 2   | 🔲     | [A plan mid-flight or already done raises no drift](phase-2.md)                           |
<?/catalog?>

## Acceptance Criteria

- [x] A `C<n>` scenario drives the real `frit drift` and shows it
      naming a plan's final phase from a commit on `main`
- [ ] A `C<n>` scenario shows drift raising nothing for a plan whose
      work has not merged, and not listing a done plan
- [x] The bijection gate `go test ./internal/scenario` is green
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
