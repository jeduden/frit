---
id: 2609031951
title: A deserted top lane no longer stalls pick's candidate walk
status: "✅"
summary: >-
  Plan 2609031211 makes pick --go's walk advance past a live-lane
  refusal on the top candidate. But that is one of two refusal gates in
  buildStart. The earlier gate, startRefusal, returns a deserted-hold,
  unparked-suffix or unmatured-takeover refusal with the same
  halt-the-walk classification, so a deserted top candidate stalls the
  walk exactly as the live lane did — observed on 2026-09-03, when a
  deserted hold on the top-ranked plan refused every pick while five
  other plans were startable. This plan classifies the startRefusal
  gate as a skip in the pick --go walk while it stays a surfaced
  refusal for an operator's explicit start, completing the walk-advance
  across both gates, and pins it with a cross-layer matrix row, S90,
  run under godog.
model: sonnet
depends-on: [2609031211, 2609031939]
---
# A deserted top lane no longer stalls pick's candidate walk

## Goal

`pick --go` walks past a top candidate whose start is refused before
the repository is read — a deserted hold, an unparked suffix, or a
takeover whose window has not matured — and starts the next ready plan
instead of halting on it. A candidate that needs recovery or more time
never stalls a fleet with other work to do. An operator's explicit
`start <id>` on that plan still meets the refusal, its `frit yield` or
wait-for-window remedy intact.

## Context

**The second refusal gate.** [`buildStart`](../../cmd/frit/start.go)
has two places it refuses a candidate before running the acquire.
Plan 2609031211 fixes the later one,
[`startLiveLaneRefusal`](../../cmd/frit/start.go), so a live-lane
refusal is a skip in the pick --go walk. The earlier one is
[`startRefusal`](../../cmd/frit/start.go): when it returns a refusal
doc, `buildStart` returns it with `lost` false, and the walk in
[`(*pickCmd).start`](../../cmd/frit/main.go) halts on it. That gate
covers [`desertedRefusal`](../../cmd/frit/start.go) and
[`parkFirstRefusal`](../../cmd/frit/start.go) — a deserted hold with an
unparked suffix, S77 — and `claimRefusal`, a takeover whose window has
not matured. All of them stall the walk today.

**Observed.** On 2026-09-03 a deserted hold on the top-ranked plan
refused every `pick --go` — "deserted hold: its branch carries an
unparked suffix; run `frit yield <id>` to park it first" — while
`frit ready` listed five other plans startable, held by nobody. The
walk never reached them. This is the same fleet-stall 2609031211
names, through the other gate.

**Why the walk should skip, not halt.** The walk's contract is "a live
hold on the top pick does not stall the fleet" — it starts the first
candidate it can and passes over the rest. A refusal from `startRefusal`
means exactly "this ranked candidate cannot be cleanly started right
now": a deserted lane wants a `frit yield` first, an unmatured takeover
wants more time. Neither is a fault, and neither is helped by halting
the whole walk. The candidate's recovery is `frit tidy`'s concern or a
later window, not a reason to leave the ready plans below it unstarted.

**The seam is the same one 2609031211 uses.** `reattach` separates the
two callers. The pick --go walk passes it false and reads `lost`;
[`startResolved`](../../cmd/frit/start.go), the explicit `start <id>`
path, passes it true and discards `lost`. So classifying the
`startRefusal` gate as a lost race when `reattach` is false makes the
walk advance without touching the explicit path. There the refusal and
its remedy — `frit yield`, or waiting the window out — stay surfaced
for the operator who named the plan.

**Not the fault path.** A diverging local branch is a real fault:
`pick --go` exits non-zero and stands nothing up, pinned by
`TestPickGoRefusesADivergingLocalBranch`. That travels the
`return nil, false, err` path, not the refusal path this plan touches,
so it is unaffected and must stay a halt.

**What is reused.** The `lost`-bool contract, the walk, and the
`reattach` discriminator are 2609031211's. This plan applies the same
classification to the one remaining gate. The godog row reuses the
herdr-fake and lease vocabulary the cross-layer step file already
carries. It takes the next free id after 2609031211's S88 and
2609031939's S89 — S90 — in the same "Cross-layer" section, beside S77.

**Why depends-on both.** 2609031211 establishes the walk-advance
mechanism and edits the same `buildStart` gate region. 2609031939 edits
the same deserted-refusal wording, and the same cross-layer matrix and
feature files. Landing both first keeps this plan's one-line change
clean, and its S-id from colliding.

**Out of scope.** No change to the lease protocol, to how a takeover
or a deserted hold is classified, or to the diverging-branch fault. The
refusal wording is 2609031939's; the live-lane gate is 2609031211's.

## Tasks

1. Phase 1 (proving slice): classify the `startRefusal` gate as a lost
   race in the pick --go walk only (`reattach` false), so the walk
   advances past a deserted, unparked-suffix or unmatured-takeover top
   candidate to the next ready plan; the explicit `start <id>` refusal
   is unchanged. Reproduce the deserted-hold stall first, then reframe
   any pin that asserted it.
2. Phase 2: document the guarantee as cross-layer matrix row S90 and
   run it for real under godog — a deserted top lane plus a free next
   candidate starts the next — so the stall cannot return.

## Execution

| Phase | Title                                              | Tier   | Gate                                                                                                                                                        |
| ----- | -------------------------------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | pick --go's walk advances past a deserted top lane | sonnet | New unit test: pick --go with a deserted top candidate and a free next candidate starts the next; explicit `start <id>` still refuses; suite and lint clean |
| 2     | S90 runs the deserted-lane walk skip under godog   | sonnet | `TestFeatures/S90:` passes with no SKIP; `go test ./internal/scenario` bijection stays green; `go test ./...` and lint clean; `mdsmith check .` passes      |

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

| #   | Status | Phase                                                                                                                                      |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | ✅     | [pick --go's walk advances past a deserted top lane](phase-1.md)                                                                           |
|     | ↳      | buildStart's startRefusal gate is now a skip in pick --go's walk, matching the sibling live-lane gate.                                     |
| 2   | ✅     | [S90 runs the deserted-lane walk skip under godog](phase-2.md)                                                                             |
|     | ↳      | S90 documents and runs the deserted-lane walk skip under godog; the matrix and feature file are in bijection and TestFeatures/S90: passes. |
<?/catalog?>

## Acceptance Criteria

- [x] `pick --go`, run against a fleet whose top-ranked candidate is a
      deserted hold with an unparked suffix and at least one lower ready
      plan held by nobody, starts the lower plan and does not refuse on
      the deserted one
- [x] `pick --go` against a fleet whose only candidate is such a
      deserted hold reports `nothing startable`, not a refusal
- [x] An explicit `start <id>` on the deserted hold still renders the
      "deserted hold … run `frit yield <id>`" refusal, unchanged
- [x] A diverging local branch still makes `pick --go` exit non-zero
      and stand nothing up — the fault path is untouched
- [x] Cross-layer matrix row S90 is documented in
      [docs/research/lease-protocol.md](../../docs/research/lease-protocol.md),
      and a `@S90` scenario in
      [features/cross-layer.feature](../../features/cross-layer.feature)
      runs for real, not `@pending`
- [x] `go test ./cmd/frit -run 'TestFeatures/S90:'` reports S90 PASS,
      not SKIP; `go test ./internal/scenario` stays green
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
