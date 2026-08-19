---
id: 2608191811
title: Report a checkout stranded on a landed branch
status: "✅"
summary: >-
  A worktree left on a branch that has since merged is invisible to
  frit orphans. The merged ref is dropped from the lane's holds, so the
  lane carries a worktree and no hold, and the orphan report only names
  the opposite shape. Give the report a category for a checkout whose
  claim has landed, so stranded work is surfaced, not silently kept.
model: sonnet
depends-on: [2608142306]
---
# Report a checkout stranded on a landed branch

## Goal

Make `frit orphans` surface a worktree whose claim has landed. That is a
checkout still sitting on a branch that has since merged into the default
branch. A lane that outlived its work should be seen, not silently kept.

## Context

A lane joins a plan's holds to the worktrees standing on them, in
[internal/lanes](../internal/lanes/lanes.go). `Build` drops any ref
already merged into the default branch before it matches a hold pattern,
so a landed plan stops reading as an active claim. That filter runs on
the ref loop only. The worktree loop has no such filter, so a worktree
still checked out on a merged branch is added to the lane while its ref
is not.

The result is a lane with a worktree and no hold: `Holds == 0`,
`Worktrees > 0`. `Lane.Unstaffed()` tests the opposite — a hold with no
worktree — so this shape matches no predicate. `frit orphans` reads the
lane categories in [cmd/frit/main.go](../cmd/frit/main.go) and reports
`claimed, no checkout`, `never started`, and `prunable`, none of which
this is. So a checkout whose branch has merged is reported by nothing,
even though it is exactly the stranded state a cleanup pass wants to
find.

## Tasks

1. Add a `Lane` predicate for a worktree with no hold — a checkout whose
   claim is no longer active — driven by a failing test first
2. Carry that shape into the orphan report as its own category, distinct
   from an unstaffed hold
3. Print it under `frit orphans` with a label that names it as landed or
   stranded work, and add it to the `--json` document
4. Record the golden files for the report with `go test ./internal/report
   -update`, and read the diff before committing

## Acceptance Criteria

- [x] A worktree left on a merged branch is reported by `frit orphans`
- [x] The new category is distinct from `claimed, no checkout`
- [x] The `--json` document carries the same lanes as the table
- [x] An ordinary staffed lane, and a landed branch with no worktree,
      are unaffected
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
