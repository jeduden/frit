---
n: 8
title: cmd/frit/reap.go reaches 100% line coverage
status: "✅"
result: true
summary: >-
  cmd/frit/reap.go reached 100% line coverage — eleven new tests and
  two dead branches deleted, no seam or exclusion needed — proving a
  coverage gap is not always a missing test.
---
## Handoff

`go test ./cmd/frit -coverprofile` started this phase with `reap.go`
at 90.0% of statements, spread across nine partial functions. The
phase's own RED analysis held: ten gaps were plainly reachable, and
two were structurally dead code rather than undertested branches.

- `Run`'s `gatherFleet` error check (its own `discover.Repos` call one
  line earlier already ruled the same failure out) and its
  `repoRemoteBase` error-surfacing check (`repoLanes` already loaded
  the identical config moments earlier) were both deleted, not tested.
  Both calls now discard the error they can no longer produce; no
  behavior changed, since neither branch ever fired.
- `strandedForPlan`, `reapStranded`, `planFor` and `holdRefusal`
  closed with direct unit calls — no CLI, no repository needed for
  the pure ones.
- `parkBranch` and `repoRemoteBase`'s own error path closed with
  direct calls too, bypassing the CLI orchestration that never
  isolates them from their callers' own prior checks.
- `tearDownWorktree`'s branch-delete failure closed with a stub
  `runtime{git: func(...)}` that lets `worktree remove` succeed and
  fails only `branch -D`.
- `reapUnstaffed`'s dry-run rescue preview and its `--go`-mode
  `Scavenge` failure both closed by reusing the existing `deadHold`
  fixture — an added unlanded commit for the first, a broken origin
  remote (`git remote set-url origin /nonexistent`) for the second.
- A broken `.frit.yml` on a second repository, reused from
  `gather_test.go`'s established fixture, closed `Run`'s own
  `repoLanes`-error-to-`doc.AddProblem` path, alongside a healthy
  repository in the same fleet to prove one broken repo does not stop
  the rest from being reaped.

`go tool cover -func` filtered to `reap.go` now reads 100.0% on every
function. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are both green. No BDD scenario was needed — ten
gaps closed with unit tests only, and the two deletions are
behavior-preserving dead-code removal, not a lease-protocol change.
`./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s CI
call; that waits for [phase 12](phase-12.md), the last file in the
package.

**Inherited by phase 9:** `yield.go` is next, and per the plan's own
task list it is where the exclusion-list mechanism
(`scripts/coverage-exclude.txt`) gets built and proven for the first
time — this phase found no gap needing one, so the mechanism itself
is still unbuilt going in.
