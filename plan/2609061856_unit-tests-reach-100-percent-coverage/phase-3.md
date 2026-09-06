---
n: 3
title: internal/fleet reaches 100% line coverage and joins the gate
status: "✅"
result: false
---
Drive [internal/fleet](../../internal/fleet) to 100% line coverage —
88.1% today. Add it to `scripts/check-coverage.sh`'s CI call alongside
`internal/report` and `internal/claim`.

**BDD coverage.** None applies. This phase changes no lease-protocol
behavior — it adds unit tests and extends the coverage gate, no new
seam or behavior change. No cross-host claim, release, yield or
takeover behavior changes, so no `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** internal/fleet's only apparent process boundary is not
one. `progress.go`'s `Reporter` interface looks like the terminal
writer, but the real stderr-writing reporter lives in
`cmd/frit/progress.go`, outside this package. `progress.go` here only
declares the interface and `DiscardReporter`. `DiscardReporter`'s three
methods have empty bodies — zero statements each. They already cost
nothing against the package's statement count, so none needs its own
test. Every uncovered line in `current.go` and `gather.go` is a plain
function of its arguments (`gitwt.Runner`, refs, config). Each gets a
direct unit test with a fake runner or a fabricated `gitobj.Ref`
slice. Only `Gather` and `gatherRepo` need real on-disk repositories,
since only they call `discover.Repos`, `repocfg.Load` and
`plans.Collect` against the filesystem.

**Value.** internal/fleet is the walk every board, orphan and dispatch
verb reads through; proving every line — every git-fault fallback, the
ambiguous-name guard, the held-branch dedup — is the widest slice of
the remaining gap and sits directly under every read verb's output.

**RED.** `go test ./internal/fleet -coverprofile` records the
zero-count ranges. Read the raw profile, not just `-func`'s
per-function percentages — that hides which branch of a partial
function is the gap. Two ranges sit in `current.go`: `ForeignHold`'s
two early returns and `RepoName`'s git-fault fallback. Nineteen more
sit across `gather.go`, enumerated in this phase's result:

- `Gather`'s two error paths
- every `gatherRepo` error return
- `recordCoord`'s repeat-collision guard
- `parseProblems`'s error branch
- `hasRemoteTracking`'s no-match case
- `laggingDefaultBranch`'s two early-outs
- `commitsBehind`'s two failure paths
- `ReleasedRefs`'s two `continue`s
- `heldBranches`'s two error returns, plus its held-branch dedup

**GREEN, the tests.** No production code changes; each gap gets the
cheapest fake that reaches it:

- `ForeignHold`: a non-repository cwd (`site.Root == ""`) and a branch
  outside the holds convention (`!ok`), both via `repoOnBranch` and
  `gitwt.Exec` as existing tests already do.
- `RepoName`: a fake `gitwt.Runner` that fails `worktree`, asserting
  the fallback to `filepath.Base`.
- `Gather`: a root that does not exist, so `discover.Repos` errors and
  `Gather` returns it unwrapped; a `gatherRepo` failure already has
  partial coverage via the broken-repo tests — the remaining branch
  needs a repository whose `.frit.yml` fails to parse.
- `gatherRepo`'s four error returns: a malformed `.frit.yml`
  (`repocfg.Load`), a fake `run` that fails the plan walk's ref list
  before `plans.Collect` returns (its own error, distinct from the
  later one), a counting fake that lets the plan walk's own
  `for-each-ref` succeed once but fails `gatherRepo`'s second,
  identical call, and a `.frit.yml` whose `holds` pattern carries no
  `{id}` token so `heldBranches`'s `cfg.Compiled()` fails.
- `recordCoord`: three repositories sharing a basename, asserting only
  one collision `Problem` is recorded, not two.
- `parseProblems`: a plan file in the right place with no front
  matter, so `index.Build` reports it and the loop that wraps
  `errors.Is(e, planmeta.ErrNoFrontMatter)` runs.
- `hasRemoteTracking`: a direct call with refs naming no matching
  remote.
- `laggingDefaultBranch`: a direct call with `preferred` naming no
  branch (`Branch()` false) and with local and tracking OIDs that
  differ but where a fake `run` answers merge-base false (not an
  ancestor).
- `commitsBehind`: a direct call where `run` fails, and one where it
  returns non-numeric output.
- `ReleasedRefs`: a direct call with a ref whose name is not a branch
  and one whose branch matches no hold pattern.
- `heldBranches`: a direct call with an uncompilable hold pattern, one
  where a fake `run` fails only the `--merged` call, and a real
  repository where the same branch exists as both a local and a
  remote-tracking ref, asserting the gathered plan's `Holds` lists it
  once.

Re-run `go test ./internal/fleet -cover` until it reports 100%.

**GREEN, the gate.** Add `./internal/fleet` to the
`scripts/check-coverage.sh` call in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml), beside
`./internal/report` and `./internal/claim`. Verify red the same way:
a throwaway untested branch drops the package below 100% and the
script exits non-zero; remove it once seen.

**Guard the edges.** No new exclusion list entry: `progress.go`'s
`DiscardReporter` is a zero-statement, not a boundary. Branch coverage
stays out of scope.

**Gate.** `go test ./internal/fleet -cover` reports 100%; the gate
call in CI covers `./internal/report`, `./internal/claim` and
`./internal/fleet` and reddens on an added untested line in any of
them; `go test ./...` and `go tool -modfile=tools/go.mod golangci-lint
run` are green.
