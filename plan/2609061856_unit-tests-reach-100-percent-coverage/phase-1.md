---
n: 1
title: internal/report hits every branch, under a tool that proves it
status: "🔲"
result: false
---
Adopt a branch-coverage tool and prove it on one package. Drive
[internal/report](../../internal/report) so every condition it holds is
taken both ways, and make the tool's clean report a gate. This fixes
the tool, the pinning and the pattern the later packages copy.

**BDD coverage.** None applies. This phase adds unit tests and a
coverage tool; its gate is the branch report and the suite, not a
scenario.

**Assumes.** The Go toolchain measures statements, not branches, so
`go test -cover` cannot prove a condition was taken both ways.
[internal/report](../../internal/report) is pure document construction
— no git, no herdr, no terminal — so it reaches full branch coverage
with plain constructor-and-read tests. Its `AddProblem` and `Warn`
helpers in `discovery.go` and `dispatch.go` are run by no test today.
Other tools are version-pinned in
[tools/go.mod](../../tools/go.mod) and run via
`go tool -modfile=tools/go.mod`.

**Value.** The package proves the ruler works: a tool that flags a
one-sided condition, pinned and wired as a gate, on a package clean
enough to reach zero one-sided conditions. Every later package copies
the tool invocation and the gate; none has to re-litigate the
approach.

**RED.** Evaluate `gobco` (github.com/rillig/gobco), or an equivalent
that reports condition outcomes, against `internal/report`. Its first
run is the RED: it lists the untested helpers and every condition taken
only one way. Capture that list — it is the phase's worklist. Confirm
`go test ./internal/report` is green first, so the branch report is the
only thing changing.

**GREEN.** Write unit tests that construct each report document and
exercise both outcomes of every condition the tool flags — a problem
carried and none carried, a warning set and unset, an empty list and a
populated one. Cover `AddProblem` and `Warn` directly. Re-run the tool
until it reports no one-sided condition in the package.

**GREEN, the gate.** Pin the tool in [tools/go.mod](../../tools/go.mod)
and add a CI step, or a `go test` wrapper, that runs it over
`internal/report` and fails on any one-sided condition. Adding a throw
away untested branch to the package must redden it; remove the throw
away branch once seen.

**Guard the edges.** Keep the tool's scope to `internal/report` in this
phase — the later packages are their own phases, and `cmd/frit`'s
process boundary is not addressed here. Do not exclude anything in this
package: it has no syscall, so a one-sided condition here is a missing
test, not a boundary. Statement coverage for the package should read
100% as a side effect; if it does not, a branch is still uncovered.

**Gate.** The branch tool reports no one-sided condition in
`internal/report`, and reddens when a throwaway untested branch is
added. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are green.
