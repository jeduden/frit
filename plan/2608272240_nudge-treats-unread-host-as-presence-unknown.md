---
id: 2608272240
title: nudge treats an unread host as presence unknown, not an absent lane
status: "🔲"
summary: >-
  nudge special-cases only an unreachable herdr as "presence unknown"; a
  configured host that went unread (no cache, or no cache path) leaves
  `herdrErr` nil, so nudge falls through to `nudgeSend`, finds no lane,
  and refuses with "no live lane" — asserting nobody works the plan when
  presence was never read and a lane may be live behind the gap. Route
  nudge's refusal through the shared `presenceUnknown` helper, the way
  open already does, so an unread host refuses as presence unknown.
model: sonnet
depends-on: [2608260639]
phases:
  - n: 1
    title: nudge refuses on unread presence, not an absent lane
    status: "🔲"
---
# nudge treats an unread host as presence unknown, not an absent lane

## Goal

When nudge cannot read a plan's presence, it refuses on that unread
presence, not on an absent lane. So its refusal never claims nobody
works a plan whose lane may be live behind a gap frit never saw through.

## Context

Plan [2608260639](2608260639_dispatch-report-carries-next-action.md)
added `presenceUnknown(herdrErr, hostProbs)` in
[dispatch.go](../cmd/frit/dispatch.go) and wired it into `open`, so open
withholds its `next_action` when presence was never read. `nudge` shares
`liveLaneFor` and the same `hostProbs`, but its
[Run](../cmd/frit/dispatch.go) special-cases only `herdrErr != nil` as
presence unknown. A `noPresence` host — unreachable with no cache, or the
no-cache-path degraded mode's `unreadHosts` — carries `herdrErr == nil`,
so nudge falls through to
[nudgeSend](../cmd/frit/dispatch.go), sees `found == false`, and refuses
with `no live lane for plan <id>`. That reads as "nobody is working it"
when presence was in fact never read.

Reuse: `presenceUnknown` already draws exactly this line and is the
helper open consults; the fix is to consult it in nudge too, not to add
a second predicate. `nudgeSend`'s `!found` branch stays — it is the
honest answer once presence *was* read and no lane was found. The herdr
branch keeps its own `herdr unreachable` reason; the new branch names the
unread host.

nudge stays safe either way: it already never sends into a lane it could
not confirm idle, so this corrects the refusal *reason*, not a send. A
misleading refusal is the whole defect.

## Tasks

1. Route nudge's refusal through `presenceUnknown`: an unread host
   refuses as presence unknown rather than falling through to
   `nudgeSend`'s "no live lane".

## Phase 1: nudge refuses on unread presence, not an absent lane

`nudge.Run` refuses on unread presence before it reaches `nudgeSend`. It
keeps the existing `herdrErr` branch (reason `herdr unreachable`) and
adds a branch for a `noPresence` host — `presenceUnknown(herdrErr,
hostProbs)` with `herdrErr` nil — refusing with a reason that names the
unread host rather than an absent lane. `nudgeSend` is reached only when
presence was read, so its `no live lane` answer stays honest.

RED is an integration test in
[dispatch_test.go](../cmd/frit/dispatch_test.go), beside the other nudge
cases. It drives the no-cache-path degraded mode, as
`TestFleetPresenceMarksHostsUnreadWithoutACachePath` does. Unset
`XDG_CACHE_HOME` and `HOME`, and configure `--hosts`. The local socket
carries no lane. It asserts:

- nudge refuses, and its `refused` reason names presence unknown, not
  `no live lane`.
- The unread host still travels in `problems`.
- No prompt is ever sent (the one-way door holds).

GREEN: in [dispatch.go](../cmd/frit/dispatch.go), replace nudge's
`if herdrErr != nil { … } else if nudgeSend(…)` with a switch whose
middle case is `presenceUnknown(herdrErr, hostProbs)`, refusing with a
reason that names the unread host. The herdr case is unchanged.

Gate: run the built frit — `go run ./cmd/frit nudge <id> --hosts <box>`
against a fleet whose host is unread — and confirm the refusal names
presence unknown, not `no live lane`; the unit case and `go test ./...`
and `mdsmith check .` are clean.

## Execution

One phase, a one-verb refusal correction reusing a landed helper. The
design is settled above; the phase implements from written assertions and
is guarded by an integration assertion plus a built-binary check that the
refusal reason changed.

| Phase                | Design | Implement | Gate that catches a wrong answer                                                    |
| -------------------- | ------ | --------- | ----------------------------------------------------------------------------------- |
| 1 nudge unread refus | opus   | sonnet    | an unread host refuses "presence unknown", not "no live lane"; herdr case unchanged |

## Acceptance Criteria

- [ ] nudge refuses with a presence-unknown reason when a configured host
      went unread and `herdrErr` is nil
- [ ] The unread host still travels in the document's `problems`
- [ ] nudge sends no prompt when presence is unknown
- [ ] An unreachable herdr still refuses with `herdr unreachable`
- [ ] `nudgeSend`'s `no live lane` refusal is unchanged for a read-but-
      laneless plan
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
