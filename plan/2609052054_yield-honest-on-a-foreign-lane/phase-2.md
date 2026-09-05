---
n: 2
title: A non-fetching pass keeps the window it did not read
status: "✅"
result: false
---
Stop `observeHolds` from throwing away a held plan's accumulated
staleness window on a pass that could not confirm the ref gone. A
non-fetching verb — yield among them — reads a stale or absent local
view of a foreign hold's work ref, so the plan arrives with an empty
`HoldTip` though it is still held. Deleting the window on that evidence
resets `frit start`'s takeover clock to zero, so the hold can never
mature into a takeover.

**Assumes.** `observeHolds` in
[main.go](../../cmd/frit/main.go) folds every held work ref into the
per-host observation store. Its first branch deletes a plan's window
whenever `p.HoldTip == ""`, before it reads whether the pass was
authoritative. `fleet.Result` carries a `Summary` with `Fetched` — the
count of repositories whose remote-tracking refs this walk actually
refreshed (`Fetch` defaults true, `--no-fetch` and an offline verb
leave it zero). `discovery.Observe`, `discovery.StaleHold` and the
window's `Span` already do the accrual; nothing about the clock itself
changes. The S23 window-void behavior — a window restarted because the
observer went dark past the sample gap — is a separate path and stays
as it is.

**Value.** A foreign hold a yield or any non-fetching verb could not
see keeps maturing toward its takeover window across passes, rather
than resetting to zero each time an unrefreshed pass misses the ref.
The one verb the deserted-row recipe reaches for stops erasing the very
maturity `frit start`'s takeover needs.

**RED.** Two tests in
[main_test.go](../../cmd/frit/main_test.go). Each builds a synthetic
`fleet.Result`: a held plan with an empty `HoldTip`, its window already
seeded hours deep in the observation store. It is the same shape the
partitions section builds.

- `TestObserveHoldsKeepsAWindowANonFetchingPassCouldNotConfirmGone`:
  with `Summary.Fetched == 0`, `observeHolds` leaves the seeded window
  in the store, its span and sample count unchanged. It is not deleted
  and not reset.
- `TestObserveHoldsPrunesAWindowAFetchingPassConfirmedGone`: with
  `Summary.Fetched > 0`, the same empty `HoldTip` deletes the window —
  a fetching pass that finds no ref confirms it gone, and the store
  drops what this host no longer watches.

**GREEN.** In `observeHolds`, gate the empty-`HoldTip` delete on
`res.Summary.Fetched > 0`. A pass that refreshed nothing keeps the
window untouched — neither deleted nor re-observed — so its accrued
span survives; a fetching pass prunes as before. Nothing else in the
loop changes.

**Guard the edges.** The delete stays for a fetching pass, so a
genuinely released or landed hold is still dropped from the store on
the next authoritative read. A ref that exists (`HoldTip != ""`) is
observed exactly as today, on every pass. The fleet-wide `Fetched`
count is the authority the plan named; a per-repository refresh bit is
not threaded here — note it in the result file if a mixed pass ever
needs finer than whole-pass granularity.

**Gate.** `go test ./cmd/frit -run TestObserveHolds` is green: a
non-fetching pass preserves a held plan's window, a fetching pass still
prunes a gone one. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green, and
`mdsmith check .` is clean.
