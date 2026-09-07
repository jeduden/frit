---
n: 13
title: cmd/frit/claim.go, dispatch.go and drift.go close the package, which joins the gate
status: "✅"
result: false
---
Drive [cmd/frit/claim.go](../../cmd/frit/claim.go),
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go) and
[cmd/frit/drift.go](../../cmd/frit/drift.go) to 100% of their reachable
lines. These three files carry the rest of `cmd/frit`'s gap that this
plan's task list did not enumerate — [phase 12](phase-12.md)'s own
result found them once every named file's own gap had already closed.
Then add `./cmd/frit` to `scripts/check-coverage.sh`'s CI call, beside
`internal/report`, `internal/claim`, `internal/fleet`,
`internal/observe`, `internal/repocfg` and `internal/herdr`. This
closes plan task 2 in full. It reuses the recipes named in
[phase 11](phase-11.md) and the exclusion mechanism [phase 9](phase-9.md)
built.

**BDD coverage.** None applies. This phase adds unit tests only, no
seam, no behavior change. No `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Value.** The last three files close the package: `cmd/frit` reaches
100% of its reachable lines and, for the first time, is added to the
CI gate — completing plan task 2 (every `internal/*` package plus
`cmd/frit`). Task 3, adopting a branch-coverage tool, remains a
separate, unplanned stage.

**RED and GREEN, by function.** Line ranges come from `go tool cover
-func` filtered to each file. Re-verify against a fresh
`-coverprofile` before writing each test — some may have shifted or
already closed incidentally.

`claim.go`:

- `claimCmd.Run` 36-38 — R1 (missing root). 105-107 — `mintClaim`'s
  own git-fault error, propagated: an unreachable origin on an
  otherwise fresh, claimable plan (`git remote set-url origin
  /nonexistent`), distinct from the lost-race path already tested.
- `resumeOwnLease` 135-137 — direct: a hand-built `runtime` whose
  `git` fails only `push`, called against a real resumable lane so
  `claim.Resume` itself is what fails.
- `resolveOwnLane` 219-221 — direct: a stub git runner that lets the
  first `rev-parse --show-toplevel` (inside `inOwnLane`) succeed but
  fails the second (`resolveOwnLane`'s own `herdr.Resolve` call) — a
  call-counting stub, not a timing trick, so it stays deterministic.
- `inOwnLane` 249-251 — direct: an empty cwd.
- `unwindFailedStandUp` 344-346 — direct: a lease already fenced by
  another machine's takeover before the unwind's own release runs, so
  both the stand-up cause and the release failure are named.
- `mintOrTakeOver` 480-482 — a matured window's takeover that itself
  loses its CAS: another machine's own takeover already moved the ref
  between the read and this push. Reached by calling `mintOrTakeOver`
  directly after a real competing takeover from a second clone.
- `scavengeGlyph` 412-414 — direct: a fleet result withholding a
  coordinate for the plan's repository.
- `resetWindow` 521-523 — direct: `observe.Path()` failing the same
  way `presence_test.go`'s cache-path fixture already forces it
  (`XDG_CACHE_HOME`/`HOME` both cleared).
- `vetoRefusal` 588-590, 597 — direct: a marker with `Holder: "-"`,
  and a `Renewed: false` veto.
- `claimRefusal` 626-627 — direct: a superseded plan.
- `printClaim` 684-686, 687-689 — direct: a refused doc carrying both
  a rescue ref and a warning.

`dispatch.go`:

- `openCmd.Run` 36-38, 40-42 — R1 and an unresolvable selector.
  51-53 — a configured host that goes unread alongside a successful
  local read (R6). 59-61 — `herdr.Focus`'s own error, propagated.
- `printOpenNextStep` 170-172 — direct: an empty `NextAction`.
- `holdKindFor` 200-202 — direct: `coordOK=false`.
- `nudgeCmd.Run` 239-241, 243-245 — R1 and an unresolvable selector.
  276-278 — `nudgeSend`'s own `herdr.Prompt` error (311-313),
  propagated through `Run`; one test closes both.
- `messageCmd.Run` 376-378, 380-382 — R1 and an unresolvable
  selector. 406-408 — `messageSend`'s own `herdr.Prompt` error
  (444-446), propagated through `Run`; one test closes both.

`drift.go`:

- `driftCmd.Run` 33-35 — R1. 47-48 — an ambiguous repo name leaves a
  not-done plan with no coordinate to read evidence from.
- `newDriftRepoContext` 115-117, 119-121, 123-125, 131-133 — direct:
  `repocfg.Load`, `cfg.Compiled`, `gitobj.Refs` and `allCommits` each
  fail in turn, the last via a stub runner that fails only `git log`.
  141-142 — direct: a merged ref that is a tag, not a branch.
- `bucketByID` 169-170 — direct: a digit run too long for
  `strconv.ParseInt` — a subject that happens to contain a very long
  number.
- `allCommits` 210-212 — direct, same stub as above. 220-221 —
  direct: a log line with no unit-separator field boundary.
- `lastPhaseNumber` 240-241 — direct: a non-numeric phase `N` among
  otherwise-numeric ones. 247-252 — direct: no phase `N` parses as a
  plain integer at all, falling back to the last entry.
- `printDrift`, `landedLabel`, `lastPhaseLabel` 283-289, 294-297, 299,
  303-306, 308 — one direct call against two hand-built rows covering
  every landed/last-phase combination. Every existing `drift` test
  reads `--json`; none exercises the table.

**Guard the edges.** No seam, no exclusion: every gap in these three
files is reachable through a direct call or an existing CLI fixture
shape, none needing a process boundary.

**GREEN, the gate.** Add `./cmd/frit` to the `scripts/check-coverage.sh`
call in [.github/workflows/ci.yml](../../.github/workflows/ci.yml),
beside the six packages already there. With `scripts/coverage-exclude.txt`
carrying every `cmd/frit` entry from phases 9, 10 and 12, the script
reports the package's non-excluded statements at 100%. Verify red the
same way earlier phases did: a throwaway untested line drops the
package below its adjusted 100%, and the script exits non-zero; remove
it once seen.

**Gate.** `go test ./cmd/frit -coverprofile=cover.out && go tool cover
-func=cover.out` shows every line at 100.0% except the six declared
exclusions. `scripts/check-coverage.sh ./internal/report
./internal/claim ./internal/fleet ./internal/observe
./internal/repocfg ./internal/herdr ./cmd/frit` passes. The CI call in
`.github/workflows/ci.yml` carries all seven packages, and `go test
./...` plus `go tool -modfile=tools/go.mod golangci-lint run` are
green. Plan task 2 is complete; the branch-coverage criterion is all
that task 3 still owns.
