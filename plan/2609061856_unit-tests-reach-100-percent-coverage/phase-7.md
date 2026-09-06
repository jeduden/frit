---
n: 7
title: cmd/frit/main.go and progress.go reach 100% line coverage and join the gate
status: "🔲"
result: false
---
Drive [cmd/frit/main.go](../../cmd/frit/main.go) and
[cmd/frit/progress.go](../../cmd/frit/progress.go) to 100% line
coverage. `cmd/frit` sits at 90.0% today, concentrated in these two
files. Add `cmd/frit`'s testable set to `scripts/check-coverage.sh`'s
CI call, alongside the six packages already there. This is the first
of five `cmd/frit` phases, one per file. The package's gap is far
wider than any package closed so far: main.go alone carries over a
hundred uncovered ranges across roughly forty functions.

**BDD coverage.** None applies. This phase adds unit tests and, where
named below, a small seam — no lease-protocol behavior changes, so no
`@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `main.go` already carries the seam the plan's own context
expected to have to add. `main()` delegates everything to `run(args,
stdout, stderr) int`, and every existing test already drives `run`
directly. `main()` itself — three lines, `os.Exit(run(...))` — is the
only line in the file that is a genuine process boundary. `remoteGit`
looked like a second one, but it shells out through `herdr.Run("ssh",
...)`. That is the same general-purpose runner `internal/herdr`'s
`Exec` uses for `"herdr"`, and [phase 6](phase-6.md) already proved it
testable by putting a throwaway script first on `$PATH`; the same
trick reaches `remoteGit` with a fake `ssh`. `terminalWidth` and
`progressFor`/`isTerminalWriter` in progress.go are the real boundary.
The branch where the writer is a live interactive terminal needs a
real tty, which this repo has no pty dependency for today — `creack/pty`
sits in `go.sum` only as another tool's indirect dependency. That
branch is a listed exclusion. The "writer is a plain `*os.File` that
is not a terminal" branch (a temp file, `/dev/null`) is not a boundary
and stays in scope.

**Value.** main.go wires every verb's fleet gather, board and report
rendering — it is the widest single file in the codebase and the
largest remaining slice of the plan's whole gap. Closing it (bar the
one listed exclusion) proves the entrypoint pattern the remaining four
`cmd/frit` phases repeat.

**RED.** `go test ./cmd/frit -coverprofile` and the raw profile's
zero-count ranges are the worklist, enumerated in this phase's result.
They fall into four recurring shapes:

- **Sequential error guards.** Most gaps are an `if err != nil { return
  ... }` after a call to an injected `rt.git`, `rt.gitPipe`, `rt.herdr`,
  `repocfg.Load` or a `gitobj`/`plans`/`index` helper — `repoLanes`,
  `gatherFleetOpts`, `printPlans`, `printDoctor`, `whoLanes`,
  `holdsForRoot`, `laneOverride`, `folderPlanPhases`, `resolveSelector`,
  `emptyStart`, `rescueRefsFor`, `liveByBranch`, `gitForHost` and
  several `Run` methods among them — never exercised because every
  existing test feeds a runner that succeeds. `bdd_landed_evidence_test
  .go`'s `failingLsRemote` already shows the shape: wrap the runner and
  fail one named subcommand, pass everything else through to the real
  fake.
- **JSON-vs-table and doc-state branches** in the `Run` methods and the
  `print*` renderers (`printNext`, `printPhase`, `printRescue`,
  `printDep`, `emptyDepsNote`, `statusLabel`, `order`): call the
  function with the struct field or `c.JSON` value existing tests never
  set, the same direct-input shape [phase 1](phase-1.md) used for
  `internal/report`.
- **Board layout edges** (`fitBoard`, `allocateFlex`, `selectBoardColumns`,
  `printBoard`, `fitLastColumn`): a width, column count or overflow
  existing golden tables never hit.
- **The process boundary** above: `main`, `remoteGit`, `terminalWidth`,
  `progressFor`/`isTerminalWriter`.

**GREEN, the tests.** Cover each guard and branch by its own shape —
a failing-subcommand runner fake for the git/herdr-fault branches, a
`c.JSON`/struct-field variant for the rendering branches, a narrow or
wide width for the board-layout branches. For `remoteGit`: a
`t.TempDir()` script named `ssh` on `$PATH` via `t.Setenv`, mirroring
phase 6's `herdr` fake, asserting the composed `ssh <host> git -C <dir>
...` args reach it. For `terminalWidth`/`isTerminalWriter`'s
non-terminal-`*os.File` branch: open a real file (`os.Open(os.DevNull)`
or a `t.TempDir()` file) and pass it in — `term.IsTerminal` answers
false for it without any tty. Re-run `go test ./cmd/frit -cover` and
inspect `main.go`/`progress.go`'s own function list until each reports
100% but for the listed exclusion.

**GREEN, the gate.** `scripts/check-coverage.sh` today requires exactly
`100.0%`. `cmd/frit` is one package with five files' worth of gap. This
phase closes one file, so `go test ./cmd/frit -cover` still reads
below 100% afterward. reap.go, release.go, start.go and yield.go carry
the rest, and the file's own listed exclusion caps the package under
100% even once every file is done. Give the script a per-package
minimum instead — a `pkg=pct` pair, defaulting to `100.0` when bare —
rather than loosen the check for every package, so the six
already-gated packages keep demanding exact 100%. Add `cmd/frit` to
the CI call in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml) at whatever
percentage it reads once this phase's tests land — a ratchet, not yet
the ceiling — recorded in this phase's result; each of phases 8
through 11 raises that same number as its file closes, and phase 11
sets it at its final, post-exclusion value.

**Guard the edges.** One listed exclusion:
`terminalWidth`/`progressFor`'s real-terminal branch in
`cmd/frit/main.go` and `cmd/frit/progress.go`. Reaching
`term.GetSize` succeeding needs a real interactive tty or a pty
dependency this repo does not carry. `main()` itself is `os.Exit`
wrapping the already-tested `run` seam, and is conventionally excluded
the same way, not separately listed as a defect. Both get a one-line
comment at the excluded line and an entry in this phase's result.

**Gate.** `go test ./cmd/frit -cover` reports 100% but for the listed
exclusion; the CI gate call covers `cmd/frit` alongside every package
already there and reddens on an added untested line in any of them;
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are green.
