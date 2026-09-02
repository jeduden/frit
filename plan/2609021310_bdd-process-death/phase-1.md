---
n: 1
title: The five lease-API rows of process death run for real
status: "🔲"
result: false
---
Convert the five process-death rows the lease API alone can drive —
S1, S2, S7, S10, S11 — from `@pending` declarations into passing
scenarios. This fixes three things the later phases copy: the
section's step file and its registration, the world it threads, and
the convention for a row the doc resolves by argument.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. That file
already defines "holds the lease for plan", "commits work on the lane
it never pushes" and "takes the lease over", over `claimableRepo`,
`cloneAgain` and the `claim` API. `claim.Takeover` mints a child of
the tip it is handed. `claim.RemoteTip` reads origin's current tip.

**Value.** The section stops being five declarations and becomes five
executable promises: a killed claim leaves origin clean, a takeover
inherits exactly what was pushed, an observation resets when the tip
moves. Any of those regressing fails the build, and the file the
remaining eight rows join already exists.

**RED.** Drop `@pending` from S1, S2, S7, S10 and S11 in
[process-death.feature](../../features/process-death.feature) and
write each one's Given/When/Then. Run `go test ./cmd/frit -run
TestFeatures/S1_`: strict mode reports the new steps undefined and the
subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S1, killed before the local ref write. Given a claimable plan, when
  the claim dies before writing anything, then origin has no work ref
  and a retry acquires at epoch 1. "Dies before writing" is a claim
  never started; the scenario asserts what origin shows.
- S2, killed after the local write, before the push. Given the local
  ref exists and origin has none, when another machine claims, then it
  wins at epoch 1 and the first machine's stale local ref is refused
  on its retry (`LocalDivergesError` is the shape the unit tests pin).
- S7, an observer saw a claim that then unwound. Given an observation
  recorded against a tip, when the ref moves or is deleted, then the
  observation resets. Drive `resetWindow` and the observation store
  with explicit times; no clock is read.
- S10, killed mid-phase with work pushed. Given the holder pushed a
  work commit, when another machine takes over, then the takeover's
  parent is that pushed tip and the work is in the takeover's history.
- S11, killed mid-phase with work only local. Given the holder
  committed but never pushed, when another machine takes over, then
  the takeover's parent is the last pushed tip, and the local commit
  is in no history on origin.

**GREEN.** Add `cmd/frit/bdd_process_death_test.go`: a world for this
section (or the lease world extended through its own file — decide,
and record which), the step functions, and an `init` appending the
registrar. Reuse every step `bdd_lease_test.go` already defines;
define only what the five rows add. A quoted machine name in a step
is checked against its role, as the lease world does, so a scenario
cannot pass by naming the wrong box. Every step function ships with a
unit test of its own, per CLAUDE.md.

**Guard the edges.** A step text `bdd_lease_test.go` already defines
must not be redefined: strict mode reports it ambiguous. The world
must refuse a machine the scenario never introduced. A scenario that
only passes by weakening an assertion is a finding for the handoff,
not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(1|2|7|10|11)_'`
passes with every one of the five reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the rows that needed a
step the lease world lacked. Record any finding a row exposed. Say
what the verb-level rows, S3..S6, will need from the resume path and
the herdr fake.
