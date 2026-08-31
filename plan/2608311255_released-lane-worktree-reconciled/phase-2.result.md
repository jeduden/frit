---
n: 2
title: pick and start reconcile a pre-existing worktree for a free plan
status: "✅"
result: true
summary: reconcileLeftoverWorktree runs in startExecute ahead of startAcquire — a live herdr pane on a leftover worktree refuses before anything is claimed, a dead one is parked (reap.go's own parkBranch) and its worktree registration cleared, so pick/start stand a fresh checkout up where herdr's raw worktree_create_failed used to strike.
---
## Handoff

`start`'s escalation now reconciles a leftover worktree before it ever
mints a claim. `reconcileLeftoverWorktree` in
[cmd/frit/start.go](../../cmd/frit/start.go) runs in `startExecute`'s
non-resume branch, ahead of `startAcquire`: `gitwt.List` finds a
worktree already registered on the plan's canonical branch, `livePaneOn`
(built on `herdr.List`/`herdr.Resolve`, the same primitives
`herdr.LiveRoots` already proved) checks whether a herdr pane is still
sitting on it. A live pane refuses with a frit-authored message naming
the pane and the path — nothing is claimed, so there is nothing to
unwind. A dead leftover is parked by reusing
[reap.go](../../cmd/frit/reap.go)'s own `parkBranch` unmodified — it
resolves the branch's current tip and walks back past frit's own
markers to the real work, exactly the chain a released branch's
history still carries — then its worktree registration is removed via
a plain `git worktree remove`. The branch itself is never deleted:
`parkBranch` reads the branch's own current tip, so parking is correct
whether it runs before or after a fresh claim moves that tip, and
deleting the branch would destroy whatever claim is about to be minted
on it. Because the reconcile runs before `startAcquire`, that
ambiguity about *which* tip to trust never arises — this is the
opposite ordering `phase-1.result.md`'s handoff assumed the fix would
need to protect against, and RED here is what forced the correction:
an early draft tried to park a `gitwt.Worktree.Head` captured before
`startAcquire` under the assumption a worktree's HEAD is a fixed
snapshot; it is not — a worktree checked out *on* a branch always
resolves HEAD live, and `Release` itself already moves that branch
onto a bare marker commit no worktree checkout ever sees as a
file-level checkout, so `Head` reads the *branch's* current position,
never a frozen one. Reconciling ahead of `startAcquire` sidesteps the
question entirely: `parkBranch`'s ordinary branch-tip resolution is
exactly right, unmodified.

`pick --go` needed no separate wiring: `pickCmd.start` already calls
the same `buildStart` → `startExecute` path `start --go` does, so the
reconcile covers it for free.

**Proven.** `TestStartGoReapsADeadLeftoverWorktreeBeforeRecreating` and
`TestStartGoRefusesALiveHerdrPaneOnTheLeftover` in
[cmd/frit/start_test.go](../../cmd/frit/start_test.go), built on a
`leftoverWorktree` fixture that reproduces the real shape end to end —
`claim.Acquire`, a worktree checked out on the live branch, a real
commit, a push, then `claim.Release` from that commit — the same
sequence a released lane actually leaves behind, not an approximation
of it. `TestStartGoDispatchesAPhaselessPlan` (unchanged) pins the
no-op path: no leftover, no behavior change. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run ./...` are clean.

**Not addressed.** A live herdr pane refusal is `--go`-only; a dry run
does not preview it, since the Gate never asked for one and the read
itself (`herdr.List`) is cheap enough not to need caching either way.
Nothing here changes what counts as a live hold — that was Phase 1's
own, closed change.
