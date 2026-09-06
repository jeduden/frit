---
n: 4
title: internal/observe reaches 100% line coverage and joins the gate
status: "✅"
result: true
summary: >-
  internal/observe reached 100% line coverage — one seam over its
  atomic write, seven new tests — and joined the hard CI gate beside
  internal/report, internal/claim and internal/fleet.
---
## Handoff

`go test ./internal/observe -coverprofile` started this phase at
69.0% of statements. The raw profile's zero-count ranges were narrow:
`Save` was missing all five of its error returns, `Path` was missing
`os.UserCacheDir`'s.

Four of `Save`'s five gaps needed no seam, only a crafted input:

- `os.MkdirAll`'s failure: a file where a parent directory component
  belongs, the same trick `internal/presence`'s
  `TestReadSurvivesAnUnwritableCache` already uses.
- `json.Marshal`'s failure: a `Window.First` set to
  `time.Date(10000, ...)` — `time.Time.MarshalJSON` refuses a year
  outside `[0,9999]`, the only shape this package's own values can
  take to make `Marshal` fail.
- `os.CreateTemp`'s failure: an existing but unwritable directory
  (`os.Chmod(dir, 0o555)`), skipped under `os.Geteuid() == 0`.
- `os.Rename`'s failure: a destination that is itself an existing
  directory, so renaming a real, already-written temp file onto it
  fails with "file exists" — no fake needed.

The remaining two — `tmp.Write` and `tmp.Close` failing after
`os.CreateTemp` already succeeded — cannot be forced through a real
`*os.File` on demand. `observe.go` gained a narrow seam: a `tempFile`
interface over `Write`/`Close`/`Name`, and a `createTemp` package var
`Save` calls instead of `os.CreateTemp` directly. Two tests swap it
for a fake `tempFile` whose `Write` or `Close` returns an error,
asserting `Save` returns it and never reaches `os.Rename`. No other
behavior changed.

`Path`'s one gap: `t.Setenv("XDG_CACHE_HOME", "")` and
`t.Setenv("HOME", "")` together, the one combination
`os.UserCacheDir` refuses on Linux.

`go test ./internal/observe -cover` now reports 100.0% of statements.
`scripts/check-coverage.sh`'s CI call now covers `./internal/report
./internal/claim ./internal/fleet ./internal/observe` together.
Verified red the same way earlier phases did: a throwaway untested
function dropped the package to 90.9% and the script exited 1;
removing it restored the exit-0, 100.0% state. No BDD scenario was
needed — no lease-protocol behavior changed, only tests and one
non-behavioral seam.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are both green. `mdsmith check .` is clean — the Execution table's
Gate column needed trimming across all four rows to keep MDS026's
column-width ratio under its limit as a fourth row was added; the
wording lost the "and reddens on an added untested line" clause but
the coverage-gate behavior it described is unchanged.

**Inherited by the next line-coverage phase:** `internal/repocfg` is
next per plan.md's task list — `pattern.go`'s `String` sits at 0% and
`Compile`, `config.go`'s `Load` and `template.go`'s `Init` are
partial. Watch the Execution table's Gate column width again when
adding phase 5's row; MDS026 fires past a certain point as the table
grows, not just from an unusually long single cell.
