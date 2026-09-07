---
n: 11
title: cmd/frit/main.go reaches 100% line coverage on its discovery, doctor and plans verbs
status: "✅"
result: false
---
Drive the first half of [cmd/frit/main.go](../../cmd/frit/main.go) to
100% line coverage: `repoLanes` through `carryHostProblems`. That is
the discovery, doctor, who and plans machinery. `main.go` alone carries
roughly fifty partial functions across ~3200 lines. That is far more
than one proving slice can hold under this plan's token budget, so it
splits into this phase and [phase 12](phase-12.md), the last two
`cmd/frit` phases.

**BDD coverage.** None applies. This phase adds unit tests, one small
seam, and removes one structurally-dead branch; no lease-protocol
behavior changes. No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** Headline findings from the research pass:

- `remoteGit` (main.go:984-989) is not a process boundary, despite the
  plan's own Context section naming it one. It calls `herdr.Run("ssh",
  host, "git", ...)`, which resolves `"ssh"` through `$PATH` at run
  time — the exact mechanism [phase 6](phase-6.md) already proved for
  `herdr.Exec`'s hardcoded `"herdr"`. A throwaway `$PATH` script named
  `ssh` that does `shift; exec "$@"` drops the fake host argument and
  runs the rest as a real local git command, driving the real body
  end-to-end with no ssh installed and no network.
- `orphansCmd.Run`'s second `gatherFleet` error check (274-276) is
  structurally dead: it can never fire once the first check (267-269,
  `discover.Repos` on the identical root) has already succeeded. Same
  category [phase 8](phase-8.md) found in `reap.go` — remove it rather
  than chase an input that cannot exist.
- `hostname()`'s `os.Hostname()` failure (1350-1352) has no portable
  failure trick — unlike `os.Getwd`, it essentially never fails on a
  real machine. A package-var seam, the same shape `openEditor`/
  `agentStartPause` use in `start.go`, is the cheapest honest fix.
- The `os.Getwd()` call sites this half touches need no seam:
  `t.Chdir(dir); os.RemoveAll(dir)` fails `os.Getwd()` directly — a
  first-class Go 1.25 testing helper, already the mechanism
  `isolate(t)` uses everywhere in this suite.

**Value.** First of the two closing slices. Discovery, doctor and
plans are the read-only verbs every other command's fixtures already
lean on, so proving them first gives phase 12's orchestration-heavy
half a settled base.

**Recipes**, named once and reused across this phase and phase 12:

- **R1 — missing root**: `run([]string{"<verb>", "--root",
  filepath.Join(t.TempDir(), "absent")}, ...)`. Closes the
  `discover.Repos`/`gatherFleet` error branch in any command calling it
  once on `c.Root`.
- **R2 — malformed `.frit.yml`**: an uncompilable `holds:` pattern
  fails `cfg.Compiled()`; invalid YAML fails `repocfg.Load` itself.
- **R3 — wrap-and-fail-one-git-subcommand**: the established
  `failingLsRemote` shape (`bdd_landed_evidence_test.go`) — wrap the
  runner, intercept one `args[...]` match, delegate the rest.
- **R4 — `os.Getwd()` failure**: `t.Chdir(dir); os.RemoveAll(dir)`.
- **R5 — refuse-to-clobber**: pre-create one target file so
  `repocfg.Init`/`scaffold.writeAsset` refuses it (`force=false`).
- **R6 — unreachable host**: `t.Setenv("XDG_CACHE_HOME", "")`,
  `t.Setenv("HOME", "")` with `--hosts box`, mirroring
  `presence_test.go`/`dispatch_test.go`.
- **R7 — bad glob**: a `plan-dir` like `plans[` makes
  `filepath.Glob` return `ErrBadPattern` inside `doctorpkg.Scan`.

**RED and GREEN, by function.** Line ranges come from `go tool cover
-func` filtered to `main.go`. Re-verify against a fresh
`-coverprofile` before writing each test — some may have shifted or
already closed incidentally:

- `repoLanes` 176-198 — R2 (`Compiled`, `Load`-adjacent), R3
  (`gitobj.Refs`, `MergedRefs`), and a `gitPipe`-wrapped R3 variant for
  `plans.Collect`'s later stage.
- `laneOf` 239-240 — direct: a lane whose hold ref is absent from the
  current ref set.
- `orphansCmd.Run` 267-269 — R1. **274-276 — remove, see Guard the
  edges.** 298-300 — R3 (`failingLsRemote`), also closes `rescuedHeld`'s
  own 511-513.
- `boardUnproven` 451-453 — direct: a plan whose repo is missing from
  `res.Coords`.
- `tokenlessIDs` 467-469 — direct: `gitwt.List` against a bad path.
- `localPanes` 489-495 — direct: nil `rt.herdr`, then a herdr runner
  that errors.
- `printOrphans` 589-609 — direct: hand-built `report.OrphansDoc` with
  non-empty Prunable/Foreign/Deserted, asserting each rendered row.
- `staleCmd.Run` 633-654 — R1, R6, R3 (`for-each-ref` variant for
  `gitobj.RefTimes`).
- `doctorCmd.Run` 763-799 — R1, R2, R7 (twice: the command's own
  `planDir`, and the lane-scoped `overrideLaneFindings` path).
- `overrideLaneFindings` 824-832 — R7/direct for `ScanID`'s error;
  direct for the `f.ID != id` retention branch (feed a finding with a
  different ID alongside the lane's own).
- `printDoctor` 849-851 — direct: an empty-findings doc.
- `gitForHost`/`remoteGit` 977, 984-989 — the fake-`ssh`-on-`$PATH`
  fixture from the Assumes section, run against a real temp repo.
- `whoCmd.Run` 1024-1026 — R6.
- `whoLanes` 1059-1061 — direct: two lanes, same repo, different plan
  IDs, exercising the sort's tie-break.
- `holdsForRoot` 1075-1081 — R2, direct (unexported, no CLI needed).
- `repoLabel` 1107-1109 — trivial: `repoLabel("")`.
- `initCmd.Run` 1148-1163 — 1148-1150: pre-create `.frit.yml` mode
  `0o200` and run `init --mdsmith --force` (force skips the
  exists-check, so `Init`'s write succeeds but the immediately
  following `repocfg.Load` fails to read it back — the chmod idiom
  `internal/observe/observe_test.go` already uses). 1152-1163: R5, one
  pre-existing target file per test.
- `planDir` 1259-1266 — direct: `--dir` override; R2.
- `plansCmd.Run` 1275-1294 — R1, R2 (via `planDir`), `gitPipe`-wrapped
  R3 for `plans.Collect`.
- `printPlans` 1337 — direct: `--detail`/`-d` with real plans present.
- `hostname` 1350-1352 — **seam**, see Guard the edges.
- `gatherFleetOpts` 1366-1368 — R1.
- `carryHostProblems` 1583-1585 — R6, or direct: any `report.NewX` doc
  satisfies the one-method `problemAdder` interface.

Re-run `go test ./cmd/frit -coverprofile` and `go tool cover -func`
filtered to `main.go` until every function above reads 100.0%.

**Guard the edges.** Add one seam: `var osHostname = os.Hostname` in
`main.go`, with `hostname()` calling it, so a test can swap in a
failing stub (save/restore) and assert the `"localhost"` fallback.
Remove `orphansCmd.Run`'s 274-276 second `gatherFleet` error check —
dead once 267-269 already succeeded on the identical root. The removal
changes no observable behavior: the branch could never fire.

**Gate.** `go tool cover -func` filtered to `main.go` shows every
function through `carryHostProblems` at 100.0%; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green. `main.go`
is not yet fully closed — [phase 12](phase-12.md) carries the rest —
so `./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s CI
call.
