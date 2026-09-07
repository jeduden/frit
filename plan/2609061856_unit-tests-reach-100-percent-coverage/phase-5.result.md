---
n: 5
title: internal/repocfg reaches 100% line coverage and joins the gate
status: "✅"
result: true
summary: >-
  internal/repocfg reached 100% line coverage — four new tests, no
  seam needed — and joined the hard CI gate beside internal/report,
  internal/claim, internal/fleet and internal/observe.
---
## Handoff

`go test ./internal/repocfg -coverprofile` started this phase at
94.6% of statements. The raw profile's zero-count ranges were four
plain functions of their input, none needing a fake:

- `pattern.go`'s `String` (0%): never called by any existing test.
  `Compile("plan/{id}-*")` then `p.String()` covers it.
- `pattern.go`'s `Compile` (90%): its own `regexp.Compile` call never
  failed. `expandWildcards` runs the segments around `{id}` through
  `regexp.QuoteMeta`, which escapes every ASCII regex metacharacter
  but passes an invalid UTF-8 byte through unchanged —
  `regexp.Compile` then rejects the resulting expression as invalid
  UTF-8, not as bad syntax. `Compile("\xff{id}")` reaches it.
- `config.go`'s `Load` (96.4%): the `os.ReadFile` branch returning a
  non-`fs.ErrNotExist` error never ran. A directory named `.frit.yml`
  makes the read fail with "is a directory", which
  `errors.Is(err, fs.ErrNotExist)` reports false for.
- `template.go`'s `Init` (90%): the `os.Stat` branch returning a
  non-`fs.ErrNotExist` error never ran. `TestInitFailsOnAMissingDirectory`
  already covered the sibling case (directory merely absent, so
  `Stat`'s `fs.ErrNotExist` is true and `WriteFile` fails instead).
  This gap needed the file-as-parent-directory shape specifically: a
  plain file where a parent path component belongs makes `os.Stat`
  fail with "not a directory", also not `fs.ErrNotExist`.

`go test ./internal/repocfg -cover` now reports 100.0% of statements,
with no production code changed. `scripts/check-coverage.sh`'s CI
call now covers `./internal/report ./internal/claim ./internal/fleet
./internal/observe ./internal/repocfg` together. Verified red the
same way earlier phases did: a throwaway untested function in
`pattern.go` dropped the package to 96.1% and the script exited 1;
removing it restored the exit-0, 100.0% state. No BDD scenario was
needed — no lease-protocol behavior changed, only tests.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are both green.

**Inherited by the next line-coverage phase:** `internal/herdr` is
next per plan.md's task list — `dispatch.go`'s `worktreePane`,
`parseWorktreePane` and `parseCurrentPane`, `resolve.go`'s
`matchPlan`, and `herdr.go`'s `runContext` are partial;
`herdr.go`'s `ExecContext` and `Exec` are at 0% and may be this
plan's first real process boundary — `Exec` looks like the raw
process-spawn syscall the plan's context names, so check whether it
needs a seam or an exclusion-list entry rather than a direct test.
