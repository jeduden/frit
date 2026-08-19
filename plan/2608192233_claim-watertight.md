---
id: 2608192233
title: Harden the claim against squash-merges and shared checkouts
status: "🔳"
summary: >-
  Two demonstrated gaps let a stale or foreign hold read as live. A
  squash-merged plan is landed yet its branch is no ancestor, so the
  merged filter keeps its claim as a phantom hold. And a lane stood up
  in a shared clone lets a second agent's empty-selector command infer
  a plan the other agent holds. Close both.
model: sonnet
depends-on: []
---
# Harden the claim against squash-merges and shared checkouts

## Goal

Make a claim ref read as live only while it is. A plan already done on
the default branch is landed work, not a hold, even when a squash-merge
left its branch behind. And a command that infers its plan from the
current worktree refuses when that worktree stands on a claim another
host holds, rather than handing one agent the lane of another.

## Context

Both gaps were reproduced in one `plan-pick` session, and the evidence
is recorded here so the fix is driven by fact.

**Gap A — squash-merge defeats the merged filter.**
[MergedRefs](../internal/gitobj/git.go) runs `git for-each-ref --merged`,
which is ancestry-based. This repository squash-merges pull requests, so
a landed plan's branch tip is no ancestor of the default branch and the
filter never lists it. Measured on `origin/main`, `for-each-ref --merged`
over the plan refs returns nothing, while `one-fleet-walk` and
`ship-skills` are `✅` there. The result is five false "claimed, no
checkout" holds in `frit orphans` and phantom holders on the board.

Both readers reach the filter through one call,
[lanes.Build](../internal/lanes/lanes.go): the fleet join at
[gather.go](../internal/fleet/gather.go) that feeds board, ready and pick,
and [repoLanes](../cmd/frit/main.go) that feeds orphans. The authoritative
signal frit already trusts is the plan's status on the default branch
([index.go](../internal/index/index.go) ranks that version first). So a
hold whose plan is `✅` or `⛔` there is landed, and the fix is a
`landed` id set passed beside `merged`, dropped in the same loop. No new
git call is needed in the fleet path, which already parses the index;
orphans reads the default-branch statuses it does not yet load.

Searched for a git-only squash signal — patch-id, `git cherry`, the
`(#NN)` trailer — and reused none: a squash collapses many commits into
one, so no per-commit identity survives, and a message trailer is not a
contract. The plan status is the honest signal.

**Gap B — a shared checkout infers a foreign claim.**
The claim is atomic at the ref push, across machines. But two agents in
one clone share the working tree. When a lane is stood up as a checkout
in the shared clone rather than a dedicated worktree, a second agent's
empty-selector command — `frit next`, `frit claim`, `frit start`, which
each infer the plan from the cwd worktree branch
([claim.go](../cmd/frit/claim.go), [start.go](../cmd/frit/start.go),
[dispatch.go](../cmd/frit/dispatch.go)) — resolves to the plan the other
agent holds, with no warning. The claim marker already records the host
that took it ([claim.go](../internal/claim/claim.go), `markerHost`), so the
guard is a comparison: the standing claim's marker host against this
run's host. Reuse the marker reader rather than a second parser.

## Tasks

1. Pass a `landed` id set beside `merged` through `lanes.Build`, dropping
   a hold whose plan is done on the default branch; wire both callers.
2. (determined after Phase 1)

## Phase 1: a landed plan is not a hold

A claim ref on a plan that is `✅` or `⛔` on the default branch must not
read as a live hold, even when a squash-merge left the branch no ancestor
of that branch.

RED. In [lanes](../internal/lanes/lanes.go), a test builds one hold whose
plan id is in a `landed` set and asserts `Build` returns no lane for it —
not Unstaffed, not held. End to end, `TestOrphansIgnoresASquashMergedClaim`
in [cmd/frit](../cmd/frit/main_test.go) sets a plan `✅` on the default
branch with a claim branch that is no ancestor of it, and asserts orphans
stays quiet — the counterpart to the existing merged-claim test.

GREEN. Add a `landed map[int64]bool` parameter to `lanes.Build`, checked
in the ref loop beside `merged[ref.Name]`: a matched hold whose id is
landed is skipped. Compute the set from the default-branch plan statuses:
in [gather.go](../internal/fleet/gather.go) from the entries already parsed,
in [repoLanes](../cmd/frit/main.go) by reading the default ref's plan index.
Update the call sites and their tests.

Gate: `TestOrphansIgnoresASquashMergedClaim` passes. The existing orphan
and merged-claim tests still pass. And `frit orphans` on this repo drops
the `✅` plans from the claimed-no-checkout list.

## Phase 2: refuse a foreign checkout

An empty-selector command infers its plan from the current worktree. It
must refuse when that worktree stands on a claim whose marker host is not
this run's host. It names the holder rather than proceeding.

RED. A test drives the inference guard with a worktree on a claim branch
whose marker records host `otherbox` while the run's host is `thisbox`,
and asserts a refusal that names `otherbox`. A second case, a marker host
equal to the run's host, asserts the command proceeds unchanged.

GREEN. Expose a marker-host reader from [claim](../internal/claim/claim.go)
— `markerHost` and `holderMarker` are the machinery, lifted to one
exported reader over a branch — and add a preflight to the empty-selector
path shared by `frit next`, `frit claim` and `frit start`. A run's host
is the value the claim marker is written with; compare the two. An
unreadable marker falls back to proceeding, so a non-frit branch is not a
false refusal.

Gate: standing on a foreign claim, each empty-selector verb refuses and
names the holder. Standing on this host's own claim, or on no claim, each
verb behaves as before.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase                       | Design | Implement | Gate that catches a wrong answer                                      |
| --------------------------- | ------ | --------- | --------------------------------------------------------------------- |
| 1 landed is not a hold      | sonnet | sonnet    | orphans ignores a squash-merged `✅` claim; merged/orphan tests hold  |
| 2 refuse a foreign checkout | opus   | sonnet    | empty-selector verb refuses on a foreign marker host, proceeds on own |

## Non-goals

- No change to the atomic push. The lease arbitration across machines is
  correct; this hardens what reads a ref back as live, not how one is
  minted.
- No status reconciliation. Flipping a stale `🔲` whose work landed is
  plan-sync's job; this trusts the status the default branch carries.
- No worktree management. frit still delegates standing a lane up to
  herdr; Phase 2 warns about a shared checkout, it does not isolate one.

## Acceptance Criteria

- [x] A claim on a plan done on the default branch is not a live hold
- [x] `frit orphans` ignores a squash-merged claim, as it does an ancestor
- [x] Board, ready and pick read a landed plan as unheld
- [ ] An empty-selector verb refuses on a worktree held by another host
- [ ] The refusal names the holder read from the claim marker
- [ ] An own-host or non-frit checkout is not falsely refused
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
