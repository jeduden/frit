---
n: 1
title: Dispatch refuses a second runner on a live lane
status: "✅"
result: true
summary: >-
  pick --go and start --go now refuse a fresh acquire whose plan's own
  hold branch already carries a live herdr agent, dispatching nothing;
  a clear lane still starts unchanged.
---
## Handoff

The pre-flight sits in `buildStart` (`cmd/frit/start.go`), reached
through the new `startLiveLaneRefusal` helper: on the fresh-acquire
branch only (`rs.active()` false, i.e. `resumeTip == ""`), it calls the
existing `liveLaneFor` join — the same one `open` and `nudge` already
dispatch against — and refuses with the pane and branch named when it
finds a live agent already sitting on one of the plan's hold branches,
in the plan's own repository. A resume skips the guard outright: the
live agent it would find is itself.

The gap this closes is the one plan 2609011611's own token-based resume
still leaves open from *outside* the lane: a matured, session-less
lease is fair game for a takeover, and neither the session veto (no
session was ever bound) nor `reconcileLeftoverWorktree` (no worktree is
registered on the branch in *this* repository — the live pane sits in
its own clone or, in the field, another host) sees the agent already
working it. `liveLaneFor` does, because it reads presence and joins it
to the plan's hold branches directly, not through a lease's own
bookkeeping.

An unreachable herdr, or a host `fleetPresence` could not read, never
blocks a legitimate start on that account alone — the opposite of
`nudge`'s own withholding, and deliberately so: nudge's whole job is
sending text into a lane it must first prove is live, while start's
other guards (the claim CAS, the session veto) still stand behind a
presence read that came back empty-handed. Those reads still travel as
problems on whichever doc results — the refusal or the eventual
success — through `carryLiveLaneProblems`, so a socket fault is visible
without being fatal.

`pick --go` reaches the same refusal through `buildStart` and reports
it rather than treating it as `claim.ErrLostRace` and walking to the
next candidate — a live lane is a fact to surface, not a race to skip
past.

Phase 2 inherits: the shape of the refusal is exactly the other
pre-flight refusals `buildStart` already renders — `Refused` carries
the reason, `PromptDispatched` and `Handoff` stay at their zero values
(`false` / `HandoffNone`) — nothing new for the skills to key on beyond
what `prompt_dispatched: true` already means on the success path.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are clean.
