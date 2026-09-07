---
id: 2609061856
title: Unit tests reach 100% line coverage, then exercise every branch
status: "🔳"
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
`os.Exit`; `herdr.Exec` is a raw process syscall every test replaces
with a fake; the reporter in
[cmd/frit/progress.go](../../cmd/frit/progress.go) writes to a
terminal. Forcing a test through a real syscall proves nothing about
frit's logic. Each gets one of two honest treatments: a thin seam that
lets a test drive the logic while the one syscall stays out of reach,
or an explicit entry in an ignore list with a one-line reason. The seam
is preferred where cheap; the exclusion is for the irreducible boundary
alone, and it is listed, not silent. `remoteGit` turned out not to be
one of these — like `herdr.Exec` before it, it resolves its subprocess
name through `$PATH`, so a fake on `$PATH` drives it for real (plan
phase 11).

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
   then `cmd/frit`, one phase per file (`release.go`, `reap.go`,
   `yield.go`, `start.go`, `main.go`+`progress.go`, then `claim.go`,
   `dispatch.go` and `drift.go` — three more files this task's own
   file list missed on the first pass, found only once every named
   file's own gap had closed and the package still read short of
   100%). `yield.go` builds and proves the exclusion-list mechanism
   `scripts/check-coverage.sh` needs; the later files add their own
   entries to it and, where a gap turns out to be dead code rather
   than a missing test, delete the branch instead. The last file adds
   `cmd/frit` to the gate.
3. Branch-coverage phases: adopt and pin a branch-coverage tool, then
   drive each package so every reachable condition is taken both ways,
   with the tool's report as the stronger gate.

## Execution

