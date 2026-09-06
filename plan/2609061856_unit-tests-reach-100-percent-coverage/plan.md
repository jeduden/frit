---
id: 2609061856
title: Unit tests reach 100% line coverage, then exercise every branch
status: "🔲"
summary: >-
  The full suite covers 92.1% of statements. Close that first: the Go
  toolchain measures line coverage natively, twenty-one functions are
  run by no test, and one hundred eighty-two are only partly run. Most
  is plainly reachable — six Error stringers, a repocfg String, and the
  report documents' AddProblem and Warn helpers sit at zero, and the
  partial functions are lines a fake fed the right input would run.
  Reach 100% line coverage package by package, gating each at 100% so
  it holds. Then raise the ruler: line coverage counts statements, so a
  one-sided if, a short-circuited && or ||, or an untaken early return
  can read as covered while one outcome never ran. A later stage adopts
  a branch-coverage tool — gobco or equivalent, since go test cannot
  report branches — and drives every reachable condition both ways. A
  few conditions sit on a process boundary — main's wiring, the raw git
  and herdr syscalls, the terminal reporter — and get a seam where
  cheap or a listed, justified exclusion.
model: sonnet
depends-on: []
---
# Unit tests reach 100% line coverage, then exercise every branch

## Goal

First, every line a unit test can reach is covered — 100% line
coverage, gated so it holds. Then the stronger ruler: every condition a
unit test can reach is taken both ways, measured by a branch-coverage
tool the Go toolchain does not provide. The few conditions on a pure
process boundary are made testable through a seam, or carved out with a
listed, justified exclusion.

## Context

**Line coverage first, because the toolchain gives it.** `go test ./...
-coverpkg=./...` reports 92.1% of statements natively, so line coverage
is the ruler in hand and the first target. Twenty-one functions are at
0%, one hundred eighty-two are partial. codecov.yml ratchets the
project on `target: auto` — coverage may only improve — so the gap
neither closes nor regresses on its own; the early phases close it and
raise the target.

**Most of it is plainly reachable.** The zero-coverage set is mostly
trivial surface: the six `Error` methods on the lease error types in
[internal/claim/lease.go](../../internal/claim/lease.go), `String` in
[internal/repocfg](../../internal/repocfg/pattern.go), and the
`AddProblem` and `Warn` helpers the report documents carry in
[internal/report](../../internal/report). A test that constructs the
value and reads it covers each. The partial functions are uncovered
lines — a refusal wording, an error path, a defensive guard. The
defensive-code rule in [CLAUDE.md](../../CLAUDE.md) holds that a branch
is added only when a test can drive it red then green, so each
uncovered line has an input that reaches it; the fakes to feed it —
`startHerdr`, `liveLaneHerdr`, the git runners — already exist.

**Then the stronger ruler: branches.** 100% line coverage is not every
branch. The toolchain counts statements, so a branch can be missed
while the line reads covered: an `if` with no `else` whose condition
never goes false, a short-circuited `&&` or `||`, an early `return`
never taken. A later stage adopts a branch-coverage tool — `gobco`
(github.com/rillig/gobco) instruments each condition and reports the
ones taken only one way, or an equivalent — pins it the way
[tools/go.mod](../../tools/go.mod) pins the others, and makes its report
the stronger gate laid over the line gate.

**A few conditions sit on a process boundary.** `main` in
[cmd/frit/main.go](../../cmd/frit/main.go) wires kong and calls
`os.Exit`; `remoteGit` and `herdr.Exec` are the raw git and process
syscalls every test replaces with a fake; the reporter in
[internal/fleet/progress.go](../../internal/fleet/progress.go) writes
to a terminal. Forcing a test through a real syscall proves nothing
about frit's logic. Each gets one of two honest treatments: a thin seam
that lets a test drive the logic while the one syscall stays out of
reach, or an explicit entry in an ignore list with a one-line reason.
The seam is preferred where cheap; the exclusion is for the irreducible
boundary alone, and it is listed, not silent.

**Out of scope.** No behavior change — this plan adds tests and a seam
or two, never new features. The exclusion list stays minimal and
justified; it is not a place to hide untested logic.

## Tasks

1. Phase 1 (proving slice): drive
   [internal/report](../../internal/report) to 100% line coverage — its
   zero-coverage `AddProblem`/`Warn` helpers and its partial lines —
   with native `go test -cover`, and add the gate that reddens if the
   package later drops below 100%. It has no process boundary, so it
   reaches 100% cleanly and fixes the pattern the later packages copy.
2. Line-coverage phases: `internal/claim`, `internal/fleet`,
   `internal/observe`, `internal/repocfg` and `internal/herdr` to 100%;
   then `cmd/frit`, where `main`, `remoteGit` and the reporter get a
   seam or a listed exclusion, and the target is raised to 100% of the
   remainder.
3. Branch-coverage phases: adopt and pin a branch-coverage tool, then
   drive each package so every reachable condition is taken both ways,
   with the tool's report as the stronger gate.

## Execution

| Phase | Title                                                          | Tier   | Gate                                                                                                                   |
| ----- | -------------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------- |
| 1     | internal/report reaches 100% line coverage and is locked there | sonnet | `go test ./internal/report -cover` reports 100%; the new gate reddens on an added untested line; `go test ./...` green |

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
| 1   | 🔲     | [internal/report reaches 100% line coverage and is locked there](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] Every unit-testable line is covered; `go test ./...
      -coverpkg=./...` reports 100% of the non-excluded set
- [ ] Each package's line coverage is gated at 100% so a later
      untested line reddens the check
- [ ] A branch-coverage tool is adopted and pinned, and every
      reachable condition outside the exclusion list is taken both ways
- [ ] Each excluded item is a pure process boundary — the entrypoint or
      a raw syscall — listed with a one-line reason, and nothing else
- [ ] Where a seam was cheaper than an exclusion, a test drives the
      logic and only the syscall stays out of reach
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
