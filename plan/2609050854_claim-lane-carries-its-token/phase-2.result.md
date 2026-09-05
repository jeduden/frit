---
n: 2
title: Refusals from a token-less lane name the way out
status: "✅"
result: true
summary: >-
  `release` and `start`, refused from a hold this machine cannot
  prove, now carry `next_action` naming the honest way out — the same
  wait-or-take-over wording `open` already gives an identical
  `HoldUnproven` hold, factored into a shared `unprovenNextAction` in
  `internal/report/dispatch.go`. `ReleaseDoc` gains the field and a
  `RefuseUnproven` setter; `StartDoc` gains an off-the-wire `holdKind`
  and a `SetHoldKind` setter that reprojects `NextAction` through
  `startNextAction`, now keyed on the handoff and the kind together.
  `release` tells a token-less own lane (`tokenlessOwnLane`, new in
  `cmd/frit/claim.go`) apart from a genuinely foreign hold, wording it
  honestly rather than "held live by another lane". `start` asks
  `holdKindFor` — the same read `open` already runs — wherever
  `claimRefusal`'s generic "already held" reason fires. Both table and
  `--json` renderers print the field. `testdata/release.json`
  re-recorded for the new field.
---
## Handoff

**Done.** `internal/report/dispatch.go`: `unprovenNextAction(id
int64) string` factors the wording `openNextAction`'s `HoldUnproven`
case already carried; `ReleaseDoc.NextAction` and
`ReleaseDoc.RefuseUnproven(reason, id)` join it; `StartDoc` gains an
unexported `holdKind HoldKind` field and `SetHoldKind`, and
`startNextAction` takes `kind` alongside `handoff` — unchanged for
`HandoffRunning`, `unprovenNextAction` for `HandoffNone` with
`kind == HoldUnproven`, empty otherwise.

`cmd/frit/claim.go`: `tokenlessOwnLane(rt, plan, cwd) bool` is true
only when `inOwnLane` holds and the lane's `claim.ReadToken` reads
empty — told apart from a token that exists but no longer proves the
tip, a genuine foreign move that must keep reading as foreign (S86's
negative case).

`cmd/frit/release.go`: `releaseHeld`'s `!ok` branch now runs through
`refuseUnproved`, which keeps `plan.Stale`/`plan.Dead` routed to
`foreignHoldRefusal` unchanged, then checks `tokenlessOwnLane` before
falling back to `foreignHoldRefusal` for a genuinely foreign hold.
`tokenlessOwnLaneRefusal` words the S49 shape honestly. `printRelease`
prints `NextAction` on a refusal.

`cmd/frit/start.go`: `startRefusal`'s `claimRefusal` branch calls
`holdKindFor` when `plan.Held` and hands it to the refused doc's
`SetHoldKind` — the same read `open`'s `holdKindFor` already performs
for the identical hold. `printStart` prints `NextAction` on a refusal
too.

**Proven.** `TestStartNextActionIsAPureProjectionOfHandoff` (extended
for the new `kind` argument),
`TestStartNextActionNamesTheWaitForAnUnprovenRefusal`,
`TestStartSetHoldKindNamesTheWaitOnAnUnprovenRefusal` and
`TestReleaseRefuseUnprovenNamesTheWaitForATokenlessOwnLane` in
internal/report/dispatch_test.go;
`TestReleaseNamesTheWayOutForATokenlessOwnLane` in
cmd/frit/release_test.go;
`TestStartRefusalNamesTheWayOutForAnUnprovableHold` in
cmd/frit/start_test.go, both asserting the table and `--json` alike.
The `@S49` scenario's own step,
`startRefusesAlreadyHeldNotTakeable` in
cmd/frit/bdd_identity_and_cross_layer_test.go, gained an assertion for
the printed way-out sentence alongside its existing wording checks.
Every pre-existing release/start refusal test — including the S86
negative case, `TestReleaseStillRefusesAGenuineTakeoverAfterItsOwnRenewal`,
and the live/unparked/unread-liveness start refusals — passes
unchanged, pinning that only `HoldUnproven` gained wording.

**Verified against the built frit.** `go build ./...`, `go test
./...`, `go vet ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are all clean.
`go test ./internal/report -update` re-recorded
`testdata/release.json` for the new `next_action` field; the diff was
read before committing (one added line, empty string, on every
fixture that does not exercise the new refusal).

**Inherits to phase 3.** `holdKindFor` is now read from two more call
sites (`release`, `start`) beyond `open`, always the same read — no
verb here computes a hold's kind its own way. Phase 3's job — surfacing
a token-less held lane in `board`, `orphans`, `show` and `next` — can
lean on that same function unchanged; nothing in this phase narrowed
or widened what `HoldUnproven` means.

You may clear this session now; phase 3 starts fresh from
`go run ./cmd/frit phase 2609050854`.
