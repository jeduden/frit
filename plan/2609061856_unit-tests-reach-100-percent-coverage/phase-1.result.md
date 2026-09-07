---
n: 1
title: internal/report reaches 100% line coverage and is locked there
status: "✅"
result: true
summary: >-
  internal/report reached 100% line coverage and gained a hard CI gate
  that reddens the moment it drops.
---
## Handoff

`go test ./internal/report -coverprofile` started this phase at 85.2%
of statements, with twenty-eight lines at 0% — the `AddProblem` and
`Warn` helpers across every dispatch and discovery document, `hostOf`'s
no-colon branch, `AddRow`'s nil-commits default, `AddStale`'s no-op
guard for an unrecorded repo, and `firstHold`'s no-holds branch. Each
got a constructor-and-read test in the file its source already lives
beside — `board_test.go`, `discovery_test.go`, `dispatch_test.go`,
`orphans_test.go` — plus a new `drift_test.go`, since `drift.go` had
none. `go test ./internal/report -cover` now reports 100.0% of
statements.

The gate is `scripts/check-coverage.sh`: it runs `go test <pkg> -cover`
for each package named on its command line and fails if any reports
below 100.0%. `.github/workflows/ci.yml` calls it with
`./internal/report` right after the full-suite coverage run. Verified
red: a throwaway untested branch dropped the package to 99.6% and the
script exited 1; reverting it restored the exit-0, 100.0% state. The
script takes further packages on the same command line, so the next
line-coverage phase adds its own package to that one call rather than
inventing a second gate.

`internal/report` has no process boundary — no git, no herdr, no
terminal — so nothing here needed a seam or an exclusion; every
uncovered line was a missing test. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are both green.

**Inherited by the next line-coverage phase:** the pattern is
constructor-and-read tests beside the source, plus one more package
argument to `scripts/check-coverage.sh`'s CI call. `internal/claim` is
next per plan.md's task list.
