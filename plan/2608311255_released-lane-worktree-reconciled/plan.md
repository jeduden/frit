---
id: 2608311255
title: A released lane's leftover worktree is seen and reconciled
status: "✅"
summary: >-
  When a lane is released or taken over, its branch stays and its local
  worktree is left on disk. frit orphans and frit reap miss it because
  the one liveness rule that would exclude a released hold lives as a
  fleet-only overlay, not a shared input, so their walk and the fleet's
  disagree. Meanwhile pick and start re-select the now-free plan and die
  on herdr's raw worktree-create collision instead of reconciling. Fix
  it structurally: decide a ref's live-hold verdict in one place — a
  required Build input, not an overlay a caller can forget — so every
  consumer reads the same set and a released lane's leftover strands,
  which orphans names and reap tears down; then let pick and start
  reconcile a pre-existing worktree. Closes issue 118.
model: sonnet
depends-on: []
---
# A released lane's leftover worktree is seen and reconciled

## Goal

A worktree left behind when a lane is released or taken over is named by
`frit orphans` and torn down by `frit reap`, and `pick`/`start`
re-selecting that free plan reconciles the leftover rather than dying on
herdr's `worktree_create_failed`. The fix is structural: whether a ref
is a live hold is decided in the one place every consumer reads it, so
the fleet and the lanes walk cannot drift apart again. This is issue 118.

## Context

**The asymmetry, and why it is structural.** `Build` in
[internal/lanes/lanes.go](../../internal/lanes/lanes.go) reads holds from
the refs and drops the ones that are merged or landed — two exclusions
handed to it as data. [internal/fleet/gather.go](../../internal/fleet/gather.go)
does not build its own holds; it calls `Build`, then bolts one more
exclusion on top inline — a hold whose tip fails `claim.Held` — so a
released plan reads as startable. That extra filter lives only in the
fleet walk. `frit orphans` and `frit reap` read `lanes.Find` over the
same `Build` and never see it, so a released tip still counts as a live
hold for them. The bug is not that lanes forgot a rule; it is that one
liveness rule lives as a consumer-side overlay instead of a shared
`Build` input, so the two walks can disagree.

**Why the leftover is invisible.** `Release` in
[internal/claim/lease.go](../../internal/claim/lease.go) mints a release
marker and deletes nothing — the `refs/heads/plan/<id>` branch and the
local worktree persist. So the lane still has a hold and a worktree.
`Find` calls it neither unstaffed (a hold with no worktree) nor stranded
(a worktree with no hold), and the worktree sits at the very lane path
the marker recorded, so it is not a foreign checkout either. It falls
through every rule.

**Why re-start collides.** `frit start` stands the lane up through
`laneStandUpPane` in [cmd/frit/start.go](../../cmd/frit/start.go), which
calls `herdr.WorktreeCreate` with no pre-check; only a genuine resume
skips it, and a released plan is not a resume. `frit pick --go` walks the
same path. So the second claim hits herdr's create error over the
directory still on disk.

**Reuse first, and where the seam goes.** `merged` and `landed` are
already git-computed above the pure `Build` — `gitobj.MergedRefs` and
`index.LandedIDs` — and handed in as data; `Build` stays git-free. The
liveness verdict is the same shape: `claim.Held` and `claim.Released`
read a ref's tip, and both fleet and the `repoLanes` helper in
[cmd/frit/main.go](../../cmd/frit/main.go) already hold the repo path and
a `gitwt.Runner` to compute it. So the seam is to make that verdict a
required `Build` input beside `merged` and `landed`, decided once by a
single `claim` classifier over the refs, and to delete the fleet-only
overlay. A separate enrichment pass is rejected deliberately: a pass a
caller must remember to run is the same forgettable overlay in another
guise. As a required input it is compile-forced on every caller, present
and future, so the asymmetry cannot return through a minimal edit. Once a
released ref is no longer a hold, the leftover worktree is stranded — a
shape `frit reap`'s stranded teardown already removes. For the reconcile
branch, `claim.checkedOut` already lists worktrees by branch through
`gitwt.List`; a start-side pre-check reuses it to adopt or reap the
leftover before creating one.

**Out of scope.** The issue notes a downstream herdr `agent_name_taken`
from a pane name that outlived its workspace. That is a herdr-side reap
gap, not frit's, and is not addressed here.

## Tasks

1. Phase 1 (proving slice): a single `claim` classifier decides a ref's
   live-hold verdict; `Build` takes it as a required input beside merged
   and landed, and the fleet-only overlay is deleted — so both walks read
   one live-hold set, a released lane's leftover worktree strands, and
   `frit orphans` names it while `frit reap` tears it down.
2. Phase 2: `pick`/`start` reconcile a pre-existing worktree for a
   now-free plan — adopt it, or reap and recreate — instead of surfacing
   herdr's `worktree_create_failed`.

## Execution

| Phase | Title                                                              | Tier   | Gate                                                                                                                                                                             |
| ----- | ------------------------------------------------------------------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The live-hold verdict is one Build input, shared by every consumer | opus   | A released-tip worktree unreported by `orphans` at HEAD is named after; `reap --go` removes it; fleet and lanes share one live-hold input, no fleet-only filter; `go test` green |
| 2     | pick and start reconcile a pre-existing worktree for a free plan   | sonnet | With a worktree already at the lane path, `start --go` adopts or recreates it instead of erroring; a fake-herdr create collision is handled                                      |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                                            |
| --- | ------ | -------------------------------------------------------------------------------- |
| 1   | ✅     | [The live-hold verdict is one Build input, shared by every consumer](phase-1.md) |
| 2   | ✅     | [pick and start reconcile a pre-existing worktree for a free plan](phase-2.md)   |
<?/catalog?>

## Acceptance Criteria

- [x] `frit orphans` names a released or taken-over lane's leftover
      worktree, and `frit reap --go` tears it down
- [x] `frit pick --go` / `frit start --go` re-selecting a freed plan
      reconcile the pre-existing worktree instead of failing on herdr's
      `worktree_create_failed`
- [x] A plan that was never released, and a released tip with no
      worktree, are unaffected — no new false orphans
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
