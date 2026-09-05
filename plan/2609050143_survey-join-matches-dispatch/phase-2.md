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
`presenceUnknown(err, hostProbs)` exactly as `open` does. Add unit
tests, leading with the observed case:

- A held plan whose bound session is confirmed gone, one configured
  host (`--hosts`) that answered with neither a live read nor a cache,
  and a *different* host's live pane herdr did show: `board --json`'s
  row for that plan clears `dead`, names the agent, but carries
  `ask: ""`; the unread host still rides in `problems[]`.
- The same fixture through `ready --json` (and via `cardsOf`, pick and
  find): the card's `dead` clears, `ask` stays `""`.
- A host served from stale cache (present in `problems[]` as a
  reachability warning, `noPresence` false) does not withhold the ask
  — `presenceUnknown` already draws this line; the survey must not
  redraw it.

Each fails today: `liveByBranch`'s middle return is a bare `bool`, and
`agentFor`/`presenceFor` have no way to know presence was incomplete,
so every row offers its ask exactly as if every host had answered.
Commit the red.

**GREEN.** Change `agentFor` and `presenceFor` to take one more
argument: the `unknown bool` `board`/`ready`/`pick`/`find` each
compute once via `presenceUnknown`. When a lane has a live pane (a
non-empty status) and `unknown` is true, downgrade the returned status
to `herdr.StatusUnknown`. `askOf`'s existing `askable` gate already
refuses that status, so `Ask` comes back `""`. Neither `askOf`,
`cardsOf`, nor `BoardDoc.AddPlan` changes. A non-empty status, unknown
included, still clears `Dead` — a pane being there is not in question,
only what it is doing is. This is the seam the plan's own Context
names: no new field on `PlanCard` or `BoardPlan`, only the string the
callback answers.

Wire `board`, `ready`, `pick` and `find`'s `Run` methods. Read
`liveByBranch`'s three return values. Compute
`unknown := presenceUnknown(liveErr, hostProbs)` once. Pass it into
every `agentFor`/`presenceFor` call for that command. `board`'s
`doc.Presence` keeps its exact meaning (`liveErr == nil`) — do not
fold it into `unknown`, which also covers a stale-cache-only gap
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
