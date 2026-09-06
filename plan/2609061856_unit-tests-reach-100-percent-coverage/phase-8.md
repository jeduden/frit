---
n: 8
title: cmd/frit/reap.go reaches 100% line coverage and raises the cmd/frit gate
status: "🔲"
result: false
---
Drive [cmd/frit/reap.go](../../cmd/frit/reap.go) to 100% line coverage
— thirteen zero-count ranges across nine functions today. Raise
`cmd/frit`'s ratchet in `scripts/check-coverage.sh`'s CI call to match.

**BDD coverage.** None applies. Unit tests only, no behavior change.
`reap` tears down deserted worktrees and parks rescue refs, which
touches the lease's own refs, but this phase adds no new claim,
release, yield or takeover path — every case below is an existing
`Run`/helper fed an input its current tests never construct. No
`@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** No new process boundary. reap.go has no direct syscall of
its own; every gap sits behind `rt.git`, `rt.herdr` or a helper already
faked elsewhere in `cmd/frit`'s test suite (`failingLsRemote`'s
wrap-and-fail-one-subcommand shape, the fake herdr socket the board and
dispatch tests already build).

**Value.** reap is the one verb that deletes worktrees and branches;
its uncovered lines are disproportionately the refusal and warning
paths that stop it from deleting the wrong thing — `holdRefusal`,
`strandedForPlan`'s classification guards, `tearDownWorktree`'s error
path. Proving them is worth more here than in a purely additive verb.

**RED.** `go test ./cmd/frit -coverprofile` and reap.go's own
zero-count ranges are the worklist, enumerated in this phase's result.
They group into:

- **Sequential error guards** in `Run`, `repoRemoteBase`,
  `strandedForPlan`, `reapStranded`, `parkBranch`, `reapUnstaffed` and
  `planFor` — an `if err != nil` after `rt.git`/`rt.herdr` that every
  existing test's runner currently satisfies. Same
  wrap-and-fail-one-subcommand fake as phase 7's git-fault branches.
- **`tearDownWorktree`'s herdr-failure branch** (80.0%): existing tests
  drive it against a herdr that succeeds or reports the worktree
  already gone; a herdr that returns a real error is untested.
- **`holdRefusal`'s untested wording branch** (75.0%): a refusal
  reason existing tests never construct — read the function to find
  which `held`/`stale`/`deserted` combination is missing and add it.

**GREEN, the tests.** One test per gap, each feeding the input the RED
analysis names — a failing named git subcommand, a failing herdr call,
or the missing state combination for the pure classifiers. Re-run `go
test ./cmd/frit -run Reap -cover` (or the package-wide run) until
reap.go's own function list in `go tool cover -func` reads 100%.

**Guard the edges.** No exclusion list entry, no seam: every gap is
reachable through fakes this suite already builds elsewhere.

**Gate.** `go test ./cmd/frit -cover` reads higher than phase 7 left
it, reflecting reap.go's closed gap; the CI ratchet in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml) is raised to
match, recorded in this phase's result; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are green.
