---
id: 2609152125
title: >-
  Landed lost-race reports landed even when no marker survives
status: "✅"
summary: >-
  heldError only computes Landed after a marker is read, so a lease
  branch whose history never carries a claim/beat/release marker at
  all — a squashed rewrite, not S95's masked-but-walkable shape —
  falls through lostRaceRefusal's Known guard to the bare "lost the
  race" wording instead of naming the landed state it already has the
  evidence for. Compute Landed independently of marker readability.
model: sonnet
depends-on: []
---
# Landed lost-race reports landed even when no marker survives

## Goal

`claim`/`start` refuse a landed lane whose marker cannot be read with
the bare, holder-less "lost the race to another machine" — the least
actionable refusal in the set — instead of "already landed; set plan
`<id>` to ✅". Make the landed classification independent of whether
the winning marker parses at all.

## Context

Filed as issue #195, an uncovered sibling of #148. That earlier fix
closed the same landed-but-unread-marker gap in `board`/`ready`.

Reproduced against a downstream repo. A plan's lease branch merged
onto `main` two days earlier. Its tip is a masked work commit — `plan
2609072342: recalibrate the star + nebula VRT checks` — with no
claim/beat/release marker anywhere in its reachable history. That is
a squashed rewrite, not a work commit merely riding on top of one.
`board`, `orphans` and `git merge-base --is-ancestor` all agree the
lane landed. `claim`/`start` alone blame a nonexistent competitor.

Root cause, read off
[lease.go](../internal/claim/lease.go) and
[claim.go](../cmd/frit/claim.go): `heldError` returns early when
`fetchedMarker` fails, before `e.Landed` is ever set —

```go
e := &HeldError{PlanID: opts.PlanID, Tip: tip}
m, ok := fetchedMarker(repoDir, opts, tip, run)
if !ok {
    return e            // Known=false; Landed never computed
}
...
e.Landed = landedTip(...)
```

— and `lostRaceRefusal` short-circuits on `!held.Known` before its own
`held.Landed` case is ever reached. `landedTip` reads pure ancestry
evidence and needs no marker, so both facts are independently knowable
today; only the wiring conflates them.

Checked the matrix first, per CLAUDE.md's Plan Maintenance rule. Plan
2609091905 gave the landed lost-race its own row, `@S95` in
[landed-evidence.feature](../features/landed-evidence.feature). Its
own Context section already flags the gap this plan closes. S95's
fixture (`pushesWorkTitledWithTheMarkersOwnPrefix`) pushes masking
commits on top of a real claim, minted by `holdsTheLease`'s
`Acquire` call. So the marker stays walkable and `Known` stays true.
It never exercises `heldError`'s `!ok` early return, the exact branch
this issue hits. That is the gap this plan closes, with a sibling
scenario.

Reuse, unit level.
`TestHeldErrorNeverReadsAPlanAuthoringCommitAsAMarker`
([lease_test.go](../internal/claim/lease_test.go)) already builds the
precise repro shape: a branch carrying a single plan-authoring
commit, no marker ever minted, merged ancestor-preserving onto
`main`. It currently asserts `held.Landed` is `false`. That assertion
is the bug's own contract. Flipping it to `true` is this plan's RED
at the unit level, with no new fixture needed.

Reuse, BDD level. Four steps in
[bdd_landed_evidence_test.go](../cmd/frit/bdd_landed_evidence_test.go)
are S95's own When/Then halves, unchanged:
`clonesTheRepositoryIntoAFleetRoot`,
`machineClaimsPlanOverTheLandedHold`,
`theClaimReportsThePlanAlreadyLanded`, and
`originsWorkRefForThePlanIsGone`.

Only the Given differs. S95's `"holds the lease for plan"` step calls
`claim.Acquire`, which mints a real marker. Any `Acquire` call leaves
a walkable marker in history by construction, so that step is not
reused here. The new Given instead builds the fixture
`TestHeldErrorNeverReadsAPlanAuthoringCommitAsAMarker` already proved
at unit level: `claimableRepo`
([claim_test.go](../cmd/frit/claim_test.go)) plus one
plan-authoring commit, as a single BDD step. No existing step
composes a plan branch without an `Acquire` underneath it.

Deviation found during Phase 1. This plan's own id-selection check
predates `@S96`, `@S97` and `@S98` landing on `main`: issue #189's fix
(plan 2609092113) and the two races/traps rows beside it. By the time
this phase ran, all three were already real, tagged scenarios. `@S96`
is `features/lifecycle.feature`'s "a lane's own fast-forward survives
its renewal", unrelated to this issue. The next free id at RED time
was `S99`. Every task and acceptance criterion below cites `S99`, not
the `S96` the plan text originally named.

## Tasks

1. In [lease.go](../internal/claim/lease.go), compute
   `e.Landed = landedTip(...)` in `heldError` before the `!ok` early
   return, so `Landed` no longer depends on the marker being read.
