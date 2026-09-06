---
n: 9
title: cmd/frit/release.go reaches 100% line coverage and raises the cmd/frit gate
status: "🔲"
result: false
---
Drive [cmd/frit/release.go](../../cmd/frit/release.go) to 100% line
coverage — six zero-count ranges across `Run`, `releaseHeld` and
`printRelease` today. Raise `cmd/frit`'s ratchet in
`scripts/check-coverage.sh`'s CI call to match.

**BDD coverage.** None applies. `release` ends a lease, but every gap
here is an existing path fed an input its current tests never
construct, not a new lease-protocol behavior. No `@S<n>` scenario is
needed per [docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** No new process boundary; every gap sits behind `rt.git`,
`gatherFleet` or `resolveSelector`, already faked elsewhere in
`cmd/frit`'s suite.

**Value.** release is the lease's own clean-exit path; its two
uncovered branches are the exact places a bug would silently release
a plan release should have refused, or fail to release one it should
have.

**RED.** `go test ./cmd/frit -coverprofile` and release.go's zero-count
ranges are the worklist, enumerated in this phase's result:

- `Run` (80.0%): the `gatherFleet` error return (30-32), the
  `resolveSelector` error return (42-44), and the `ambiguousRepo`
  branch (52-56) where `res.Coords` has no entry for the plan's repo —
  each an early return existing tests never trigger because the fleet
  and selector fakes they build always succeed and always resolve to a
  known repo.
- `releaseHeld` (80.0%, lines 105-109): the `claim.Release` error
  branch — existing tests only exercise it succeeding.
- `printRelease` (80.0%, lines 223-228): a rendering branch existing
  golden tests never feed.

**GREEN, the tests.** `Run`'s three guards: a `gatherFleet` fed a
failing git subcommand, a `resolveSelector` fed an unresolvable
selector, and a fleet result whose `Coords` map omits the plan's repo
(construct the `fleetResult` directly rather than through a real
gather, the same shape other `cmd/frit` tests already use to hit
`ambiguousRepo`-shaped branches). `releaseHeld`: a `claim.Release`
reached through a runner that fails the push. `printRelease`: the
struct field combination its golden tests do not yet cover. Re-run `go
test ./cmd/frit -cover` and check release.go's own function list in
`go tool cover -func` until it reads 100%.

**Guard the edges.** No exclusion list entry, no seam.

**Gate.** `go test ./cmd/frit -cover` reads higher than phase 8 left
it; the CI ratchet in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml) is raised to
match, recorded in this phase's result; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are green.
