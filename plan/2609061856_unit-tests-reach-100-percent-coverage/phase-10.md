---
n: 10
title: cmd/frit/start.go reaches 100% of its reachable lines
status: "🔲"
result: false
---
Drive [cmd/frit/start.go](../../cmd/frit/start.go) to 100% line
coverage, minus one pair of lines that sit on a real OS boundary. It is
at roughly 90% today across ten partial functions, the densest cluster
in `editInEditor`. Reuses the exclusion mechanism
[phase 9](phase-9.md) built.

**BDD coverage.** None applies. Unit tests only, no behavior change.
No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** The package's existing fixtures cover every gap but one:

- `withHerdr`/`startHerdr`/`recordingHerdr` script herdr's replies.
- `claimableRepo`/`resumableRepo`/`liveLeaseLane`/`seedWindow` build
  the git fixtures.
- `unwindGit` fakes `rt.git` for direct calls.
- `openEditor`/`agentStartPause` are already package-var seams built
  for exactly this purpose, mirroring `internal/herdr`'s pattern from
  [phase 6](phase-6.md).
- `os.Chmod(dir, 0o555)` is the established fault-injection idiom
  `internal/observe/observe_test.go` already uses for a
  read-only-directory write failure.

**Value.** Fourth of five `cmd/frit` files. `editInEditor`'s two
lines — a failed `Write` and a failed `Close` on an already-open
temp-file handle — are the one place in this plan a boundary is a raw
OS fault rather than a git or herdr subprocess: once `os.CreateTemp`
succeeds, POSIX permission checks already happened at open time, so no
directory or file permission trick makes a later write or close fail on
the same fd. Recording that finding, rather than forcing a flaky
rlimit or quota trick, keeps the exclusion list honest.

**RED.** `go test ./cmd/frit -coverprofile`, then `go tool cover -func`
filtered to `start.go`:

- `Run` (47-49): `resolveSelector`'s "no plan given" error uncovered.
- `Run` (51-53): `gatherFleet`'s error uncovered.
- `buildStart` (123-126): the `!coordOK` `ambiguousRepo` refusal
  uncovered — tested for `claim`, not for `start`.
- `startResume` (302-304): the same `!coordOK` early return, same root
  cause — `startResume` runs before `buildStart`'s own check.
- `unparkedSuffix` (526-528): the `local == "" || err != nil` branch
  uncovered — every existing fixture creates the local `plan/<id>` ref
  in the same repo it discovers from, so `local` is never empty.
- `startExecute` (704-706): `openEditor` returning an error uncovered.
- `startExecute` (769-771): `releaseLease` failing *inside* the unwind
  (not `releaseLease` itself, already 100%) uncovered — no test moves
  the ref between mint and release while the handoff also fails.
- `agentStartPause` (947, its own literal): the default `time.Sleep`
  closure body never runs — every retry test overrides the var.
- `reconcileLeftoverWorktree` (1013-1015): `gitwt.List`'s error
  uncovered.
- `reconcileLeftoverWorktree` (1045-1047): `parkBranch`'s error
  uncovered.
- `reconcileLeftoverWorktree` (1048-1050): `worktree remove`'s error
  uncovered.
- `livePaneOn` (1069-1070): the `p.Host != ""` skip uncovered — every
  existing pane fixture is local.
- `laneStandUpPane` (1139-1141): `herdr.WorktreeCreate`'s error on a
  fresh (non-resumed) start uncovered.
- `standUpLane` (1190-1192): `herdr.Focus`'s error uncovered.
- `editInEditor`: five reachable gaps (1213-1215, 1217-1219, 1222-1224,
  1238-1240, 1243-1245) and one boundary pair (1226-1233).

**GREEN, the tests.** One test per gap:

- `Run`/resolveSelector: no selector, cwd outside any held checkout;
  assert the "no plan given" error.
- `Run`/gatherFleet: `--root` at a nonexistent path; assert exit 1.
- `buildStart`+`startResume`/ambiguous repo: the
  `TestClaimRefusesAnAmbiguousRepoName` fixture through `run(["start",
  "7", "--root", root])`; one test closes both functions' gaps, since
  `startResume`'s own check runs first.