| Phase | Title                                                                                            | Tier   | Gate                                                                                                                              |
| ----- | ------------------------------------------------------------------------------------------------ | ------ | --------------------------------------------------------------------------------------------------------------------------------- |
| 1     | internal/report reaches 100% line coverage and is locked there                                   | sonnet | `go test ./internal/report -cover`: 100%; new gate; suite green                                                                   |
| 2     | internal/claim reaches 100% line coverage and joins the gate                                     | sonnet | `go test ./internal/claim -cover`: 100%; gate covers every closed package; suite green                                            |
| 3     | internal/fleet reaches 100% line coverage and joins the gate                                     | sonnet | `go test ./internal/fleet -cover`: 100%; gate covers every closed package; suite green                                            |
| 4     | internal/observe reaches 100% line coverage and joins the gate                                   | sonnet | `go test ./internal/observe -cover`: 100%; gate covers every closed package; suite green                                          |
| 5     | internal/repocfg reaches 100% line coverage and joins the gate                                   | sonnet | `go test ./internal/repocfg -cover`: 100%; gate covers every closed package; suite green                                          |
| 6     | internal/herdr reaches 100% line coverage and joins the gate                                     | sonnet | `go test ./internal/herdr -cover`: 100%; gate covers every closed package; suite green                                            |
| 7     | cmd/frit/release.go reaches 100% line coverage                                                   | sonnet | `go tool cover -func` filtered to release.go: 100%; suite green                                                                   |
| 8     | cmd/frit/reap.go reaches 100% line coverage                                                      | sonnet | `go tool cover -func` filtered to reap.go: 100%; two dead branches removed; suite green                                           |
| 9     | cmd/frit/yield.go reaches 100% of its reachable lines, check-coverage.sh gains an exclusion list | sonnet | `go tool cover -func` filtered to yield.go: 100% but one listed exclusion; exclusion mechanism proven red then green; suite green |
| 10    | cmd/frit/start.go reaches 100% of its reachable lines                                            | sonnet | `go tool cover -func` filtered to start.go: 100% but one listed exclusion; suite green                                            |
| 11    | cmd/frit/main.go reaches 100% line coverage on its discovery, doctor and plans verbs             | sonnet | `go tool cover -func` filtered to main.go's first half: 100%; one dead branch removed; suite green                                |
| 12    | cmd/frit/main.go and progress.go reach 100% of their reachable lines                             | sonnet | `go tool cover -func` filtered to main.go/progress.go: 100% but for four listed exclusions; suite green                           |
| 13    | cmd/frit/claim.go, dispatch.go and drift.go close the package, which joins the gate              | sonnet | `go test ./cmd/frit -cover`: 100% but for the listed exclusions; `cmd/frit` joins the CI gate; suite green                        |

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

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                            |
| --- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [internal/report reaches 100% line coverage and is locked there](phase-1.md)                                                                                                                                                                                                                                                     |
|     | ↳      | internal/report reached 100% line coverage and gained a hard CI gate that reddens the moment it drops.                                                                                                                                                                                                                           |
| 2   | ✅     | [internal/claim reaches 100% line coverage and joins the gate](phase-2.md)                                                                                                                                                                                                                                                       |
|     | ↳      | internal/claim reached 100% line coverage — one seam, thirty-odd new tests — and joined the hard CI gate beside internal/report.                                                                                                                                                                                                 |
| 3   | ✅     | [internal/fleet reaches 100% line coverage and joins the gate](phase-3.md)                                                                                                                                                                                                                                                       |
|     | ↳      | internal/fleet reached 100% line coverage — twenty-five new tests, no seam needed — and joined the hard CI gate beside internal/report and internal/claim.                                                                                                                                                                       |
| 4   | ✅     | [internal/observe reaches 100% line coverage and joins the gate](phase-4.md)                                                                                                                                                                                                                                                     |
|     | ↳      | internal/observe reached 100% line coverage — one seam over its atomic write, seven new tests — and joined the hard CI gate beside internal/report, internal/claim and internal/fleet.                                                                                                                                           |
| 5   | ✅     | [internal/repocfg reaches 100% line coverage and joins the gate](phase-5.md)                                                                                                                                                                                                                                                     |
|     | ↳      | internal/repocfg reached 100% line coverage — four new tests, no seam needed — and joined the hard CI gate beside internal/report, internal/claim, internal/fleet and internal/observe.                                                                                                                                          |
| 6   | ✅     | [internal/herdr reaches 100% line coverage and joins the gate](phase-6.md)                                                                                                                                                                                                                                                       |
|     | ↳      | internal/herdr reached 100% line coverage — six new tests, no seam and no exclusion needed — and joined the hard CI gate beside internal/report, internal/claim, internal/fleet, internal/observe and internal/repocfg.                                                                                                          |
| 7   | ✅     | [cmd/frit/release.go reaches 100% line coverage](phase-7.md)                                                                                                                                                                                                                                                                     |
|     | ↳      | cmd/frit/release.go reached 100% line coverage — six new tests, no seam and no exclusion needed — proving the fixture-reuse approach extends cleanly into cmd/frit's first file.                                                                                                                                                 |
| 8   | ✅     | [cmd/frit/reap.go reaches 100% line coverage](phase-8.md)                                                                                                                                                                                                                                                                        |
|     | ↳      | cmd/frit/reap.go reached 100% line coverage — eleven new tests and two dead branches deleted, no seam or exclusion needed — proving a coverage gap is not always a missing test.                                                                                                                                                 |
| 9   | ✅     | [cmd/frit/yield.go reaches 100% of its reachable lines, and check-coverage.sh gains exclusions](phase-9.md)                                                                                                                                                                                                                      |
|     | ↳      | cmd/frit/yield.go reached 100% of its reachable lines — seven new tests and one listed exclusion — and scripts/check-coverage.sh now understands a declared, justified exclusion list, proven red then green against a scratch fixture before the real entry was added.                                                          |
| 10  | ✅     | [cmd/frit/start.go reaches 100% of its reachable lines](phase-10.md)                                                                                                                                                                                                                                                             |
|     | ↳      | cmd/frit/start.go reached 100% of its reachable lines — twenty new tests, one dead branch deleted, and one listed exclusion — the densest of the five cmd/frit files.                                                                                                                                                            |
| 11  | ✅     | [cmd/frit/main.go reaches 100% line coverage on its discovery, doctor and plans verbs](phase-11.md)                                                                                                                                                                                                                              |
|     | ↳      | cmd/frit/main.go's first half — repoLanes through carryHostProblems — reached 100% line coverage: forty-plus new tests, one seam (hostname), one dead branch deleted, no exclusions.                                                                                                                                             |
| 12  | ✅     | [cmd/frit/main.go and progress.go reach 100% of their reachable lines](phase-12.md)                                                                                                                                                                                                                                              |
|     | ↳      | cmd/frit/main.go's second half and progress.go reached 100% of their reachable lines — fifty-plus new tests, one dead branch deleted, one decision extracted, four exclusions listed — but a scope gap surfaced: claim.go, dispatch.go and drift.go were never assigned to any phase, so cmd/frit does not join the CI gate yet. |
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
