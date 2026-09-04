---
n: 2
title: S90 runs the deserted-lane walk skip under godog
status: "✅"
result: true
summary: >-
  S90 documents and runs the deserted-lane walk skip under godog; the
  matrix and feature file are in bijection and TestFeatures/S90: passes.
---

## Handoff

**The S90 row, as landed**, in the "Cross-layer: herdr and frit
disagree" section of
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md),
after S89:

> S90 | a deserted top lane in pick's walk | `pick --go` treats
> `startRefusal`'s own refusals as a candidate to skip too: it advances
> to the next ready plan; an explicit `start <id>` still meets the
> refusal (plan 2609031951)

**The `@S90` scenario**, in
[features/cross-layer.feature](../../features/cross-layer.feature),
beside `@S89`:

```gherkin
@S90
Scenario: a deserted top lane in pick's walk
  Given this machine holds plan 7 in a lane bound to a session, whose branch carries an unparked suffix
  And herdr shows no agent on the lane
  And plan 8 is ready and held by nobody
  When pick --go runs
  Then plan 8 is the one started
  And plan 7 is not refused on
```

**Only one new Given step was needed.** Phase 1's handoff identified
`pick --go runs`, `plan (\d+) is the one started`, `plan (\d+) is not
refused on` and `plan (\d+) is ready and held by nobody` as already
bound and directly reusable — all four are S90's When/Then and its
second Given, unchanged.

**Why S77's own Given did not reuse for the first Given, and what the
new step does instead.** S77's `a takeover bound to a session at a new
epoch lands on plan 7` mints its new marker as a child of the tip this
lane's own local ref already sits on, so that local ref stays an
ancestor of the new tip — `unparkedSuffix` reads that as parked, not
unparked. That shape is exactly right for S77's own scenario, which
runs `start --go` from inside the lane, where `desertedRefusal`'s cwd
check fires on the marker mismatch alone. `pick --go`'s walk never
chdirs into a candidate's lane, so `desertedRefusal` never fires there
(S77's shape would silently fall through to the ordinary claim path,
racing the fixture's own already-landed takeover — a false read, not a
deserted-lane skip). `parkFirstRefusal` is the gate that must fire
instead, and it is `coord.Path`-based, not cwd-based, so it needs the
one thing S77's fixture does not carry: a real local commit the dead
lane made and never pushed. `cmd/frit/bdd_identity_and_cross_layer_test.go`
adds exactly that:

- New Given step text: "this machine holds plan (\d+) in a lane bound
  to a session, whose branch carries an unparked suffix", bound to
  `thisMachineHoldsPlanInALaneBoundToASessionWhoseBranchCarriesAnUnparkedSuffix`
  in `cmd/frit/bdd_identity_and_cross_layer_test.go`. It builds the
  same live lane `buildLiveLane` gives S77, then commits an empty,
  unpushed commit on the lane's own branch. Registered in
  `registerPickWalkIdentityAndCrossLayer`, beside S88's own Given.
- Its own unit test — named for the step function, with
  `BuildsTheFixture` appended — pins that the local commit never
  reached origin.

No other step text was touched; strict mode never reported an
ambiguous binding.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S90:'` reports S90
PASS; the full `TestFeatures` run carries no SKIP; `go test
./internal/scenario` (the matrix/feature bijection) is green; `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are clean;
`mdsmith check .` passes.

**One open note, not acted on.** The plan's own Context describes the
observable as "the start document ... read through `--json`, per the
JSON Contract." S90 instead reuses S88's existing `pickGoRuns` /
`planIsTheOneStarted` / `planIsNotRefusedOn` steps, which read `pick
--go`'s plain-text report, not `--json` — the precedent Phase 1's
handoff pointed at and the phase's own GREEN instruction ("reusing ...
and defining only what S90 adds") endorsed. Switching those shared
steps to `--json` would touch S88's own passing scenario too, which is
outside a phase whose RED/GREEN steps named only the matrix row, the
feature scenario and one new Given binding. Left as a documented gap
rather than unrequested scope.

This closes plan 2609031951: both Acceptance Criteria phases are met,
the walk-advance across both `buildStart` refusal gates is complete,
and it is pinned at both the unit level and the cross-layer godog
level.
