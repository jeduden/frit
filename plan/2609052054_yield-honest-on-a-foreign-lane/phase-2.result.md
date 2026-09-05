---
n: 2
title: A non-fetching pass keeps the window it did not read
status: "✅"
result: true
summary: >-
  `observeHolds` no longer deletes a held plan's accumulated staleness
  window on a pass that could not confirm the ref gone. The empty-
  `HoldTip` delete is now gated on `res.Summary.Fetched > 0`: a pass
  that refreshed nothing leaves the window untouched — neither deleted
  nor re-observed — so its accrued span survives, while a fetching pass
  that finds no ref still prunes it as before. Pinned by
  `TestObserveHoldsKeepsAWindowANonFetchingPassCouldNotConfirmGone` and
  `TestObserveHoldsPrunesAWindowAFetchingPassConfirmedGone`. `go test
  ./...`, `golangci-lint run` and `mdsmith check .` are green.
---
## Handoff

The coupled sampler fault is closed. A foreign hold that a yield — or
any non-fetching verb — could not see keeps its accumulated staleness
window across passes, rather than resetting to zero each time an
unrefreshed pass reads an empty `HoldTip`. So `frit start`'s takeover
clock matures instead of restarting, and the takeover the window exists
to open can actually arrive.

The discriminator is the fleet-wide `Summary.Fetched` count, the
authority the plan named: zero means the pass refreshed nothing and
cannot confirm a ref gone, so it prunes nothing. A finer,
per-repository refresh bit was deliberately not threaded — a single
gather's `Fetch` is one global flag, so a mixed pass does not arise in
practice; if one ever does, `Coord` is where a per-repo bit would live.

Both faults the plan named are now fixed: yield refuses a foreign,
unfenced hold honestly and names the way out (Phase 1), and a
non-authoritative pass no longer prunes the window it was watching
(Phase 2). The plan is complete.
