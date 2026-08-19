---
id: 2608192322
title: A claimed lane is its own worktree, never the shared clone
status: "🔲"
summary: >-
  frit claim mints the hold but stands up no checkout, so a claimed lane
  is worked in whatever clone the agent sits in — the shared one — and a
  second agent or tool lands on it. start already isolates a lane through
  herdr; give claim the same, so a hold always arrives with a worktree of
  its own.
model: sonnet
depends-on: [2608192233]
---
# A claimed lane is its own worktree, never the shared clone

## Goal

Make a claim arrive with its own worktree, so a lane is never worked in
the shared clone. frit claim already mints the atomic ref; standing the
checkout up beside it, through herdr as start does, closes the
shared-clone collision at its source — where the guard from plan
[2608192233](2608192233_claim-watertight.md) only warns after the fact.

## Context

The collision is real. It was hit twice in one session. A `plan-pick`
run and a forked `code-review --fix` both landed in the shared clone on
a lane another agent held. The review then wrote into that lane's tree.

The claim is atomic at the ref push, but a ref is not a checkout.
[claimCmd.Run](../cmd/frit/claim.go) mints the lease and stops. Nothing
stands a worktree up, so the agent works the lane wherever it already
is. One clone, one working tree, many agents — the checkout is shared,
and every tool that reads the cwd collides on it.

The isolating half already exists for the heavier verb.
[standUpLane](../cmd/frit/start.go) hands a lane to
[herdr.WorktreeCreate](../internal/herdr/dispatch.go), which checks the
branch out into a worktree of its own and opens a pane. So start yields
an isolated lane; claim does not. This plan gives claim that same step,
reusing herdr rather than shelling `git worktree` — herdr owns
worktrees, and frit consumes rather than reimplements.

The guard in [2608192233](2608192233_claim-watertight.md),
[fleet.ForeignHold](../internal/fleet/current.go), refuses an empty-selector
frit verb standing on a foreign checkout. That is a smoke alarm, not
isolation: it fires after the fact and guards only frit's own verbs, not
code-review, `go run` or an editor. The durable fix is that the shared
checkout never carries a lane at all.

Searched for existing worktree management to reuse: herdr owns it, and
start already drives it; frit must not grow a second path. The claim
delegates to `herdr.WorktreeCreate`, the same call start makes.

## Tasks

1. After minting the lease, frit claim stands the lane's worktree up
   through herdr and reports its path.
2. (determined after Phase 1)

## Phase 1: claim stands the worktree up

After the lease is minted, frit claim checks the branch out into its own
worktree through herdr and reports the path, so a successful claim yields
an isolated checkout to work in, not a bare ref that tempts a shared
clone. The agent is start's rung; claim stands up the checkout only.

RED. A test drives claim past a stub herdr and asserts it calls
`herdr.WorktreeCreate` with the claim branch, and that the rendered
report names the worktree path. A second case: herdr cannot stand the
worktree up, and claim reports the failure as a warning while the lease
stands — the ref is already atomic, so a failed checkout is not a lost
claim.

GREEN. In [claimCmd.Run](../cmd/frit/claim.go), after `mintClaim` succeeds,
call `herdr.WorktreeCreate` with the branch and the coordinate's path,
reusing the spec [standUpLane](../cmd/frit/start.go) builds. Carry the
worktree path, or the herdr failure, into the claim report.

Gate: after `frit claim`, a worktree stands on the claim branch. The
shared clone's checkout is untouched. A herdr failure warns without
dropping the lease.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase                      | Design | Implement | Gate that catches a wrong answer                                    |
| -------------------------- | ------ | --------- | ------------------------------------------------------------------- |
| 1 claim stands worktree up | opus   | sonnet    | after claim, a worktree stands on the branch; a herdr failure warns |

## Non-goals

- No agent or prompt. Standing the agent up and prompting its phase stays
  start's rung; claim stands up the checkout only.
- No new git write. frit still owns exactly one mutation, the lease; the
  worktree is herdr's, reached through its socket.
- No forced layout. Where herdr puts the worktree is herdr's call; frit
  reports the path it is told, it does not choose one.

## Acceptance Criteria

- [ ] A successful frit claim stands a worktree up on the claim branch
- [ ] The claim report names the worktree path the agent works in
- [ ] The shared clone's checkout is left untouched by a claim
- [ ] A herdr failure warns and leaves the atomic lease standing
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
