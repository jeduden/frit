---
id: 2608282218
title: A timed-out subprocess is killed, not left to linger
status: "✅"
summary: >-
  frit's timeout decorators bound the caller's wait but leave the
  stalled git or herdr subprocess running: it reparents to init when
  frit exits, and across a fleet walk stuck children pile up holding
  index.lock and network connections. Every decorator's comment names
  the fix — a context handed to `exec.CommandContext` — as a later
  refinement. This is that refinement: thread a context down into the
  exec so a fired bound kills the child. The plain Runner and ExecFunc
  every call site uses are unchanged; only the exec core and the
  decorators change.
model: sonnet
depends-on: [2608281623]
phases:
  - n: 1
    title: gitwt kills a timed-out git subprocess
    status: "✅"
  - n: 2
    title: herdr kills a timed-out herdr subprocess
    status: "✅"
  - n: 3
    title: presence kills a timed-out probe
    status: "✅"
---
# A timed-out subprocess is killed, not left to linger

## Goal

When a git or herdr call outlasts its bound, frit terminates the
subprocess instead of returning while it keeps running. A context
handed to `exec.CommandContext` kills the child the moment the bound
fires. A fleet walk that times out on many repositories then leaves no
pile of stuck git processes behind it.

## Context

Three decorators bound frit's subprocess calls:
[gitwt.WithTimeout / WithDeadline / WithTimeoutPipe](../internal/gitwt/git.go),
[herdr.WithTimeout](../internal/herdr/herdr.go) and
[presence.WithTimeout](../internal/presence/timeout.go). Each races an
opaque call against `time.After`. On timeout it returns an error while
the goroutine — and its subprocess — keep running. Every one says so:
"bounds the wait, not the process … killing it too needs a context
handed down to `exec.CommandContext`, which is a later refinement."
This plan is that refinement.

**Why it matters.** A returning `main` does not kill its children;
they reparent to init and run on. One stalled fetch is invisible, but
frit walks many repositories in one run and read verbs run in a loop,
so timed-out fetches accumulate — each holding a connection, some
holding `index.lock` — until they die on their own. Killing on timeout
bounds that to nothing.

**The mechanism.** `exec.CommandContext(ctx, …)` kills the child when
`ctx` is done, and `Run` then returns. So the bound moves down into the
exec. The decorator builds a context — `WithTimeout` from
`context.WithTimeout`, `WithDeadline` from `context.WithDeadline` — and
calls a context-aware base. `Run` returns within the bound because the
child was killed. The decorator words the same timeout error it words
today.

**Reuse first.** No new dependency: `exec.CommandContext` is the
std-lib follow-on both prior plans named. The base exec functions gain
a `ctx` parameter. The plain `Runner`, `PipeRunner` and `ExecFunc` that
every call site uses stay contextless, because the decorator still
returns one. Only the base and the decorators change. The seam in
[main.go](../cmd/frit/main.go) swaps `gitwt.Exec` for
`gitwt.ExecContext` under the same `WithTimeout` call.

**Scope.** Killing the direct child unblocks frit and stops the leader
lingering. A `git fetch` can spawn `ssh` or a credential helper of its
own; a `SIGKILL` to git closes git's pipes, so the grandchild is
orphaned but no longer blocks frit. Killing the whole process group
(`Setpgid`, then kill the negative pgid) is a further refinement, noted
not taken: it is separate from the context plumbing and can be its own
phase if a lingering `ssh` proves to matter.

**Testing the kill.** The proof a child was killed rather than
abandoned is that the exec's own `Run` returns within the bound — `Run`
cannot return before its process ends. A test runs a genuinely slow
command (`sleep`) through the context-aware core with a tiny bound and
asserts it returns in well under the sleep, which is impossible unless
the child was killed. The existing timeout tests, which assert the
caller returns fast, stay green; this adds the stronger claim.

## Tasks

1. gitwt: add a context-aware exec core and rebuild `WithTimeout`,
   `WithDeadline` and `WithTimeoutPipe` over it, so a timed-out git
   subprocess is killed.
2. herdr: give herdr a context-aware exec and rebuild
   `herdr.WithTimeout` over it, so a timed-out herdr subprocess is
   killed.
3. presence: rebuild `presence.WithTimeout` over a context-aware exec,
   so a timed-out remote probe is killed.

## Phase 1: gitwt kills a timed-out git subprocess

Establish the context-aware exec core and the kill-proof test, then
rebuild gitwt's three decorators over it. This is the proving slice: it
fixes the test shape Phases 2 and 3 copy.

RED lives in
[gitwt_timeout_test.go](../internal/gitwt/gitwt_timeout_test.go). A
test runs the context-aware core against a real slow child —
`runContext(ctx, "sleep", "5")` — under a 20ms context, and asserts it
returns in under one second with an error. `runContext` does not exist
yet, so this fails to compile. Once it does, the fast return proves the
child was killed, because `Run` cannot return before its process ends.

GREEN, in [git.go](../internal/gitwt/git.go):

- Add `runContext(ctx, name, args…)`, the shared exec core over
  `exec.CommandContext`, wording stdout, stderr and the error exactly
  as `Exec` does today.
