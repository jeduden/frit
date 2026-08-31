---
id: 2608310454
title: Doctor and next read phase state from phase front matter
status: "✅"
summary: >-
  A folder plan can now carry per-phase state as phase-file front
  matter, but frit still reads it only from the plan.md phases: ledger
  and the result-file Handoff. So a ledger-free folder plan loses
  doctor's Execution-row validation and its open phase is not found
  from the phase files. Teach the phase model to assemble a folder
  plan's phases from phase-*.md front matter when present, so frit
  doctor validates it and frit next and phase find its open phase the
  same as a ledgered plan. Once both read the front matter, a
  ledger-free folder plan is a first-class plan and the old ledgers can
  retire.
model: sonnet
depends-on: [2608310418]
---
# Doctor and next read phase state from phase front matter

## Goal

Teach `frit doctor`, `frit next` and `frit phase` to read per-phase
state from a folder plan's `phase-*.md` front matter, so a ledger-free
folder plan is validated and its open phase found exactly as a ledgered
one. This is issue 111, the companion to issue 110.

## Context

Plan [2608310418](../2608310418_phase-front-matter-generated-status/plan.md)
gave folder-plan phase files `{n, title, status}` front matter and made
`plan-new` stop writing the `phases:` ledger for new plans. But frit's
own reading did not move with it.

**Where the state is read today.**
[internal/planmeta/plan.go](../../internal/planmeta/plan.go) assembles
`Plan.Phases` from the front-matter `phases:` ledger, or falls back to
`## Phase N` headings. A folder plan has neither: its phases are
separate `phase-*.md` files, invisible to `Parse`, which sees only the
`plan.md` bytes. So `Plan.Phases` is empty, `FirstOpenPhase` finds
nothing, and [internal/doctor/doctor.go](../../internal/doctor/doctor.go),
which walks `p.Phases` to check each has an `## Execution` row, silently
validates nothing. Separately,
[internal/planmeta/resume.go](../../internal/planmeta/resume.go)'s
`Resume` decides a phase is done by the result file's `## Handoff`
marker, not the phase's own `status`.

**Reuse first.** `resume.go` already globs a plan directory's
`phase-*.md` files in `phaseSpecNumbers` — the same walk can parse each
file's front matter into a `Phase`. `doctor.Scan` already holds each
plan's path, so it knows the folder directory to read the phase files
from; `Resume` is already handed `dir`. The change is a dir-aware phase
assembly feeding the readers, not a new discovery path.

**Precedence and no regression.** A plan that still carries a `phases:`
ledger (every existing one) must read exactly as before — the ledger
wins where it is present, and phase-file front matter fills in only a
ledger-free folder plan. A phase closes by flipping its own
`phase-N.md` `status`; `## Handoff` in the result file stays an optional
living record, no longer the sole done-signal.

## Tasks

1. Phase 1 (proving slice): assemble a ledger-free folder plan's phases
   from `phase-*.md` front matter so `frit doctor` validates its
   Execution rows — a phase with no Execution row is flagged, as it is
   for a ledgered plan.
2. Phase 2: `frit next`/`frit phase` find and report the open phase of
   a ledger-free folder plan from phase-file `status`, with the ledger
   still winning where present.
3. Later phases as handoffs reveal them.

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

| #   | Status | Phase                                                                                        |
| --- | ------ | -------------------------------------------------------------------------------------------- |
| 1   | ✅     | [Doctor validates a ledger-free folder plan from phase front matter](phase-1.md)             |
|     | ↳      | doctor validates a ledger-free folder plan's Execution rows from phase-*.md front matter.    |
| 2   | ✅     | [next and phase find a ledger-free folder plan's open phase from status](phase-2.md)         |
|     | ↳      | frit next and frit phase find a ledger-free folder plan's open phase from phase-*.md status. |
<?/catalog?>

## Execution

| Phase | Title                                                                  | Tier   | Gate                                                                                                                                                                      |
| ----- | ---------------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Doctor validates a ledger-free folder plan from phase front matter     | sonnet | A ledger-free folder-plan fixture whose phase lacks an Execution row is unflagged by doctor at HEAD and flagged after; `go test ./...` green                              |
| 2     | next and phase find a ledger-free folder plan's open phase from status | sonnet | A ledger-free folder plan whose phase-1 status is ✅ resumes at phase 2 and its open phase is found from phase-file status, not the Handoff marker; `go test ./...` green |

## Acceptance Criteria

- [x] A ledger-free folder plan's phases are assembled from `phase-*.md`
      front matter, feeding `FirstOpenPhase` and doctor's Execution
      check
- [x] `frit doctor` flags a missing Execution row for a ledger-free
      folder plan, the same as for a ledgered plan
- [x] `frit next`/`frit phase` find and report the open phase of a
      ledger-free folder plan from phase-file `status`
- [x] A plan carrying a `phases:` ledger reads exactly as before; the
      ledger wins where present
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
