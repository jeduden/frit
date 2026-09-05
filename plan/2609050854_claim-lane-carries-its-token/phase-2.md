---
n: 2
title: Refusals from a token-less lane name the way out
status: "✅"
result: false
---
Make `release` and `start` say the honest next step for a hold this
machine cannot prove. Reuse the wait-or-take-over wording `open`
already carries in `next_action` for `HoldUnproven` (#122). Feed it
onto both documents rather than inventing it twice.

**Assumes.** `foreignHoldRefusal` in
[cmd/frit/release.go](../../cmd/frit/release.go) words every hold
`ownToken` could not prove as "held live by another lane". That is
wrong when `inOwnLane` (claim.go) says the calling directory is this
exact plan's own lane — the S49 shape: a checkout that never carried a
token, or whose write never landed. `ownToken` cannot tell that apart
from a genuine foreign move on its own — S86's own-advance case, or a
real takeover after this lane's last renewal both read as its `!ok`
too.

`startRefusal` in [cmd/frit/start.go](../../cmd/frit/start.go) meets
the same shape too. It falls into `claimRefusal`'s own wording. That
wording is generic: "already held ... not takeable until the window
matures" (main.go's `notMaturedReason`).

The row is `@S49`. It lives in
[features/identity.feature](../../features/identity.feature).

The unit test is `TestStartDoesNotResumeALaneWhoseTokenIsGone`. It
lives in start_test.go.

`holdKindFor` (cmd/frit/dispatch.go) already classes this hold as
`HoldUnproven`, for `open`. `start` never asks it.

`StartDoc` and `ReleaseDoc` in
[internal/report/dispatch.go](../../internal/report/dispatch.go) carry
no `next_action` on a refusal at all. `startNextAction` only fires for
`HandoffRunning`, and `ReleaseDoc` has no such field yet.

**Value.** An agent meeting either refusal branches on one field
instead of parsing a sentence for whether waiting helps. A person
reads the same sentence `open` already gives the same hold. The
existing wordings stay put — "not takeable until the window matures",
"held live by another lane" for a genuinely foreign hold. Only the
token-less own-lane shape gains a next step; today it has none.

**RED.** In
[internal/report/dispatch_test.go](../../internal/report/dispatch_test.go):

- `TestStartNextActionIsAPureProjectionOfHandoff` gains a third
  `HoldKind` argument to `startNextAction`; update its call sites.
- `TestStartNextActionNamesTheWaitForAnUnprovenRefusal`:
  `SetHoldKind(HoldUnproven)` on a refused `StartDoc` sets `NextAction`
  to `openNextAction`'s own `HoldUnproven` wording. `SetHoldKind
  (HoldLive)` leaves it empty — that kind carries no wording here.
- `TestReleaseUnprovenNamesTheWaitForATokenlessOwnLane`:
  `ReleaseDoc.RefuseUnproven` sets both `Refused` and `NextAction` in
  one call, `NextAction` matching the same `HoldUnproven` wording.

In [cmd/frit/release_test.go](../../cmd/frit/release_test.go):

- `TestReleaseNamesTheWayOutForATokenlessOwnLane`: from inside a lane
  this host claimed and stood up, its token dropped, `release` refuses
  without "held live". The JSON document's `next_action` names the
  takeover window.

In [cmd/frit/start_test.go](../../cmd/frit/start_test.go):

- `TestStartRefusalNamesTheWayOutForAnUnprovableHold`: the same fixture
  `TestStartDoesNotResumeALaneWhoseTokenIsGone` builds, read as JSON.
  `next_action` names the takeover window and `frit start 7`,
  non-empty exactly where the existing "not takeable" wording already
  sits.

**GREEN.** In internal/report/dispatch.go: factor `openNextAction`'s
`HoldUnproven` case into a shared helper. Call it
`unprovenNextAction(id int64) string`. Three call sites use it.

Give `StartDoc` an unexported `holdKind HoldKind` field. Keep it off
the wire, like `OpenDoc.presenceUnknown`. Add a `SetHoldKind` setter
that reprojects `NextAction`. `startNextAction` takes `kind` alongside
`handoff` now. `HandoffRunning` is unchanged. `HandoffNone` with
`kind == HoldUnproven` returns `unprovenNextAction`. Every other case
stays empty.

Give `ReleaseDoc` an exported `NextAction` field. Add
`RefuseUnproven(reason string, id int64)`: it calls `Refuse`, then
sets `NextAction` from the same helper.

In cmd/frit/claim.go, add `tokenlessOwnLane(rt, plan, cwd) bool`
beside `ownToken`/`inOwnLane`. It is true when `inOwnLane` holds and
the lane's `claim.ReadToken` reads empty. That is the one `ownToken`
failure mode genuinely this lane's own. A token that exists but no
longer proves the tip is a different case, and must keep reading as
foreign.

In cmd/frit/release.go's `releaseHeld`, when `ownToken` fails: keep
routing `plan.Stale`/`plan.Dead` to `foreignHoldRefusal`, unchanged. A
matured or confirmed-dead hold is takeable regardless of whose lane
this is. Otherwise call `tokenlessOwnLane`. When true, refuse through
the new `RefuseUnproven`. Word it as this lane's own unproven
checkout, not "held live by another lane". Otherwise keep
`foreignHoldRefusal`, as today.

In cmd/frit/start.go's `startRefusal`, where `claimRefusal` produces
the generic "already held" reason: call `holdKindFor` when `plan.Held`
holds, and hand the result to the refused doc's `SetHoldKind`. `open`
already runs this same read for the identical hold. A live or
unparked kind is already caught earlier — by `liveHoldRefusal`, or the
park-first guards, when reattached — so it contributes no wording
here. Neither does anything else that is not `HoldUnproven`.

Print the field where a person reads it. `printRelease` and
`printStart` gain a line for `doc.NextAction` on a refusal, beside the
existing `Warning` line.

**Guard the edges.** A token that exists but fails `tokenProves` must
keep reading as foreign. That is a real takeover after this lane's
own renewal — S86's negative case
(`TestReleaseStillRefusesAGenuineTakeoverAfterItsOwnRenewal`).

`tokenlessOwnLane` checks one thing: the token is empty. A mere
`ownToken` failure is not enough, so that case stays untouched.

Two more tests keep their existing wording too:
`TestStartRefusalNamesALiveAgentInsteadOfTheWindow`, and
`TestStartWithUnconfirmedLivenessDoesNotResume`.

`SetHoldKind` only ever adds text for `HoldUnproven`. A live or
unread-liveness kind leaves `NextAction` empty, as before.

**Gate.** Build a token-less lane the way
`TestStartDoesNotResumeALaneWhoseTokenIsGone` and the new release
fixture do. `release` and `start --go`, run against it, print the new
wording and a non-empty `next_action` — in both the table and
`--json`. The `@S49` scenario, and every existing release/start
refusal test, still pass unchanged. Run
`go test ./internal/report -update` to re-record
`testdata/release.json` for the new field, and read the diff before
committing it. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
