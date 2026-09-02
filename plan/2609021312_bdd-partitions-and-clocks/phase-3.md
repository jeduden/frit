---
n: 3
title: The cross-host clock skew row runs for real, closing the matrix's ten
status: "🔲"
result: false
---
Convert the matrix's last row, S36. Phase 2's own handoff already
named the shape: two independent `discovery.Window`/clock pairs
watching the same tip. One host's clock is skewed years from the
other's; both converge on the same `StaleHold` answer. A
lease-API-level row, not a verb-level one — this closes the plan's
own ten.

**Assumes.** Everything Phase 1 and Phase 2's handoffs recorded.
`discovery.Observe`/`StaleHold` take whatever `now` a caller passes
and never see wall time (Phase 2's own finding). Two callers can
therefore run the identical maturation loop
`observerWatchesTipGoStale` already uses, on two unrelated `now`
sequences over the same tip. Neither call ever shares a variable, so
one host's clock cannot enter the other's `StaleHold` call.
`pcState`'s existing `window`/`clock` pair is S20-S35's own shape,
singular by construction. This row needs a second pair kept apart — a
small map keyed by host, not the singular field grown ambiguous.

**Value.** Every other clock row (S33-S35) pins a single observer
against its own clock. S36 is the one row that puts two side by
side: the doc's own claim, "no marker timestamp enters any decision"
extended to "no cross-machine timestamp is ever compared", stays
untested until two independent windows are actually built and read
each against only its own clock. Landing it closes the ten rows this
plan opened.

**RED.** Drop `@pending` from S36 in
[clocks.feature](../../features/clocks.feature):

```gherkin
@S36
Scenario: cross-host clock skew
  Given "box-a" holds the lease for plan 36
  And a second host's clock is skewed years from the first's
  When both hosts watch "box-a"'s tip go stale, each on its own clock
  Then both hosts' windows read the hold stale
```

`go test ./cmd/frit -run 'TestFeatures/S36:'` reports the new steps
undefined — commit that red.

**GREEN.** New steps join `registerVerbLevelPartitionsAndClocks` in
[bdd_partitions_and_clocks_test.go](../../cmd/frit). Split off a
third registrar only if the lint budget forces it. `pcState` grows
two new maps, `hostWindows` and `hostClocks`, lazily built in `pc()`
next to the ones already there.

- "a second host's clock is skewed years from the first's" seeds two
  entries in `hostClocks`: one host at this section's own `s.clock`,
  the other years forward (`AddDate`), no window yet for either.
- "both hosts watch \"box-a\"'s tip go stale, each on its own clock"
  reads holder's current tip once (as
  `observerWatchesTipGoStale` does), then for every entry in
  `hostClocks` runs that same maturation loop independently — a fresh
  `discovery.Window{}`, `discovery.Observe` on that host's own `now`
  until `Span() > DefaultTakeoverWindow` — writing the matured window
  back to `hostWindows` and the advanced clock back to `hostClocks`.
  Two hosts, two independent loops; neither reads the other's map
  entry at any point.
- "both hosts' windows read the hold stale" asserts `len(hostWindows)
  == 2` (a wrong loop that skipped a host is not silently the row
  passing on one), then for each host calls `discovery.StaleHold`
  with only that host's own window and clock — proving convergence
  without the test itself ever passing one host's clock against the
  other's window, the same discipline the production code holds to.

**Guard the edges.** The maturation loop must build a fresh `Window{}`
per host, never share the singular `s.window` Phase 1's rows use —
sharing it would silently make this row about one clock twice, not
two. The Then step's assertion must read each host's `StaleHold` off
only its own recorded clock; passing a shared or wrong clock would
defeat the row's whole point without failing loudly.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S36:'` passes, PASS,
not SKIP. The full ten-row run,
`go test ./cmd/frit -run 'TestFeatures/S(20|21|22|23|24|25|33|34|35|36):'`,
stays all-PASS. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

This is the plan's last phase. Close it: tick every Acceptance
Criterion in `plan.md` against this and the two prior phases' own
evidence. Flip `status` to ✅ and run `mdsmith fix PLAN.md`. Write
`phase-3.result.md` too, recording anything this row still leaves
open for a sibling conversion plan.
