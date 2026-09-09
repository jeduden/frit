---
id: 2609092113
title: Resume relays a locally advanced lease branch instead of discarding it
status: "🔲"
summary: >-
  A lane that merges origin/main into its own checked-out plan/<id>
  branch — the documented way to pull main in without rebasing the
  lease token — advances that branch locally, ahead of the tip Renew
  or Resume was given. advance mints the next beat as a child of that
  given tip regardless, and syncLocalRef force-moves the local ref onto
  it with a bare update-ref: the local merge commit is silently
  orphaned, with no reflog entry to recover it by. Teach advance to
  build on the local branch's own tip when it is a clean fast-forward
  of the one it was given, so nothing pushed and nothing local is
  lost; refuse instead of resetting when the two have actually
  diverged. Addresses [issue #189][issue].
model: opus
depends-on: []
---
# Resume relays a locally advanced lease branch instead of discarding it

## Goal

`frit claim`/`start --go` resuming a lane it already holds never
silently discards a local advance on that lane's own `plan/<id>`
branch. Say the local checkout is a clean fast-forward beyond the tip
the renewal reads. Then the new beat carries that local work forward
instead of orphaning it. Say the two have genuinely diverged instead.
Then the renewal refuses, naming what it would have discarded, rather
than resetting past it with no trace.

## Context

**The report.** Issue #189: on a worktree checked out on
`plan/2609031104`, the reporter ran `git merge --ff-only origin/main`.
That is a clean, ancestor-preserving fast-forward of the lease branch
itself. They then ran `frit claim 2609031104`. `claim` reported
`resumed plan 2609031104` and left the branch on a fresh `plan
2609031104: beat` marker built on origin's lane tip, not on the merge
commit. The worktree's index and tree still held the merged content,
so `git status` showed the whole merge staged against a HEAD that had
moved backward. `git reflog` carried no entry for the move; nothing
but shell scrollback named the discarded SHA.

**The call chain.** `resumeOwnLease`
([cmd/frit/claim.go:119](../../cmd/frit/claim.go)) calls `ownToken`
(claim.go:186). It deliberately reads `claim.RemoteTip` — origin's
current tip, "rather than trusting plan.HoldTip, which is this
clone's possibly-stale local view" (claim.go:193-196) — and proves the
persisted token against it. That proof is about *who holds the lease*.
Reading origin fresh for it is correct: a distant host's claim can
only be told apart from a stale local view by asking origin. But the
same remote tip then rides on as `from` into `claim.Resume`
(lease.go:350, an alias for `Renew`) → `advance`
([internal/claim/lease.go:1022](../../internal/claim/lease.go)). It
mints the new beat as a child of `from` regardless of what the local
checkout actually carries, CASes it in `casPush` (lease.go:1150), and
on a win calls `syncLocalRef` (lease.go:1213) — a bare `git update-ref
<ref> <tip>` with no prior-value check, documented as "best-effort...
a stale local copy, which is a stale view, not a lost lease." That
comment is true when the local ref is stale in the ordinary sense:
behind, or unchanged. It is not true here. The local ref is *ahead*,
by a commit nothing else will ever carry forward, and `update-ref`
leaves no reflog line for the move — no old value and no explicit
reason string were given — matching the report.

**Why this was not already guarded.** Plan
[2608230705](../2608230705_claim-guards-local-branch-work.md) built
exactly this class of guard — `refuseDivergingLocalBranch`
(lease.go:1189), called from `pushClaimMarker` — for a *fresh* acquire
landing on top of a same-named local branch it never read. Its own
Context scoped the resume path out deliberately: "the resume path...
already CAS from a tip that Acquire or the caller read off the remote
first. The local ref they move from is never a stray unrelated
branch." That held as long as a lane's local branch never advanced
except through pushes the lease protocol itself made. It stops holding
the moment a lane does what [CLAUDE.md](../../CLAUDE.md) tells every
lane to do: merge `origin/main` into the lane's own branch rather than
rebase it — since that merge lands directly on `plan/<id>`, the same
ref `advance` is about to force past.

**A reusable shape, not a reusable function.** Plan
[2609011611](../2609011611_bind-renews-from-the-current-tip/plan.md)
solved the mirror-image staleness. There, the *remote* had moved ahead
of a cached mint tip before the session bind renewed. The fix read the
ref's current value and renewed from it, once guarded to this lane's
own hold (`ownHold`, lease.go:338). This plan reuses that shape — read
an alternate tip, renew from it only once it is provably safe — not
that function. `ownHold` answers "is the tip that beat our CAS still
ours," read from the *remote's* re-fetched marker. This plan answers
"is the *local* checkout's tip a safe fast-forward of the one we were
handed," read from the worktree's own `plan/<id>` ref before the CAS
runs at all. `isAncestor` (lease.go, already used by
`refuseDivergingLocalBranch`) is the one piece of machinery both share.

**Scope.** `advance` backs `Renew`, `RenewToBind` and `Release` —
every transition where this machine already holds (or is releasing)
the lane it is renewing, so a local fast-forward is always this lane's
own work, never a stray branch. `Takeover` mints from the *observed
stale* tip of a lease this machine does not yet hold and calls neither
`advance` nor this guard; a takeover's own worktree, if any, is not
authoritative over someone else's lease and stays untouched. The
existing fresh-acquire guard in `pushClaimMarker` is unrelated code on
the same ref and is left as-is.

**BDD coverage.** This changes the lease protocol's renewal baseline
choice, a lifecycle/claims-and-refs behavior per
[docs/development.md](../../docs/development.md)'s executable scenario
matrix, alongside S79/S82's sibling rows in
[lease-protocol.md](../../docs/research/lease-protocol.md). Phase 1
decides its own `@S<n>` reservation against origin's matrix at
execution time, rather than fixing a number here. S94 and a further
row are already claimed by plan 2609082010's still-unlanded lane as of
this writing. A number picked now could collide before this plan
starts.

## Tasks

1. Reproduce: a lease-unit test (`internal/claim/lease_test.go`, fake
   runner) puts the local `plan/<id>` ref one ordinary commit ahead of
   the tip `Renew`/`Resume` is given — a fast-forward, as an unpushed
   `merge --ff-only origin/main` on the lane's own branch would leave
   it — then calls `Renew`. RED: assert today's behavior discards it
   (the resulting local ref does not descend from the local-ahead
   commit).
2. Teach `advance` to mint the new marker as a child of the local
   checked-out ref's tip when that tip is a fast-forward of `from`,
   keeping `casPush`'s CAS `expected` argument at `from` so remote
   arbitration is untouched; when a local `plan/<id>` ref exists and is
   not a fast-forward of `from`, refuse and name the branch and the tip
   it would have discarded, rather than resetting past it. GREEN: the
   reproduction's local commit survives in the new tip's ancestry; a
   genuine foreign race (another host's push) still fences exactly as
   before.
3. Give the ref move itself a reflog trail: `syncLocalRef` passes the
   ref's prior value and an explicit reason to `update-ref` rather than
   a bare two-argument call, so any transition it makes — relayed,
   refused, or ordinary — is recoverable by `git reflog` even when
   nothing else remembers the old SHA.

## Execution

| Phase | Title                                                            | Tier | Gate                                                                                                                                                                 |
| ----- | ---------------------------------------------------------------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | advance relays a locally fast-forwarded lease branch, or refuses | opus | a local ff-ahead commit survives Renew/Resume, a diverged branch refuses by name, a foreign race still fences, the move leaves a reflog entry; `go test ./...` green |

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

| #   | Status | Phase                                                                          |
| --- | ------ | ------------------------------------------------------------------------------ |
| 1   | 🔲     | [advance relays a locally fast-forwarded lease branch, or refuses](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] A lane whose local `plan/<id>` branch is a clean fast-forward
      beyond the tip a renewal reads keeps that commit: `Renew`,
      `Resume` and `Release` build the next marker on it, not on the
      older remote-known tip.
- [ ] A local `plan/<id>` branch that has genuinely diverged from the
      tip a renewal reads refuses, naming the branch and the tip that
      would have been discarded, instead of resetting past it.
- [ ] A foreign host's concurrent push still wins or fences exactly as
      before; nothing about `casPush`'s remote arbitration changes.
- [ ] Every move `syncLocalRef` makes to the local ref leaves a
      `git reflog` entry.
- [ ] Phase 1's `@S<n>` decision is resolved against origin's current
      lease-protocol matrix, not the number sketched in this plan's
      Context.
- [ ] `go test ./...` and
      `go tool -modfile=tools/go.mod golangci-lint run` pass.
