---
id: 2609031211
title: A live top lane no longer stalls pick's candidate walk
status: "🔲"
summary: >-
  pick --go ranks the startable plans and walks them, meaning to skip a
  candidate it cannot claim right now and start the next — "a live hold
  on the top pick does not stall the fleet". But the live-lane
  pre-flight, the #126 refusal frit raises when herdr shows a live pane
  on the top candidate's own hold branch, is classified as a hard
  refusal, not a lost race, so the walk halts on it and reports the
  busy lane even when other plans are freely startable. This plan makes
  that pre-flight a skip in the pick --go walk while it stays a
  surfaced refusal for an operator's explicit `start <id>`, and pins
  the new behavior with a cross-layer matrix row, S88, run for real
  under godog: a live top lane plus a free next candidate starts the
  next one.
model: sonnet
depends-on: [2609021314]
---
# A live top lane no longer stalls pick's candidate walk

## Goal

`pick --go` walks past a top candidate whose hold branch already
carries a live herdr pane and starts the next ready plan instead of
halting on the busy one, so a fleet with work to do never stalls
behind a lane somebody is already running. An operator's explicit
`start <id>` on that same lane still meets the named refusal.

## Context

**The bug, and where it lives.** `pick --go` runs
[`(*pickCmd).start`](../../cmd/frit/main.go). It walks
`discovery.Candidates` and, for each, calls
[`buildStart`](../../cmd/frit/start.go) with `reattach` false. The
walk advances to the next candidate only when `buildStart` returns its
`lost` bool true. That bool is reserved for one thing: the refusal
pick --go retries past, so "a live hold on the top pick does not stall
the fleet". A candidate that loses its race to a live bound session
returns `lost` true already, through `mintOrTakeOver`'s veto and
[`lostRace`](../../cmd/frit/start.go). The walk then advances, as
`TestPickGoAdvancesPastALiveHold` proves.

But `buildStart` has an earlier gate.
[`startLiveLaneRefusal`](../../cmd/frit/start.go) is a pre-flight on
the fresh-acquire branch. It fires when
[`liveLaneFor`](../../cmd/frit/dispatch.go) finds a live pane on one of
the plan's hold branches. This is the live-but-unbound lane #126 named,
which a session-less lease leaves the takeover veto unable to see. The
pre-flight returns a refusal doc, and `buildStart` returns it with
`lost` false. In the pick --go walk that `false` stops the walk.
`renderStart` prints the refusal on the busy top candidate, and the
ready plans below it are never reached. Observed live on 2026-09-03:
plan 2609021313 was running under pane `w4V:p1`, and `pick --go`
refused on it — "a live herdr pane (w4V:p1) already sits on lane
plan/2609021313" — though 2609021315 and 2609021316 were startable,
held by nobody.

**Why the pin didn't catch it.**
`TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane` pins the
current behavior — "The refusal is surfaced, not skipped" — but its
fixture has exactly one candidate. With one candidate, "surface the
refusal" and "advance to nothing" both end the walk, so the test never
told the two apart. The stall only shows when a live top lane sits
above a free next candidate, and no test built that fleet. That
untested combination is the gap this plan closes.

**The seam.** `reattach` already separates the two callers of
`buildStart`: `pick --go`'s walk passes it false and reads `lost`;
[`startResolved`](../../cmd/frit/start.go), the explicit `start <id>`
path, passes it true and discards `lost`. So classifying the live-lane
pre-flight as a lost race exactly when `reattach` is false makes the
walk advance without touching the explicit path, where the #126
refusal must stay surfaced so an operator who named that lane learns
it is busy. No new field is needed; the discriminator is in hand.

**What is reused.** The fix rides the existing `lost`-bool contract
and the existing walk. Nothing in `mintOrTakeOver`, `lostRace` or the
render path changes. The BDD row reuses the herdr-fake step vocabulary
and the `bdd_identity_and_cross_layer_test.go` registrar. Plan
2609021314 stands both up for the "Cross-layer: herdr and frit
disagree" section. This plan depends on it and adds one scenario to
that file rather than a new world. `liveLeaseFixture`,
`heldLaneOwnedBy`, `herdrReturning`, `withHerdr` and `seedWindow` in
the test package build the fixtures the row needs.

**Why cross-layer, and S88.** A live pane over a lease frit's claim
path defers to is precisely a herdr-and-frit disagreement, the
section's own subject; F1 and F11 already record pick's liveness in
the matrix. The row is a concrete Given/When/Then, not an always-
property, so it is an S row, not an F row, and takes the next free id,
S88. The godog suite admits a scenario only for a documented S row —
the bijection gate in [internal/scenario](../../internal/scenario) —
so the matrix row and the scenario land together.

**Out of scope.** No change to the lease protocol, to `liveLaneFor`,
or to how the #126 refusal reads for explicit `start <id>`. The
`open` verb's own use of `liveLaneFor` is untouched.

## Tasks

1. Phase 1 (proving slice): classify the live-lane pre-flight as a
   lost race in the pick --go walk only (`reattach` false), so the
   walk advances to the next ready candidate; the explicit `start
   <id>` refusal is unchanged. Reframe the pin that asserted the old
   stall and add the two-candidate advance case, all at the verb
   level.
2. Phase 2: document the guarantee as matrix row S88 in the
   "Cross-layer" section and write it as a real, non-`@pending` godog
   scenario in the cross-layer feature file, bound in the section's
   step file, so the bijection gate stays green and a regression fails
   the build.

## Execution

| Phase | Title                                                             | Tier   | Gate                                                                                                                                                   |
| ----- | ----------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | pick --go's walk advances past a live top lane, at the verb level | sonnet | New unit test: pick --go with a live top lane and a free next candidate starts the next; explicit `start <id>` still refuses; suite and lint clean     |
| 2     | S88 runs for real under godog                                     | sonnet | `TestFeatures/S88:` passes with no SKIP; `go test ./internal/scenario` bijection stays green; `go test ./...` and lint clean; `mdsmith check .` passes |

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

| #   | Status | Phase                                                                           |
| --- | ------ | ------------------------------------------------------------------------------- |
| 1   | 🔲     | [pick --go's walk advances past a live top lane, at the verb level](phase-1.md) |
| 2   | 🔲     | [S88 runs for real under godog](phase-2.md)                                     |
<?/catalog?>

## Acceptance Criteria

- [ ] `pick --go`, run against a fleet whose top-ranked candidate has
      a live herdr pane on its hold branch and at least one lower ready
      plan held by nobody, starts the lower plan and does not refuse on
      the busy one
- [ ] `pick --go` against a fleet whose only candidate is such a live
      lane reports `nothing startable`, not a refusal
- [ ] An explicit `start <id>` on a lane with a live herdr pane still
      renders the #126 refusal naming the pane and its branch
- [ ] Matrix row S88 is documented in the "Cross-layer" section of
      [docs/research/lease-protocol.md](../../docs/research/lease-protocol.md),
      and a `@S88` scenario in
      [features/cross-layer.feature](../../features/cross-layer.feature)
      runs for real, not `@pending`
- [ ] `go test ./cmd/frit -run 'TestFeatures/S88:'` reports S88 PASS,
      not SKIP; `go test ./internal/scenario` stays green
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
