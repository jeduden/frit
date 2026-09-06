---
n: 10
title: cmd/frit/start.go reaches 100% line coverage and raises the cmd/frit gate
status: "🔲"
result: false
---
Drive [cmd/frit/start.go](../../cmd/frit/start.go) to 100% line
coverage. Twenty-one zero-count ranges sit here today. Ten are in the
ordinary worktree and fleet functions. Eleven cluster in
`editInEditor`. Raise `cmd/frit`'s ratchet in
`scripts/check-coverage.sh`'s CI call to match.

**BDD coverage.** None applies. `start` stands up a lease, but every
gap here is an existing path fed an input its current tests never
construct, not a new lease-protocol behavior. No `@S<n>` scenario is
needed per [docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `editInEditor` looks like a process boundary — it execs a
real editor — but it is the general-purpose runner shape [phase
6](phase-6.md) already proved testable: `$EDITOR`/`$VISUAL` names a
binary resolved through `$PATH` at run time, the same mechanism a
throwaway script exploits. `openEditor` is already the seam production
code calls through (`var openEditor = editInEditor`); this phase tests
`editInEditor` itself directly with `t.Setenv`, no new seam needed. The
other ten gaps sit behind `rt.git`/`rt.herdr`, already faked elsewhere
in the suite.

**Value.** start is the widest verb — it stands up a worktree, a pane
and a lease together — so its uncovered lines are spread across the
most failure-prone joins in the codebase: a leftover worktree, a dead
pane, an editor that fails or is misconfigured.

**RED.** `go test ./cmd/frit -coverprofile` and start.go's zero-count
ranges are the worklist, enumerated in this phase's result:

- **Sequential error guards** in `Run`, `startResume`,
  `reconcileLeftoverWorktree`, `livePaneOn`, `laneStandUpPane` and
  `standUpLane` — the same `rt.git`/`rt.herdr`-fault shape as the other
  `cmd/frit` phases.
- **`unparkedSuffix`** (75.0%, line 526): a branch its existing callers
  never feed.
- **`editInEditor`** (70.4%, lines 1213-1245): `strings.Fields`
  returning empty (an editor value of pure whitespace), a
  `os.CreateTemp` failure (point `$TMPDIR` at a nonexistent directory),
  the file-write and close error paths, the editor command exiting
  non-zero, and the post-edit `os.ReadFile` failing because the fake
  editor deleted its own temp file — each a distinct input to the real
  function, not a mock.

**GREEN, the tests.** The ten ordinary gaps: a failing named git or
herdr call, or the missing input `unparkedSuffix`'s callers do not yet
construct. `editInEditor`'s cluster, each via `t.Setenv("EDITOR", ...)`
pointing at a throwaway script in `t.TempDir()`:

- `EDITOR=" "` (whitespace only) for the empty-`Fields` branch.
- `TMPDIR` pointed at a path that does not exist, for `CreateTemp`'s
  error.
- A script that exits non-zero (`exit 1`), for the editor-failure
  branch.
- A script that deletes the file it is given (`rm "$1"`) before
  exiting, for `ReadFile`'s error.
- If the write/close error paths resist an honest trigger this way,
  say so in this phase's result rather than force a fake filesystem in
  — a defensive branch stays only if it can be driven red then green
  per [CLAUDE.md](../../CLAUDE.md).

Re-run `go test ./cmd/frit -cover` and check start.go's own function
list in `go tool cover -func`. Keep going until it reads 100%, or
notes the one irreducible branch as a listed exclusion.

**Guard the edges.** No exclusion list entry expected; note one only
if `editInEditor`'s write/close error paths turn out genuinely
undrivable, with the one-line reason this phase's RED analysis
surfaces.

**Gate.** `go test ./cmd/frit -cover` reads higher than phase 9 left
it; the CI ratchet in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml) is raised to
match, recorded in this phase's result; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are green.
