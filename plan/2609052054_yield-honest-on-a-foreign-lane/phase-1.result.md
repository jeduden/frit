---
n: 1
title: Yield refuses a foreign, unfenced hold and names it
status: "✅"
result: true
summary: >-
  `frit yield` now dispatches on the gather's own plan facts before
  ever calling `claim.Yield`: a plan nobody holds (`plan.HoldTip ==
  ""`) still reaches the existing clean no-op, but a foreign hold with
  nothing of this lane's own to park (`local == ""`) refuses instead,
  worded from the same `foreignHoldRefusal` release already uses and
  carrying the wait-or-take-over sentence in a new `next_action`
  field on `YieldDoc`. The fenced-lane park-and-tear-down path is
  untouched. Pinned by `TestYieldRefusesAForeignHoldWithNothingToPark`,
  `TestYieldRefusesAMaturedForeignHoldNamingTheTakeover` and
  `TestYieldOnAnUnheldPlanIsStillACleanNoOp`; the existing
  `TestYieldParksAFencedLaneAndTearsItDown` still covers the fenced
  path. `go test ./...` and `golangci-lint run` are green;
  `mdsmith check .` passes.
---
## Handoff

`frit yield` on a foreign, unfenced lane now refuses the way `release`
does — naming the live holder, or the matured window or dead session
via `foreignHoldRefusal` — and carries the way out in a non-empty
`next_action`, in both the table and `--json` renderings. A plan
nobody holds still yields as a clean no-op, and a fenced lane still
parks its divergence and tears its worktree down; neither path
changed.

`foreignHoldRefusal` in [cmd/frit/release.go](../../cmd/frit/release.go) is now
worded verb-neutrally ("only its own lane can end it" rather than
"...release it") since yield reads it too — a deliberate, minor
wording change with no test pinning the old phrase.

Phase 2 inherits: the coupled sampler fault is still open.
`observeHolds` in [cmd/frit/main.go](../../cmd/frit/main.go) still
drops a held plan's accumulated staleness window whenever a
non-fetching pass — yield among them — sees no work ref, so `frit
start`'s takeover clock still resets to zero instead of maturing. The
plan's Context section already names `Result.Summary.Fetched` as what
phase 2 should read before pruning a window.
