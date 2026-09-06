---
n: 11
title: cmd/frit/yield.go reaches 100% line coverage and closes the cmd/frit gate
status: "🔲"
result: false
---
Drive [cmd/frit/yield.go](../../cmd/frit/yield.go) to 100% line
coverage. It carries eight zero-count ranges today, across `Run`,
`localRef`, `tearDownLane` and `renderYield`. This is the last of the
five `cmd/frit` file phases. Close the CI ratchet in
`scripts/check-coverage.sh`'s call at `cmd/frit`'s final ceiling, one
listed exclusion short of 100% — phase 7's `terminalWidth` boundary.

**BDD coverage.** `yield` ends a lease and parks a rescue ref — squarely
the cross-host claim/release/yield/takeover behavior
[CLAUDE.md](../../CLAUDE.md) asks a touching phase to decide on
explicitly. This phase adds no new behavior: every gap below is an
existing path (an error return, a warning wording, a JSON-render
branch) fed an input its current tests never construct, and no
`Acquire`/`Release`/`Takeover` call gains a new case. No `@S<n>`
scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix — record
this decision, don't leave it implicit.

**Assumes.** No new process boundary. Every gap sits behind `rt.git` or
`rt.herdr`, already faked elsewhere in `cmd/frit`'s suite — the same
fake herdr socket `tearDownLane`'s existing tests already build for
`herdr.CurrentPane`/`herdr.WorktreeRemove`.

**Value.** yield is the one verb that both ends a lease and tears down
the calling pane's own worktree. Proving its warning paths matters —
`tearDownLane`'s pane-mismatch and herdr-failure branches especially —
because each is a case where the wrong choice would strand a plan's
rescue ref while claiming success, or tear down an unrelated lane's
worktree.

**RED.** `go test ./cmd/frit -coverprofile` and yield.go's zero-count
ranges are the worklist, enumerated in this phase's result:

- **Sequential error guards** in `Run` — the same
  gather/resolve-selector shape as phases 8 and 9.
- **`localRef`** (85.7%, line 179): the branch where `rev-parse
  --verify --quiet` fails with something other than exit code 1 (a
  real fault, not "ref absent") — existing tests only feed the
  exit-1-absent and the succeeds-with-output cases.
- **`tearDownLane`** (73.3%, lines 203-221): `herdr.CurrentPane`
  failing, an empty `pane.Workspace`, `fleet.CurrentPlanID` resolving
  to a different plan than `doc.Plan`, and `herdr.WorktreeRemove`
  failing — four warning branches, each needing its own fake herdr
  response.
- **`renderYield`** (83.3%, lines 227-229): the `c.JSON` branch
  existing golden tests do not yet feed.

**GREEN, the tests.** `localRef`: a fake `rt.git` returning a non-1,
non-nil exit error from `rev-parse`, asserting it propagates rather
than reading as absent. `tearDownLane`: one test per warning branch,
each built the way this file's existing herdr-backed tests already
construct a fake pane. `renderYield`: `c.JSON = true` against a
constructed `report.YieldDoc`. Re-run `go test ./cmd/frit -cover` and
check yield.go's own function list in `go tool cover -func` until it
reads 100%.

**Guard the edges.** No new exclusion list entry, no seam. The one
exclusion in `cmd/frit` remains phase 7's `terminalWidth` real-terminal
branch; this phase's result records the exclusion list as final for
line coverage.

**Gate.** `go test ./cmd/frit -cover` reads 100% but for the one listed
exclusion — `cmd/frit`'s final line-coverage ceiling; the CI ratchet in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml) is set to
that value and `cmd/frit` now reddens on any new untested line the
same way the six packages before it do; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are green.
