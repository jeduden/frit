---
n: 6
title: internal/herdr reaches 100% line coverage and joins the gate
status: "✅"
result: false
---
Drive [internal/herdr](../../internal/herdr) to 100% line coverage.
It sits at 95.8% today. Add it to `scripts/check-coverage.sh`'s CI
call. It joins `internal/report`, `internal/claim`, `internal/fleet`,
`internal/observe` and `internal/repocfg` there.

**BDD coverage.** None applies. This phase changes no lease-protocol
behavior — it adds unit tests only, no seam and no behavior change.
No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `Exec` and `ExecContext` are not the process boundary
the plan's own context names them as. They hardcode `"herdr"` as the
binary name, but `exec.CommandContext` resolves that name through
`$PATH` at run time — the same mechanism `runContext`'s own tests
already lean on to run `echo` and `false` in place of a real herdr.
A test can put a throwaway script named `herdr` first on `$PATH` and
exercise the real code path with no herdr installation and no socket.
Every other gap is a plain function of its input.

**Value.** internal/herdr is the seam every board, dispatch and yield
verb reads presence and starts panes through. Proving every line —
including the two functions the plan's own context flagged as an
irreducible boundary — closes the largest remaining line-coverage
gap. It also removes a planned exclusion this package turns out not
to need.

**RED.** `go test ./internal/herdr -coverprofile` records the
zero-count ranges:

- `herdr.go`'s `runContext` (90.9%): the branch where a failed command
  also wrote to stderr never ran — both existing failure tests (a
  missing binary, a bare non-zero exit) leave stderr empty.
- `herdr.go`'s `ExecContext` and `Exec` (0%): neither is called by any
  test.
- `dispatch.go`'s `worktreePane` (87.5%): the runner-error branch
  never ran — `WorktreeCreate` and `WorktreeOpen` are tested only
  against a runner that succeeds or returns malformed data, never one
  that errors outright.
- `dispatch.go`'s `parseWorktreePane` (83.3%): the `json.Unmarshal`
  error branch never ran — the existing "missing pane" tests feed
  valid JSON with an empty `pane_id`, not malformed JSON.
- `dispatch.go`'s `parseCurrentPane` (75%): the same `json.Unmarshal`
  error branch, same reason — `TestCurrentPaneReturnsTheRunnerError`
  covers the runner failing, not the response being unparsable.
- `resolve.go`'s `matchPlan` (87.5%): the `site.Branch == ""` guard
  never ran — every `Join` test resolves a pane to a branch.

**GREEN, the tests.** Each gap gets a direct input that reaches it, no
production code changes:

- `runContext`: `runContext(ctx, "sh", "-c", "echo boom >&2; exit 1")`
  — a real failing process that writes to stderr, asserting the
  returned error contains "boom".
- `Exec`/`ExecContext`: a `t.TempDir()` holding an executable named
  `herdr` (a `#!/bin/sh` script echoing its arguments), prepended to
  `$PATH` with `t.Setenv`. `Exec("agent", "list")` then runs the real
  code path end to end and asserts the fake's echoed output comes
  back.
- `worktreePane`: `WorktreeCreate`/`WorktreeOpen` against a runner
  that returns an error, asserting it propagates — the same shape
  `TestAgentStartSurfacesTheRunnerError` already uses for `AgentStart`.
- `parseWorktreePane`: `WorktreeCreate` against a runner returning
  `[]byte("{not json")`, asserting an error.
- `parseCurrentPane`: `CurrentPane` against a runner returning
  `[]byte("{not json")`, asserting an error.
- `matchPlan`: called directly with `Site{Root: "/repo"}` (no
  `Branch`), asserting it returns `0` without consulting the
  `holdsFor` callback.

Re-run `go test ./internal/herdr -cover` until it reports 100%.

**GREEN, the gate.** Add `./internal/herdr` to the
`scripts/check-coverage.sh` call in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml), beside
the five packages already there. Verify red the same way earlier
phases did. A throwaway untested branch drops the package below 100%
and the script exits non-zero; remove it once seen.

**Guard the edges.** No exclusion list entry: `Exec` and `ExecContext`
turned out testable through `$PATH`, the same way `runContext`'s
existing tests already avoid needing a real herdr. No seam, no
behavior change. Branch coverage stays out of scope.

**Gate.** `go test ./internal/herdr -cover` reports 100%; the gate
call in CI covers `./internal/report`, `./internal/claim`,
`./internal/fleet`, `./internal/observe`, `./internal/repocfg` and
`./internal/herdr` and reddens on an added untested line in any of
them; `go test ./...` and `go tool -modfile=tools/go.mod golangci-lint
run` are green.
