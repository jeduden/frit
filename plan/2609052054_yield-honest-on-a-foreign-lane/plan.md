---
id: 2609052054
title: >-
  frit yield is honest on a lane it cannot end, and never drops it
  from sampling
status: "✅"
summary: >-
  frit yield on a foreign, unfenced lane — a plan another lane holds,
  run from a pane that is not that lane's own worktree — reports
  "yielded plan <id>" but parks nothing, tears nothing down, and
  leaves the claim untouched. It reads success where release, met with
  the same hold, refuses honestly and names the takeover as the way
  out. The empty-local branch in claim.Yield returns a clean no-op
  before it ever reads who holds the remote lease. So a live foreign
  hold and a plan nobody holds render alike. A second, coupled fault:
  observeHolds drops a held plan's accumulated staleness window
  whenever this pass sees no work ref. Yield does not fetch by
  default, so a pass that simply did not refresh the remote view wipes
  the window and resets start's takeover clock to zero — the plan can
  never mature into a takeover. Make yield refuse the foreign,
  unfenced hold the way release does and carry the way out in
  next_action. Then stop a non-authoritative pass from pruning a
  window it did not actually confirm gone.
model: sonnet
depends-on: []
---
# frit yield is honest on a lane it cannot end, and never drops it from sampling

## Goal

`frit yield`, run against a lane it neither owns nor is fenced under,
says so. It names the takeover as the way out, rather than reporting a
success it did not perform. A plan a yield could not end keeps its
staleness window. So `frit start`'s takeover clock matures, rather
than resetting to zero each time an unfetched pass misses the hold.

## Context

**The gap, from the report.** Issue #166: `frit yield <id>` on a plan
held by another lane, run from a pane that is not that lane's own
worktree, printed `yielded plan <id>`. It also warned "the calling
pane is not plan <id>'s own lane; its worktree was left standing". The
claim ref's tip was byte-identical before and after. No
`refs/frit/rescue/<id>/...` ref was written. `frit board` still read
the plan held, and `frit start <id>` still refused with "seen
unchanged for 0s". The reporter also saw the plan's key vanish from
`~/.cache/frit/observations.json` while sibling plans kept sampling.

**Why yield reports a false success.** `Yield` in
[internal/claim/lease.go](../../internal/claim/lease.go) takes the
local work-ref tip. When it is empty, `Yield` returns a clean no-op
`Scavenged{}` before reading who holds the remote lease. An empty
local tip is the foreign, unfenced case: no ref of this lane's own to
park. But that branch cannot tell a live foreign hold from a plan
nobody holds. Both reach it as `local == ""`. `yieldCmd.Run` in
[cmd/frit/yield.go](../../cmd/frit/yield.go) then takes the no-op as
success: `doc.Parked("")`, `tearDownLane`, and `printYield` writes
`yielded plan <id>` with only the pane warning.

**What release does with the same hold.** `releaseCmd` dispatches on
the gather's own plan facts. `plan.HoldTip == ""` is "nothing holds
it". A hold this lane cannot prove is refused by `foreignHoldRefusal`
in [cmd/frit/release.go](../../cmd/frit/release.go): "is held live by
another lane (...); only its own lane can release it". Once the window
matures or the session is confirmed gone, it reads "run `frit claim`
to take it over". Yield is the verb the "deserted row" recipe reaches
for. When there is nothing local to park and the hold is foreign, the
honest answer is release's, not a bare "yielded".

**Reuse first.** The refusal wording exists (`foreignHoldRefusal`).
The way-out wording exists too: `unprovenNextAction` in
[internal/report/dispatch.go](../../internal/report/dispatch.go) is
the wait-or-take-over sentence `open`, `release` and `start` already
carry for a `HoldUnproven` hold. `ReleaseDoc` already grew a
`next_action` field and a `RefuseUnproven` setter for exactly this
situation in the plan that just landed. `YieldDoc`, in the same
[internal/report/dispatch.go](../../internal/report/dispatch.go), has
`Refuse` and `Warning` but no `next_action` yet. Phase 1 gives yield
the same dispatch release has and the same field, fed from the same
helper — no new wording.