- Add `ExecContext(ctx, dir, args…)` and `ExecPipeContext(ctx, dir,
  stdin, args…)` over it. Keep `Exec` and `ExecPipe` as thin wrappers
  passing `context.Background()`, so any caller still on them is
  unaffected.
- Rebuild `WithTimeout` and `WithTimeoutPipe` to take the context-aware
  base and return a plain `Runner` / `PipeRunner`: build
  `ctx, cancel := context.WithTimeout(context.Background(), d)`,
  `defer cancel()`, call the base. Rebuild `WithDeadline` over
  `context.WithDeadline`. The child dies when the context fires.
- Swap the seams: `rt.git = gitwt.WithTimeout(gitwt.ExecContext, …)`
  and the pipe in [main.go](../cmd/frit/main.go); the `WithDeadline`
  swaps in [release.go](../cmd/frit/release.go) and
  [claim.go](../cmd/frit/claim.go) pass `gitwt.ExecContext`.
- Migrate the existing `gitwt_timeout_test.go` fakes to the context
  signature, and reword the "bounds the wait, not the process"
  comments to say the process is killed.

Gate: build frit; RED passes; `go test ./internal/gitwt/...` and
`go test ./cmd/frit/...` are clean; against an unreachable remote — the
manual check plan
[git-calls-cannot-hang](2608271957_git-calls-cannot-hang.md) used — the
verb returns bounded and `ps` shows no git process left behind;
`go test ./...`, `go vet ./...`, `golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: herdr kills a timed-out herdr subprocess

Apply Phase 1's mechanism to herdr, whose `run(name, args…)`
([herdr.go](../internal/herdr/herdr.go)) shells out to an arbitrary
binary — the local `herdr` or `ssh <host> herdr`.

RED, in the herdr test package ([internal/herdr](../internal/herdr),
where plan 2608281623 adds `timeout_test.go`): the context-aware herdr
core run against `sleep 5` under a 20ms bound returns in under a second.
The core does not exist yet.

GREEN: add a context-aware core to
[herdr.go](../internal/herdr/herdr.go) over `exec.CommandContext`,
rebuild `herdr.WithTimeout` over it to return a plain `Runner`, and
keep `Exec` / `Run` as `context.Background()` wrappers. Swap the seam
so `rt.herdr` wraps the context-aware exec in
[main.go](../cmd/frit/main.go). Reword the comment.

Gate: build frit; RED passes; `go test ./internal/herdr/...` and
`go test ./cmd/frit/...` are clean;
`go run ./cmd/frit who --herdr-timeout 1ns` returns bounded and leaves
no herdr process behind; `go test ./...`, `go vet ./...`,
`golangci-lint run` and `mdsmith check .` stay clean.

## Phase 3: presence kills a timed-out probe

[presence.WithTimeout](../internal/presence/timeout.go) bounds a
`herdr.ExecFunc` (`func(name, args…)`) for the multi-host board
fan-out, where a slow `ssh <host> herdr` is the call that stalls.
Rebuild it over the context-aware exec so that probe is killed too.

RED, in [timeout_test.go](../internal/presence/timeout.go)'s package: a
context-aware `ExecFunc` run against `sleep 5` under a 20ms bound
returns in under a second.

GREEN: rebuild `presence.WithTimeout` to build a context and call a
context-aware `ExecFunc` (the herdr core from Phase 2, in `ssh` form).
Wire it in [read.go](../internal/presence/read.go). Reword the comment.

Gate: build frit; RED passes; `go test ./internal/presence/...` is
clean; `go test ./...`, `go vet ./...`, `golangci-lint run` and
`mdsmith check .` stay clean.

## Execution

Phase 1 is the proving slice: it builds the context-aware exec core and
the sleep-based kill test, and rebuilds gitwt's three decorators — the
shape Phases 2 and 3 copy for herdr and presence. Each phase is one
file's decorators plus its seam.

| Phase               | Design | Implement | Gate that catches a wrong answer                                                       |
| ------------------- | ------ | --------- | -------------------------------------------------------------------------------------- |
| 1 gitwt kills child | opus   | sonnet    | context-aware exec of `sleep 5` under a 20ms bound returns in <1s; no git left in `ps` |
| 2 herdr kills child | opus   | sonnet    | context-aware herdr exec of `sleep 5` under a 20ms bound returns in <1s                |
| 3 presence kills    | opus   | sonnet    | context-aware ExecFunc of `sleep 5` under a 20ms bound returns in <1s                  |

## Acceptance Criteria

- [x] gitwt's `WithTimeout`, `WithDeadline` and `WithTimeoutPipe` run
      over `exec.CommandContext`, so a timed-out git subprocess is
      killed, proven by the exec's own `Run` returning within the bound
- [x] `herdr.WithTimeout` runs over `exec.CommandContext`, so a
      timed-out herdr subprocess is killed
- [x] `presence.WithTimeout` runs over `exec.CommandContext`, so a
      timed-out remote probe is killed
- [x] `Exec`, `ExecPipe` and `herdr.Run` keep working for any caller
      still on the contextless form
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
