---
n: 12
title: cmd/frit/main.go and progress.go reach 100% of their reachable lines
status: "✅"
result: false
---
Drive the rest of [cmd/frit/main.go](../../cmd/frit/main.go) —
`resolveSelector` through `newParser` and `main` itself — to 100% of
its reachable lines. Do the same for
[cmd/frit/progress.go](../../cmd/frit/progress.go). It reuses the
recipes named in [phase 11](phase-11.md) and the exclusion mechanism
[phase 9](phase-9.md) built.

This does not close `cmd/frit` or join the CI gate. `claim.go`,
`dispatch.go` and `drift.go` are three more files in the package this
plan's task list never enumerated when it named "one phase per file" —
`release.go`, `reap.go`, `yield.go`, `start.go`,
`main.go`+`progress.go`. `go test ./cmd/frit -cover` measures the
whole package. So `scripts/check-coverage.sh` cannot gate it honestly
until those three close too, in a phase 13 not yet written.

**BDD coverage.** None applies. This phase adds unit tests, removes one
structurally-dead branch, and extracts one decision into a testable
helper; no lease-protocol behavior changes. No `@S<n>` scenario is
needed per [docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** Headline findings from the research pass:

- `main()` (3060-3062) is the true, sole hard boundary: it reads real
  argv/stdio and calls `os.Exit`. `run()` immediately below it is the
  deliberate, already heavily tested seam carrying essentially all of
  `main`'s logic.
- Terminal detection needs a narrow exclusion, not a whole-function
  one. `terminalWidth`'s `IsTerminal`-false path is plainly reachable
  with an ordinary non-tty `*os.File` (a temp file). Only its tail, the
  `term.GetSize` call after `IsTerminal` returns true, needs a real
  connected tty this repo has no pty library to fake. Likewise
  `progressFor`'s one uncovered line only runs when `isTerminalWriter`
  answers true.
- `allocateFlex`'s `held<0` clamp (2588-2590) is arithmetically
  unreachable — `held` is only ever assigned a non-negative `maxw`
  entry, `0`, or a value a prior branch already established is
  positive. Same category as [phase 11](phase-11.md)'s
  `orphansCmd.Run` finding — remove it.
- `run()`'s bare `panic(r)` re-raise (3076), the non-`exitCode` branch
  of the top-level recover, cannot be provoked without planting a fake
  panic inside a command's `Run`, which this plan will not do. Its
  *decision* — is `r` an `exitCode` or not — is worth extracting into a
  small, directly-testable helper; the bare `panic(r)` statement itself
  stays excluded.
- Plan.md correction: the plan's Context section once cited
  `internal/fleet/progress.go` as home of "the reporter... writes to a
  terminal" — that file does not exist in this repo. The reporter is
  `cmd/frit/progress.go`, covered by this phase.

**Value.** `main.go` is the widest single file in the plan; closing its
second half, plus `progress.go`, proves the entrypoint and terminal-
detection exclusions the plan's own context named from the start.
`claim.go`, `dispatch.go` and `drift.go` — three files this plan's
task list did not enumerate — carry the rest of `cmd/frit`'s gap;
phase 13 closes them and joins the CI gate, completing plan task 2.

**RED and GREEN, by function.** Line ranges come from `go tool cover
-func` filtered to `main.go`/`progress.go`. Re-verify against a fresh
`-coverprofile` before writing each test — some may have shifted or
already closed incidentally. Recipes R1-R7 are named in
[phase 11](phase-11.md):

- `resolveSelector` 1624-1646 — R4; the "no plan given" branch,
  re-verify it is still uncovered before adding a test — an existing
  `show`-with-no-selector case may already close it.
- `laneOverride` 1677-1694 — R4; a real matching lane with its plan
  file removed or malformed (reuse `who_test.go`'s `repoOnPlan`
  helper).
- `folderPlanPhases` 1738-1740 — direct: a malformed folder-plan
  directory, unexported call.
- `order` 1775-1777 — direct: `sortFlags{Reverse:true}` with
  `Sort==""`.
- `readyCmd.Run` 1800-1802 — R1.
- `pickCmd.Run` 1841-1870 — R1, `--sort bogus`, and the plain `pick`
  success path with neither `--go` nor `--json` — every existing test
  uses one or the other; add `run([]string{"pick","--root",root})`
  against a seeded ready plan.
- `emptyStart` 1944-1946 — reuse an existing "nothing startable"
  fixture with `emit(...)` instead of plain `run(...)`.
- `rescueRefsFor` 1961-1963 — direct: the ambiguous-same-name-repo
  fixture (`discovery_test.go`'s ambiguity setup), `next`/`show` on one
  of the two repos.
- `nextCmd.Run` 1977-1979, `showCmd.Run` 2013-2015 — R1.
- `phaseCmd.Run` 2056-2104 — R1; `resolveSelector`'s own error
  (re-verify against `phase 100`'s existing test first); R4 (bare
  `phase --root root` inside a real lane); a real lane whose plan file
  is removed after `CurrentLane` resolves it; a folder-plan fixture
  whose phase bundle breaks `planmeta.Resume` specifically.
- `boardCmd.Run` 2136-2147 — R1, `--sort bogus`.
- `liveByBranch` 2217-2218 — direct: a pane in detached HEAD
  (`git checkout -q --detach`), leaving `Branch: ""`.
- `selectBoardColumns` 2334-2348 — direct: an alias (`"lane,machine"`)
  and a spec that trims to nothing (`",  ,"`).
- `printBoard` 2419-2421 — direct: width>0 with a stale or dead plan
  present, asserting the legend truncates within the given width.
- `fitBoard` 2553-2560 — direct: columns excluding both `held` and
  `title`; a very narrow width with wide fixed columns forcing the
  budget clamp.
- `allocateFlex` 2588-2594 — **2588-2590 remove, see Guard the
  edges.** 2591-2592: `--columns` with `title` but not `held`.
  2593-2594: unreachable from `fitBoard` itself; a direct call only —
  `allocateFlex(50, maxw, -1, -1)`.
- `terminalWidth` 2654-2667 — 2654-2661 plainly reachable: a real
  `*os.File` from `os.CreateTemp` (not a `*bytes.Buffer`), asserting
  `IsTerminal` reads false. **2662-2667 excluded, see Guard the
  edges.**
- `findCmd.Run` 2699-2706 — R1, `--sort bogus`.
- `printNext` 2755-2764 — direct: both `!doc.HasPhase` messages ("plan
  is done" and "(no phase ledger)").
- `printPhase` 2789-2794 — direct: the `!doc.HasPhase` "(no open
  phase)" message.
- `printRescue` 2824-2827 — direct: a non-empty rescue-refs slice.
- `printDep` 2880-2883 — direct: a `DepCard{Found:false}` node.
- `emptyDepsNote` 2919-2921, `statusLabel` 2929-2931 — trivial direct
  calls on the untested branch of each.
- `fitLastColumn` 2976-2999 — direct: zero rows, a single-column row,
  and a width narrow enough to hit the `budget<minCol` clamp.
- `main()` 3060-3062 — **excluded**, see Guard the edges.
- `run()` 3076, 3087-3090 — 3076 excluded (see Guard the edges); R4
  covers `newParser`'s propagated error (one test closes both 3087-3090
  and `newParser`'s own 3146-3148).
- `newParser` 3146-3148 — R4, same test as above.
- `progressFor` (progress.go:26) — **excluded**, see Guard the edges.

**Guard the edges.** Extend `scripts/coverage-exclude.txt` (built in
[phase 9](phase-9.md)) with:

```text
cmd/frit/main.go:3060-3062   # main(): reads real argv/stdio and calls os.Exit; run() is the tested substitute for everything else
cmd/frit/main.go:2662-2667   # terminalWidth's term.GetSize tail: requires a real controlling terminal; no pty library is vendored
cmd/frit/progress.go:26      # progressFor's real-terminal reporter path: same reason
cmd/frit/main.go:3076        # run()'s non-exitCode re-panic: re-raises a genuine unrelated bug; provoking it means planting a fake panic in a command's Run
```

Add no new package-var seam. Extract `run()`'s recover decision into a
small helper, e.g. `exitCodeFromPanic(r any) (code int, matched bool)`,
so the *decision* is unit-tested directly even though the bare
`panic(r)` stays excluded — groundwork for the later branch-coverage
stage, not required to close this phase.

Remove `allocateFlex`'s 2588-2590 `held<0` clamp — arithmetically
unreachable given how `held` is computed by every real caller. The
removal changes no observable behavior: the branch could never fire.

Fix the plan.md Context section's `internal/fleet/progress.go`
citation to read `cmd/frit/progress.go`.

**GREEN, the gate.** None yet: `./cmd/frit` is not added to
`scripts/check-coverage.sh`'s CI call here — `claim.go`, `dispatch.go`
and `drift.go` still carry gaps, and the whole-package check only
makes sense once every file in it is closed. Phase 13 adds the CI
entry once they do.

**Gate.** `go tool cover -func` filtered to `main.go` and
`progress.go` shows every line at 100.0% except the four declared
exclusions. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are green. Plan task 2 is not yet complete — it
waits on phase 13.
