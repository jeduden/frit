---
n: 1
title: internal/report reaches 100% line coverage and is locked there
status: "🔲"
result: false
---
Drive [internal/report](../../internal/report) to 100% line coverage
with native `go test -cover`, and gate it so it holds. This closes the
easiest, largest slice of the gap first and fixes the pattern the later
line-coverage phases copy.

**BDD coverage.** None applies. This phase adds unit tests and a
coverage gate; its measure is `go test -cover` and the suite, not a
scenario.

**Assumes.** `go test -cover` measures line coverage natively.
[internal/report](../../internal/report) is pure document construction
— no git, no herdr, no terminal — so it reaches 100% with plain
constructor-and-read tests. Its `AddProblem` and `Warn` helpers in
`discovery.go` and `dispatch.go` are run by no test today, and several
document methods are only partly run. Other tools are version-pinned in
[tools/go.mod](../../tools/go.mod).

**Value.** The largest single-package slice of the gap closes first,
with the native ruler and no new tooling. Every later line-coverage
phase copies the constructor-and-read approach and the per-package
gate; branch coverage is a later stage, not this one.

**RED.** Record the package's current line coverage: `go test
./internal/report -coverprofile`, then `go tool cover -func` to list
every function under 100% and every uncovered line. That list is the
worklist. Confirm `go test ./internal/report` is green first, so
coverage is the only thing changing.

**GREEN.** Write unit tests that construct each report document and run
its uncovered lines — a problem carried and none, a warning set and
unset, an empty list and a populated one. Cover `AddProblem` and `Warn`
directly. Re-run `go test ./internal/report -cover` until it reports
100%.

**GREEN, the gate.** Add a CI step, or a `go test` wrapper, that fails
when `internal/report` drops below 100% line coverage. Adding a
throwaway untested line to the package must redden it; remove the
throwaway once seen.

**Guard the edges.** Keep the gate scoped to `internal/report` in this
phase — the later packages are their own phases, and `cmd/frit`'s
process boundary is a later phase's problem. Do not exclude anything
here: the package has no syscall, so an uncovered line is a missing
test, not a boundary. Branch coverage is out of scope for this phase;
100% lines is the target.

**Gate.** `go test ./internal/report -cover` reports 100%, and the new
gate reddens when a throwaway untested line is added. `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are green.
