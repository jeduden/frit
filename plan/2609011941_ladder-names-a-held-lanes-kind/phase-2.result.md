---
n: 2
title: start's refusal names a live agent, not a blanket un-matured takeover
status: "✅"
result: true
summary: >-
  `frit start`'s refusal now names a live agent attending a held lane —
  "already held (...); a live agent is on this lane; nudge or open it
  instead of starting" — rather than the blanket un-matured-takeover
  wording, which wrongly suggested waiting would free it. A hold this
  machine cannot prove keeps its existing, already-honest wording
  unchanged. lease-protocol.md's S76 narrative now records that both
  `open` and `start` name the ladder's own rungs.
---
## Handoff

`liveHoldRefusal` in [cmd/frit/start.go](../../cmd/frit/start.go) calls
`holdKindFor` — the same read phase 1 built for `open` — from
`startRefusal`, ahead of the generic `claimRefusal` fallback. It fires
only for `report.HoldLive`; every other kind, `HoldUnproven` included,
falls through untouched, so `claimRefusal`'s "not takeable until the
window matures" wording stays exactly as it was for a hold this machine
cannot prove — that wording was already honest, and this phase does not
touch it or `claimRefusal` itself, which stays shared with the plain
`frit claim` verb. `startRefusal` and `buildStart` now thread `coordOK`
through, the same value `open` already computes, so `holdKindFor` reads
the ambiguous-repository case identically in both places (it falls back
to `HoldUnproven`, never guessing `HoldLive`).

**What a later phase inherits.** `nudge`'s own refusal — Task 3, not
started — is the last rung: today it always refuses with a bare
`"no live lane for plan %d"`, whether nothing holds the plan or a
healthy-but-unattended lane sits there with its agent gone. It should
call `holdKindFor` the same way `open` and `start` now do, and tell a
lane with no hold at all apart from one that is held but has no live
pane to prompt — reusing this phase's and phase 1's wording family
rather than inventing a third one. `lease-protocol.md`'s S76 entry is
now fully closed on the documentation side: both rungs' role in
resolving it are recorded, so no further doc phase is expected once
`nudge` lands, beyond folding `nudge` itself into the same narrative
paragraph if it reads as a fourth thing worth naming there.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.

**Post-landing correction.** Code review caught that `startRefusal` is
shared with `pick --go` (`buildStart`'s `reattach` false there), and
`liveHoldRefusal` did not gate on that: a held plan gone stale by its
clock while its agent is still genuinely attending it is a legitimate
takeover candidate through `Ready` (S76), and the new refusal fired for
it too — stopping `pick --go`'s walk on a static refusal instead of
losing the race at `mintOrTakeOver`'s own live-session veto and
advancing to the next candidate, same as any other contested claim.
Fixed by gating the `liveHoldRefusal` call on `startRefusal`'s new
`reattach` parameter (`buildStart`'s own flag, not `rs.Reattach`), so
only an explicit `start <id>` gets the friendlier wording; pinned by
`TestPickGoAdvancesPastALiveHold` in
[cmd/frit/pick_test.go](../../cmd/frit/pick_test.go).

**Post-landing correction: an unread herdr is not a confirmed live
agent.** Code review also caught that `laneUnattended`
([cmd/frit/start.go](../../cmd/frit/start.go)) collapses two different
facts into one `false`: a positively confirmed agent, and a herdr frit
simply could not reach. `laneTokenResumeTip` was always safe folding
both into "do not resume," but `holdKindFor`
([cmd/frit/dispatch.go](../../cmd/frit/dispatch.go)) read that same
`false` as `report.HoldLive` — so a plain socket outage made both
`open` and `start` assert "a live agent is on this lane" for a hold
frit never actually observed as attended. `laneUnattended` now returns
`(unattended, known bool)`; `holdKindFor` reads `HoldUnproven` when
`known` is false, never guessing `HoldLive`. Pinned by
`TestOpenReportsNoHoldKindWhenHerdrIsUnreachable` in
[cmd/frit/dispatch_test.go](../../cmd/frit/dispatch_test.go) and a
strengthened `TestStartWithUnconfirmedLivenessDoesNotResume` in
[cmd/frit/start_test.go](../../cmd/frit/start_test.go), which now pins
the refusal's exact wording rather than only its absence of "resumed".

**Post-landing correction: a resumable hold can still carry unparked
local work.** Code review also found that `holdKindFor` read
`HoldResumable` — "resume it with `frit start <id>`" — for a lane whose
token this machine holds and no agent attends, without checking
whether that lane's local branch carries commits past the token it
never pushed. `start`'s own S77 park-first guard
(`reattachParkFirstRefusal`) already refuses exactly that case, so
`open` was recommending a `frit start` that `start` would then refuse
— the very defect this plan exists to close, reappearing in a case
neither phase's own tests covered. A fourth kind, `report.HoldUnparked`
([internal/report/dispatch.go](../../internal/report/dispatch.go)),
now covers it: `holdKindFor` checks `unparkedSuffix` once a lane
otherwise reads resumable, and `openNextAction` names `frit yield <id>`
rather than a `frit start` that would refuse. `start`'s own refusal
needed no change — `reattachParkFirstRefusal` already names this case
correctly, and `liveHoldRefusal` only ever gates on `HoldLive`. Pinned
by `TestOpenNamesTheParkFirstStepForAnUnparkedResumableHold` in
[cmd/frit/dispatch_test.go](../../cmd/frit/dispatch_test.go) and
`TestOpenNextActionNamesTheParkFirstStepForAnUnparkedHold` in
[internal/report/dispatch_test.go](../../internal/report/dispatch_test.go).

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean after both corrections.
