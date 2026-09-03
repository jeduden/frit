---
n: 5
title: The two doc-by-argument rows S42 and S44 run for real
status: "✅"
result: false
---
S42 and S44 are the last two storage rows. They are the only ones the
plan resolves by argument, not by anomaly. Each says a shape is
"unsupported, documented." The scenario asserts the observable that
documents it, never a comment. Both are the first rows this section
builds with two remotes in play at once. Every prior row shared one
bare origin per scenario.

**Assumes.** The coordination remote is the one declared in
`.frit.yml`'s `remote:` field, read through
[internal/repocfg](../../internal/repocfg). `repocfg.Load` and its
`Compiled()` accessor are already read by `repoLanes` in
[cmd/frit/main.go](../../cmd/frit/main.go). A lease lands on the
configured remote alone. A second git remote on the same clone is never
consulted. A fork's own `origin` is never the coordination point.

**Value.** S42 pins that a second git remote cannot split coordination.
A lease still lands on the configured remote, so an operator who adds a
mirror or a backup remote never forks the arbitration key. S44 pins
that a fork-based flow coordinates through the shared remote, not the
fork's origin. Work pushed from a fork's checkout lands on the
configured remote, and the fork's own origin carries no work ref. A
regression that let either become the coordination point would fail the
build.

**RED.** Drop `@pending` from S42 and S44 in
[storage.feature](../../features/storage.feature) and write:

- S42, two remotes, split coordination. Given "box-a" holds the lease
  for plan 42 on the configured remote, when a person adds a second git
  remote to "box-a"'s clone, then a lease renewal still lands on the
  configured remote alone, and the second remote carries no work ref.
- S44, fork-based flow. Given a fork whose own origin is not the
  configured coordination remote, when "box-a" acquires the lease from
  a checkout of the fork, then the lease lands on the configured
  remote, and the fork's own origin carries no work ref for the plan.

Run `go test ./cmd/frit -run 'TestFeatures/S(42|44):'`. Strict mode
reports the new steps undefined and both subtests fail. That is the
red — commit it.

**GREEN.** Add to `cmd/frit/bdd_storage_test.go` the two-remote
fixtures phase 3's handoff names. For S42, a "second git remote never
consulted" step asserts a renewal lands on the configured remote and
the second carries nothing. For S44, a fork-shaped clone — a second bare
repository whose `origin` is not the fleet's shared coordination
remote — asserts a lease pushed from the fork lands on the configured
remote while the fork's origin carries no work ref. Every step function
ships with a unit test of its own, per CLAUDE.md.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(42|44):'` passes
with both subtests PASS and no SKIP. `go test ./cmd/frit -run
TestFeatures/S` reports S37..S44, S67..S69, S71 and S78 all PASS, with
no remaining SKIP. The bijection gate, `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` stay clean. This is the plan's
last phase. Tick the met Acceptance Criteria, flip `status:` to ✅, and
run `mdsmith fix PLAN.md`.

Write the handoff to `phase-5.result.md`. Say that every storage row
now runs, no scenario carries `@pending`, and the plan's Goal is met.
