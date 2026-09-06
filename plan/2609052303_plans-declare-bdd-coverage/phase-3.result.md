---
n: 3
title: A command behavior gets a BDD home of its own
status: "✅"
result: true
summary: C1 proves a second catalog; MatrixIDsAll keeps both in one bijection gate
---
## Handoff

A command behavior that is not a lease-protocol scenario now has a
home: [command-scenarios.md](../../docs/research/command-scenarios.md),
tagged `@C<n>` instead of `@S<n>`, catalogued the same way and gated
by the same test. `C1` proves it — `frit release` on a plan nobody
ever held reports a no-op, not a refusal, on one host, no lease race
in sight, matching the already-passing
`TestReleaseIsANoOpOnAnAbsentPlan`.

**RED.** Adding the `C1` row and its `@C1` scenario before any code
changed failed `go test ./internal/scenario` with "carries 0 S tags,
want exactly one" — `featureTag` recognized only `@S<n>`, and
`bijection_test.go` read only `lease-protocol.md`. That is the
single-catalog design this phase replaces.

**GREEN.** Six changes land the second catalog:

- `rowID` and `collectRowID` in
  [matrix.go](../../internal/scenario/matrix.go) now accept a leading
  `C` the same way `S` is accepted; `F`/`A` attacker rows still pass
  by uncounted.
- `featureTag` in
  [features.go](../../internal/scenario/features.go) matches `@S<n>`
  or `@C<n>`.
- `MatrixIDsAll(paths ...string)` merges every catalog document's ids,
  refusing an id two documents both claim.
- [bijection_test.go](../../internal/scenario/bijection_test.go) reads
  both `lease-protocol.md` and `command-scenarios.md` through
  `MatrixIDsAll`.
- `@C1`'s steps live in a new
  [bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go): a
  single-host `Given`/`When`/`Then`, no clone, driving the real `frit
  release` CLI.
- [docs/development.md](../../docs/development.md)'s executable
  scenario matrix section names the second catalog and
  `MatrixIDsAll`.

**Gate.** `go test ./internal/scenario` is green over both catalogs,
with new unit tests on `collectRowID`, `MatrixIDsAll` and
`featureTag`'s `@C` case. `go test ./cmd/frit -run 'TestFeatures/^C1:'`
runs the new scenario against the built dispatch.
`go test ./cmd/frit -run 'TestFeatures/^S'` passes all 93
lease-protocol scenarios, S93 included, untouched. `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` (291 files) are all green.

**Plan closed.** All three phases land: S93 proved the shape, Phase 2
made the decision explicit in CLAUDE.md, plan/proto.md and plan-new,
and Phase 3 gave a non-protocol behavior its own catalog. Every
Acceptance Criterion is met.
