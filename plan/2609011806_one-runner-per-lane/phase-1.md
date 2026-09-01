---
n: 1
title: Dispatch refuses a second runner on a live lane
status: "🔲"
result: false
---
Give `pick --go` and `start --go` the pre-flight that stops a second
runner. Before a fresh acquire stands the lane up, read whether herdr
already shows a live agent on the plan's lane; if it does, refuse and
dispatch nothing. This is the load-bearing guard both verbs funnel
through `buildStart`, so one refusal covers both. Wiring the skills to
report the dispatch is Phase 2.

**Assumes.** `liveLaneFor(c, plan, rt)` in
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go) already answers "is a
live agent on one of this plan's hold branches, in this repo," matching
by branch and repository. `buildStart` in
[cmd/frit/start.go](../../cmd/frit/start.go) composes the escalation and,
under `doGo`, runs `startExecute`; a refusal is a `doc.Refuse(reason)`
already rendered in both the table and `--json`. `pick --go` retries the
next candidate only on `claim.ErrLostRace`, so a refusal here stops the
duplicate rather than skipping to another plan.

**Value.** A fresh acquire whose lane already carries a live agent no
longer dispatches a duplicate runner into it. The guard fires on a
live-but-unbound lane — the case the takeover veto misses because no
session is stamped on the lease, and `reconcileLeftoverWorktree` misses
when no worktree is registered on the branch — so the two runners never
share one worktree. A lane with no live agent still starts.

**RED.** In [cmd/frit/start_test.go](../../cmd/frit/start_test.go) (and
[cmd/frit/pick_test.go](../../cmd/frit/pick_test.go) for the pick path),
against the herdr fake the dispatch tests already script.

- `TestStartGoRefusesWhenALiveAgentAlreadyHoldsTheLane`: script herdr to
  report a live working agent on the plan's hold branch in the plan's
  repo. Run `start --go`. Assert the doc is refused with a reason naming
  the live lane, `prompt_dispatched` is false, no pane is set, and no
  `herdr agent prompt` was sent to the fake.
- `TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane`: the same, via
  `pick --go`, and assert it does not silently skip to the next
  candidate — a live lane is a refusal to surface, not a lost race to
  retry past.
- `TestStartGoStillStartsWhenNoLiveAgentOnTheLane`: no agent on the
  lane; assert the happy path is unchanged — the lane stands up and
  `prompt_dispatched` is true.

**GREEN.** In [cmd/frit/start.go](../../cmd/frit/start.go), in
`buildStart`, on the fresh-acquire branch only. Guard `resumeTip == ""`,
so a lane resuming its own token is never refused for being live — it is
that live lane. After the readiness refusals and before `startExecute`,
call `liveLaneFor`. When it finds a live agent on the plan's lane,
return a refused doc naming the lane, the shape the other pre-flight
refusals use. Read presence the way `nudge` does, so an unreachable
herdr is carried as a problem rather than read as "no live lane".

**Guard the edges.** A resume (`resumeTip != ""`) skips the guard: the
live agent it would find is itself. An unreachable herdr does not fake a
refusal — presence unknown is carried as a problem, matching `nudge`'s
own withholding, so a socket fault never blocks a legitimate start. The
guard reads the plan's own repo, so a live agent on an identically named
branch elsewhere is never mistaken for the lane.

**Gate.** With a herdr fake reporting a live agent on the plan's lane,
`frit pick --go` and `frit start --go` refuse, dispatch nothing, and
carry the refusal in `--json`; a lane with no live agent still starts;
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are green.
