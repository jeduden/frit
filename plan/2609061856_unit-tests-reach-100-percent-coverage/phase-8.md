---
n: 8
title: cmd/frit/reap.go reaches 100% line coverage
status: "✅"
result: false
---
Drive [cmd/frit/reap.go](../../cmd/frit/reap.go) to 100% line coverage.
It sits at 90.0% today across nine partial functions. Two of its gaps
are not missing tests but redundant, unreachable defensive checks; this
phase removes them rather than chasing an input that cannot exist, the
same way [CLAUDE.md](../../CLAUDE.md)'s defensive-code rule — a branch
earns its place only when a test can drive it red then green — argues
against keeping a branch that structurally cannot be driven either way.

**BDD coverage.** None applies. Ten of the twelve gaps are unit tests
only; the two removed branches are a small, behavior-preserving
dead-code deletion (see Guard the edges), not a lease-protocol change.
No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** No new fixture is required. `reap_test.go` and its
siblings already carry every one this phase needs:

- `claimableRepo`, `deadHold`, `fenceWithATakeover` and `seedWindow`,
  from `claim_test.go` and `yield_test.go`.
- The stub-`runtime{git: func(...)}` pattern `main_test.go` already
  uses for `TestDeadSessionAnswersFalseForAnUnreadableMarker`.

**Value.** Second of five `cmd/frit` files, and the one that proves a
line-coverage gap is not always a missing test — sometimes it is a
defensive check with no reachable failure mode, and the honest fix is
to remove it rather than force a contrived one. `cmd/frit` still does
not gate as a whole until the last file closes; this phase's own gate
stays file-scoped.

**RED.** `go test ./cmd/frit -coverprofile`, then `go tool cover -func`
filtered to `reap.go`:

- `Run` (37-39): `discover.Repos` error uncovered — no test points
  `--root` at a nonexistent directory.
- `Run` (64-66): `repoLanes` error → `doc.AddProblem` uncovered — no
  test feeds a broken `.frit.yml`.
- `Run` (45-47) and `repoRemoteBase` (74-76 in `Run`, 132-134 in the
  function itself): **not missing tests.** `Run`'s `gatherFleet` error
  check at 45-47 is unreachable — `fleet.Gather`'s only error source is
  the identical `discover.Repos(c.Root, ...)` call `Run` already made
  successfully one line earlier; if that call would fail, `Run` already
  returned at 37-39. Likewise `Run`'s `repoRemoteBase` error-surfacing
  check at 74-76 is unreachable — `repoRemoteBase`'s only error source
  is `repocfg.Load(repo.Path)`, already loaded successfully by
  `repoLanes` (63) moments earlier on the identical path; if that would
  fail, `repoLanes` already hit its `continue` at 64-66.
  `repoRemoteBase`'s own 132-134 stays a real gap — it is reachable by
  calling the function directly, bypassing `repoLanes`.
- `strandedForPlan` (153-155): the `repoName != plan.Repo` early return
  uncovered — every existing selector test resolves within one repo.
- `reapStranded` (215-221): the `d.Refused != ""` branch uncovered —
  every fleet-shaped (`lanes.Build`-derived) scenario that reaches a
  Stranded lane already reads its worktree branch as landed, so
  `reap.Decide`'s refusal never fires through the real pipeline.
- `parkBranch` (279-281): the err-or-empty-tip branch from `localRef`
  uncovered — same reason, a lane's worktree branch always resolves
  when reached through the real path.
- `tearDownWorktree` (320-322): the `branch -D` failure uncovered —
  existing tests only fail `worktree remove`.
- `reapUnstaffed` (388-390): the dry-run `rescue = claim.RescueRef(...)`
  line uncovered — the existing "leaves the hold standing" test adds no
  unlanded commit.
- `reapUnstaffed` (399-402): the `--go`-mode `claim.Scavenge` error
  uncovered — the existing takeover test gets refused earlier, at
  `holdRefusal`'s live-lease branch, before ever reaching `Scavenge`.
- `holdRefusal` (441-442) and `planFor` (463): the `!ok` /
  plan-not-found branches uncovered — every existing test acquires a
  hold against a repo that already carries a committed plan document.

**GREEN, the tests.** Ten gaps get a direct input; two get removed:

- `Run`/discover.Repos: `run([]string{"reap", "--root",
  "/does/not/exist"}, ...)`; assert exit code 1.
- `Run`/repoLanes: write a broken `.frit.yml` (e.g. an unterminated
  `holds:` list) to a repo, run `reap --json`, assert `doc.Problems`
  names that repo.
- `strandedForPlan`: call directly — `strandedForPlan(stranded,
  "other-repo", discovery.Plan{Repo: "atlas", ID: 7})` — assert an
  empty slice regardless of `stranded`'s contents.
- `reapStranded`: call directly with a hand-built `lanes.Lane{PlanID:
  1, Worktrees: []gitwt.Worktree{wt}}` (a real commit) and an empty
  `landedEvidence{}`; `reap.Decide` returns a refusal immediately —
  assert it lands in the returned `refused` slice.
- `parkBranch`: call directly — `parkBranch(rt, repo, opts,
  "does-not-exist", false)` against a real repo; `localRef`'s
  `rev-parse --verify --quiet` exits 1, giving `("", nil)`; assert
  `parkBranch` returns the same.
- `tearDownWorktree`: a stub `runtime{git: func(...)}` that succeeds
  for `"worktree","remove"` and errors for `"branch","-D"`; call
  directly, assert the error propagates.
- `reapUnstaffed`/dry-run rescue: `deadHold` plus an extra commit
  pushed onto `plan/7` before running `reap` without `--go`; assert
  `"would park"` in stdout.
- `reapUnstaffed`/Scavenge error: `deadHold`, then break the push
  target (e.g. `git remote set-url origin /nonexistent`) so
  `claim.Scavenge`'s push fails; assert `"refused"` and the hold ref
  still resolves.
- `holdRefusal`/`planFor`: call `claim.Acquire` directly against a
  plain `initRepo` with no committed plan document, so `planFor`
  returns `ok=false`; assert `holdRefusal`'s refusal wording. A
  standalone `planFor(nil, "atlas", 999)` call, asserting `ok==false`,
  covers the function in isolation too.
- **Remove, don't test:** `Run`'s 45-47 `gatherFleet`-error branch and
  74-76 `repoRemoteBase`-error-surfacing branch. Both are unreachable
  given the current call order (see RED). Delete both checks — `Run`
  can call `gatherFleet` and `repoRemoteBase` and trust the errors
  `discover.Repos`/`repoLanes` already ruled out one line earlier
  cannot recur. No behavior changes: the removed branches never fired
  in production either.

Re-run `go test ./cmd/frit -coverprofile` and `go tool cover -func`
filtered to `reap.go` until every remaining line reads 100.0%.

**Guard the edges.** The two removed checks are not a process
boundary and do not become exclusion-list entries — they are ordinary
dead code, uncovered because they cannot be reached, not because they
sit on a syscall. Removing them is the smallest change that makes
`reap.go`'s coverage honest; it changes no observable behavior, since
neither branch could ever fire. Everything else closes with a direct
test, no seam. Branch coverage stays out of scope.

**Gate.** `go test ./cmd/frit -coverprofile=cover.out && go tool cover
-func=cover.out | grep reap.go` shows every remaining line at 100.0%;
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are green. `./cmd/frit` is not yet added to
`scripts/check-coverage.sh`'s CI call.
