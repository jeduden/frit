---
n: 2
title: pick and start reconcile a pre-existing worktree for a free plan
status: "🔲"
result: false
---
Give `start`'s escalation the reconcile Phase 1 made possible. Before
minting a fresh claim or takeover, notice a worktree already sitting
on the plan's own branch. Leave a genuinely live one alone. Clear a
dead one first. `pick --go` reuses the same path, since it dispatches
through `startResolved`.

**Assumes.** A worktree registered on `sp.Branch`
(`claim.Branch(plan.ID)`) ahead of `startAcquire` in
[cmd/frit/start.go](../../cmd/frit/start.go) can only be a leftover.
`Release` deletes nothing. A takeover only wins once herdr confirms
the prior holder's bound session is dead. `buildStart`'s own readiness
gate already refused a genuinely live hold before `startExecute` is
ever reached.

The worktree's branch ref is not the leftover's own frozen tip. It is
checked out *on* `sp.Branch`, so its reported HEAD always resolves
that branch live. `Release` already moved the branch ref once, onto a
bare release-marker commit a worktree checkout never sees as a
file-level checkout. Deleting the branch is therefore never this
reconcile's to do. The branch ref's own chain — release marker,
parented on the real work, parented on the claim that started it — is
what still needs protecting, not the worktree's directory.
[reap.go](../../cmd/frit/reap.go)'s own `parkBranch` already parks
exactly that chain: it resolves the branch's *current* tip and walks
back past frit's own markers to the real work beneath. It is reused
here unmodified, called before `startAcquire` so there is no fresh
claim on the ref yet to protect it from. `gitwt.List` (already
exported, already how
[internal/claim/lease.go](../../internal/claim/lease.go)'s unexported
`checkedOut` finds a worktree by branch) is the one read that finds
the leftover in the first place. `herdr.List` and `herdr.Resolve` —
both exported, both already proven by `herdr.LiveRoots`/`Join` — tell
a dead leftover from one a person or agent is still sitting in.

**Value.** `pick --go` / `start --go` re-selecting a plan whose lane
was released or taken over stop dying on herdr's raw
`worktree_create_failed`. A dead leftover is parked and cleared before
anything is claimed, so the fresh worktree stands up where the stale
one sat. A leftover a person or agent is still actually working in is
left alone, with a clear frit-authored refusal instead of herdr's
opaque one — and nothing is claimed at all in that case, so there is
no lease to unwind.

**RED.** CLI level, in
[cmd/frit/start_test.go](../../cmd/frit/start_test.go). A fixture
helper reproduces the real shape: `claim.Acquire` then a worktree
checked out on the live branch, a real commit inside it, pushed, then
`claim.Release` from that commit — the exact sequence a released lane
leaves behind, `claimableRepo`'s pushed origin carrying it throughout,
since `parkBranch` needs one.

- `TestStartGoReapsADeadLeftoverWorktreeBeforeRecreating`: the fixture
  above, no herdr pane anywhere near it. Run `start <id> --go`. At
  HEAD nothing looks for the leftover at all, so it is left standing
  once `start` finishes. Assert `start` instead succeeds, a rescue ref
  for the plan is non-empty (the leftover's real work is parked, not
  dropped), the leftover directory is gone, and `worktree create` was
  still called once — the reconcile clears the way, it does not skip
  the create.
- `TestStartGoRefusesALiveHerdrPaneOnTheLeftover`: the same fixture,
  but the fake herdr's `agent list` answers with one pane whose `cwd`
  resolves, via `herdr.Resolve`, to the leftover's own worktree root.
  Assert `start --go` exits non-zero with a frit-authored message
  naming the pane and the path, not herdr's raw error. Assert the
  leftover worktree is untouched and `worktree create` was never
  called. Assert `sp.Branch`'s tip is exactly what `claim.Release` left
  it at — nothing was claimed, so there is nothing to release again.
- `TestStartGoDispatchesAPhaselessPlan` (already green) is the no-op
  guard: no leftover sits on the branch, so `gitwt.List` finds nothing
  and `startAcquire`/`herdr.WorktreeCreate` run exactly as before. It
  must stay green with no fixture changes.

**GREEN.** Add `reconcileLeftoverWorktree(rt *runtime, sc
startContext, sp report.StartPlan, planID int64) error` in
[cmd/frit/start.go](../../cmd/frit/start.go).

- `gitwt.List(sc.repoPath, rt.git)`, find the one whose `Branch ==
  sp.Branch`; `nil` when none.
- Found: `livePaneOn(rt, leftover.Path)` — walk
  `herdr.List(rt.herdr)`, skip a remote pane (`Host != ""`, the same
  guard `LiveRoots` uses), `herdr.Resolve(pane.CWD,
  rt.git).Root == leftover.Path`. A hit returns a `fmt.Errorf` naming
  the pane id and the path.
- No hit: `parkBranch(rt, discover.Repo{Path: sc.repoPath},
  claim.LeaseOptions{PlanID: planID, Remote: sc.remote, Base: sc.base,
  Holder: hostname()}, sp.Branch, true)` — erroring the reconcile as
  `"park: " + err.Error()`, the same wording `reapStranded` uses —
  then `rt.git(sc.repoPath, "worktree", "remove", leftover.Path)`. The
  branch is never deleted.
- `startExecute` calls `reconcileLeftoverWorktree` in its non-resume
  branch, before `startAcquire`, and returns its error, if any,
  without minting anything. A refusal never reaches herdr, and never
  touches the lease. A cleared leftover leaves `startAcquire` and
  `laneStandUpPane`'s `herdr.WorktreeCreate` call untouched.

**Reject the retry.** Do not let `herdr.WorktreeCreate` fail first and
then reap-and-retry on its error string. herdr's failure text is not a
contract frit parses (`docs/architecture.md`'s "consumes rather than
reimplements"). A blind retry after clearing a *live* pane would be
exactly the silent-destruction case the refusal branch exists to
avoid. The pre-check decides before anything is claimed.

**Guard the edges.** No worktree on `sp.Branch` costs nothing.
`reconcileLeftoverWorktree` returns immediately, and `startAcquire`
runs exactly as it does today. A resume never reaches this code.
`startExecute`'s existing `if resume` branch skips straight to
`startAcquire` with the resumed lane, and `laneStandUpPane` returns
via `herdr.CurrentPane` rather than creating anything. A park failure
refuses the reconcile, and therefore the whole start, before anything
is claimed or deleted.

**Gate.** With a worktree already on the plan's canonical branch,
`start --go` reaps a dead leftover, parking first, and stands the
fresh worktree up in its place instead of surfacing herdr's raw
`worktree_create_failed`. A live herdr pane on that leftover is left
standing and refused with a frit-authored message before anything is
claimed. `go test ./...` is green.
