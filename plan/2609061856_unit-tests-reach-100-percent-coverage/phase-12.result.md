---
n: 12
title: cmd/frit/main.go and progress.go reach 100% of their reachable lines
status: "✅"
result: true
summary: >-
  cmd/frit/main.go's second half and progress.go reached 100% of their
  reachable lines — fifty-plus new tests, one dead branch deleted, one
  decision extracted, four exclusions listed — but a scope gap surfaced:
  claim.go, dispatch.go and drift.go were never assigned to any phase,
  so cmd/frit does not join the CI gate yet.
---
## Handoff

`go tool cover -func` filtered to `main.go`/`progress.go` started this
phase with every function from `resolveSelector` through `newParser`
partial. Every gap closed as the phase spec expected:

- `resolveSelector`'s and `phaseCmd.Run`'s own `os.Getwd` failures both
  needed a direct call rather than the full CLI: `run()`'s own
  `newParser` call also calls `os.Getwd` first, so removing the
  process's cwd to fail one of these deeper calls through the CLI
  always fails `newParser` first instead.
- `laneOverride`'s three gaps (`os.Getwd`, `os.ReadFile`,
  `planmeta.Parse`) each closed with a direct call.
- `folderPlanPhases`'s `planmeta.PhasesFromDir` error closed with a
  folder-plan path whose directory does not exist — `os.ReadDir`'s
  own error, simpler than it looked from the line range alone.
- `order`'s empty-`Sort` branch closed once, directly, and reused by
  every list command's own `--sort bogus`/plain-success gaps, each
  still needing its own CLI-level test since each `Run` method's own
  call site is a separate coverage point.
- `readyCmd.Run`, `pickCmd.Run` (plus its own `start`/`emptyStart`),
  `rescueRefsFor`, `nextCmd.Run`, `showCmd.Run`, `boardCmd.Run`,
  `findCmd.Run`: each closed with `gatherFleet`'s R1, `--sort bogus`,
  or a JSON/plain-render variant no existing test combined.
- `phaseCmd.Run`'s five gaps: `gatherFleet`'s error, an unresolvable
  selector, `os.Getwd`, a plan file removed after `CurrentLane`
  resolved it, and `planmeta.Resume`'s own error — the last needing a
  spec file that is *listed* by the directory read but cannot be
  *read back* (a permission bit, not a missing or misclassified
  entry: `phaseSpecNumbers` skips directories outright, so a directory
  standing in for the file is pruned before `Resume` ever tries it).
- `liveByBranch`, `selectBoardColumns`, `printBoard`, `fitBoard`,
  `allocateFlex`, `terminalWidth`'s reachable half, `printNext`,
  `printPhase`, `printRescue`, `printDep`, `emptyDepsNote`,
  `statusLabel`, `fitLastColumn`: each closed with a direct call.
- `allocateFlex`'s `held<0` clamp deleted as dead code, matching
  [phase 11](phase-11.md)'s own finding in `orphansCmd.Run`.
- `run()`'s recover decision extracted into `exitCodeFromPanic(r any)
  (code int, matched bool)`, unit-tested directly; the bare `panic(r)`
  re-raise itself stays excluded.

**The four listed exclusions**, added to `scripts/coverage-exclude.txt`:
`main()` itself (`os.Exit` wrapping the tested `run()` seam),
`terminalWidth`'s `term.GetSize` tail (needs a real controlling
terminal), `progressFor`'s matching real-terminal branch, and `run()`'s
non-`exitCode` re-panic.

**The scope gap.** Once every gap above closed, `go test ./cmd/frit
-cover` still read short of 100% even accounting for the four
exclusions. The remainder is in three files this plan's task list never
named: `claim.go`, `dispatch.go` and `drift.go` — fifty zero-count
ranges between them, comparable in size to a full file-phase like
[phase 10](phase-10.md)'s `start.go`. The plan's task 2 text said "one
phase per file (`release.go`, `reap.go`, `yield.go`, `start.go`,
`main.go`+`progress.go`)" and stopped there; `cmd/frit` actually holds
nine non-test files, not six. `scripts/check-coverage.sh` measures the
whole package, so it cannot honestly gate `cmd/frit` until these three
close too — that work is phase 13, not yet written when this phase
closed.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are both green. `./cmd/frit` is still not in `scripts/check-coverage.sh`'s
CI call.

**Inherited by phase 13:** `claim.go`, `dispatch.go` and `drift.go`
need their own RED/GREEN pass, then `./cmd/frit` is added to the CI
call and the exclusion list is checked for completeness across the
whole package for the first time — a fresh `go tool cover -func`
filtered to those three files is the right starting point, the same
way phase 11 started this file's own worklist.
