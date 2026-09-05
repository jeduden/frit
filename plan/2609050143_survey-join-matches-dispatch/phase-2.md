---
n: 2
title: the survey withholds the ask on unread presence
status: "🔲"
result: false
---
Make the survey read presence-completeness the way `open`, `nudge`
and `message` already do. `presenceUnknown(herdrErr, hostProbs)`
(in [cmd/frit/dispatch.go](../../cmd/frit/dispatch.go)) is the pinned
rule. An unreachable herdr, or a configured host that answered with
neither a live read nor a cache, leaves a lane possible behind the
gap. Those verbs refuse rather than act on it. `board`, `ready`,
`pick` and `find` must offer no `ask` in that same state.
Dead-clearing is unaffected: a pane herdr did show still disproves
"nobody is here", whether or not some other host went unread.

**Assumes.** Phase 1 landed: `liveByBranch`
([cmd/frit/main.go](../../cmd/frit/main.go)) keys live lanes by
`(repo, branch)`, and `laneFor`/`agentFor`/`presenceFor` read that map.

**RED.** `liveByBranch` today discards `fleetPresence`'s error. It
returns `(nil, false, nil)` on failure and never hands the error back,
so no caller can feed it to `presenceUnknown`. Change its signature to
`(map[repoBranch]herdr.Lane, []hostProblem, error)`, mirroring
`liveLaneFor`'s own shape. Then `board`/`ready`/`pick`/`find` can call
`presenceUnknown(err, hostProbs)` exactly as `open` does.

**Two joins, two callers of `askOf`, one gate.** `ready`, `pick` and
`find` hand [`cardsOf`](../../internal/report/discovery.go) a
`presence` callback answered by `presenceFor`. `board` hands
`BoardDoc.AddPlan` the agent and status `agentFor` reads. Both derive
`Dead`-clearing and `Ask` from that one status string. `board` also
renders it verbatim as `agent_status`. A design that withholds the ask
by having `agentFor`/`presenceFor` answer `herdr.StatusUnknown` when
presence is incomplete works for `Ask`. But it also rewrites
`agent_status` — a pane herdr confirmed working would render as
unknown, misreporting a fact the survey actually has, purely as a side
effect of gating a different field. The correct shape adds a second
input beside the status: `askOf(p, status, unknown bool)`, `cardsOf`
and `BoardDoc.AddPlan` taking the same bool and passing it through.
`agentFor`/`presenceFor` never change; they keep reporting the pane's
real status always.

Add unit tests, leading with the observed case:

- `askOf`/`cardsOf`/`BoardDoc.AddPlan`, given `unknown: true` on an
  otherwise-attended dead-session plan: `Ask` is `""`, `Dead` still
  clears, and (for `BoardDoc.AddPlan`) `AgentStatus` still reads the
  pane's real status, not `unknown`. Construct the plan directly
  (`discovery.Plan{Held: true, Dead: true, ...}`), not through a real
  fleet: a held plan needs `Dead` or `Stale` to be a `ready`/`pick`/
  `find` candidate at all, and both are computed by `observeHolds` off
  the same `os.UserCacheDir()` a broken-presence-cache fixture would
  also have to break, so no plan a real "host went unread" fixture can
  build ever carries an ask to withhold in the first place. Pin the
  behavior at the report package instead, where a plan's `Dead` needs
  no fleet at all.
- `liveByBranch` hands `fleetPresence`'s error straight back instead of
  swallowing it (a herdr it cannot reach yields `(nil, nil, err)`, not
  `(nil, false, nil)`).
- A full command test that is testable end to end: with the local
  herdr unreachable, `ready` (and, by the same three lines, `pick` and
  `find`) now carries `"herdr"` in `problems[]`, matching what `board`
  already implies through `Presence` and what `open`/`nudge`/`message`
  already give through `presenceUnknown`.

Each fails today: `liveByBranch`'s middle return is a bare `bool` with
no error to hand back; `askOf` takes no second input, so `ready`,
`pick` and `find` never learn presence was incomplete; and the
unreachable-herdr problem never reaches `ready`/`pick`/`find`'s
`problems[]` at all. Commit the red.

**GREEN.** `askOf(p discovery.Plan, status string, unknown bool) string`
refuses on `unknown` before ever consulting `askable(status)`.
`cardsOf` takes the same `unknown bool` and threads it to `askOf`,
touching nothing else. `ReadyDoc.SetPlans`, `PickDoc.SetPlans` and
`FindDoc.SetPlans` take it too, forwarding it to `cardsOf`.
`BoardDoc.AddPlan(p, agent, status string, unknown bool)` threads it to
its own `askOf` call, leaving `AgentStatus: status` exactly as before.
Wire `board`, `ready`, `pick` and `find`'s `Run` methods. Read
`liveByBranch`'s three return values. Compute
`unknown := presenceUnknown(liveErr, hostProbs)` once. Carry
`liveErr` into `problems[]` via `doc.AddProblem("herdr", liveErr)` when
non-nil — `ready`, `pick` and `find` have no `Presence`-style field of
their own, so this is the only channel they have for it. `board`'s
`doc.Presence` keeps its exact meaning (`liveErr == nil`); it is not
folded into `unknown`, which also covers a stale-cache-only gap
`doc.Presence` was never about.

**Guard the edges.** A fleet with no `--hosts` configured is
unaffected: `hostProbs` is empty, `unknown` is `false` exactly when
`liveErr` is nil, matching today's behavior byte for byte. A host
serving stale cache carries a reachability problem but never
`noPresence: true`, so `unknown` stays false and the ask still shows.
`open`, `nudge` and `message` are untouched — they call
`presenceUnknown` directly off `liveLaneFor`'s own return, never
through `liveByBranch`.

**Gate.** The new tests pass; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-2.result.md`, ticking every remaining
Acceptance Criterion this closes the plan on.
