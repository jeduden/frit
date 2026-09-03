---
n: 2
title: The scavenge-by-release-evidence rows run for real
status: "✅"
result: true
summary: >-
  S53, S55 and S57 drop `@pending` and run as real scenarios in
  `cmd/frit/bdd_lifecycle_test.go`: a plan id reused after the old
  plan's lease released has its old ref scavenged by evidence before
  a fresh claim mints epoch 1, the same shape a plan re-opened after
  being marked done gets, and a plan merged with its branch already
  auto-deleted has nothing to scavenge and claims fresh straight
  away. All three share one Then, reading the fresh claim's marker
  epoch off `ReadMarker` rather than trusting the CLI's own "claimed"
  wording alone. `go test ./...` and golangci-lint stay clean, and
  the whole `TestFeatures` suite — every section landed so far —
  still runs with no ambiguous step.
---
## Handoff

**All three rows landed exactly as phase-2.md predicted.** S53 and
S57 share one Given (`planIsDoneAndItsLeaseIsReleased`) and one
scavenge step (`theReleasedRefIsScavengedByEvidence`), differing only
in their own When: S53 replaces the plan file under the same id with
`commitPlan` and a new title; S57 edits the same file in place, ✅
then back to 🔲. S55 needed no scavenge step at all —
`resumableRepo` already left nothing on the ref to scavenge — so it
reuses only the shared Then. That Then
(`fritClaimsPlanFreshAtEpoch`) is now the section's second reusable
assertion beside S70's own base check, both built the same way: run
the verb, then read the fresh marker back with `ReadMarker` rather
than trust the CLI's own wording.

**No finding: `claim.Scavenge` needed no code change to scavenge a
released (not landed) tip.** Driving it directly against
`planIsDoneAndItsLeaseIsReleased`'s recorded release tip worked on
the first try — `Scavenge`'s own CAS never inspects the marker kind,
only that the observed tip still matches the remote's current one, so
a release marker is exactly as scavengeable as a landed one. The gap
phase 1's handoff named — nothing in `claim.go` or `release.go` calls
`Scavenge` on a merely released ref — is still true and still
untouched; this phase proved the mechanism the matrix names without
adding the wiring that would call it on its own.

**What S52 needs.** `claim.Scavenge` and `claim.ParkUnlanded` are
already proven, by this phase and by
`TestScavengeParksUnlandedWorkThenDeletes`, to park unlanded work
before deleting regardless of which evidence prompted the call. What
S52 lacks is not a mechanism but a Given: "the plan file is deleted
while claimed" is not itself evidence anything in frit reads today,
confirmed again this phase — no code names "plan-gone" evidence,
exactly as phase 1 found. S52's own phase should build a Given that
deletes the plan's own file while a lease is live, commits unpushed
work on the lane so there is something to park, then drives
`claim.Scavenge` directly (as this phase did for a released tip) and
asserts the rescue ref carries the parked work and the branch is gone
from origin. The "after a window" half of the matrix's own outcome
column is not checkable without the missing evidence wiring; that
half stays a named gap in S52's own phase, not a fix made there.

**What S58 needs.** `Release` then a second `Acquire` — no
`Scavenge` at all, since the released ref never needs deleting, only
reacquiring, the same shape `TestClaimReacquiresAReleasedLease`
already proves at epoch 2. This phase's shared fixture pattern
reaches exactly that far: `planIsDoneAndItsLeaseIsReleased` already
builds the released lease S58's own Given wants; S58's phase needs
only its own When (a second machine's claim) and Then (the new
holder's marker at epoch 2, origin's tip matching it), not a new
fixture.

All tests are green: `go test ./cmd/frit -run
'TestFeatures/S(53|55|57)'` reports three PASS, none SKIP; `go test
./cmd/frit -run TestFeatures` (every landed section) stays green; `go
test ./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean; `mdsmith check .` is clean.
