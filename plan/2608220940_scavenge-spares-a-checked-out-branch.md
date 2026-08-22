---
id: 2608220940
title: Scavenge never deletes a branch a worktree still stands on
status: "🔳"
summary: >-
  claim.Scavenge and claim.Mint's rollback both run a plain
  `update-ref -d` on a plan's hold branch in the repo's primary
  worktree path. Every linked worktree of that repo shares the same
  ref database, so the delete lands wherever the branch is actually
  checked out too — even a worktree the command never touched. Git's
  own porcelain (`git branch -d`, `git worktree remove`) refuses this;
  the plumbing `update-ref` frit calls does not, and empirically
  succeeds, leaving that worktree's HEAD a dangling symbolic ref (S79).
model: sonnet
depends-on: []
phases:
  - n: 1
    title: Scavenge spares a branch checked out elsewhere
    status: "🔲"
  - n: 2
    title: Mint's rollback gets the same guard
    status: "🔲"
---
# Scavenge never deletes a branch a worktree still stands on

## Goal

A destructive `update-ref -d` on a plan's hold branch must first
check whether any worktree has it checked out. If one does, the local
ref stays. The remote delete and rescue-park still happen exactly as
today; only the local copy survives where it is in use.

## Context

Reproduced directly in this session. `frit release` on a landed plan
ran `claim.Scavenge` from the repo's primary worktree path.
[internal/fleet/gather.go](../internal/fleet/gather.go)'s `coordOf`
always resolves `Coord.Path` to
[internal/discover](../internal/discover)'s `Repo.Path`. That is the
*main* worktree, per `Repos`' `worktrees[0]` — never whichever linked
worktree a command happens to run from.

`Scavenge`'s local `update-ref -d refs/heads/plan/<id>`
([internal/claim/lease.go](../internal/claim/lease.go) lines 257 and
287) runs against the *shared* ref database every linked worktree of
that repository reads. It deleted the branch out from under a second
worktree that had it checked out, though the command's own `repoDir`
was the primary worktree the whole time. That worktree's `git status`
started reporting every tracked file as newly added. `git branch
--show-current` still named the deleted branch. Nothing was lost —
the commit stayed a reachable loose object — but nothing said so
either.

A throwaway two-worktree repo confirms the mechanism precisely.
`git update-ref -d <ref>` run against a branch checked out in a
*different* linked worktree succeeds silently, exit 0. Only git's
porcelain — `branch -d`, `checkout`, `worktree remove` — carries a
"checked out elsewhere" guard. The plumbing frit already uses for
every marker write does not.
[internal/claim/lease.go](../internal/claim/lease.go)'s `syncLocalRef`
carries a comment claiming the opposite: "the branch may be checked
out in the lane's worktree, where git refuses an update from
outside". The same reproduction disproves it. Phase 2 corrects it.

**Reuse first.** [internal/discover](../internal/discover)'s
`Repo.Worktrees []gitwt.Worktree` already lists every worktree of a
repository. Each entry carries `.Path` and `.Branch`
([internal/gitwt/worktree.go](../internal/gitwt/worktree.go)). It is
read via the porcelain parser `gitwt.List`
([internal/gitwt/git.go](../internal/gitwt/git.go)), already imported
by `internal/claim`. No new parser, no new subprocess kind is needed.
The guard is one fresh `gitwt.List(repoDir, run)` call at the point
of deletion, checked against the branch about to be removed.
`fleet.Coord` does not currently carry `Worktrees`. It does not need
to: the guard reads worktree state itself, rather than threading it
through.

Every call site that runs `update-ref -d` on a plan's hold branch:
[internal/claim/lease.go](../internal/claim/lease.go) lines 257 and
287 (`Scavenge`, both branches — already-gone and
post-push-delete), and
[internal/claim/claim.go](../internal/claim/claim.go) lines 135 and
145 (`Mint`'s two rollback paths, on a ref `Mint` itself just created
moments earlier in the same call). `Scavenge` is the proven,
reproduced case and is Phase 1's slice. `Mint`'s rollback is the same
class of risk on a tighter window, and is Phase 2.

## Tasks

1. Add a worktree-checked-out guard and wire it into `Scavenge`'s two
   deletes.
