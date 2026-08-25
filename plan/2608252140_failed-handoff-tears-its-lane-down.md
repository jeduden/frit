---
id: 2608252140
title: A failed handoff tears its lane down, so start leaves no half-built lane
status: "🔳"
summary: >-
  start mints the claim, then hands the lane to herdr: worktree, pane,
  agent. When the agent start loses a race with a pane whose shell is
  not yet ready, start releases the lease but only names the worktree
  and pane it created as "left behind". The abort is asymmetric — a
  freed claim over a live worktree — and the "no lane half-built"
  contract is broken. Fix it: a handoff that fails after the pane
  exists tears its own worktree and pane down before releasing the
  lease, a clean abort; only a teardown that itself fails falls back to
  naming what was left. Then retry the pane-not-ready race so the
  failure path is rarely reached at all.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: A failed handoff tears down the worktree and pane it stood up
    status: "✅"
  - n: 2
    title: Retry the pane-not-ready race so the handoff rarely fails
    status: "🔲"
---
# A failed handoff tears its lane down, so start leaves no half-built lane

## Goal

When `start` — and `pick --go` through it — mints the claim but then
fails to stand the lane up, it tears down the worktree and pane it
created and then releases the lease, so the abort is atomic: no freed
claim left standing over a live worktree. A teardown that itself fails
is the only case that falls back to naming what was left behind.

## Context

**The scenario, observed.** A `pick --go` minted the claim for a plan,
stood up the worktree and pane, then failed with:

```json
{"error":{"code":"agent_pane_busy",
  "message":"agent target pane w2B:p1 is not an available shell"}}
```

The pane had just been created and its shell had not settled, so the
immediate agent start lost the race. `pick` then released the lease but
left the worktree and pane running. The result was an inconsistent
half-state: the plan read unheld, yet a worktree and a live pane stood
on its branch — exactly the "no lane half-built" contract the code
means to keep.

**The mechanism.** [standUpLane](../cmd/frit/start.go) hands the
checkout, the agent, the prompt and the focus to herdr in turn. When
`herdr.AgentStart` fails it returns the pane it already opened, and the
caller's failure branch in [buildStart](../cmd/frit/start.go)
wraps the cause with [handoffError](../cmd/frit/start.go) — which only
*names* the worktree and pane — and calls
[releaseLease](../cmd/frit/start.go). The lease unwinds; the herdr
resources do not. The rollback is asymmetric.

**Reuse first.** The teardown primitive already exists.
[herdr.WorktreeRemove](../internal/herdr/dispatch.go) tears a checkout
down by the workspace it is open in, and
[reap](../cmd/frit/reap.go) already drives it. `WorktreeRemove` takes
only the workspace handle; `yield` resolves it with
[herdr.CurrentPane](../internal/herdr/dispatch.go), but here the pane
`standUpLane` returns already carries it — herdr names a pane
`<workspace>:<pane>`, so the workspace is the segment before the colon.
This plan reuses `WorktreeRemove` rather than adding a second teardown.

**Not a delete.** The lease release stays a pushed marker, never a ref
delete, so the next acquire reads epoch E+1 — the existing
`releaseLease` behavior is unchanged. Only the herdr side of the
unwind is added.

**The test seam.** [startHerdr](../cmd/frit/start_test.go) is a fake
`herdr.Runner` that records calls and returns canned responses. A test
drives the failure by making `agent start` return an error, then
asserts what the unwind did — `worktree remove` called, the lease
released — the same seam the existing start tests use.

## Tasks

1. A handoff that fails after the pane exists tears down the worktree
   and pane, then releases the lease; only a teardown that itself fails
   names what was left behind.
2. (determined after Phase 1)

## Phase 1: A failed handoff tears down the worktree and pane it stood up

A handoff that fails once a pane exists must remove that worktree and
pane before releasing the lease, so the abort leaves nothing standing.
This slice proves the symmetric unwind end to end and fixes the test
approach Phase 2 copies.

RED, at the [cmd/frit](../cmd/frit) level with the `startHerdr` fake
extended so `agent start` returns an error:

- The common case: after the failure, `worktree remove --workspace wZ`
  is called, the lease is released, and the reported error does not
  claim a worktree and pane were "left behind" — the abort is clean.
- The teardown-fails case: when `worktree remove` itself errors, the
  reported error names the worktree and pane left behind, and the lease
  is still released, so a failed unwind is surfaced for `orphans`, not
  swallowed.

GREEN: in `buildStart`'s failure branch, when `standUpLane` returns a
non-empty pane, derive the workspace from the pane (the segment before
`:`) and call `herdr.WorktreeRemove`. On success the error stays the
plain cause. On teardown failure, wrap with `handoffError` and join the
teardown error. Then `releaseLease` as today. Add a small helper for
the pane-to-workspace teardown so it carries its own unit test.

Gate: both RED cases pass; a simulated agent-start failure runs
`worktree remove` and reports a clean abort with no "left behind";
`go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: Retry the pane-not-ready race so the handoff rarely fails

The failure Phase 1 cleans up after is a transient race: the pane herdr
just created is not yet an available shell when the agent start targets
it. A bounded retry turns that transient into a success, so the
teardown path is rarely reached at all.

RED, at the [cmd/frit](../cmd/frit) or
[internal/herdr](../internal/herdr) level:

- The fake `agent start` returns the pane-not-ready error once, then
  succeeds. The lane comes up started: no `worktree remove`, no lease
  release, and `agent start` was attempted more than once.
- A non-transient `agent start` error is not retried: it fails after a
  single attempt and drops into Phase 1's teardown.

GREEN: wrap the agent start in `standUpLane` in a bounded retry that
retries only when the error matches herdr's pane-not-ready signal (code
`agent_pane_busy`, message `not an available shell`). Between attempts,
pause through an injectable delay seam — a package variable set to a
no-op in tests, mirroring `openEditor`. Confirm `herdr.AgentStart`
surfaces the error body so the code is matchable; surface it if it does
not.

Gate: a transient pane-busy error retried to success stands the lane up
with no teardown and no release; a non-transient error is not retried;
`go test ./...`, `go vet ./...`, `golangci-lint run` and
`mdsmith check .` stay clean.

## Execution

Phase 1 makes the abort atomic — the load-bearing fix for the observed
half-state. Phase 2 removes the trigger, so the atomic abort is the
rare path rather than the common one.

| Phase                           | Design | Implement | Gate that catches a wrong answer                                     |
| ------------------------------- | ------ | --------- | -------------------------------------------------------------------- |
| 1 failed handoff tears down     | opus   | sonnet    | a simulated agent-start failure runs worktree remove; abort is clean |
| 2 retry the pane-not-ready race | opus   | sonnet    | a transient pane-busy error retries to success with no teardown      |

## Acceptance Criteria

- [x] A handoff that fails after the pane exists tears down the
      worktree and pane, then releases the lease
- [x] After such a failure nothing is left standing and the plan reads
      unheld
- [x] A teardown that itself fails falls back to naming the worktree
      and pane left behind, and still releases the lease
- [ ] The transient pane-not-ready agent-start error is retried and the
      lane comes up without teardown
- [ ] A non-transient agent-start error is not retried
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
