---
n: 1
title: open names the honest next step per held-lane kind
status: "✅"
result: true
summary: >-
  `frit open` now reads a held plan's own kind — a token this machine
  holds unattended, one it cannot prove, or one a live agent already
  attends — and names the true next step for each: a resume, the
  takeover window's wait, or the live agent, never a bare `frit start`
  that would refuse. A plan with no hold at all is unchanged.
---
## Handoff

`openNextAction` in
[internal/report/dispatch.go](../../internal/report/dispatch.go) now
takes a `report.HoldKind` alongside `focused` and `presenceUnknown`,
and `OpenDoc` carries it as `HoldKind` (`hold_kind` on the wire),
refreshed by the same `refreshNextAction` that already governs
`NextAction` — a new `SetHoldKind` joins `Focus` and `PresenceUnknown`
as its writers. The four values: `HoldNone` (no hold, the zero value),
`HoldResumable`, `HoldUnproven`, `HoldLive`. `HoldResumable` still
projects the bare `frit start <id>` — it is genuinely the same verb,
honest now that plan 2609011836 landed the resume — while `HoldUnproven`
and `HoldLive` project full sentences rather than a runnable verb, since
neither has one to offer.

The read that feeds it lives in `cmd/frit/dispatch.go`'s new
`holdKindFor`, called from `open`'s `Run` only when `liveLaneFor` found
nothing and the plan reads `Held`. It reuses the exact reads plan
2609011836 already runs for `start`'s own resume: `heldLaneMarker` (a
new function factored out of `laneTokenResumeTip` in `cmd/frit/start.go`,
which now calls it too) reads the marker and origin's tip, `tokenProves`
answers whether the persisted token still proves the lease, and
`laneUnattended` answers whether an agent — bound session or a pane
sitting in the checkout — is on it. A repository coordinate that could
not be resolved, or a marker that could not be read or proved, both
fall to `HoldUnproven` — the safe generic wording — rather than ever
guessing `HoldResumable`.

**What a later phase inherits.** `holdKindFor` and `heldLaneMarker` are
the seam: `start`'s refusal and `nudge`'s "no live lane" both want the
same kind read from the same plan, and should call `holdKindFor` rather
than re-derive it. The wording chosen here — "resume it with `frit
start <id>`", "wait for the takeover window, or take it over once it
matures with `frit start <id>`", "a live agent is already on this
lane" — is `open`'s own phrasing in `printOpenNextStep`
(`cmd/frit/dispatch.go`); a later phase should keep `start`'s and
`nudge`'s wording recognizably the same family, not invent a second
vocabulary for the same three kinds. The `lease-protocol.md` S76 update
this plan's Context calls for is **not done** in this phase — S76 says
`open` *and* `start` name the resume, and `start`'s own wording is
still the old blanket "not takeable until the window matures"; the doc
update belongs with that phase, not this one.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are green.
