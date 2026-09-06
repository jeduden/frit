---
n: 3
title: internal/fleet reaches 100% line coverage and joins the gate
status: "✅"
result: true
summary: >-
  internal/fleet reached 100% line coverage — twenty-five new tests, no
  seam needed — and joined the hard CI gate beside internal/report and
  internal/claim.
---
## Handoff

`go test ./internal/fleet -coverprofile` started this phase at 88.1%
of statements. The raw profile's zero-count ranges gave the real
worklist: two early returns in `current.go` (`ForeignHold`'s
not-a-worktree and off-the-convention guards, `RepoName`'s
git-fault fallback) and nineteen spots across `gather.go`.

`progress.go`'s `Reporter` interface turned out not to be the process
boundary the plan's context expected: the real stderr-writing reporter
lives in `cmd/frit/progress.go`, outside this package. `DiscardReporter`'s
three methods have empty bodies — zero statements each — so `go test
-cover`'s statement count never counted them against the package, and
`go test ./internal/fleet -cover` already read 100.0% once every real
gap closed, with `go tool cover -func`'s three 0.0% rows for `Start`,
`Repo` and `Done` left as the harmless artifact of a zero-statement
function. No seam, no exclusion-list entry.

Every real gap was a direct function of its arguments, so no
production code changed:

- `current.go`: `ForeignHold` against a non-repository cwd and a
  branch off the holds convention; `RepoName` against a fake
  `gitwt.Runner` that fails `worktree`.
- `gather.go`'s `Gather`: a root `discover.Repos` cannot even read,
  and a `gatherRepo` fault (a fake `run` failing `for-each-ref`)
  reaching the loop's own problem-and-continue, distinct from the
  already-covered `discover.Repos`-level skip.
- `gatherRepo`'s four error returns: a malformed `.frit.yml`
  (`repocfg.Load`), a fake `run` failing the plan walk's own ref list
  before `plans.Collect` returns, a counting fake that lets that first
  `for-each-ref` call succeed and fails only the second, otherwise
  identical one `gatherRepo` makes itself, and a `.frit.yml` holds
  pattern with no `{id}` token surfacing through `heldBranches`.
- `recordCoord`: a third repository sharing an already-ambiguous name,
  pinning that the guard records no second problem.
- `parseProblems`: a plan file in the right place with no front
  matter, so `index.Build` — not `plans.Collect`'s own mislaid-file
  filter — is what rejects it.
- `hasRemoteTracking`, `laggingDefaultBranch`, `commitsBehind`,
  `ReleasedRefs`, `heldBranches`: each called directly with fabricated
  refs and a fake `run`, no repository needed — a tag and an
  off-convention branch skipped before `claim.LiveHold`, a non-branch
  or non-ancestor preferred ref, a failed or unparsable `rev-list`, an
  uncompilable holds pattern, a failed `--merged` read.
- `heldBranches`'s held-branch dedup: a real repository where the same
  claim branch exists as both a local ref and its remote-tracking
  copy, pinning that the gathered plan's `Holds` lists it once.

`go test ./internal/fleet -cover` now reports 100.0% of statements.
`scripts/check-coverage.sh`'s CI call now covers `./internal/report
./internal/claim ./internal/fleet` together. Verified red the same
way: a throwaway untested function in `current.go` dropped the
package to 99.5% and the script exited 1; removing it restored the
exit-0, 100.0% state. No BDD scenario was needed — no lease-protocol
behavior changed, only tests and the gate.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are both green.

**Inherited by the next line-coverage phase:** the same
pattern — a fake `gitwt.Runner` failing one named subcommand or, when
a call repeats identically, a counting fake; a direct call for a pure
function; one more package argument to `scripts/check-coverage.sh`'s
CI call. `internal/observe` is next per plan.md's task list.
