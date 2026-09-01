---
n: 2
title: start's refusal names a live agent, not a blanket un-matured takeover
status: "✅"
result: false
---
Climb to the second rung. A held lane's refusal from `start` reads the
same "already held … not takeable until the window matures" whether
the hold is one this machine cannot prove — where waiting really is the
honest next step — or one a live agent is actively attending, where
waiting will not free it and the operator should reach for `nudge` or
`open` instead. Teach the refusal the one kind that changes the honest
answer.

**Assumes.** Phase 1 built `holdKindFor` in
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go). It reads a held
plan's kind off the marker, its persisted token and `laneUnattended`:
`report.HoldResumable`, `report.HoldUnproven`, `report.HoldLive`. A
resumable hold no longer reaches a refusal at all — `startResume` in
[cmd/frit/start.go](../../cmd/frit/start.go) already resumes it (plan
2609011836). So `startRefusal` only ever sees `HoldUnproven` or
`HoldLive` for a held plan. `HoldUnproven`'s existing wording is
already honest for that kind and stays untouched: `claimRefusal` in
[cmd/frit/claim.go](../../cmd/frit/claim.go), via `notMaturedReason` in
[cmd/frit/main.go](../../cmd/frit/main.go), is shared with the plain
`frit claim` verb, which never resumes and has no live-agent shortcut
of its own — not the seam to change. `HoldLive` is the one gap.
`startRefusal` falls through `desertedRefusal` and `parkFirstRefusal` —
both gated on `plan.Dead`, which a live hold never sets — straight into
`claimRefusal`'s blanket wording today.

**Value.** An operator staring at a refusal can tell, from the message
alone, whether to wait or to look for the agent already on the lane —
`nudge` it, or `open` it to watch. `TestStartStillVetoesALaneWithALiveAgent`
already pins that the refusal stands and never resumes; this phase
narrows what changes to the wording alone.

**RED.** In
[cmd/frit/start_test.go](../../cmd/frit/start_test.go).

- `TestStartRefusalNamesALiveAgentInsteadOfTheWindow`: a held lane whose
  token this machine holds, with a live agent bound to the marker's own
  session (the same fixture `TestStartStillVetoesALaneWithALiveAgent`
  builds). Assert the refusal names the live agent and does not carry
  the un-matured-takeover wording.
- `TestStartRefusalStillNamesTheWindowForAnUnprovableHold`: a held lane
  with no token on disk here (the existing
  `TestStartDoesNotResumeALaneWhoseTokenIsGone` fixture). Assert the
  refusal is unchanged — "not takeable until the window matures" still
  stands, since waiting is the honest answer there.

**GREEN.** A new `liveHoldRefusal` in
[cmd/frit/start.go](../../cmd/frit/start.go), called from
`startRefusal` ahead of `claimRefusal`: for a held plan whose
`holdKindFor` reads `HoldLive`, it names the live agent; every other
kind returns `""` and `claimRefusal` stays the arbiter, unchanged.
`startRefusal` and `buildStart` thread `coordOK` through so
`holdKindFor` can be called the same way `open` calls it.

**Guard the edges.** An ambiguous repository (`coordOK` false) never
claims `HoldLive` — `holdKindFor` already falls back to `HoldUnproven`
there — so the ambiguous-repo refusal `buildStart` gives afterward
stays the arbiter, unchanged. A reattach's own park-first refusal
(`rs.Reattach`) returns before this ever runs, unaffected. `frit claim`
and `claimRefusal`'s own wording are untouched — this phase changes
only the seam `start` reaches through `startRefusal`.

**Docs.** [lease-protocol.md](../../docs/research/lease-protocol.md)'s
S76 entry and its narrative paragraph are updated to record that `open`
(phase 1) and `start` (this phase) now name the hold's own kind, so a
stuck operator reads it at the rung they are standing on rather than
only from `orphans`.

**Gate.** A held lane a live agent attends refuses naming the agent,
never the takeover-window wording; a held lane this machine cannot
prove still names the window, unchanged. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green; `mdsmith
check .` is clean.
