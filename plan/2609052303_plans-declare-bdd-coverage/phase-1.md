---
n: 1
title: The yield-honesty behavior gets scenario S93
status: "✅"
result: false
---
Give the yield-honesty behavior of plan 2609052054 its place in the
executable scenario matrix. A distant host runs `frit yield` against a
plan another lane holds. This host never fetched the ref. It refuses
honestly, names the takeover, parks nothing, and leaves the lease
untouched. That behavior already passes its `cmd/frit` unit tests. This
phase writes the `@S93` scenario over it, so the matrix catalogs it the
way it catalogs every other cross-host lease behavior.

**Assumes.** `internal/scenario` holds
[features/](../../features) in bijection with the matrix in
[lease-protocol.md](../../docs/research/lease-protocol.md). The gate is
`TestMatrixAndFeaturesAreInBijection`: a row with no tag, or a tag with
no row, fails `go test ./...`. Scenarios run from `cmd/frit`'s
`TestFeatures`. The fenced-lane yield scenarios (S16, S20) in
[host-death.feature](../../features/host-death.feature) already thread a
shared `world`. Its steps — `holdsTheLease`, a foreign holder, the
yield command, an origin-tip assertion — live in
[the host-death steps](../../cmd/frit/bdd_host_death_and_races_test.go)
and [the lease steps](../../cmd/frit/bdd_lease_test.go). The behavior
under test is already implemented and covered by
`TestYieldRefusesAForeignHoldWithNothingToPark` in
[yield_test.go](../../cmd/frit/yield_test.go).

**Value.** The one cross-host yield behavior that landed with no
scenario gets one, closing the gap this plan exists to fix and giving
Phase 2's instruction a concrete worked example to point at. The matrix
stops under-describing what yield does when a host cannot end a hold.

**RED.** Add the S93 row to the Host-death section table in
[lease-protocol.md](../../docs/research/lease-protocol.md) — id, the
scenario, its mitigation column — with no scenario yet tagged `@S93`.
`go test ./internal/scenario` fails: a matrix row naming no tagged
scenario. This proves the bijection gate is what forces the scenario to
exist.

**GREEN.** Add a `@S93` scenario to
[host-death.feature](../../features/host-death.feature): a distant host
that never fetched the ref runs yield against a foreign hold and is
refused. Bind its steps in
[the host-death steps](../../cmd/frit/bdd_host_death_and_races_test.go),
reusing the lease world's existing vocabulary where it fits and adding
only the steps this scenario needs — a host that holds no local copy of
the ref, the yield run, and Then steps asserting the refusal names the
holder or the takeover, carries a non-empty way out, writes no rescue
ref, and leaves origin's lease tip unchanged. Drop no `@pending`: write
the steps so the scenario runs. `go test ./cmd/frit -run
'TestFeatures/^S93:'` and `go test ./internal/scenario` go green.

**Guard the edges.** Reuse a step's existing text rather than restating
it — godog runs strict, so a second definition of the same text fails
as ambiguous. Section-specific state lives in a struct reached through
`section[T]`, never a new field on `world`. The scenario drives the
real `frit` command through the world, not the unit-test helper, so it
proves the dispatch, not a re-implementation. The existing S16/S20
fenced-yield scenarios stay green and untouched.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/^S93:'` runs the
scenario against the built dispatch and passes; it asserts the refusal,
the named way out, the untouched lease tip, and that nothing was
parked. `go test ./internal/scenario` confirms the bijection. `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are green,
and `mdsmith check .` is clean.
