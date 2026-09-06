---
id: 2609061856
title: Unit tests exercise every branch, not just every statement
status: "🔲"
summary: >-
  The full suite covers 92.1% of statements, but statement coverage is
  the wrong ruler: the Go toolchain counts statements, so an if with no
  else whose condition never goes false, a short-circuited && or ||, or
  an early return can read as covered while one outcome was never run.
  The goal is every branch — both outcomes of every condition. That
  needs a branch-coverage tool, since go test cannot report it; gobco
  instruments each condition and flags one only ever taken one way.
  Most of the work is plainly reachable: six Error stringers, a repocfg
  String, and the report documents' AddProblem and Warn helpers are run
  by no test, and every one-sided branch has an input that flips it,
  which the defensive-code rule already promises. A few conditions sit
  on a process boundary — main's wiring, the raw git and herdr
  syscalls, the terminal reporter — and get a seam where cheap or a
  listed exclusion where the boundary is irreducible. Then the gate
  measures branches, so a one-sided condition reddens rather than
  hiding behind a covered statement.
model: sonnet
depends-on: []
---
# Unit tests exercise every branch, not just every statement

## Goal

Every branch is exercised — both outcomes of every condition a unit
test can reach. The gate measures branch coverage, not statement
coverage, so a condition only ever taken one way reddens rather than
passing behind a covered statement. The few conditions on a pure
process boundary are made testable through a seam, or carved out with a
listed, justified exclusion.

## Context

**Statement coverage is the wrong ruler.** `go test ./...
-coverpkg=./...` reports 92.1% of statements, and the Go toolchain
measures only statements. So a branch can be missed while the line
reads covered: an `if` with no `else` whose condition never goes false,
a short-circuited `&&` or `||` never evaluated to its second operand,
an early `return` never taken. Hitting every branch is a stronger
target than 100% statement coverage, and the toolchain cannot report
it.

**So the gate needs a branch-coverage tool.** `gobco`
(github.com/rillig/gobco) instruments each condition and reports the
ones taken only one way — the exact signal statement coverage hides. An
equivalent tool that measures condition outcomes serves as well. Phase
1 adopts one, pins its version the way the other tools in
[tools/go.mod](../../tools/go.mod) are pinned, and makes its report the
gate. Where a package's branch report is clean, statement coverage in
that package is 100% as a side effect.

**Most branches are plainly reachable.** The functions run by no test
are mostly trivial surface — the six `Error` methods on the lease error
types in [internal/claim/lease.go](../../internal/claim/lease.go),
`String` in [internal/repocfg](../../internal/repocfg/pattern.go), the
`AddProblem` and `Warn` helpers the report documents carry in
[internal/report](../../internal/report). A one-sided condition has an
input that flips it: the defensive-code rule in
[CLAUDE.md](../../CLAUDE.md) holds that a branch is added only when a
test can drive it red then green, so every guard already has a reaching
input. The fakes to feed it — `startHerdr`, `liveLaneHerdr`, the git
runners — already exist.

**A few conditions sit on a process boundary.** `main` in
[cmd/frit/main.go](../../cmd/frit/main.go) wires kong and calls
`os.Exit`; `remoteGit` and `herdr.Exec` are the raw git and process
syscalls every test replaces with a fake; the reporter in
[internal/fleet/progress.go](../../internal/fleet/progress.go) writes
to a terminal. Flipping a condition inside a real syscall proves
nothing about frit's logic. Each gets one of two honest treatments: a
thin seam that lets a test drive the branching while the one syscall
stays out of reach, or an explicit entry in the tool's ignore
configuration with a one-line reason. The seam is preferred where it is
cheap; the exclusion is for the irreducible boundary alone, and it is
listed, not silent.

**Reach and hold.** The branch report becomes a gate in CI, so a later
one-sided condition reddens the check rather than slipping in behind a
covered line. codecov keeps measuring statements as before; the branch
gate is the stronger ruler laid on top, not a replacement.

**Out of scope.** No behavior change — this plan adds tests and a seam
or two, never new features. The exclusion list stays minimal and
justified; it is not a place to hide an untested branch.

## Tasks

1. Phase 1 (proving slice): adopt a branch-coverage tool, pin it, and
   drive [internal/report](../../internal/report) to full branch
   coverage — its untested `AddProblem`/`Warn` helpers and every
   one-sided condition — then make the tool's clean report a gate. The
   package has no process boundary, so it reaches full branch coverage
   cleanly and fixes the tool and pattern the later packages copy.
2. Later phases: `internal/claim`, `internal/fleet`, `internal/observe`,
   `internal/repocfg` and `internal/herdr` to full branch coverage;
   then `cmd/frit`, where `main`, `remoteGit` and the reporter get a
   seam or a listed exclusion.

## Execution

| Phase | Title                                                          | Tier   | Gate                                                                                                                                        |
| ----- | -------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | internal/report hits every branch, under a tool that proves it | sonnet | the branch-coverage tool reports no one-sided condition in `internal/report`; it reddens on an added untested branch; `go test ./...` green |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                        |
| --- | ------ | ---------------------------------------------------------------------------- |
| 1   | 🔲     | [internal/report hits every branch, under a tool that proves it](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] A branch-coverage tool is adopted and version-pinned, and its
      report is the gate — statement coverage alone is no longer the
      measure
- [ ] Every condition a unit test can reach is taken both ways; the
      tool reports no one-sided condition outside the exclusion list
- [ ] Each excluded item is a pure process boundary — the entrypoint or
      a raw syscall — listed with a one-line reason, and nothing else
- [ ] Where a seam was cheaper than an exclusion, a test drives the
      branching and only the syscall stays out of reach
- [ ] The branch gate reddens in CI on a later one-sided condition
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
