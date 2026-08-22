---
id: 2608221559
title: Remove the superseded Mint claim path
status: "🔳"
summary: >-
  claim.Mint is the pre-lease single-marker claim, superseded by the
  lease protocol's Acquire in commit d559318 and called only by its
  own tests. Its first act, an unconditional `update-ref
  refs/heads/<branch> <marker>`, moves the branch to a claim marker
  built from the base's tree; run against a branch a linked worktree
  has checked out, it silently rewrites that worktree's HEAD and
  leaves its working tree showing phantom edits. The live path
  (Acquire) never does this: syncLocalRef only moves to a
  tree-preserving marker. Deleting the dead cluster closes the hazard
  at its root rather than guarding code nothing calls.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: Sever the live MarkerHost path from the Mint-era Options
    status: "✅"
  - n: 2
    title: Delete the dead Mint cluster and correct S79
    status: "🔲"
---
# Remove the superseded Mint claim path

## Goal

Remove `claim.Mint` and the code reachable only through it. That
deletes the unguarded `update-ref` that can corrupt a checked-out
worktree. Then correct the scenario doc that still credits a "mint"
delete.

## Context

The `/code-review` of PR #54 flagged
[internal/claim/claim.go](../internal/claim/claim.go) line 108. `Mint`
runs `update-ref refs/heads/<branch> <marker>` before its push, with
no `checkedOut` guard. I reproduced it in a throwaway two-worktree
repo. A *move* — not only a delete — of a branch a linked worktree has
checked out succeeds silently, exit 0. It repoints that worktree's
`HEAD` and leaves `git status` reporting phantom edits. The cause: the
claim marker's tree, `opts.Base^{tree}`, differs from the worktree's.

**Is it reachable?** No. `Mint` is the pre-lease, single-marker claim.
The lease protocol superseded it in commit `d559318`, "The hold is the
work ref, a self-healing lease". That path is what every `frit`
command now claims through:
[internal/claim/lease.go](../internal/claim/lease.go)'s `Acquire`,
`Renew`, `Release`, `Takeover`. Nothing calls `claim.Mint`. `grep`
finds its only callers in
[internal/claim/claim_test.go](../internal/claim/claim_test.go). The
`internal/claim` package is import-restricted to this module. So no
external consumer reaches it either. It is dead code.

**Why the live path is safe, so only Mint needs this.** `Acquire`
reconciles its local ref through `syncLocalRef`
([internal/claim/lease.go](../internal/claim/lease.go)), which moves
the branch only to a marker `mintMarker` built from `parent^{tree}` —
the same tree the worktree already holds. A tree-preserving move
leaves the working tree clean, which is why Phase 2 of plan 2608220940
deliberately declared that move acceptable. `Mint`'s move is the one
that changes the tree.

**Why remove, not guard.** A `checkedOut` guard on line 108 would be a
defensive branch drivable red/green only by calling the dead function,
which perpetuates it — against
[CLAUDE.md](../CLAUDE.md)'s "Defensive Code" rule. Removal closes the
hazard and shrinks the surface, in keeping with "frit consumes rather
than reimplements".

**Reuse first — what is dead vs. shared.** Reachable only through
`Mint`, so removable: `Mint`, `Result`, `LostRaceError`, `Holder`,
`readHolder`, `markerMessage`, `landed`, and the `Options` struct.
Must stay, because the live lease path uses them: `ErrLostRace`
(wrapped by `HeldError` and `VetoError`), `remoteHolder`,
`remoteHolderErr`, `landedTip`, `freshBase`, `baseBranch`,
`isAncestor`, `Branch`, `trimmed`. The one tangle is `holderMarker`:
shared by dead `readHolder` and live `MarkerHost`
([internal/fleet](../internal/fleet) reads it), but it takes an
`Options` only to read `opts.PlanID`. Phase 1 severs that so `Options`
can go with the rest.

## Tasks

1. Refactor `holderMarker` and `MarkerHost` to take `planID int64`,
   not `Options`, keeping `MarkerHost`'s behavior identical.