2. In [claim.go](../cmd/frit/claim.go), reorder `lostRaceRefusal`'s
   guard so its `held.Landed` case is checked before the `!held.Known`
   fallback, not after.
3. Add `@S99` (see the Phase 1 deviation note above) to
   [landed-evidence.feature](../features/landed-evidence.feature): a
   lost race against a landed lane whose history never carried a
   lease marker at all still reports "already landed".
4. Bind its new Given in
   [bdd_landed_evidence_test.go](../cmd/frit/bdd_landed_evidence_test.go),
   reusing S95's clone, claim and assertion steps for the When/Then.
5. Add the matrix row in
   [lease-protocol.md](../docs/research/lease-protocol.md) beside S95,
   and cite S99 alongside S54/S95 in
   [claiming.md](../docs/claiming.md)'s failure-reasons discussion.

## Phase 1: Landed is read off ancestry alone, not the marker

**RED.** Three additions, each confirmed failing first:

- Unit: in
  [lease_test.go](../internal/claim/lease_test.go),
  flip `TestHeldErrorNeverReadsAPlanAuthoringCommitAsAMarker`'s
  `assert.False(t, held.Landed, ...)` to `assert.True`, with a comment
  naming the corrected contract (`Known` stays false; `Landed` no
  longer depends on it). Confirm it fails against today's code.
- Unit: in [claim_test.go](../cmd/frit/claim_test.go),
  add a case to `TestLostRaceRefusalNamesTheHolder` for
  `&claim.HeldError{PlanID: 7, Landed: true}` (Known false, Landed
  true), expecting the "already landed; ... set plan 7 to ✅" wording.
  Confirm it fails against today's code.
- BDD: in
  [bdd_landed_evidence_test.go](../cmd/frit/bdd_landed_evidence_test.go),
  add a Given step building the no-marker landed fixture (`claimableRepo`
  plus one plan-authoring commit on `plan/<id>`, merged
  ancestor-preserving onto `main`, mirroring
  `TestHeldErrorNeverReadsAPlanAuthoringCommitAsAMarker`), reusing
  `clonesTheRepositoryIntoAFleetRoot`,
  `machineClaimsPlanOverTheLandedHold` and
  `theClaimReportsThePlanAlreadyLanded` for the rest of the scenario.
  Tag `@S99 @pending` first, confirm `TestFeatures` skips it, then
  remove `@pending` and confirm it fails with "lost the race to
  another machine" rather than "already landed".

**GREEN.** Apply the `heldError` and `lostRaceRefusal` changes from
Tasks 1–2. Confirm all three RED assertions above now pass.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/^S99:'` and
`go test ./internal/claim/... ./cmd/frit/...` pass; S54, S79, S82,
S84, S85, S94 and S95 (the existing landed-evidence rows) stay green;
the matrix row and `docs/claiming.md` citation are in place; `go test
./...`, `go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith
check .` all pass.

BDD coverage decision: this is a lease-protocol behavior. It gets
`@S99` per the executable scenario matrix procedure in
[development.md](../docs/development.md), not folded into the unit
tests alone — the same call plan 2609091905 made for S95.

Post-close fix, found by code review against this same phase's diff.
Reordering `Landed` ahead of the marker lookup meant `heldError` could
judge ancestry against a tip whose objects were never fetched.
`Acquire`'s existing-ref path fetches first. `Takeover`'s and a fresh
claim's own lost-CAS paths read the winning tip off a bare
`ls-remote` and never did. `heldError` now fetches the lease ref
itself before judging `Landed`, proven by
`TestHeldErrorFetchesTheWinningTipBeforeJudgingLanded`.

Checked against the matrix. This is an internal invariant `heldError`
must hold across every call path, not a new CLI-observable shape. The
wording it protects ("already landed") is the same one `S95`/`S99`
already prove. Only the internal git call reaching it differs. Unit
coverage alone is enough — the same call
`TestHeldErrorWalksPastWorkCommitsToTheGoverningMarker` made for its
own found-while-fixing consequence in plan 2609082010.

## Execution

| Phase | Work                                                       | Tier   |
| ----- | ---------------------------------------------------------- | ------ |
| 1     | Proving slice: the fix, S99, its bindings, matrix and docs | sonnet |

## Acceptance Criteria

- [x] `heldError` sets `Landed` off ancestry evidence alone; a
      landed lane with no readable marker reports `Known: false,
      Landed: true`.
- [x] `lostRaceRefusal` reports "already landed; ... set plan `<id>`
      to ✅" whenever `Landed` is true, whether or not `Known` is.
- [x] `@S99` in `landed-evidence.feature` exercises a lost race
      against a landed lane whose history carries no lease marker at
      all, and fails without the fix.
- [x] S54, S79, S82, S84, S85, S94 and S95 remain green.
- [x] The matrix row exists in `lease-protocol.md`; `claiming.md`
      cites S99 beside S54/S95.
- [x] `go test ./...`, `go tool -modfile=tools/go.mod golangci-lint
      run` and `mdsmith check .` pass.
