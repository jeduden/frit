---
id: 2608290818
title: doctor and pick warn a plan has no room for another phase
status: "🔳"
summary: >-
  A plan can sit at its markdown line cap with no room for a `## Phase
  N` section, and nothing in frit says so until an executor has picked
  it, started the lane, and tried to write the phase. Add a `headroom`
  finding to `doctor`, and carry the same signal on `pick`, computed
  with an oracle that pads an in-memory copy and asks mdsmith whether it
  now trips `max-file-length` — so the cap never has to leave mdsmith.
  The reserve is one repo-level `.frit.yml` value; 0 disables it.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: doctor reports a plan with no headroom
    status: "✅"
  - n: 2
    title: pick carries the headroom signal
    status: "✅"
  - n: 3
    title: init and help name the reserve
    status: "🔲"
---
# doctor and pick warn a plan has no room for another phase

## Goal

`doctor` and `pick` tell an orchestrator that a plan has no room to
append a `## Phase N` section. They say so before a lane is stood up. So
the plan is split first, cheaply, not at push time or mid-authoring.

## Context

Issue [#99](https://github.com/jeduden/frit/issues/99). A plan capped at
its `max-file-length` cannot record another phase section. Today that
surfaces only when an executor picks the plan, starts the phase, and
finds no section fits. `pick --go` ranks such a plan as startable and
dispatches its next phase into a lane that cannot be written to.

The oracle, not the cap. mdsmith does not expose the configured
`max-file-length` value through its public
[pkg/mdsmith](../go.mod) API — it lives behind an internal `config`
package. But [doctor](../internal/doctor/doctor.go) already opens a
`mdsmith.Session` against the repo's own `.mdsmith.yml` and calls
`sess.Check(rel, source)` on each plan. So frit never needs the cap: it
appends `## Phase N` -sized padding to an in-memory copy of the source
and asks the session whether `max-file-length` now fires. This is the
approach smalt's `tools/headroom` proved.

Reserve, one repo value. How much room a plan must keep free is a
per-project convention, so it belongs in
[.frit.yml](../.frit.yml) beside `plan-dir` and `holds`, parsed by
[repocfg](../internal/repocfg/config.go). It is a percent of the plan's
length, matching smalt's gate; `0` disables the finding. A repo with no
`.frit.yml` gets the default.

Shared oracle. Both `doctor` (Phase 1) and the fleet index `pick` reads
(Phase 2) need the same check. Phase 1 lands it as a small
`internal/headroom` package so Phase 2 reuses it rather than
re-implementing the pad-and-check. `internal/discovery` stays pure — it
is tested against an in-memory fleet with no repo on disk — so the check
runs where mdsmith is already loaded, and its result is carried onto the
plan record the fleet index builds.

## Tasks

1. Add `headroom-reserve` to `.frit.yml` and a `headroom` finding to
   `doctor`, computed by a new `internal/headroom` oracle.
2. (determined after Phase 1)
3. (determined after Phase 1)

## Phase 1: doctor reports a plan with no headroom

A proving slice: the reserve config, the oracle, and the `doctor`
finding, end to end.

`repocfg` gains a `headroom-reserve` int (percent). It defaults to `10`,
reads from `.frit.yml` when set, and `0` disables the finding. Follow
the `plan-dir` copy-if-set path in
[Load](../internal/repocfg/config.go); add the field to both `Config`
and `fileConfig` and a default in `Default`.

A new `internal/headroom` package holds the oracle. Given a
`*mdsmith.Session`, the plan's rel path, its source, and a reserve line
count, it returns how many of those reserve lines fit before
`max-file-length` fires — by appending `<!-- headroom padding -->` lines
to an in-memory copy and calling `sess.Check`, narrowing over
`[0, reserve]`. The reserve line count is `ceil(reserve% × body
lines)`. A plan short of its full reserve has no headroom; the shortfall
is `reserve − room`.

[doctor.Scan](../internal/doctor/doctor.go) computes the reserve from
the passed-in percent, runs the oracle per plan on the session it
already holds, and emits a `Finding` with `Check: "headroom"` and a
message naming how many lines short the plan is when `room < reserve`.
When the percent is `0`, no headroom finding is ever emitted. Thread the
percent from `repocfg` through the `doctorCmd.Run` call in
[main.go](../cmd/frit/main.go).

RED, against the fixture idiom in
[doctor_test.go](../cmd/frit/doctor_test.go) (which copies frit's own
`.mdsmith.yml` into a temp repo), plus a unit test on the oracle:

- The oracle: a source with plenty of room returns `room == reserve`; a
  source padded to the cap returns `room < reserve`.
- `Scan` with reserve > 0: a plan at the cap yields a `headroom` finding
  naming the shortfall; a short plan yields none.
- `Scan` with reserve 0: no `headroom` finding on either.

GREEN: add `internal/headroom`, the `repocfg` field, and the
`checkPlan`/`Scan` wiring in
[doctor.go](../internal/doctor/doctor.go).

Gate: build frit and run `go run ./cmd/frit doctor` in a repo holding a
plan at the cap; confirm a `headroom` row appears with the shortfall,
and none when `headroom-reserve: 0`. `go test ./...` and `mdsmith check
.` are clean.

## Phase 2: pick carries the headroom signal

The fleet index computes each plan's headroom with the Phase 1 oracle.
It carries the result onto its plan record. So `pick` surfaces it
without `internal/discovery` losing its purity. A startable plan with no
headroom keeps its rank — it is startable, it just cannot be written to.
It carries the signal in its row and in `--json`.

Add a headroom field to the fleet's plan record in
[internal/fleet](../internal/fleet) — a bool plus the shortfall. Set it
from the oracle where the index already reads plan files. The index can
open a `mdsmith.Session` per repo there. Carry the field onto
`discovery.Plan`. Project it in `cardOf` onto a new
[PlanCard](../internal/report/discovery.go) field. Render it — a column,
or a per-row note — in a pick renderer. `printReady` is shared with
`ready`, so a column added there reads blank for a plan with room. If
that couples the two verbs awkwardly, give `pick` its own renderer.

RED, against [pick_test.go](../cmd/frit/pick_test.go) and
[json_test.go](../cmd/frit/json_test.go):

- (RED spec fixed by Phase 1's oracle shape and the fleet plumbing it
  exposes; write it against `pick --json` carrying the headroom field
  for a capped plan and the field absent-but-present for a plan with
  room.)

GREEN: (sites determined after Phase 1 — the fleet record, `cardOf`, and
the pick renderer.)

Gate: build frit and run `go run ./cmd/frit pick --json` in a repo with
a capped startable plan; confirm the plan is still ranked and its card
carries the headroom signal, and a plan with room carries it false.
`go test ./...` and `mdsmith check .` are clean.

## Phase 3: init and help name the reserve

`frit init` writes `.frit.yml` "with every default present and
commented". The new `headroom-reserve` must appear there. The `doctor`
Help must catalogue the `headroom` check too.

Add the commented `headroom-reserve` block to the `.frit.yml` template
the [init writer](../internal/scaffold/scaffold.go) emits, explaining it
is a percent of a plan's length kept free for a phase section and that
`0` disables the finding. Add the `headroom` line to `doctorCmd.Help` in
[main.go](../cmd/frit/main.go), beside `goal`, `execution-row`, `tier`
and `schema`.

RED:

- The init `.frit.yml` template test asserts the commented
  `headroom-reserve` key is present with its default.
- A `doctor --help` test asserts the `headroom` check is documented.

GREEN: extend the template and the Help string.

Gate: run `go run ./cmd/frit init` in a temp dir and confirm the written
`.frit.yml` carries the commented `headroom-reserve`; run `go run
./cmd/frit doctor --help` and confirm `headroom` is listed. `go test
./...` and `mdsmith check .` are clean.

## Execution

Tier is per phase. Phase 1 designs the oracle and the config surface —
opus for the design, sonnet to implement from the written assertions.
Phases 2 and 3 implement a settled shape. Each phase that claims a verb
behaves a certain way gates by running the built frit and reading its
output, per `plan/proto.md`.

| Phase           | Design | Implement | Gate that catches a wrong answer                                          |
| --------------- | ------ | --------- | ------------------------------------------------------------------------- |
| 1 doctor+oracle | opus   | sonnet    | `frit doctor` shows a headroom row at the cap, none at reserve 0          |
| 2 pick signal   | opus   | sonnet    | `frit pick --json` carries the headroom field, capped plan still ranked   |
| 3 init+help     | opus   | sonnet    | `frit init` writes the commented key; `frit doctor --help` lists headroom |

## Acceptance Criteria

- [ ] `.frit.yml` `headroom-reserve` is read by `repocfg`, defaults to
      10, and `0` disables the finding
- [ ] An `internal/headroom` oracle reports how much of a reserve fits
      by padding an in-memory copy and asking mdsmith, never reading the
      cap directly
- [ ] `doctor` emits a `headroom` finding, naming the shortfall, for a
      plan with no room for another phase section
- [ ] `pick` carries the same signal in its row and `--json`, and a
      capped plan keeps its rank
- [ ] `frit init` writes the commented `headroom-reserve` default and
      `doctor --help` catalogues the `headroom` check
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
