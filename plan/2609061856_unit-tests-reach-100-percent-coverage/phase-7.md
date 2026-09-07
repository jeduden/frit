---
n: 7
title: cmd/frit/release.go reaches 100% line coverage
status: "✅"
result: false
---
Drive [cmd/frit/release.go](../../cmd/frit/release.go) to 100% line
coverage. It sits at 87.5% today (6 of 64 statements uncovered, spread
across three functions). This is the first slice of `cmd/frit`, the
package the plan's own context names as carrying a process boundary —
but nothing in this file touches one, so it closes the same way every
`internal/*` package did: constructor-and-read tests plus one CLI
fixture reuse, no seam and no exclusion.

**BDD coverage.** None applies. This phase adds unit tests only, no
seam, no exclusion, no behavior change. No `@S<n>` scenario is needed
per [docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `Run` reassigns `rt.git =
gitwt.WithDeadline(gitwt.ExecContext, ...)` unconditionally near its
top. That discards any runner a test injected into the `runtime` it
was given — the same shape every other `cmd/frit` verb uses. It also
forecloses injecting a fake git runner through the CLI entry point
(`run([]string{"release", ...})`) for the push-failure gap below.
Close that gap instead by calling the unexported `releaseHeld` directly
with a hand-built `runtime`, the same pattern `claim_test.go` and
`main_test.go` already use elsewhere in the package.

**Value.** `scripts/check-coverage.sh` measures a package, not a file,
so `cmd/frit` does not gate as a whole until every file in it is
closed. This phase's own gate stays file-scoped (`go tool cover -func`
filtered to `release.go`) rather than adding `./cmd/frit` to CI. It is
the cleanest of the five remaining files: no seam, no exclusion. It
proves the fixture-reuse approach that closed `internal/report`
through `internal/herdr` extends into `cmd/frit`, before a later phase
also has to stand up the exclusion mechanism the harder files need.

**RED.** `go test ./cmd/frit -coverprofile`, then `go tool cover -func`
filtered to `release.go` records the zero-count ranges:

- `Run` (30-32): the `gatherFleet` error branch never ran — every
  existing `release_test.go` case points `--root` at a readable
  directory.
- `Run` (42-44): the `resolveSelector` error branch never ran — every
  test passes selector `"7"` explicitly.
- `Run` (52-56): the `!ok` branch on `res.Coords[plan.Repo]` — the
  `ambiguousRepo` refusal — never ran — no test puts two repos under
  root sharing a basename.
- `releaseHeld` (105-109): the `claim.Release` push-failure branch
  never ran — no test makes the CAS push fail once the lane's own
  token is already proven.
- `printRelease` (223-225): the `doc.Rescue != ""` branch never ran —
  the existing scavenge test asserts on the `ReleaseDoc`'s own fields,
  never calls `printRelease` to check its rendering.
- `printRelease` (226-228): the `doc.Warning != ""` branch never ran,
  same reason.

**GREEN, the tests.** Each gap gets a direct input that reaches it, no
production code changes:

- `Run`/gatherFleet: `run([]string{"release", "7", "--root",
  filepath.Join(t.TempDir(), "missing")}, ...)` — a nonexistent root
  fails `filepath.WalkDir` at the root itself, propagating through
  `fleet.Gather`. Assert exit code 1 and the walk error in `errb`.
- `Run`/resolveSelector: omit the selector and `t.Chdir` into a plain
  `t.TempDir()` that is not a repo, so `fleet.CurrentPlanID` reads
  `ok=false`. Assert exit code 1 and `"no plan given and none inferred
  from the current directory"` in `errb`.
- `Run`/ambiguousRepo: mirror `TestClaimRefusesAnAmbiguousRepoName`
  (claim_test.go) — two repos under root sharing a basename — then
  `run([]string{"release", "7", "--root", root})`; assert `"refused"`
  and `"shared by another checkout"`.
- `releaseHeld`/push failure: call `releaseHeld` directly with
  `rt := &runtime{git: failPush}`, where `failPush` delegates every
  call to `gitwt.Exec` except returns an error when `args[0]=="push"`.
  Build a real own-lane first (`claim.Acquire` + worktree add +
  `Renew`, the same shape `TestReleaseEndsTheLanesOwnLease` uses), then
  assert `doc.Warning` carries the injected error and `doc.Released`
  stays false.
- `printRelease`/Rescue: `doc := report.NewRelease(...)`, call
  `doc.ScavengedRef("plan/7", "refs/frit/rescue/7/host-abc")`, then
  `printRelease(&out, doc)`; assert `"rescued: refs/frit/rescue/7/host-abc"`
  in `out`.
- `printRelease`/Warning: `doc.Warn("release: boom")`, call
  `printRelease(&out, doc)`; assert `"warning: release: boom"` in
  `out`.

Re-run `go test ./cmd/frit -coverprofile` and `go tool cover -func`
filtered to `release.go` until every line reads 100.0%.

**Guard the edges.** No seam, no exclusion: every gap is reachable
either through the CLI or by calling the unexported helper directly
with a hand-built `runtime`/`report.ReleaseDoc`, both patterns already
established elsewhere in the package. Branch coverage stays out of
scope.

**Gate.** `go test ./cmd/frit -coverprofile=cover.out && go tool cover
-func=cover.out | grep release.go` shows every line at 100.0%; `go
test ./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
green. `./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s
CI call — that waits for the last file in the package (plan task 2,
[phase 12](phase-12.md)).
