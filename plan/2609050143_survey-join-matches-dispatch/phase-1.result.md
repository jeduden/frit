---
n: 1
title: the survey keys live lanes by repository
status: "✅"
result: true
summary: >-
  liveByBranch now keys every staffed lane by `repoBranch{repo, branch}`
  rather than branch alone, through a new `laneRepo` helper in
  `cmd/frit/main.go` that `liveLaneFor` in `cmd/frit/dispatch.go` now
  calls too. Two repositories holding the same plan id on the same
  branch name no longer collide in board, ready, pick or find.
---
## Handoff

**The helper, and where it lives.** `laneRepo(lane herdr.Lane, git
gitwt.Runner) string` in
[cmd/frit/main.go](../../cmd/frit/main.go), beside `liveByBranch`:

```go
func laneRepo(lane herdr.Lane, git gitwt.Runner) string {
	return fleet.RepoName(lane.Root, gitForHost(git)(lane.Pane.Host))
}
```

It is the exact expression `liveLaneFor` already had inline —
`fleet.RepoName` over the lane's worktree root, through the host's own
git — lifted so both joins call the one copy. `liveLaneFor` now reads
`laneRepo(lane, rt.git) == p.Repo` where it used to spell out
`fleet.RepoName(...)` itself; its own tests pass unedited.

**The map's key shape.** A new unexported type,
`repoBranch{repo, branch string}`, replaces the bare `string` key in
`liveByBranch`'s returned map and in `laneFor`'s parameter. `laneFor`
looks up `repoBranch{repo: p.Repo, branch: branch}` for each of a
plan's `Holds` — the plan already carries its own repository, so no
new parameter was needed, just the richer key. `agentFor` and
`presenceFor` take the same map type; every call site (`board`,
`ready`, `pick`, `find`) reaches them only through `liveByBranch` and
`laneFor`, so no caller changed shape.

**The fixture Phase 2 should reuse.** `sharedPlanRepos(t, root)` in
[cmd/frit/discovery_test.go](../../cmd/frit/discovery_test.go): two
repositories, `atlas` and `orrery`, each holding plan 7 on `plan/7`
via a real `claim.Acquire` with `Session: "wOld:p1"` — a session no
fake herdr pane will ever answer to, so both read `Dead` on their own
before any live pane is joined in. Phase 2's "a host with `noPresence`
and a live pane elsewhere" is this same two-repository shape read
through a fleet with one host unread rather than two repos compared
directly — swap one repo's claim for a configured, unread host and the
comparison is the same: which one the survey correctly withholds the
ask from.

**RED → GREEN.** `TestBoardKeysLiveLaneByRepositoryAndBranch` and
`TestReadyKeysLiveLaneByRepositoryAndBranch` build `sharedPlanRepos`
plus a live pane in a linked worktree under `orrery` — its directory
deliberately not named `orrery`, to prove the join resolves through
`fleet.RepoName`'s worktree-list read rather than the lane's own
basename. Before the fix both failed to build (`repoBranch` did not
exist); after, `orrery`'s row clears `dead` and carries `agent`/`ask`,
`atlas`'s stays `dead: true` with both empty.
`TestPresenceForMissesASameNamedBranchInAnotherRepo` pins the same
guard at the unit level, alongside the two existing `presenceFor`
tests updated to the new keyed map.

**Gate.** `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are both clean. `who` is untouched — it never
called `liveByBranch` or `laneFor`.

**Addendum, post-landing.** `/code-review` found two real gaps in this
phase's own diff, both fixed: `laneRepo` had no dedicated unit test of
its own (only indirect coverage through `liveByBranch`/`liveLaneFor`'s
integration tests) — `TestLaneRepoResolvesThroughTheMainWorktreeList`
and `TestLaneRepoFallsBackToTheRootsBasenameWhenGitCannotAnswer` pin it
directly now. And `liveByBranch` ran an uncached `git worktree list`
(an ssh round trip for a remote pane) per staffed lane on every
board/ready/pick/find call; it now resolves each distinct
`(host, root)` pair once per call and shares the answer across every
pane in the same lane, pinned by
`TestLiveByBranchResolvesEachWorktreeRootOnlyOnce`. `liveLaneFor` was
already at most one call per plan and needed no change. The review
also flagged `boardPlanByID`/`boardPlanByRepo`/`planCardByRepo`'s
identical find-loops in test code; folded onto one generic `findFirst`
helper in `main_test.go`.

**Left for a later plan, not this phase.** The review's other two
findings are real but out of this phase's boundary: `herdr.Lane.Repo`
(set in `herdr.Join` as the linked worktree's own basename) is the
same wrong-repo-name bug this phase fixed for the survey, still read
directly by `who`'s sort, print and `--json` output — `who` was
deliberately left alone here, per the plan's own scope. And
`fleet.RepoName`'s fallback to the root's basename on an unreadable
worktree list can, in principle, silently reintroduce a same-named-
branch collision or read a live lane as unknown; surfacing that
properly would mean threading a new state through the join, a design
change rather than a local fix.
