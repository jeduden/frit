---
n: 4
title: internal/observe reaches 100% line coverage and joins the gate
status: "✅"
result: false
---
Drive [internal/observe](../../internal/observe) to 100% line
coverage. It sits at 69.0% today. Add it to
`scripts/check-coverage.sh`'s CI call alongside `internal/report`,
`internal/claim` and `internal/fleet`.

**BDD coverage.** None applies. This phase changes no lease-protocol
behavior — it adds unit tests and one non-behavioral seam over the
package's own atomic write, no cross-host claim, release, yield or
takeover behavior. No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `internal/observe` has no external process boundary —
`Load`, `Save` and `Path` only ever call the standard library
directly. The gap is entirely `Save`'s five error returns and `Path`'s
one, each a standard-library failure ordinary inputs cannot reach on
demand but a crafted input or a swapped seam can.

**Value.** internal/observe is the per-host memory every takeover
staleness check reads and writes. `Save` folds every write failure
into a returned error rather than a silent drop. Proving every line is
the remaining line-coverage gap nearest the lease protocol's own risk,
even though this phase changes no protocol behavior itself.

**RED.** `go test ./internal/observe -coverprofile` records the
zero-count ranges. Read the raw profile: `Save` (52.9%) is missing all
five of its error returns, and `Path` (75.0%) is missing
`os.UserCacheDir`'s error return.

**GREEN, the seam.** `os.CreateTemp` returns a concrete `*os.File`;
Save's later `tmp.Write` and `tmp.Close` failures cannot be forced
through a real file without one. Add a small seam scoped to those two
calls alone:

```go
type tempFile interface {
	Write([]byte) (int, error)
	Close() error
	Name() string
}

var createTemp = func(dir, pattern string) (tempFile, error) {
	return os.CreateTemp(dir, pattern)
}
```

`Save` calls `createTemp(dir, "observations-*.json")` in place of
`os.CreateTemp` directly. No other behavior changes; a test swaps
`createTemp` for the duration of one test to return a fake `tempFile`
whose `Write` or `Close` returns an error, then restores it.

**GREEN, the tests.** Cover each zero-count range by its own shape:

- `Save`'s `os.MkdirAll` failure: a file where a parent directory
  component belongs (the same trick `internal/presence`'s
  `TestReadSurvivesAnUnwritableCache` already uses), so `MkdirAll`
  fails with `ENOTDIR`.
- `Save`'s `json.Marshal` failure: a `State` whose `Window.First` is
  `time.Date(10000, ...)` — `time.Time.MarshalJSON` refuses a year
  outside `[0,9999]`, the only way this package's own value shape can
  make `Marshal` fail.
- `Save`'s `os.CreateTemp` failure: a directory that already exists
  (so `MkdirAll` no-ops) but is not writable (`os.Chmod(dir, 0o555)`
  after creating it, restored via `t.Cleanup` before `t.TempDir()`
  removes it) — skip with `t.Skip` when `os.Geteuid() == 0`, since
  root ignores the permission bit.
- `Save`'s `tmp.Write` and `tmp.Close` failures: swap `createTemp` for
  a fake `tempFile` whose `Write` (respectively `Close`) returns an
  error, asserting `Save` returns it and never calls `os.Rename`.
- `Save`'s `os.Rename` failure: a real temp file plus a destination
  `path` whose directory component is itself a file, so `Rename`
  fails with `ENOTDIR` after `Write`/`Close` already succeeded — needs
  `createTemp`'s real implementation, so this test does not swap the
  seam.
- `Path`'s `os.UserCacheDir` failure: `t.Setenv("XDG_CACHE_HOME", "")`
  and `t.Setenv("HOME", "")`, the one combination `os.UserCacheDir`
  refuses on Linux.

Re-run `go test ./internal/observe -cover` until it reports 100%.

**GREEN, the gate.** Add `./internal/observe` to the
`scripts/check-coverage.sh` call in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml), beside
`./internal/report`, `./internal/claim` and `./internal/fleet`. Verify
red the same way earlier phases did. A throwaway untested branch drops
the package below 100% and the script exits non-zero; remove it once
seen.

**Guard the edges.** No new exclusion list entry: the seam covers the
one spot that needed one. Branch coverage stays out of scope.

**Gate.** `go test ./internal/observe -cover` reports 100%; the gate
call in CI covers `./internal/report`, `./internal/claim`,
`./internal/fleet` and `./internal/observe` and reddens on an added
untested line in any of them; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are green.