**The coupled sampler fault.** `observeHolds` in
[cmd/frit/main.go](../../cmd/frit/main.go) folds every held work ref
into the observation store. When a plan's `HoldTip` is empty, it
`delete`s that plan's key. That runs as a side effect of every
fleet-reading verb, yield included, and yield does not fetch by
default. So a pass whose remote-tracking view was never refreshed sees
no work ref for a still-held plan, and deletes its accumulated window.
The next pass that does see the ref rebuilds the window fresh:
`First = Last = now`, `Span == 0`. `notMaturedReason` then reads "seen
unchanged for 0s" for a hold that has in truth sat unchanged for
hours. The delete throws away real maturity because a
non-authoritative pass could not see the ref. `Result.Summary.Fetched`
already records how much of the fleet this walk refreshed. Phase 2
reads it, so a pass that did not confirm the ref gone does not prune
the window it was watching.

**Out of scope.** Yield still never guards a foreign hold the way
claim and start do when there *is* something to park. A fenced lane
acting on a hold another host's claim covers is exactly what yield is
for, and that path is unchanged. Loosening the token or lease rules is
not in scope. A foreign hold is still ended only by its own lane or a
matured takeover, never by yield.

## Tasks

1. Phase 1 (proving slice): `frit yield` on a foreign, unfenced lane
   refuses the way `release` does — naming the live holder, or the
   matured window or dead session — and carries the way out in
   `next_action`. A plan nobody holds still yields as a clean no-op.
   The fenced-lane park-and-tear-down path is unchanged. Pinned by a
   cmd-level test against a foreign holder and confirmed against the
   built frit.
2. Phase 2: `observeHolds` no longer drops a held plan's accumulated
   staleness window on a pass that did not fetch, so a foreign hold a
   yield or any non-fetching verb could not see keeps maturing toward
   its takeover window rather than resetting to zero.

## Execution

| Phase | Title                                                | Tier   | Gate                                                                                                                                            |
| ----- | ---------------------------------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Yield refuses a foreign, unfenced hold and names it  | sonnet | against the built frit in a scratch fleet, yield on a foreign hold prints the refusal and a non-empty `next_action`, parks nothing; tests green |
| 2     | A non-fetching pass keeps the window it did not read | sonnet | a non-fetching observe pass over a still-held plan preserves its window; `start`'s reported span keeps growing across passes; tests green       |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [Yield refuses a foreign, unfenced hold and names it](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
|     | ↳      | `frit yield` now dispatches on the gather's own plan facts before ever calling `claim.Yield`: a plan nobody holds (`plan.HoldTip == ""`) still reaches the existing clean no-op, but a foreign hold with nothing of this lane's own to park (`local == ""`) refuses instead, worded from the same `foreignHoldRefusal` release already uses and carrying the wait-or-take-over sentence in a new `next_action` field on `YieldDoc`. The fenced-lane park-and-tear-down path is untouched. Pinned by `TestYieldRefusesAForeignHoldWithNothingToPark`, `TestYieldRefusesAMaturedForeignHoldNamingTheTakeover` and `TestYieldOnAnUnheldPlanIsStillACleanNoOp`; the existing `TestYieldParksAFencedLaneAndTearsItDown` still covers the fenced path. `go test ./...` and `golangci-lint run` are green; `mdsmith check .` passes. |
| 2   | ✅     | [A non-fetching pass keeps the window it did not read](phase-2.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
|     | ↳      | `observeHolds` no longer deletes a held plan's accumulated staleness window on a pass that could not confirm the ref gone. The empty- `HoldTip` delete is now gated on `res.Summary.Fetched > 0`: a pass that refreshed nothing leaves the window untouched — neither deleted nor re-observed — so its accrued span survives, while a fetching pass that finds no ref still prunes it as before. Pinned by `TestObserveHoldsKeepsAWindowANonFetchingPassCouldNotConfirmGone` and `TestObserveHoldsPrunesAWindowAFetchingPassConfirmedGone`. `go test ./...`, `golangci-lint run` and `mdsmith check .` are green.                                                                                                                                                                                                             |
<?/catalog?>

## Acceptance Criteria

- [x] `frit yield` on a foreign, unfenced lane refuses instead of
      reporting a success, naming the live holder or the matured
      takeover, and carries a non-empty `next_action`
- [x] A `frit yield` on a plan nobody holds still succeeds as a clean
      no-op, and a fenced lane still parks its divergence and tears its
      worktree down
- [x] A held plan's accumulated staleness window survives a pass that
      did not fetch, so `frit start`'s reported span keeps growing and
      its takeover window can mature
- [x] Both the table and `--json` renderings of yield carry the field
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
