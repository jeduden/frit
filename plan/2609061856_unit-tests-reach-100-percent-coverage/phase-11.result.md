---
n: 11
title: cmd/frit/main.go reaches 100% line coverage on its discovery, doctor and plans verbs
status: "✅"
result: true
summary: >-
  cmd/frit/main.go's first half — repoLanes through carryHostProblems —
  reached 100% line coverage: forty-plus new tests, one seam
  (hostname), one dead branch deleted, no exclusions.
---
## Handoff

`go tool cover -func` filtered to `main.go` started this phase with
every function from `repoLanes` through `carryHostProblems` partial.
Every gap closed as the phase spec expected, plus two gaps its own RED
analysis had not enumerated (both closed the same day, listed below)
and no surprises beyond the ones already named in the spec's own
Assumes section (`remoteGit`'s testability, `orphansCmd.Run`'s dead
branch, `hostname`'s seam).

Closed by function group, each via the recipe R1-R7 named it, or a
direct call when the phase spec called for one:

- `repoLanes`: `repocfg.Load`, `cfg.Compiled`, `gitobj.Refs` and
  `plans.Collect` each closed with R2/R1-shaped direct calls. One gap
  the spec's line-range scan missed: `gitobj.MergedRefs`'s own error,
  closed with a stub runner that fails only the `for-each-ref
  --merged` call, distinct from `Refs`' own plain one.
- `laneOf`: the ref-absent skip, direct-called with no refs supplied.
- `orphansCmd.Run`: `discover.Repos`'s error (R1) and the rescue-sweep
  failure (a broken origin remote) both closed; the second
  `gatherFleet` error check deleted as dead code, per the spec.
- `boardUnproven`, `tokenlessIDs`, `localPanes`, `rescuedHeld`: each
  closed with a direct call reaching its own named error source.
- `printOrphans`: one direct call against a hand-built doc exercising
  the prunable, foreign and deserted rows together — no CLI fixture
  combines all three in one repository.
- `staleCmd.Run`: `discover.Repos`'s error (R1), a host problem (R6),
  and `gitobj.RefTimes`'s error — the last by calling `Run` directly
  with a hand-built `runtime`, since the CLI's own `run()` always
  wires the real `gitwt.Exec` and cannot take an injected failure at
  this depth.
- `doctorCmd.Run`, `overrideLaneFindings`, `printDoctor`: `discover
  .Repos`'s error, a broken `.frit.yml`, a malformed `plan-dir` glob
  (R7, `"plans["` as a literal directory name so `os.Stat` finds
  `proto.md` but `filepath.Glob` then fails), a lane whose own working
  copy lost its schema, and the finding-retention branch — the last
  needing two plans in the fleet's default-branch scan so overriding
  one leaves the other's finding standing, which the phase spec named
  as a gap but did not spell out the two-plan shape needed to reach.
- `gitForHost`/`remoteGit`: called directly rather than through the
  full `whoCmd`/presence pipeline — both are ordinary functions taking
  a host string, so the `$PATH`-resolution trick from phase 6 reaches
  them with no live multi-host setup at all.
- `whoCmd.Run`: a host problem (R6) reported alongside a successful
  local read.
- `whoLanes`: the sort's own two comparisons, each needing a shape the
  phase spec's single line-range entry did not distinguish — same
  repository, same plan (pane-id tie-break) and same repository *name*
  from two different checkouts, different plans (plan-id compare). The
  ambiguous-basename fixture already established for `claim`/`release`
  /`yield`/`start` supplied the second shape directly.
- `holdsForRoot`, `repoLabel`: direct calls, no CLI needed.
- `initCmd.Run`: the `--force`-then-unreadable chmod idiom for the
  post-write `repocfg.Load`, and three separate refuse-to-clobber
  fixtures (R5) for the mdsmith config, the proto schema, and the plan
  index — one target file pre-created per test, so each closes exactly
  one of the three writes.
- `planDir`/`plansCmd.Run`/`printPlans`: the `--dir` override,
  `repocfg.Load`'s error (both direct-called via `p.planDir` and
  through the CLI), `plans.Collect`'s error (a stub `gitPipe`, `Run`
  called directly the same way `staleCmd.Run` was), and `--detail`'s
  own per-plan rendering.
- `hostname`: gained the `osHostname` seam, the same shape
  `start.go`'s `openEditor`/`agentStartPause` already use — `os
  .Hostname` essentially never fails on a real machine, so there is no
  portable way to fail it without one.
- `carryHostProblems`: one direct call against a `report.OrphansDoc`,
  which satisfies the one-method `problemAdder` interface.

`go tool cover -func` filtered to `main.go` now reads 100.0% on every
function through `carryHostProblems`; every function from
`resolveSelector` onward, [phase 12](phase-12.md)'s own scope, is
untouched. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are both green. No BDD scenario was needed — every
closed gap is an existing path fed an input its tests never
constructed, the one seam adds no behavior, and the one deletion is
behavior-preserving.

`./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s CI
call; that waits for phase 12, the last file in the package.

**Inherited by phase 12:** `main()`, `terminalWidth`/`progressFor`,
and `run()`'s panic re-raise are the three remaining exclusion
candidates the plan's own context named from the start — all in the
second half. `allocateFlex`'s `held<0` clamp is the one dead branch
already flagged there. Two new patterns from this phase are worth
carrying forward: calling a command's own `Run` method directly with a
hand-built `runtime` reaches an error a git/gitPipe stub cannot inject
through the CLI's real wiring; and `sort.Slice`'s own tie-break
comparisons often need more than one line-range entry's worth of
fixture shape to close, since each comparison level needs its own
distinguishing setup.
