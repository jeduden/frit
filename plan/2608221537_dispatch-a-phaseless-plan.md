---
id: 2608221537
title: The dispatch ladder starts a phase-less plan instead of demanding --phase
status: "✅"
summary: >-
  proto.md blesses a plan "small enough to land in one go" with no
  phases and no Execution table, and `next`/`show`/the plan-phase
  skill all handle one. But `pick --go`, `start` and `nudge` refuse a
  phase-less plan with "carries no phase ledger; pass --phase", so
  pick's "one verb, never ask" contract breaks on exactly the
  candidate it ranked top. Treat a phase-less plan as one whole-plan
  dispatch; still refuse a phased plan whose phases are all done.
model: sonnet
depends-on: []
phases:
  - {n: 1, title: "buildStart dispatches a phase-less plan", status: "✅"}
  - {n: 2, title: "nudge dispatches it; all-done still refuses", status: "✅"}
---
# The dispatch ladder starts a phase-less plan instead of demanding --phase

## Goal

`frit pick --go` (and `start`, `nudge`) dispatch a plan that carries
no phase ledger by composing `/plan-phase <id>`, instead of aborting
with "carries no phase ledger; pass --phase". A phased plan whose
every phase is ✅ still refuses, because that is genuinely nothing
open.

## Context

Reproduced this session. `frit pick --go` ranked plan 2608220941
top, verified its deps, then aborted before minting any claim with
`plan 2608220941 carries no phase ledger; pass --phase`. That plan is
not malformed: [plan/proto.md](proto.md) lines 49–50 say the phase
sections and `## Execution` table are "optional for a plan small
enough to land in one go", and 2608220941 is exactly that — one
`## Tasks` step, `phases: []`. `frit next 2608220941` prints
`(no phase ledger)` and exits 0; `frit show` renders its Goal; the
[plan-phase skill](../internal/skills/assets/plan-phase/SKILL.md)
drives a plan through `next`/`show`, so it already works a phase-less
plan. Only the dispatch verbs balk.

The refusal lives in one shared helper and one sibling. `buildStart`
([cmd/frit/start.go](../cmd/frit/start.go)) backs both `start` and
`pick --go`. `nudgeCmd.Run`
([cmd/frit/dispatch.go](../cmd/frit/dispatch.go)) backs `nudge`. Both
call `dispatch.Phase`
([internal/dispatch/dispatch.go](../internal/dispatch/dispatch.go)).
It returns `ok=false` whenever `FirstOpenPhase` finds nothing. That
folds two different situations into one refusal: a plan with **no
ledger**, which should dispatch the whole plan, and a phased plan with
**every phase done**, which is genuinely nothing to send. The callers
already branch on `len(plan.Phases) == 0` for the error text. So the
distinction is known at the call site. It is just wired to two error
messages instead of one dispatch and one refusal.

**Reuse first.** The composition already exists:
`dispatch.Command(planID, phase)` builds `/plan-phase <id> <phase>`.
For a phase-less plan the truthful seed is `/plan-phase <id>` with no
phase token — the plan-phase skill defaults the phase itself — so
`Command` folding an empty phase to a trailing-space-free string is
the whole primitive. `planmeta.FirstOpenPhase` stays the arbiter for
a phased ledger; nothing new parses a plan. The report docs
(`report.NewStart`, `report.NewNudge`,
[internal/report](../internal/report)) already carry a phase string
per dispatch; a phase-less dispatch passes an empty phase, rendered as
a whole-plan label rather than a blank cell.

A phase-less plan is not the same as a plan whose phases are all ✅.
The first has work to do and no slices to name it by; the second is
finished. This plan keeps the second a refusal — re-dispatching a
completed plan is the bug that refusal prevents — and only turns the
first into a dispatch.

## Tasks

1. Distinguish "no ledger → dispatch the whole plan" from "phased,
   all done → refuse" in `internal/dispatch`, and wire `buildStart`
   so `start` and `pick --go` dispatch a phase-less plan as
   `/plan-phase <id>`.
2. Wire `nudge` to the same rule, render a phase-less dispatch in the
   report docs, and pin that a phased plan with every phase ✅ still
   refuses.

## Phase 1: buildStart dispatches a phase-less plan

**RED.** In [internal/dispatch](../internal/dispatch/dispatch_test.go),
assert `Command(2608220941, "")` returns `/plan-phase 2608220941`
with no trailing space. Assert the resolver reports three distinct
cases from a phase list plus override. An override wins. An empty
ledger resolves to a whole-plan dispatch: empty phase, dispatchable.
A non-empty ledger with no open phase resolves to "none open": not
dispatchable. Then, at the verb level in
[cmd/frit](../cmd/frit/start_test.go), a phase-less plan run through
`buildStart` under `--go` composes `/plan-phase <id>` and returns no
error. Reuse `commitPhasedPlan`'s neighbour in
[discovery_test.go](../cmd/frit/discovery_test.go) for a phase-less
fixture, adding one if none exists.

**GREEN.** In `internal/dispatch`: make `Command` trim an empty
phase, and expose the three-way outcome (e.g. a small `Mode` or a
second bool) so a caller can tell no-ledger from none-open without
re-inspecting `len(phases)`. In
[cmd/frit/start.go](../cmd/frit/start.go), replace the
`len(plan.Phases) == 0` error with a whole-plan dispatch (empty
`phase`, `dispatch.Command` seeds `/plan-phase <id>`); leave the
"has no open phase" refusal for a phased-but-done ledger intact.

**Gate.** `go test ./internal/dispatch ./cmd/frit`.

## Phase 2: nudge dispatches it; all-done still refuses

**RED.** In [cmd/frit](../cmd/frit/dispatch_test.go), a `nudge`
dry-run on a phase-less plan prints `/plan-phase <id>` and does not
error. In [internal/report](../internal/report), a start/nudge doc
built for a phase-less dispatch renders a whole-plan label, not a
blank phase, in both the table and `--json`
([golden files](../internal/report/testdata)). Pin the regression: a
phased plan whose every phase is ✅ still refuses under both `start`
and `nudge` with "has no open phase".

**GREEN.** Apply the Phase 1 resolver to
[cmd/frit/dispatch.go](../cmd/frit/dispatch.go)'s `nudgeCmd.Run`.
Adjust the report rendering
([internal/report](../internal/report)) so an empty phase shows as a
whole-plan dispatch; re-record golden files with
`go test ./internal/report -update` and read the diff.

**Gate.** `go test ./...`; `mdsmith check .`;
`go tool -modfile=tools/go.mod golangci-lint run`.

## Execution

| Phase | Tier   | Gate                                     |
| ----- | ------ | ---------------------------------------- |
| 1     | sonnet | `go test ./internal/dispatch ./cmd/frit` |
| 2     | sonnet | `go test ./...`; `mdsmith check .`; lint |

## Acceptance Criteria

- [x] `frit pick --go` whose top candidate is a phase-less plan claims
      and starts it, composing `/plan-phase <id>`, with no `--phase`
      and no error
- [x] `frit start <phase-less-id> --go` and `frit nudge
      <phase-less-id>` likewise dispatch `/plan-phase <id>`
- [x] `dispatch.Command(id, "")` yields `/plan-phase <id>` with no
      trailing space
- [x] A phased plan whose every phase is ✅ still refuses with "has no
      open phase" under `start` and `nudge`
- [x] A phase-less dispatch renders a whole-plan label, not a blank
      phase, in the table and `--json`
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
