---
id: 2608221450
title: Landed evidence reads origin's default branch, not a lagging local one
status: "🔳"
summary: >-
  DefaultRef resolves the default branch to a local `refs/heads/main`
  when `refs/remotes/origin/HEAD` is unset, and a local default branch
  normally lags origin — you do not pull a branch checked out in
  another worktree. Landed glyph and ancestry evidence then read
  pre-merge status, so a squash-landed plan reads as held: `release`
  and `reap` refuse landed work and `board`/`orphans` under-report it
  (S84, S85). The fix makes DefaultRef prefer the remote-tracking
  default over any local branch, so evidence follows origin's view,
  the invariant S13 and S75 already state.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: DefaultRef prefers the remote-tracking default over local heads
    status: "✅"
  - n: 2
    title: Landed detection is proven against a lagging local main
    status: "🔲"
---
# Landed evidence reads origin's default branch, not a lagging local one

## Goal

`DefaultRef` resolves the default branch to origin's remote-tracking
ref, never a local branch that normally lags. Landed evidence then
follows origin's view, and landed work stops reading as held.

## Context

Reproduced this session. `release 2608220940` refused after its PR
squash-merged. `board` and `orphans` under-reported six landed plans
at the same time. The cause: this checkout has
`refs/remotes/origin/HEAD` unset, so
[internal/gitobj/git.go](../internal/gitobj/git.go)'s `DefaultRef`
fell through to the local `refs/heads/main`. That branch sat at
`e62f3be`, far behind `origin/main`. A working checkout's default
branch advances only on an explicit `merge` or `pull`. Nobody runs
that on a branch checked out in another worktree.

`DefaultRef` feeds every landed signal.
[internal/fleet/gather.go](../internal/fleet/gather.go) passes its
result as `preferred` into two checks: `index.LandedIDs`, the status
glyph, and `gitobj.MergedRefs`, the `--merged` ancestry. Read against
a stale local `main`, a squash-landed `✅` reads as `🔲`. The hold
then reads live. Pointing `refs/remotes/origin/HEAD` at `origin/main`
made `orphans` reclassify all six plans as landed.

The Terms section of
[the protocol note](../docs/research/lease-protocol.md) settles which
ref is right: "Landed: the plan's work has reached origin's default
branch." Origin's remote-tracking ref is authoritative by definition;
a local branch is not. Scenarios S84 and S85 record this; S13 and S75
already state the invariant. This plan makes the code uphold it.

**Reuse first.** `DefaultRef` already resolves
`refs/remotes/origin/HEAD` first. It already probes a candidate list
with `rev-parse --verify --quiet`. The fix extends that list. Try
`refs/remotes/origin/main` and `refs/remotes/origin/master` before
the local `refs/heads/*`. No new subprocess kind, no new parser.

**Scope.** `DefaultRef` hardcodes the `origin` remote today, in its
`refs/remotes/origin/HEAD` probe. This plan keeps that, matching
current behavior. Threading the configured `.frit.yml` remote is a
separate, pre-existing concern. It is a deliberate non-goal here.

**Siblings, not dependencies.** Plan 2608220941 names the lag as a
`fleet.Problem` but changes no evidence source; plan 2608212010
teaches the Scavenge *park* decision content-evidence. Neither touches
`DefaultRef`. This plan is independent of both and blocks on neither.

## Tasks

1. Make `DefaultRef` prefer the remote-tracking default over any local
   branch when `origin/HEAD` is unset.
2. Prove landed detection reads correctly when local `main` lags, and
   update scenario S85 to describe the shipped behavior.

## Phase 1: DefaultRef prefers the remote-tracking default over local heads

`DefaultRef` ([internal/gitobj/git.go](../internal/gitobj/git.go))
must reach the remote-tracking default before it ever falls to a
local branch. A lagging local default is then never the evidence
source.

RED extends the real-repo fixture the
[internal/gitobj](../internal/gitobj) tests already use, in
`defaultref_test.go`. Build a repo whose `origin/HEAD` is unset and
whose local `main` sits behind `origin/main`. Assert `DefaultRef`
returns the remote-tracking `origin/main`, not the local `main`. A
second case sets `origin/HEAD` and asserts it still wins.

GREEN: insert `refs/remotes/origin/main` and
`refs/remotes/origin/master` into the candidate loop, ahead of the
existing `refs/heads/main` and `refs/heads/master` entries.

Gate: both RED cases pass; `go test ./internal/gitobj/...` is clean;
`go test ./...`, `go vet ./...` and `mdsmith check .` stay clean.

## Phase 2: Landed detection is proven against a lagging local main

Phase 1 fixes the ref `DefaultRef` returns. This phase proves the
signals that consume it. The status glyph and `--merged` ancestry
must read a squash-landed plan as landed when the local default
branch lags.

RED, at the [internal/fleet](../internal/fleet) level with its
existing multi-ref fixture: a repo whose `refs/remotes/origin/main`
carries a plan at `✅` while the local `refs/heads/main` still carries
it at `🔲`, `origin/HEAD` unset. Assert `Gather` reports the plan's
hold as landed (excluded from held), not live. Before Phase 1 this
read it as held; after, as landed.

GREEN: no new code beyond Phase 1 if the fix is sufficient; if a
second read path still consults a local ref, correct it the same way.
Then update S85 in
[the protocol note](../docs/research/lease-protocol.md) from the
"must reach ... else" wording to the shipped behavior: `DefaultRef`
reaches the remote-tracking default first, so landed work is seen
however far the local branch lags.

Gate: the fleet RED case passes; `go test ./...`, `go vet ./...`,
`golangci-lint run` and `mdsmith check .` stay clean.

## Execution

Phase 1 is a small, well-fenced change to one resolver; Phase 2 proves
the consumer end to end and reconciles the doc. Both read cheaply off
the Context.

| Phase                       | Design | Implement | Gate that catches a wrong answer                             |
| --------------------------- | ------ | --------- | ------------------------------------------------------------ |
| 1 DefaultRef prefers remote | opus   | sonnet    | with origin/HEAD unset, a lagging local main is not returned |
| 2 Landed proven end to end  | opus   | sonnet    | Gather reads a squash-landed plan as landed, not held        |

## Acceptance Criteria

- [ ] `DefaultRef` returns the remote-tracking default when
      `origin/HEAD` is unset and the local branch lags
- [ ] `DefaultRef` still honors `refs/remotes/origin/HEAD` when set
- [ ] `Gather` reads a squash-landed plan as landed though local
      `main` lags
- [ ] S85 in
      [the protocol note](../docs/research/lease-protocol.md) describes
      the shipped behavior
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
