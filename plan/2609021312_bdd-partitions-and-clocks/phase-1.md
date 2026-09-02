---
n: 1
title: The five lease-level partition and clock rows run for real
status: "✅"
result: false
---
Convert the five partition and clock rows the lease API can drive on
its own: S20, S21, S25, S33, S34. Each goes from a `@pending`
declaration to a passing scenario. This fixes three things the later
phases copy. The partition Runner cuts one machine off origin. The
observer samples a window on a clock the step chooses. And the
convention for a row the doc resolves by argument is set.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. That file
already defines "holds the lease for plan", "commits work on the lane
it never pushes", "takes the lease over", "comes back and renews its
lease", "the renewal is fenced, naming" and "yield parks", over
`claimableRepo`, `cloneAgain` and the `claim` API. Every `claim`
transition takes a `gitwt.Runner`, so a Runner that wraps `gitwt.Exec`
and fails `push`, `fetch` and `ls-remote` is a partition. `casPush`
in [lease.go](../../internal/claim/lease.go) reports a push whose
confirming read also failed as `UnconfirmedPushError` and moves
nothing; a push that errored yet landed reads as a win.
`claim.Renew` persists the lane's token only on success, and
`claim.OwnAdvance` accepts a tip that descends from the token under
the same epoch and holder. `claim.Release` is a CAS from the tip it
is handed; lost, it is a `FenceError` and deletes nothing.
`discovery.Observe` and `discovery.StaleHold` in
[stale.go](../../internal/discovery/stale.go) take an explicit `now`.
A `git commit-tree` run under `GIT_AUTHOR_DATE` and
`GIT_COMMITTER_DATE` carries that date, as
[lease_test.go](../../internal/claim/lease_test.go) relies on.

**Value.** The two sections stop being five declarations and become
five executable promises: a cut holder stops instead of pushing, a
push that landed under an error is still the lane's own, a stale
release cannot delete, and no commit date changes a liveness
decision. Any of those regressing fails the build, and the file the
remaining five rows join already exists.

**RED.** Drop `@pending` from S20, S21 and S25 in
[partitions.feature](../../features/partitions.feature) and from S33
and S34 in [clocks.feature](../../features/clocks.feature), and write
each one's Given/When/Then. Run `go test ./cmd/frit -run
TestFeatures/S20_`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S20, worker partitioned mid-work. Given "box-a" holds the lease and
  commits work it never pushes, when the network cuts "box-a" off and
  it renews, then the renewal reports the push unconfirmed and origin's
  tip has not moved. When an observer samples that tip past the window
  on its own clock, "box-b" takes the lease over. When the partition
  heals and "box-a" renews, the renewal is fenced naming "box-b", the
  error suggests yield, and yield parks "box-a"'s work. The observer
  is `discovery.Observe` fed chosen times, `StaleHold` true before the
  takeover; no sleep, no wall clock.
- S21, push landed during partition. Given "box-a" holds the lease
  with a real lane worktree, when its renewal's push commits on origin
  but the connection drops before the client hears — a Runner that
  runs the real push, returns an error, and fails the confirming read
  — then the beat is origin's tip and the renewal reports unconfirmed.
  When the partition heals, then `OwnAdvance` accepts origin's tip as
  the lane's own from its persisted token, and `Resume` from that tip
  lands a beat at the same epoch. The lane is a `git worktree add` on
  the plan branch, as the resume unit tests build it.
- S25, stale unwind delete after heal. Given "box-a" was partitioned
  and "box-b" took the lease over, when the partition heals and
  "box-a" releases from its recorded tip, then the release is fenced
  naming "box-b", origin still holds the takeover, and the work ref
  still exists. A release is a CAS on the holder's own tip; there is no
  unleased delete for a stale holder to fire.
- S33, frozen clock on worker. Given "box-a" holds the lease under a
  commit clock pinned to one instant, when it renews twice, then the
  two beats carry the same commit date and different SHAs. When an
  observer that saw the first beat samples the second, then its window
  resets to one sample with no void, and `StaleHold` is false however
  far the observer's own clock has run. Liveness is tip change.
- S34, clock steps backward. Given "box-a" holds the lease, when its
  next beat is dated years before the last, then the tip still moved,
  the observer's window still resets, and `git log --format=%ct` on
  the tip is smaller than on its parent. The date misleads a human
  reading the log; no decision read it.

**GREEN.** Add `cmd/frit/bdd_partitions_and_clocks_test.go`: a world
for these sections (or the lease world extended through its own file —
decide, and record which), the step functions, and an `init` appending
the registrar. The world carries a Runner per machine, `gitwt.Exec`
until a partition step swaps in the failing wrapper and a heal step
swaps it back; a partitioned machine's renewal goes through its own
Runner, never the healed one. The observer is a stored
`discovery.Window` the steps advance with times they choose. Reuse
every step `bdd_lease_test.go` already defines; define only what the
five rows add. A quoted machine name in a step is checked against its
role, as the lease world does, so a scenario cannot pass by naming
the wrong box. Every step function ships with a unit test of its
own, per CLAUDE.md; the partition Runner's unit test shows it fails
the three network verbs and passes every other call through.

**Guard the edges.** A step text `bdd_lease_test.go` already defines
must not be redefined: strict mode reports it ambiguous. The world
must refuse a machine the scenario never introduced, and a heal for a
machine that was never cut. `GIT_AUTHOR_DATE` is set through
`t.Setenv` on the subtest, so it cannot leak into a sibling scenario.
A scenario that only passes by weakening an assertion — sampling a
window on `time.Now()`, or reading an observer's clock into a marker
— is a finding for the handoff, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(20|21|25|33|34)_'`
passes with every one of the five reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the rows that needed a
step the lease world lacked. Record any finding a row exposed. Say
what the verb-level rows, S22, S23, S24 and S35, will need from a
hand-built runtime, `observeHolds` with explicit times, and a dead or
split remote URL, and how S36 should assert two clocks converging.