2. Wire the same guard into `Mint`'s two rollback deletes; correct
   `syncLocalRef`'s comment; add scenario S79 to the protocol doc.

## Phase 1: Scavenge spares a branch checked out elsewhere

`Scavenge` must not run its local `update-ref -d` when any worktree
of the repository still has the plan's hold branch checked out. The
remote delete, the rescue park, and the fence/no-op paths are
unaffected. Only the local ref's survival changes.

RED, extending the `originAndClone`/`cloneAgain` fixture idiom
[internal/claim/lease_test.go](../internal/claim/lease_test.go)
already uses, with a real linked worktree instead of a second clone:

- A repo with a linked worktree (`git worktree add`) checked out on
  the plan's hold branch. `Scavenge` runs against the primary
  worktree's path, exactly as `frit release` does. Assert: the remote
  ref is gone, unchanged behavior. The local
  `refs/heads/plan/<id>` in the primary worktree's repo still
  resolves to the pre-scavenge tip. The linked worktree's own `HEAD`
  still resolves — `git -C <linked> rev-parse HEAD` succeeds and
  reads the same tip.
- The existing case with no linked worktree at all
  ([internal/claim/lease_test.go](../internal/claim/lease_test.go)'s
  `TestScavengeParksUnlandedWorkThenDeletes`) keeps deleting the
  local ref exactly as before. The guard changes nothing when nobody
  is standing on the branch.

GREEN: a `checkedOut(repoDir, branch string, run gitwt.Runner) bool`
helper in [internal/claim/lease.go](../internal/claim/lease.go),
built on `gitwt.List`, read fresh at each of `Scavenge`'s two
`update-ref -d` call sites. Skip the delete when it answers true.

Gate: the two RED cases pass; `go test ./internal/claim/...` is
clean; `go test ./...` and `mdsmith check .` stay clean.

## Phase 2: Mint's rollback gets the same guard

`claim.Mint`'s two rollback paths run the identical hazard on a
narrower window: a ref `Mint` just created locally, rolled back
after a lost push. Any worktree that checked it out in that window
must be spared the same way `Scavenge` now is.

RED, mirroring Phase 1's fixture. A linked worktree checks out
`Options.Branch` first. Then `Mint`'s push loses the race, reusing
[internal/claim/claim_test.go](../internal/claim/claim_test.go)'s
race-losing setup. Assert the rollback still reports the lost race
correctly. Assert the linked worktree's `HEAD` still resolves
afterward.

GREEN: the same `checkedOut` helper, at
[internal/claim/claim.go](../internal/claim/claim.go) lines 135 and
145. `syncLocalRef`'s comment in
[internal/claim/lease.go](../internal/claim/lease.go) is rewritten.
Say what Phase 1's reproduction actually found. Plumbing carries no
such protection. That is exactly why the guard exists. Drop the
disproven claim.
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md)'s
scenario matrix gains S79 under "Lifecycle anomalies", beside its
nearest sibling S56 ("local branch deleted by hand"): frit's own
scavenge deleting a branch live under another worktree, closed by
the `checkedOut` guard (CAS, PARK).

Gate: the rollback RED case passes; a live holder's rollback path is
never taken by surprise; `go test ./...`, `go vet ./...`,
`golangci-lint run`, and `mdsmith check .` stay clean.

## Execution

Both phases apply the one guard this plan's Context already
settles. Implementing from the written RED cases is cheap either
way.

| Phase                    | Design | Implement | Gate that catches a wrong answer                              |
| ------------------------ | ------ | --------- | ------------------------------------------------------------- |
| 1 Scavenge spared        | opus   | sonnet    | a linked worktree's HEAD survives a scavenge from the primary |
| 2 Mint's rollback spared | opus   | sonnet    | a linked worktree's HEAD survives a lost-race rollback        |

## Acceptance Criteria

- [ ] A worktree checked out on a plan's hold branch survives a
      `Scavenge` run against the repo's primary worktree
- [ ] A worktree checked out on a plan's hold branch survives a
      `Mint` rollback after a lost race
- [ ] `Scavenge`'s existing no-worktree behavior is unchanged
- [ ] `syncLocalRef`'s comment states git's actual behavior
- [ ] Scenario S79 is recorded in
      [docs/research/lease-protocol.md](../docs/research/lease-protocol.md)
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
