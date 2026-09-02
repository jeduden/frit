---
n: 1
title: The six lease-API rows of host death and races run for real
status: "✅"
result: true
summary: >-
  S17, S19, S26, S27, S28 and S30 drop `@pending` and pass as real
  godog scenarios, each asserting an observable fact on origin or a
  typed lease error — never a comment. All twelve rows in the two
  sections' new step file, `bdd_host_death_and_races_test.go`, reuse
  the shared `world` `bdd_lease_test.go` built, threading their own
  state through the existing `section[T]` mechanism rather than adding
  fields to `world` or standing up a second world type.
---
## Handoff

**World: shared, not new.** Every row in this phase runs through the
one `world` `bdd_lease_test.go` defined. The new file adds no field to
`world` itself; each section's own state — a race's claim attempts, a
re-claim's release marker and epoch — lives behind `section[T]`, the
per-type-per-scenario slot `bdd_test.go` already built for exactly
this. Two structs, `racesState` and `hostDeathState`, carry it. No step
text already bound in `bdd_lease_test.go` was redefined; strict mode
would have reported that ambiguous rather than let it shadow.

**Steps the lease world lacked, one family per row shape.** A
claimant's own attempt (`"X" claims plan N` / `"X" retries plan N`)
drives S26, S27 and S28's retry; its result is kept, win or loss, so a
later step reads it rather than re-running the race. A rename
(`"X" knows the plan file by a new name`) actually renames the plan
file in a fresh clone and commits it locally, unpushed — S27 proves the
work ref is keyed on the plan id alone, so nothing of that commit
reaches it. A ref deleted from outside frit (`a human deletes the work
ref on origin` / `the plan completes and its ref is deleted on
origin`) is one function bound under both texts, since the git-visible
shape is identical; S19 is the doc-by-argument row the phase's title
promised — the same mechanics standing in for two matrix stories. S30
adds one fact to S16's own reused steps: the raw push's rejection is
asserted specifically non-fast-forward, not any refusal. S17 adds a
release and a re-claim (`"X" releases the lease`, `"X" claims the
released plan`, `the re-claim lands at epoch N, a child of the release
marker`) and its own closing step,
`yieldParksLeavingReclaim` — S16's `yieldParks` checks origin against
the takeover's tip, but S17's origin has moved past that to the
re-claim, so reusing it would have weakened the assertion rather than
proven the row; a new step checks the re-claim's tip instead, per the
phase's own edge-guard.

Every new step function that refuses on a bad role or a missing prior
result carries its own unit test (`TestAttemptsClaimRefusesAPlanThe...`
through `TestYieldParksLeavingReclaimRefusesTheWrongRoles`), the same
edge-guard convention `bdd_lease_test.go` set. The happy path for each
stays proven by its scenario, matching that file's own precedent
(`pushIsRejected`, `yieldParks` carry no separate happy-path unit
test either).

**Finding, not a row.** The six sibling `bdd-*` phase-1 plans'
documented gate — `go test ./cmd/frit -run 'TestFeatures/S(..)_'` —
never selects anything: `TestFeatures` names each subtest
`"<id>: <title>"`, and go's `-run` sanitizes only the space to `_`, so
the actual name is `S17:_...`, not `S17_...`. The pattern silently
matches zero tests and reports a vacuous pass rather than a failure,
in both the RED and the GREEN direction. This phase verified RED and
GREEN with the full `TestFeatures` run filtered by name instead. The
gate text is wrong across every sibling plan's phase-1.md, not just
this one's; worth a doc fix, out of scope here since it names no
row's Given/When/Then.

### What the next phases need

- S14 (power loss mid-push), S15 (host dies holding a claim), S18
  (zombie re-runs its own claim), S31 (orphan report vs sleeping
  host) and S32 (two same-host sessions race) are verb-level, not
  lease-API: they need the resume path (`start`'s from-outside resume,
  landed in plan 2609011836), an explicit-time window a test can move
  without a real sleep, and a herdr fake standing in for the pane
  roster resume's liveness check reads.
- S29 (release races a loser's read) needs a runner wrapper around
  `gitwt.Runner` that can inject a release push between a loser's CAS
  loss and the read that names the winner — the one row in this
  matrix pair that cannot be driven by sequencing two real machines
  alone.

`go test ./...`, `go test ./internal/scenario` (the bijection gate),
`go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith check .`
are all clean.