2. Delete the dead cluster and its tests; correct S79 in the protocol
   doc to drop the "mint" claim and record the removal.

## Phase 1: Sever the live MarkerHost path from the Mint-era Options

`holderMarker`
([internal/claim/claim.go](../internal/claim/claim.go)) takes an
`Options` but reads only `opts.PlanID`. Its two callers are dead
`readHolder` and live `MarkerHost`. Retype it to `planID int64` so the
surviving `MarkerHost` path no longer names `Options`, isolating the
dead cluster for Phase 2's pure delete. No behavior changes.

RED is an existing green kept green. `TestMarkerHostReadsALeaseMarker`
([internal/claim/lease_test.go](../internal/claim/lease_test.go)) and
`TestMarkerHost`
([internal/claim/claim_test.go](../internal/claim/claim_test.go))
already pin `MarkerHost`'s output. This phase is a pure refactor. The
assertion is that both stay green across the signature change. That is
the proof the live behavior is unchanged.

GREEN: change `holderMarker`'s signature to
`(repoDir string, planID int64, tip string, run gitwt.Runner)`, use
`planID` in its grep pattern, and update both call sites. `MarkerHost`
passes its own `planID`; `readHolder` passes `opts.PlanID` (it is
deleted in Phase 2, but must compile now).

Gate: `TestMarkerHost` and `TestMarkerHostReadsALeaseMarker` pass;
`go build ./...`, `go test ./...`, `go vet ./...` clean; no reference
to `Options` remains inside `holderMarker`.

## Phase 2: Delete the dead Mint cluster and correct S79

With `holderMarker` freed, nothing but `Mint` reaches the dead
cluster. Delete it all from
[internal/claim/claim.go](../internal/claim/claim.go): `Mint`,
`Result`, `LostRaceError`, `Holder`, `readHolder`, `markerMessage`,
`landed`, and the `Options` struct. Delete their tests too. They are
the 21 `Mint(` call sites and the Holder- and LostRace-only cases in
[internal/claim/claim_test.go](../internal/claim/claim_test.go).

RED has two parts. First, a guard that the live claim path is
untouched: reassert one existing `Acquire` race case in
[internal/claim/claim_test.go](../internal/claim/claim_test.go), with
the winner named and the loser fenced. Second, the mechanical proof of
absence. A `grep` for `Mint`, `Result`, `LostRaceError`, and `Options`
across the tree returns nothing outside historical `plan/` files.

GREEN: the deletions. Then correct S79 in
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md):
its row reads "scavenge/mint delete a branch a worktree still stands
on", but `Mint` is gone, so scavenge is the only deleter left. Reword
to name scavenge alone, and note in the row that the legacy Mint path
was removed rather than guarded.

Gate: `go build ./...`, `go test ./...`, `go vet ./...`,
`golangci-lint run` clean; `grep` finds no surviving reference to the
deleted symbols; `mdsmith check .` stays clean.

## Execution

Phase 1 is a signature refactor proven by an unchanged test; Phase 2
is a delete plus a one-row doc correction. Both read cheaply off the
Context above.

| Phase                      | Design | Implement | Gate that catches a wrong answer                         |
| -------------------------- | ------ | --------- | -------------------------------------------------------- |
| 1 Sever MarkerHost/Options | opus   | sonnet    | MarkerHost tests stay green across the signature change  |
| 2 Delete cluster, fix S79  | opus   | sonnet    | grep finds no deleted symbol; Acquire race case is green |

## Acceptance Criteria

- [ ] `claim.Mint` and its exclusive types are gone from the tree
- [ ] `MarkerHost` behaves identically, proven by its unchanged tests
- [ ] The live lease path (`Acquire`, `Renew`, `Release`, `Takeover`)
      is untouched
- [ ] S79 in
      [docs/research/lease-protocol.md](../docs/research/lease-protocol.md)
      names scavenge alone and records the Mint removal
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
