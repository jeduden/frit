---
n: 6
title: internal/herdr reaches 100% line coverage and joins the gate
status: "✅"
result: true
summary: >-
  internal/herdr reached 100% line coverage — six new tests, no seam
  and no exclusion needed — and joined the hard CI gate beside
  internal/report, internal/claim, internal/fleet, internal/observe
  and internal/repocfg.
---
## Handoff

`go test ./internal/herdr -coverprofile` started this phase at 95.8%
of statements. The plan's own context had flagged `herdr.Exec` as one
of "the raw git and herdr syscalls every test replaces with a fake" —
a process boundary bound for the exclusion list `cmd/frit`'s phase was
expected to carry. That assumption did not survive contact with the
code, the same way phase 3 found `internal/fleet`'s `Reporter` was
not the boundary it looked like:

- `Exec` and `ExecContext` hardcode `"herdr"` as the binary name, but
  `exec.CommandContext` resolves that name through `$PATH` at run
  time — exactly the mechanism `runContext`'s own existing tests
  already lean on to run `echo` and `false` in place of a real herdr.
  A throwaway script named `herdr`, placed first on `$PATH` for the
  duration of one test, drives both functions through their real code
  path with no herdr installation and no socket. Two direct tests —
  one on `Exec`, one on `ExecContext` since it is also the
  `ContextRunner` `cmd/frit` wires as `herdrRunner` — closed both gaps
  at 100%, no seam and no exclusion-list entry.

Every other gap was a plain function of its input, same as the last
three phases:

- `runContext` (90.9%): the branch where a failed command also wrote
  to stderr never ran — the existing failure tests (a missing binary,
  a bare non-zero exit) both leave stderr empty. `runContext(ctx,
  "sh", "-c", "echo boom >&2; exit 1")` reaches it.
- `dispatch.go`'s `worktreePane` (87.5%): the runner-error branch
  never ran — `WorktreeCreate`/`WorktreeOpen` were tested only against
  a succeeding or malformed-data runner, never a failing one. The same
  shape `TestAgentStartSurfacesTheRunnerError` already uses for
  `AgentStart` closed it via `WorktreeCreate`.
- `parseWorktreePane` (83.3%) and `parseCurrentPane` (75%): each
  package's own `json.Unmarshal` error branch never ran — the existing
  "missing pane" tests feed valid JSON with an empty `pane_id`, never
  malformed JSON. `WorktreeCreate`/`CurrentPane` against a runner
  returning `"{not json"` closed both.
- `resolve.go`'s `matchPlan` (87.5%): the `site.Branch == ""` guard
  never ran — every `Join` test resolves a pane to a real branch.
  Called directly with `Site{Root: "/repo"}` and no `Branch`, it
  closed the gap and pinned that the `holdsFor` callback is never
  invoked when there is no branch to match.

`go test ./internal/herdr -cover` now reports 100.0% of statements,
with no production code changed. `scripts/check-coverage.sh`'s CI
call now covers `./internal/report ./internal/claim ./internal/fleet
./internal/observe ./internal/repocfg ./internal/herdr` together.
Verified red the same way earlier phases did: a throwaway untested
function in `resolve.go` dropped the package to 98.2% and the script
exited 1; removing it restored the exit-0, 100.0% state. No BDD
scenario was needed — no lease-protocol behavior changed, only tests.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are both green.

**Inherited by the next line-coverage phase:** `cmd/frit` is next per
plan.md's task list — the last line-coverage phase, where `main`,
`remoteGit` and the reporter are named in the plan's own context as
needing a seam or a listed exclusion. Given this phase's outcome,
check each one directly rather than assuming the exclusion is needed:
`herdrRunner` (`cmd/frit/main.go:1226`) is already a swappable
package var, so wiring a test through it may close some of that gap
without a new seam. This is likely the phase where the plan's
Acceptance Criteria's exclusion-list entries actually get written for
the first time — establish the list's shape here, since no prior
phase needed one.