- `unparkedSuffix`: mint the hold from a *second* clone of the same
  origin (never touching the discovered repo itself, mirroring
  `liveLeaseLane`'s `git clone --branch` shape), then run `start 7 --go
  --root root` from the discovered repo with the window seeded mature
  (as `TestStartTakesOverAStaleLease` does); assert no "park it first"
  refusal fires.
- `startExecute`/openEditor error: `openEditor = func(string) (string,
  error) { return "", errors.New("boom") }`; assert `run(...)` returns
  the error.
- `startExecute`/unwind-releaseLease failure: make `standUpLane` fail
  (as `TestStartUnwindTearsDownTheLaneOnAFailedHandoff` already does)
  *and* have the herdr fake's failing handler first push a real commit
  to `origin`'s `plan/7` (via a second holder's `claim.Takeover`) so
  the later `claim.Release` CAS on the stale tip fails; assert the
  error names both the handoff cause and the orphaned remote ref.
- `agentStartPause`: a direct test calling the unmocked
  `agentStartPause()` and asserting elapsed time is roughly 500ms —
  the one test in this phase that spends real wall-clock time, because
  every other call site correctly overrides the var.
- `reconcileLeftoverWorktree`/`gitwt.List` error: call the function
  directly against a nonexistent `sc.repoPath`.
- `reconcileLeftoverWorktree`/`parkBranch` error: the leftover-worktree
  fixture, with the park's CAS made to fail (someone else already
  parked or moved the branch first).
- `reconcileLeftoverWorktree`/worktree-remove error: the same fixture,
  with the herdr fake erroring on `worktree`/`remove` (or a real lock
  file blocking it without `--force`).
- `livePaneOn`: a leftover-worktree fixture whose fake `agent list`
  answer includes `"host":"otherhost"` on a pane matching the leftover
  path; assert the leftover is still parked, proving a same-path pane
  on another host is ignored.
- `laneStandUpPane`: a fresh `start --go` whose herdr fake errors on
  `worktree`/`create` only; assert the lease still releases (pane is
  empty, so the handoff teardown is skipped).
- `standUpLane`: a fresh start whose fake errors on `agent`/`focus`
  only (worktree/create, agent/start, prompt all succeed); assert
  `"focus:"` in the error and the lease/pane unwind.
- `editInEditor`, five reachable cases:
  - default editor: `t.Setenv("VISUAL","")`, `t.Setenv("EDITOR","")`,
    a non-interactive fake `vi` on `$PATH`; assert it ran.
  - no editor set: `t.Setenv("EDITOR", "   ")` so `strings.Fields`
    yields nothing; assert the error.
  - `os.CreateTemp` error: `t.Setenv("TMPDIR", dir)` where `dir` is
    `os.Chmod`'d `0o555`.
  - `cmd.Run` error: a fake editor script `exit 1`.
  - `os.ReadFile` error: a fake editor script that deletes its
    argument before exiting 0.

Re-run `go test ./cmd/frit -coverprofile` and `go tool cover -func`
filtered to `start.go` until every line but 1226-1233 reads 100.0%.

**Guard the edges.** Add to `scripts/coverage-exclude.txt`:

```text
cmd/frit/start.go:1226-1233  # editInEditor's WriteString/Close on an already-open temp file: POSIX permission checks happen at open time, so no directory or file permission trick makes a write or close fail on an fd that already opened successfully; forcing it needs a resource limit or quota trick with no portable, deterministic form
```

No seam elsewhere in this file. `agentStartPause`'s real-sleep test is
the one accepted cost of proving the default path runs at all; every
other timing-sensitive call site keeps overriding the var, so it stays
a single, isolated addition to the suite's wall-clock time. Branch
coverage stays out of scope. `./cmd/frit` is not yet added to
`scripts/check-coverage.sh`'s CI call — `main.go`/`progress.go` still
carries gaps.

**Gate.** `go test ./cmd/frit -coverprofile=cover.out && go tool cover
-func=cover.out | grep start.go` shows every line but 1226-1233 at
100.0%; `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are green.
