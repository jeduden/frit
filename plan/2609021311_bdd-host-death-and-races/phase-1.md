---
n: 1
title: The six lease-API rows of host death and races run for real
status: "✅"
result: false
---
Convert the six rows the lease API alone can drive — S17, S19, S26,
S27, S28, S30 — from `@pending` declarations into passing scenarios.
This fixes three things the later phases copy: the two sections'
shared step file and its registration, the world it threads, and the
convention for a row the doc resolves by argument.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. That file
already defines "holds the lease for plan", "commits work on the lane
it never pushes", "takes the lease over", "comes back and renews its
lease", "the renewal is fenced, naming", "push of that work is
rejected" and "yield parks", over `claimableRepo`, `cloneAgain` and
the `claim` API. `claim.Acquire` on an absent ref mints epoch 1, and
on a release marker mints epoch E+1. A lost acquire is a `HeldError`
carrying the winner's marker. `claim.Release` leaves a release marker.
`claim.RemoteTip` reads origin's current tip. When the ref is absent,
`casPush`'s reconciliation read answers empty, so a renewal against a
completed plan surfaces as a push fault, not a `FenceError`.

**Value.** The two sections stop being six declarations and become
six executable promises: one ref has one winner however many claim
it, a rename cannot fork the key, a zombie's push is refused, a
completed plan's ref cannot be renewed onto. Any of those regressing
fails the build, and the file the remaining six rows join already
exists.

**RED.** Drop `@pending` from S17 and S19 in
[host-death.feature](../../features/host-death.feature) and from S26,
S27, S28 and S30 in [races.feature](../../features/races.feature),
and write each one's Given/When/Then. Run `go test ./cmd/frit -run
TestFeatures/S26_`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S26, N claimants, one plan. Given "box-a" holds the lease, when
  "box-b" and "box-c" each claim the same plan, then each claim
  loses, each loser's error names "box-a" at epoch 1, and origin
  carries one work ref. Mirror `TestAcquireRaceHasOneWinnerAndNames`
  `TheLease`; assert `HeldError.Known` and the marker's holder.
- S27, rename between two claimants. Given "box-a" holds the lease,
  and "box-b" knows the plan file by a new name, when "box-b" claims,
  then it loses and origin's `refs/heads/plan/*` holds exactly one
  ref. Mirror `TestAcquireIsRenameProof`: the rename is a local
  commit on "box-b" that never reaches the ref.
- S28, human deletes ref mid-claim. Given "box-a" holds the lease and
  "box-b"'s claim lost, when a human deletes the work ref on origin
  and "box-b" retries, then the retry acquires at epoch 1 and origin's
  tip is "box-b"'s claim marker. "box-a"'s next renewal is refused,
  which is what TRUST looks like: frit reports, it does not defend.
- S30, zombie vs new claimant on one branch. Given "box-a" holds the
  lease and commits work it never pushes, and "box-b" takes the lease
  over, when "box-a" pushes that work raw, then origin rejects it as
  non-fast-forward and still holds "box-b"'s takeover. Reuse S16's
  steps; add only the non-fast-forward assertion on the push output.
- S17, suspended weeks, plan re-claimed. Given "box-a" holds the
  lease and commits work it never pushes, and "box-b" takes the lease
  over, releases it, and "box-c" claims the released plan, when
  "box-a" comes back and renews, then the renewal is fenced naming
  "box-c", and yield parks "box-a"'s work while origin keeps "box-c"'s
  claim. The re-claim lands at epoch 3, a child of the release marker.
- S19, zombie pushes to a completed plan. Given "box-a" holds the
  lease, when the plan completes and its ref is deleted on origin, and
  "box-a" renews from its own tip, then the renewal fails and origin
  still has no work ref. And when "box-a" pushes its tip raw, origin
  accepts it: that acceptance is the TRUST observable, asserted as a
  fact about origin, never as a comment.

**GREEN.** Add `cmd/frit/bdd_host_death_and_races_test.go`: a world
for these two sections (or the lease world extended through its own
file — decide, and record which), the step functions, and an `init`
appending the registrar. Reuse every step `bdd_lease_test.go` already
defines; define only what the six rows add: a second and third
claimant, a rename on one machine, a hand-deleted ref, a release and
a re-claim, a completed plan, a raw push that lands. A quoted machine
name in a step is checked against its role, as the lease world does,
so a scenario cannot pass by naming the wrong box. Every step function
ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** A step text `bdd_lease_test.go` already defines
must not be redefined: strict mode reports it ambiguous. The world
must refuse a machine the scenario never introduced. S17's yield
assertion checks origin against the re-claim's tip, not the
takeover's; the lease world's `yieldParks` checks the takeover, so
S17 needs its own final step rather than a weakened reuse. A scenario
that only passes by weakening an assertion is a finding for the
handoff, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(17|19|26|27|28|30)_'`
passes with every one of the six reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the rows that needed a
step the lease world lacked, and whether the world was shared or new.
Record any finding a row exposed. Say what the verb-level rows — S14,
S15, S18, S31, S32 — will need from the resume path, the explicit-time
window and the herdr fake, and what S29's runner wrapper must
intercept.
