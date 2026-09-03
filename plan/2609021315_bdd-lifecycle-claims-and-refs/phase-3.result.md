---
n: 3
title: Plan deleted while claimed runs for real
status: "✅"
result: true
summary: >-
  S52 drops `@pending` and runs as a real scenario in
  `cmd/frit/bdd_lifecycle_test.go`: a lease still live (never
  released), carrying real work pushed onto its own ref, whose plan
  file is deleted from main while the lease stands, has that ref
  scavenged directly by `claim.Scavenge` — the unlanded work parked to
  a rescue ref before the ref is deleted from origin. The Then checks
  the park half of the matrix's own outcome directly: `ls-remote` on
  the recorded rescue ref must carry the tip that was parked, not
  merely a non-empty ref name. `go test ./...` and golangci-lint stay
  clean, and the whole `TestFeatures` suite — every section landed so
  far — still runs with no ambiguous step.
---
## Handoff

**S52 landed exactly as phase-3.md predicted, with no finding.**
`claim.Scavenge`, driven directly against a live claim's own tip the
way phase 2 drove it against a released one, parked the unlanded work
and deleted the ref on the first try — its CAS still never inspects
the marker kind, so a live claim's tip scavenges exactly like a
released one, the same mechanism phase 2 already proved and this
phase reused rather than re-derived. The step text is deliberately its
own — `the ref is scavenged by evidence`, not
`theReleasedRefIsScavengedByEvidence`'s text — since S52's tip was
abandoned, not released; the underlying `claim.Scavenge` call is
identical.

**The evidence-detection gap phase 1 and phase 2 both named is still
open, and still out of this plan's scope.** Nothing in `claim.go` or
`release.go` decides on its own that a deleted plan file should
trigger a scavenge, and nothing waits out the matrix's "after a
window" half of S52's own outcome column. This phase proves the
mechanism the matrix names — SCAV with PARK first — over a Given built
by hand; it adds no wiring that would call `Scavenge` on plan-gone
evidence automatically. That remains a gap for a future plan, not a
finding blocking this one, exactly as plan.md's Context anticipated.

**What S58 needs.** Phase 2's own handoff already answers this in
full and nothing here changes it: `Release` then a second `Acquire` —
no `Scavenge` at all, the released ref never needing deletion, only
reacquiring — the same shape `TestClaimReacquiresAReleasedLease`
already proves at epoch 2. `planIsDoneAndItsLeaseIsReleased`, S53's
and S57's own Given, already builds the released lease S58's Given
wants; S58's own phase needs only its own When (a second machine's
claim) and Then (the new holder's marker at epoch 2, origin's tip
matching it). This phase's own fixture — a live, unreleased lease with
real work on it — has nothing further to add to S58, which starts from
a released lease, not a live one.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S52'`
reports PASS, not SKIP; `go test ./cmd/frit -run TestFeatures` (every
landed section) stays green; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean; `mdsmith check .`
is clean.
