---
n: 5
title: internal/repocfg reaches 100% line coverage and joins the gate
status: "✅"
result: false
---
Drive [internal/repocfg](../../internal/repocfg) to 100% line
coverage. It sits at 94.6% today. Add it to
`scripts/check-coverage.sh`'s CI call. It joins `internal/report`,
`internal/claim`, `internal/fleet` and `internal/observe` there.

**BDD coverage.** None applies. This phase changes no lease-protocol
behavior — it adds unit tests only, no seam and no behavior change.
No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `internal/repocfg` has no process boundary — `Load` and
`Init` only ever call the standard library, and `Pattern.Compile`
wraps `regexp.Compile` over a string this package quotes itself. Each
gap is a plain function of its input, reachable without a fake.

**Value.** internal/repocfg is what every repository's `.frit.yml`
parses into. It is also what every hold pattern compiles from.
Proving every line is the smallest remaining line-coverage gap, and
it sits directly under the claim convention every lane depends on.

**RED.** `go test ./internal/repocfg -coverprofile` records the
zero-count ranges. Four gaps, none needing a fake:

- `pattern.go`'s `String` (0%): never called.
- `pattern.go`'s `Compile` (90%): the `regexp.Compile` error return
  never runs.
- `config.go`'s `Load` (96.4%): the `os.ReadFile` branch that returns
  a non-`fs.ErrNotExist` error never runs.
- `template.go`'s `Init` (90%): the `os.Stat` branch that returns a
  non-`fs.ErrNotExist` error never runs.

**GREEN, the tests.** Each gap gets a direct input that reaches it, no
production code changes:

- `String`: `Compile("plan/{id}-*")` then assert `p.String()` equals
  the source string.
- `Compile`'s `regexp.Compile` failure: `expandWildcards` runs the
  segments around `{id}` through `regexp.QuoteMeta`, which escapes
  every ASCII metacharacter but passes an invalid UTF-8 byte through
  unchanged — `regexp.Compile` then rejects the resulting expression
  as invalid UTF-8, not as bad syntax. A pattern like `"\xff{id}"`
  reaches it.
- `Load`'s `os.ReadFile` failure: a directory named `.frit.yml` — a
  read against a directory fails with "is a directory", which
  `errors.Is(err, fs.ErrNotExist)` reports false for, unlike a missing
  path.
- `Init`'s `os.Stat` failure: a path whose parent component is a
  plain file, not a directory — `os.Stat` fails with "not a
  directory", which `errors.Is(err, fs.ErrNotExist)` also reports
  false for. `TestInitFailsOnAMissingDirectory` already covers the
  sibling case where the directory is merely absent (`WriteFile`
  fails, `fs.ErrNotExist` true for `Stat`), so this test needs the
  file-as-parent shape specifically.

Re-run `go test ./internal/repocfg -cover` until it reports 100%.

**GREEN, the gate.** Add `./internal/repocfg` to the
`scripts/check-coverage.sh` call in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml), beside
the four packages already there. Verify red the same way earlier
phases did. A throwaway untested branch drops the package below 100%
and the script exits non-zero; remove it once seen.

**Guard the edges.** No new exclusion list entry, no seam. Branch
coverage stays out of scope.

**Gate.** `go test ./internal/repocfg -cover` reports 100%; the gate
call in CI covers `./internal/report`, `./internal/claim`,
`./internal/fleet`, `./internal/observe` and `./internal/repocfg` and
reddens on an added untested line in any of them; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
