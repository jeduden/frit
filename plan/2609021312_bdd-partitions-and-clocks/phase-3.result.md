---
n: 3
title: The cross-host clock skew row runs for real, closing the matrix's ten
status: "✅"
result: true
summary: >-
  S36 is a real Given/When/Then scenario, no longer @pending, bound
  in a third registrar in the existing
  cmd/frit/bdd_partitions_and_clocks_test.go. pcState carries two new
  maps, hostWindows and hostClocks, keyed by host rather than the
  singular pair every earlier row in this file shares. Each host
  matures its own window independently, on a clock skewed years from
  the other's, through the same maturation loop S20's own
  observerWatchesTipGoStale already uses; the Then step reads each
  host's StaleHold only against its own recorded clock, so
  convergence is proven by construction, never asserted as prose.
  Every new step function carries its own unit test. All ten rows
  this plan opened — S20-S25, S33-S36 — pass together.
---
## Handoff

### What is green

`go test ./cmd/frit -run
'TestFeatures/S(20|21|22|23|24|25|33|34|35|36):'` reports all ten as
PASS, none SKIP — the whole matrix section this plan set out to
convert. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run ./...` are clean.

S36 needed no fake Runner, no CLI round trip, and no repository beyond
what "box-a" holds the lease" already builds: two independent
maturation loops, each fed its own `now` sequence, prove the doc's
claim — no marker timestamp, and no cross-machine timestamp either,
ever enters a liveness decision — by never letting one host's clock
touch the other's window in the test's own code, not by asserting the
absence of a comparison that could not happen anyway.

### Findings

None new. Phase 2's own two findings — the pure discovery functions
cannot mature a window early from a single clock jump, and
`fetchRemote`/`staleFetch` already degrade a failed fetch gracefully
with no fake Runner needed — both held for this row too, unchanged.

### Plan-level close

Every Acceptance Criterion in `plan.md` is met:

- No scenario in either feature file carries `@pending`; all ten of
  S20-S25 and S33-S36 report PASS, none SKIP.
- Every step is bound in `bdd_partitions_and_clocks_test.go` or
  reused from `bdd_lease_test.go`; `bdd_test.go` was never touched
  across all three phases (`git diff` against the plan's own first
  commit confirms it).
- Each scenario asserts an observable: a verb's JSON document
  (`board`), origin's refs, a marker's own commit date, or a stored
  `observe.State` window — never a comment standing in for a check.
- No scenario reads the wall clock into a *decision*: every
  maturity, void or backoff read is against a time a step chose.
  S22 and S24 go through the real CLI, which reads `time.Now()`
  internally, but neither row's own assertion is a staleness
  decision — S22 reads a display field, S24 reads a Problem and a
  `held` flag — so this criterion holds in the sense that mattered:
  no takeover or fence ever turns on a clock this file did not pick.
- Every finding a row exposed is recorded in its phase's own
  handoff, not fixed silently: `persistToken`'s pre-worktree no-op
  (Phase 1), the phase's own gate-regex bug (Phase 1), the
  impossibility of an early-firing window under the pure functions
  (Phase 2), and `fetchRemote`/`staleFetch`'s undocumented degrade
  (Phase 2).
- `go test ./...` passes; `golangci-lint run` is clean.

This plan is closed. The sibling conversion plans this plan's own
context names — process death, host death and races, storage,
identity and cross-layer, the two lifecycle halves — each still own
their own file and can land in any order; none depended on this one,
and none is unblocked by it beyond the vocabulary this file's steps
are now free to be grepped for and reused the way `bdd_lease_test.go`
already was.
