---
n: 1
title: The live-hold verdict is one Build input, shared by every consumer
status: "✅"
result: true
summary: lanes.Build takes the live-hold verdict as a required third exclusion beside merged and landed; a released lane's leftover worktree now strands, frit orphans names it, and frit reap --go removes it; fleet's inline claim.Held overlay is gone.
---
## Handoff

`claim.LiveHold` in [internal/claim/lease.go](../../internal/claim/lease.go)
is the one classifier every consumer of a plan's holds now calls: it is
`Held` under the name its callers reason in (a claim or takeover with
no release since is live). `lanes.Build` in
[internal/lanes/lanes.go](../../internal/lanes/lanes.go) takes a third
exclusion, `released map[string]bool` keyed by ref name, beside
`merged` and `landed` — a required parameter, so every caller was
compile-forced to supply it. `cmd/frit/main.go`'s `repoLanes` and
`internal/fleet/gather.go`'s `heldBranches` each compute it with the
same loop: walk the refs, resolve a plan id through the repository's
own hold patterns, ask `claim.LiveHold`. Fleet's old post-`Build`
`claim.Held` overlay is deleted — the verdict now arrives with the
holds themselves, so fleet's startable set is unchanged by construction
rather than by a second filter kept in step by hand.

**reap's own gate needed the same third input.** `reapStranded`'s
`landed` closure in [cmd/frit/reap.go](../../cmd/frit/reap.go) only
read `evidence.Merged`/`evidence.ByPlanID` — exactly the two exclusions
`Build` had before this phase, so a Stranded lane was always covered by
one of them. Adding `released` as a third way to strand broke that
invariant: a released-but-never-merged, never-landed lane now reaches
`reapStranded` uncovered, and `reap.Decide` refuses anything the
caller's evidence does not confirm. `landedEvidence` gained a
`Released map[string]bool` field, filled from the same `repoLanes`
computation, and the closure now ORs it in. The delete still honors
park-before-delete, so nothing unlanded is lost by trusting a released
verdict over a landed one.

**The conflict this phase actually hit, and how it was resolved.**
`Held`'s "no reachable marker at all is not a hold" rule is deliberate
and already load-bearing for fleet (pinned by
`TestGatherLeavesAMarkerlessBranchUnheld`) and for claim/dispatch/
discovery generally (their own "not merely a name match" comments,
issue 2608212203). `lanes.Build`, before this phase, never looked at
markers at all — every name-matched branch read as a live hold
regardless. `TestOrphansReportsAClaimDoneOnlyOnItsBranch` had
incidentally relied on that gap: its fixture built a `plan/<id>-<slug>`
branch with a plan file committed but no claim marker, and asserted it
still reported "claimed, no checkout". Once the shared verdict reached
lanes, that fixture flipped to reading as released (no marker
reachable), which silently dropped the assertion's premise rather than
testing what the test was actually for — the squash-merge landed-guard.
Rather than relax `LiveHold` (which would have reopened the exact
fleet-vs-lanes asymmetry this phase closes, just moved to the
markerless case), the fixture was corrected to mint a real `plan <id>:
claim` marker before `landPlan`'s commit, the same way `claimBranch`
already does elsewhere in the same file. Its actual assertion — a plan
done only on its own branch is not read as landed — is unchanged and
still passes.

**Proven.** `TestBuildDropsAReleasedHold` (and the no-worktree/
live-hold guard variants beside it) in
[internal/lanes/lanes_test.go](../../internal/lanes/lanes_test.go) pin
the pure `Build` behavior. `TestOrphansNamesAReleasedLanesLeftoverWorktree`
in [cmd/frit/main_test.go](../../cmd/frit/main_test.go) and
`TestReapRemovesAReleasedLanesLeftoverWorktreeWithGo` in
[cmd/frit/reap_test.go](../../cmd/frit/reap_test.go) prove the CLI
gate end to end: `frit orphans` names a released lane's leftover as
"landed, still checked out", and `frit reap --go` tears the worktree
and branch down. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run ./...` are clean.

**What Phase 2 inherits.** The plan's own lane path is still recorded
on the released hold's marker until the moment `Build` drops it — read
today through `laneOf`/`WithLanePaths` while the hold is still live,
and, once dropped, the worktree itself still sits at that exact path
on disk (this phase does not move it). Phase 2's `pick`/`start`
reconciliation should check for a pre-existing worktree at the plan's
lane path directly — `claim.checkedOut`'s `gitwt.List` read is already
proven for exactly this — rather than depending on `frit reap` having
run first: the two are independent cleanups of the same underlying
gap, and phase 2 must not assume the operator ran the first one.
