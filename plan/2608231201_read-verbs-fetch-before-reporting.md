---
id: 2608231201
title: Read verbs refresh from origin before reporting landed evidence
status: "✅"
summary: >-
  The fleet walk reads landed evidence off remote-tracking refs
  (origin's default branch and the lease branches) but never fetches,
  so a checkout that has not fetched since a PR merged reads landed —
  and merged-then-deleted — work as still held. board, orphans, plans,
  ready, pick, next, show, find and who all under-report at once,
  because they share the one gather. Make the gather fetch --prune per
  repo by default, gated by a global --fetch/--no-fetch flag, reusing
  the freshBase fetch pattern the mutating verbs already run. Offline
  or fetch-off falls back to the local view and names the staleness.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: The gather fetches --prune before reading landed evidence
    status: "✅"
  - n: 2
    title: Fetch is gated and best-effort, so offline falls back named
    status: "✅"
  - n: 3
    title: A global --fetch flag reaches every read verb
    status: "✅"
---
# Read verbs refresh from origin before reporting landed evidence

## Goal

Every read verb refreshes its remote-tracking refs before it reads
landed evidence. Landed work then never reads as still held. This
holds for merged-then-deleted work, however long the checkout has gone
unfetched.

## Context

Reproduced this session. Two PRs (#74, #75) squash-landed plans
`2608231006` and `2608230952` and deleted their lease branches on
origin, yet `board` reported both `🔲` and held. The checkout's
`refs/remotes/origin/main` sat two merges behind, and the deleted
lease branches' remote-tracking refs still existed locally. Nobody
had fetched.

Plan `2608221450` made
[DefaultRef](../internal/gitobj/git.go) prefer the remote-tracking
default over a lagging local branch, and plan `2608220941` names a
local default that lags its remote-tracking ref. Both landed. Both
presuppose a fetch: `2608220941`'s own title says "a **fetched**
remote-tracking ref". Neither makes any read verb fetch. This plan
supplies the missing fetch. It depends on neither — both are done —
and it does not change what they added.

The fleet walk is one place.
[gatherFleet](../cmd/frit/main.go) is the sole caller of
[fleet.Gather](../internal/fleet/gather.go); every read verb reads
its result. `gatherRepo` there resolves `DefaultRef`, `Refs`,
`MergedRefs` and `LandedIDs` off whatever remote-tracking refs
already exist on disk. `leaseTips` even prefers the remote-tracking
copy of a lease ref — so a stale, un-pruned copy of a deleted branch
keeps a landed plan reading as held. A single fetch at the top of
`gatherRepo` corrects all of these signals at once, because they all
read the refs it refreshes.

**Reuse first.**
[freshBase and baseBranch](../internal/claim/claim.go) already fetch
the base branch for the mutating verbs — the pattern to copy. They
fetch one branch into `FETCH_HEAD`; the read path instead needs the
whole remote-tracking view refreshed, including every lease branch,
with deleted branches pruned. So the read path runs
`git fetch --prune --quiet <remote>`, not a single-branch fetch into
`FETCH_HEAD`. [reap's best-effort fetch](../cmd/frit/reap.go) is the
spirit of the failure path: a fetch that cannot run must not fail the
walk. The mutating verbs keep their own `freshBase`; this plan does
not touch them and does not make them fetch twice.

**Not laggingDefaultBranch.**
[laggingDefaultBranch](../internal/fleet/gather.go) deliberately reads
only the ref list already gathered (S80) and never fetches. This plan
leaves it exactly so: it makes the fetch happen upstream, so the ref
list `laggingDefaultBranch` reads is already fresh. The two compose —
after the fetch, a still-lagging *local* default branch is the real
"fetch ran, merge did not" case that check exists to name.

**A remote must exist.** A repository with no configured remote is
not stale; it is local-only, and a failed fetch there is expected. The
ref list the gather already read carries a `refs/remotes/<remote>/*`
ref exactly when a remote is configured, so the fetch is attempted —
and a failure named — only for those. No extra subprocess probes it.

## Tasks

1. Fetch `--prune` per repo in `gatherRepo` (hardwired on for the
   slice) and prove a squash-landed, branch-deleted plan reads as
   landed, not held, against a stale remote-tracking view.
2. Make the fetch best-effort and gated: `--no-fetch` skips it, an
   offline fetch falls back to the local view, and a fetch that fails
   for a repo with a configured remote records a staleness `Problem`.
3. Thread a global `--fetch` flag (default on) through `gatherFleet`
   into `Gather`, and prove every read verb inherits it through the
   one walk.

## Phase 1: The gather fetches --prune before reading landed evidence

[gatherRepo](../internal/fleet/gather.go) must refresh remote-tracking
refs before it reads any landed signal. This slice hardwires the fetch
on and proves the corrected evidence end to end; Phase 2 gates it.

RED, at the [internal/fleet](../internal/fleet) level, extending the
real-repo fixture the gather tests already use (`update-ref
refs/remotes/origin/main`, `gitCmd`). Build a repo with a real
`origin` remote whose `main` carries a plan at `✅` and whose lease
branch has been deleted, while the checkout's `refs/remotes/origin/*`
still reflects the pre-merge state (stale default, surviving lease
ref). Assert `Gather` reports the plan landed — excluded from held —
and its lease branch gone. Before the fetch this reads as held; after,
as landed.

GREEN: add a `Fetch` field to a `Gather` options input (an options
struct or explicit parameter — smallest change that threads a bool to
`gatherRepo`). When set, run `git fetch --prune --quiet <remote>` once
at the top of `gatherRepo`, before `Refs`, using `cfg.Remote`. The
existing callers and tests pass it true for this phase.

Gate: the RED case passes; the same repo without the fetch still reads
held (a second case pins the contrast); `go test ./internal/fleet/...`
is clean; `go test ./...`, `go vet ./...` and `mdsmith check .` stay
clean.

## Phase 2: Fetch is gated and best-effort, so offline falls back named

Phase 1 always fetches. This phase makes it optional and safe.

RED, at the [internal/fleet](../internal/fleet) level:

- With the `Fetch` option false, `Gather` runs no fetch subprocess and
  reads the local view unchanged. Assert via a `Runner` that records
  its calls that no `fetch` is issued.
- A fetch that errors (a repo whose `origin` is unreachable) does not
  fail the walk: `Gather` returns the plans read from the local view,
  and records one `Problem` naming the repo and that its view may be
  stale — but only when a `refs/remotes/<remote>/*` ref shows a remote
  is configured. A local-only repo with no remote records no such
  `Problem`.

GREEN: guard the Phase 1 fetch on the option. On fetch error, when the
gathered ref list carries any `refs/remotes/<remote>/*` ref, append a
staleness `Problem`; otherwise continue silently. Reuse
`refOID`/the ref scan already in `gather.go` to detect the remote.

Gate: both RED cases pass; the no-fetch case issues zero fetch calls;
`go test ./...`, `go vet ./...`, `golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 3: A global --fetch flag reaches every read verb

The behaviour exists in `fleet`; this phase exposes it on the CLI and
proves the read verbs inherit it through the single gather.

RED, at the [cmd/frit](../cmd/frit) level:

- A `Fetch` bool on the [cli](../cmd/frit/main.go) struct defaults to
  true and is negatable as `--no-fetch`, resolvable from env and
  config like the other globals (assert the default and the negation
  parse).
- `gatherFleet` passes `c.Fetch` into `Gather`. Assert through a read
  verb (e.g. `board` or `orphans`) run against the Phase 1 fixture
  that, by default, landed-and-deleted work reads landed, and with
  `--no-fetch` it reads held — proving the flag reaches the walk every
  read verb shares.

GREEN: add the flag, thread it through `gatherFleet` to `Gather`.
Update the `--no-fetch` note in
[the protocol note](../docs/research/lease-protocol.md) if a scenario
there states the read path's freshness; add the scenario if none does.

Gate: the RED cases pass; `frit board --no-fetch` and `frit board`
differ on the fixture as asserted; `go test ./...`, `go vet ./...`,
`golangci-lint run` and `mdsmith check .` stay clean.

## Execution

Phase 1 proves the fetch and the corrected evidence — the load-bearing
slice. Phase 2 adds the gate and the fallback. Phase 3 is CLI wiring
over a proven core.

| Phase                         | Design | Implement | Gate that catches a wrong answer                                   |
| ----------------------------- | ------ | --------- | ------------------------------------------------------------------ |
| 1 Fetch --prune in the gather | opus   | sonnet    | a squash-landed, branch-deleted plan reads landed only after fetch |
| 2 Best-effort and gated       | opus   | sonnet    | no-fetch issues zero fetches; an offline fetch falls back, named   |
| 3 Global --fetch through walk | opus   | sonnet    | board vs board --no-fetch differ on the fixture                    |

## Acceptance Criteria

- [x] `Gather` fetches `--prune` per repo when fetch is enabled, using
      the configured remote
- [x] A squash-landed plan whose lease branch was deleted on origin
      reads as landed, not held, from an unfetched checkout
- [x] `--no-fetch` runs no fetch subprocess and reads the local view
- [x] An offline or failed fetch falls back to the local view and,
      when a remote is configured, records a staleness `Problem`
- [x] The global `--fetch` flag defaults on, is negatable, and reaches
      every read verb through `gatherFleet`
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
