---
n: 3
title: A command behavior gets a BDD home of its own
status: "✅"
result: false
---
`internal/scenario`'s bijection reads exactly one matrix document,
[lease-protocol.md](../../docs/research/lease-protocol.md), against
the whole `features/` tree. A command behavior worth a scenario but
not a lease-protocol one has nowhere to go but that one protocol
catalog, or unit tests alone — a refusal's wording, a `--json` shape,
a `next_action`. This phase gives it a second, independently numbered
catalog. One worked example proves it:
`TestReleaseIsANoOpOnAnAbsentPlan` in
[release_test.go](../../cmd/frit/release_test.go) — `frit release` on
a plan nobody ever held reports a no-op, not a refusal, on one host,
no lease race in sight.

**BDD coverage.** This phase's own subject — widening where a
scenario may live — is exactly what Phase 2's rule now asks every
plan to decide. `release`'s no-op is a command behavior, not a
cross-host lease one, so it gets the new home this phase builds, `@C1`
rather than an `@S<n>` row in
[lease-protocol.md](../../docs/research/lease-protocol.md).

**Assumes.** `MatrixIDs` in
[matrix.go](../../internal/scenario/matrix.go) already takes a path
argument; only `bijection_test.go`'s constant hardcodes lease-protocol
as the one matrix. `collectRowID` there records an id only when its
letter is `S`, passing `F`/`A` attacker rows by uncounted.
`featureTag` in [features.go](../../internal/scenario/features.go)
matches only `@S<n>`. `Scenarios`/`FeatureTagIDs` already walk their
`dir` argument recursively, so a new `.feature` file anywhere under
`features/` is found with no change to either function.

**Value.** A command behavior's scenario stops being homeless. Its
own id namespace (`C` instead of `S`) makes a reader's glance tell
which catalog a scenario belongs to — this phase's own Goal, proven
rather than asserted.

**RED.** Add `docs/research/command-scenarios.md` with one row, `C1`,
naming the no-op behavior above, and tag a new scenario `@C1` in
`features/commands.feature`. Run `go test ./internal/scenario`: it
fails two ways at once —
`collectRowID` rejects `C1` as a malformed id (the row's letter is not
`S`, `F` or `A`), and, once that is worked around, the tag shows up as
naming no matrix row, since `bijection_test.go` never reads the new
document. Both failures are the hardcoded, single-catalog design this
phase replaces.

**GREEN.** Six changes land the second catalog:

1. Widen `rowID` and `collectRowID` in
   [matrix.go](../../internal/scenario/matrix.go) to accept a leading
   `C` the same way `S` is accepted — recorded as a scenario id
   needing coverage, not passed by like `F`/`A`.
2. Widen `featureTag` in
   [features.go](../../internal/scenario/features.go) to match
   `@S<n>` or `@C<n>`.
3. Add `MatrixIDsAll(paths ...string)` to
   [matrix.go](../../internal/scenario/matrix.go): merges every
   document's ids, refusing an id two documents both claim.
4. Update `bijection_test.go` to call `MatrixIDsAll` over both
   catalog documents instead of the one hardcoded `matrixPath`.
5. Bind `@C1`'s steps in a new
   `cmd/frit/bdd_commands_test.go`, registered from `init` like every
   section — a single-host `Given`/`When`/`Then`, no clone, no second
   machine.
6. Note the second catalog in
   [docs/development.md](../../docs/development.md)'s executable
   scenario matrix section: where it lives, its own `@C<n>` tag, one
   bijection gate covering both.

**Guard the edges.** `F`/`A` rows must still pass `collectRowID`
uncounted — a pre-existing lease-protocol row must not start
demanding a scenario it never needed. An id claimed by both catalogs
must fail loudly, not silently prefer one. The command scenario's
`world` and its steps stay in their own file; nothing here touches
`bdd_lease_test.go` or `bdd_host_death_and_races_test.go`. `frit
release`'s own behavior is untouched — this phase writes a scenario
over code that already passes `TestReleaseIsANoOpOnAnAbsentPlan`.

**Gate.** `go test ./internal/scenario` is green over both catalogs.
`go test ./cmd/frit -run 'TestFeatures/^C1:'` runs the new scenario
against the built dispatch. `go test ./cmd/frit -run
'TestFeatures/^S'` confirms every lease-protocol scenario, S93
included, is untouched. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green, and
`mdsmith check .` is clean.
