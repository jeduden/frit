---
n: 2
title: the survey withholds the ask on unread presence
status: "✅"
result: true
summary: >-
  askOf takes a second input, unknown, alongside status: cardsOf and
  BoardDoc.AddPlan thread it through untouched, so an incomplete
  presence read withholds Ask without rewriting Dead-clearing or
  agent_status. board, ready, pick and find each compute it once via
  presenceUnknown off liveByBranch's now-honest error return, and
  ready/pick/find carry that error into problems[] the way board's own
  Presence field already implied. Plan 2609050143 is done.
---
## Handoff

**The design, corrected mid-phase.** The first attempt withheld the
ask by having `agentFor`/`presenceFor` answer `herdr.StatusUnknown`
when presence was incomplete. That worked for `Ask`, but `board` also
renders that same string as `agent_status`, so a pane herdr confirmed
working would render as unknown — a misreport, not a withheld ask. An
external review of this plan's own spec caught it before it landed.
The corrected shape: `agentFor`/`presenceFor` never change, always
reporting the pane's real status. `askOf(p, status string, unknown
bool)` in `internal/report/discovery.go` refuses on `unknown` before
ever consulting `askable(status)`. `cardsOf` takes the same bool and
threads it through untouched; `ReadyDoc.SetPlans`, `PickDoc.SetPlans`
and `FindDoc.SetPlans` (`internal/report/discovery.go`) take it and
forward it. `BoardDoc.AddPlan` (`internal/report/board.go`) takes it
too, leaving `AgentStatus: status` exactly as before.

**The join side.** `liveByBranch` (`cmd/frit/main.go`) now returns
`(map[repoBranch]herdr.Lane, []hostProblem, error)` — the third value
is `fleetPresence`'s own error, handed straight back rather than
swallowed, mirroring `liveLaneFor`'s shape. `board`, `ready`, `pick`
and `find`'s `Run` methods each compute
`unknown := presenceUnknown(liveErr, hostProbs)` once and pass it to
every `AddPlan`/`SetPlans` call. `board`'s `doc.Presence` keeps its
exact meaning (`liveErr == nil`), independent of `unknown`. `ready`,
`pick` and `find` had no channel for an unreachable local herdr at
all before this phase — they now call `doc.AddProblem("herdr",
liveErr)` when it is non-nil, the same fact `board` already implied
through `Presence` and `open`/`nudge`/`message` already give through
`presenceUnknown`.

**Why no full end-to-end fixture for the ask-withheld case.** A held
plan needs `Dead` or `Stale` to be a `ready`/`pick`/`find` candidate at
all (`discovery.Ready`'s `candidate`), and both are set by
`observeHolds` off `observe.Path()`, which — like
`presence.CachePath()` — resolves through `os.UserCacheDir()`. The
existing convention for "a configured host went unread with no cache"
(`TestNudgeSaysPresenceUnknownWhenAHostIsUnread` and its siblings)
breaks that resolution on purpose, by unsetting `XDG_CACHE_HOME` and
`HOME`. Doing that also blanks every held plan's `Dead`, so no plan a
real fleet built under that fixture can ever carry an ask to withhold
— the two full-command tests first written for this phase (combining
`claim.Acquire`'s session-death with a broken cache path) passed, but
for the wrong reason: `Dead` was false regardless of the fix. They are
gone. The ask-withheld-without-rewritten-status behavior is pinned
directly against the report package instead
(`TestBoardAddPlanWithholdsAskOnIncompletePresenceWithoutRewritingStatus`,
`TestReadySetPlansWithholdsAskOnIncompletePresence`), where a plan's
`Dead` is set on the struct literal, no fleet involved.
`TestReadyCarriesAnUnreachableHerdr` is the one half of this phase
that a full command test can prove end to end — a local herdr the
socket read itself fails on, no `Dead` needed — and stands in for
`pick`/`find` too, since all three wire the identical three lines.

**Gate.** `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are both clean. `open`, `nudge` and `message`'s own
tests pass unedited — none of them call `liveByBranch`.

**Plan 2609050143 is done.** Both phases landed; every Acceptance
Criterion is checked. One gap the plan's own Context names as
deliberately out of scope: `AskCommand` composes a bare plan id, and
`Resolve` refuses an id two repositories share as ambiguous, so the
live row's own printed ask can still refuse when run outside that
lane's checkout in the plan's own two-repos-same-id fixture. A
repository-qualified ask selector is a plan of its own.
