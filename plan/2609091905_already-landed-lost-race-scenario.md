---
id: 2609091905
title: Scenario coverage for the masked already-landed lost-race refusal
status: "🔲"
summary: >-
  heldError's Known/Landed detection depends on the same marker walk
  plan 2609082010 fixed for resume, and the masked shape was
  reproduced against this repository's own history — yet the
  "already landed" lost-race refusal has never had an @S<n> row. Add
  one alongside its landed-evidence siblings.
model: sonnet
depends-on: []
---
# Scenario coverage for the masked already-landed lost-race refusal

## Goal

A lost race against a landed lease reports "already landed" and
scavenges the leftover ref. That needs `heldError` to read the
winning marker at all. Plan 2609082010 fixed that read for a work
commit sharing the marker's own `plan <id>: ` prefix. This plan gives
that contract point its own `@S<n>` scenario, so the matrix — not
only a unit test — pins it.

## Context

Prompted by the owner during plan 2609082010. CLAUDE.md's Plan
Maintenance rule now covers a lease-protocol behavior surfaced
mid-execution, not only a phase's named scope. This one: fixing
`latestMarker` ([lease.go](../internal/claim/lease.go)) for resume
also fixed `heldError`'s early return, before `e.Landed` is ever set.
Reproduced against this repository's own history: plan 2609061856's
leftover branch, where `git log -1 --grep='^plan 2609061856: '` lands
on an ordinary work commit, never the real claim/beat markers
beneath it.

Checked the matrix first, per the corrected rule. No `@S<n>` covers a
lost race reporting landed at all — only `claim_test.go` and
`lease_test.go` (unit level) do. The closest neighbors, S54/S79/S82/
S84/S85 in
[landed-evidence.feature](../features/landed-evidence.feature),
exercise `Scavenge`, `reap` or a read verb. None exercise `Acquire`'s
own lost-race classification, the site `HeldError.Landed` and
`lostRaceRefusal` ([claim.go](../cmd/frit/claim.go)) live on. That is
the gap this plan closes. `HeldError.Landed` reads the same
content-landed evidence `Scavenge`'s park decision does.

Reuse: `landedLeaseRepo` (`cmd/frit/claim_test.go`) already builds "a
claimable repo whose lease landed". Its work commit is titled `work
on plan 7`, not the masking `plan <id>: <title>` shape. It does not
exercise the bug. The new fixture is this same shape, with a masking
subject instead.

The Then half reuses `originsWorkRefForThePlanIsGone`
(`cmd/frit/bdd_landed_evidence_test.go`) verbatim.

`machineClaimsPlan`
(`cmd/frit/bdd_identity_and_cross_layer_test.go`) runs `claim` as a
second machine over an existing hold. That is the When step's
pattern. It is not reused directly, since it writes into that other
section's own state type.

## Tasks

1. Add `@S95` to
   [landed-evidence.feature](../features/landed-evidence.feature): a
   holder's masked, landed lease loses a race to a second machine's
   claim, which reports "already landed" and scavenges the ref.
2. Bind its new Given/When/Then in
   [bdd_landed_evidence_test.go](../cmd/frit/bdd_landed_evidence_test.go),
   reusing `originsWorkRefForThePlanIsGone`.
3. Add the matrix row in
   [lease-protocol.md](../docs/research/lease-protocol.md) and cite S95
   beside S54 in [claiming.md](../docs/claiming.md)'s failure-reasons
   discussion.

## Phase 1: a masked landed winner still reports landed

**RED.** In
[bdd_landed_evidence_test.go](../cmd/frit/bdd_landed_evidence_test.go),
add the scenario's steps. A Given pushes two or more empty commits
titled `plan <id>: <title>` on the holder's own branch — the masking
shape, not `landedLeaseRepo`'s plain `work on plan <id>`. A Given
merges that branch onto the default branch for real,
ancestor-preserving (`git merge --no-ff`; squash is S54's own shape
and not needed here). A When runs `frit claim <id>` from a second
machine's clone. A Then asserts the output contains "already landed"
— `lostRaceRefusal`'s exact wording lives in
[claim.go](../cmd/frit/claim.go); read it rather than retype it by
hand — and that `originsWorkRefForThePlanIsGone` passes.

Tag the scenario `@S95 @pending` first. Confirm `TestFeatures` skips
it. Remove `@pending`, then confirm the scenario fails. With
`latestMarker` reverted — temporarily, to prove red; see plan
2609082010's own stash-and-restore pattern — the claim falls through
to "lost the race to another machine" instead.

**GREEN.** No production code changes — the fix already landed in
plan 2609082010. Restore `latestMarker` and confirm the scenario
passes.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/^S95:'` passes; S54,
S79, S82, S84, S85 (landed-evidence's existing rows) and S94 (plan
2609082010's own resume scenario, sharing the fixed function) stay
green; the matrix row and `docs/claiming.md` citation are in place;
`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` all pass.

## Execution

| Phase | Work                                                  | Tier   |
| ----- | ----------------------------------------------------- | ------ |
| 1     | Proving slice: S95, its bindings, matrix row and docs | sonnet |

## Acceptance Criteria

- [ ] `@S95` in `landed-evidence.feature` exercises a lost race
      against a landed lease masked by `plan <id>: <title>` work
      commits, and fails without the marker-walk fix.
- [ ] The claim reports "already landed" and scavenges the leftover
      ref, proven through the built binary's own output, not a library
      call alone.
- [ ] S54, S79, S82, S84, S85 and S94 remain green.
- [ ] The matrix row exists in `lease-protocol.md`; `claiming.md` cites
      S95 beside S54.
- [ ] `go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run`
      and `mdsmith check .` pass.
