---
n: 1
title: The live-hold verdict is one Build input, shared by every consumer
status: "🔲"
---
Prove the structural fix on the smallest slice. Make the live-hold
verdict a required `Build` input, decided once, so the fleet and the
lanes walk read the same set and a released lane's leftover worktree
strands — which `frit orphans` already names and `frit reap` already
tears down. Delete the fleet-only overlay in the same move, so there is
no second place left to drift. Fix the test approach the second phase
reuses.

**Assumes.** `Release` deletes nothing, so a freed lane keeps its
`refs/heads/plan/<id>` branch and its local worktree. `merged` and
`landed` are already computed above the pure `Build` — `gitobj.MergedRefs`
and `index.LandedIDs` — and handed in as data. Fleet does not build its
own holds; it calls `Build` and then applies `claim.Held` inline, the one
liveness filter that never became a shared input.

**Value.** A worktree wedged on disk after a release stops being
invisible: `frit orphans` names it and `frit reap --go` removes it. And
the rule that made it invisible cannot come back — the verdict lives in
one place every consumer must pass through.

**RED.** Two levels, the pure one first.

- In [internal/lanes/lanes_test.go](../../internal/lanes/lanes_test.go),
  mirror `TestBuildDropsLandedClaims`: hand `Build` a released verdict for
  a ref that also carries a worktree, and assert `Find` reports the lane
  as stranded. At HEAD `Build` has no such input and keeps the ref as a
  live hold, so the lane is neither stranded nor unstaffed and the
  assertion fails.
- In [cmd/frit/main.go](../../cmd/frit/main.go)'s orphans path, add a
  test that a released-tip lane with a worktree is named by `frit
  orphans`; it is silent at HEAD.

**GREEN.** Add one classifier to
[internal/claim/lease.go](../../internal/claim/lease.go) that decides a
ref's live-hold verdict over its tip — the single authority, built on the
marker-kind primitive `claim.Held` and `claim.Released` already share, so
the two per-ref walks collapse into one. Make
[internal/lanes/lanes.go](../../internal/lanes/lanes.go)'s `Build` take
that verdict as a required input beside `merged` and `landed`, and drop a
non-live hold in the same filter loop. Compute it where git already lives
— the `repoLanes` helper in `main.go` and fleet's own hold walk in
[internal/fleet/gather.go](../../internal/fleet/gather.go) — and pass it
in. Delete fleet's inline `claim.Held` filter: the shared input now
carries it, so fleet's startable set is unchanged while the lanes walk
gains the same verdict.

**Reject the pass.** Do not add the verdict as a separate enrichment pass
beside `WithLanePaths`. A pass a caller must remember to run is the same
forgettable overlay that caused this bug. A required `Build` parameter is
compile-forced on every caller, so a future consumer cannot read holds
without it.

**Guard the edges.** A released tip with no worktree drops to zero holds
and zero worktrees, so it strands nothing. A live hold is unchanged. A
re-acquired plan's tip is a fresh hold, not a release marker, so it is
never dropped. Cover the no-worktree and live-hold cases so the drop
cannot over-fire.

**Gate.** A released-tip lane carrying a worktree is unreported by `frit
orphans` at HEAD and named after; `frit reap --go` removes its worktree;
fleet and the lanes walk derive their holds from the one shared verdict
with no fleet-only filter remaining; `go test ./...` is green.
