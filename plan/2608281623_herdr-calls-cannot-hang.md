---
id: 2608281623
title: A stalled herdr call cannot hang a frit verb
status: "✅"
summary: >-
  `frit release` and any verb that reads presence for a held plan calls
  `herdr agent list` through `rt.herdr`, a raw unbounded subprocess. A
  wedged herdr socket hangs the verb forever with nothing printed. The
  git seam already fixed the identical shape: `rt.git` is wrapped with
  `gitwt.WithTimeout` at one place. This gives `rt.herdr` the same
  bound — a new `herdr.WithTimeout` decorator wired at the same seam
  behind a `--herdr-timeout` flag — so a stalled herdr fails fast and
  the verb finishes with presence merely absent, not hanging.
model: sonnet
depends-on: [2608271957]
phases:
  - n: 1
    title: A bounded herdr runner fails a stalled call instead of hanging
    status: "✅"
---
# A stalled herdr call cannot hang a frit verb

## Goal

No frit verb hangs on a stalled herdr subprocess. A `herdr agent list`
call that never returns fails after `--herdr-timeout` instead, and the
verb finishes with presence read as absent — exactly as an unreachable
herdr already reads — rather than blocking with nothing printed.

## Context

Plan [git-calls-cannot-hang](2608271957_git-calls-cannot-hang.md)
bounded every **git** call at one seam: [main.go](../cmd/frit/main.go)
wraps `rt.git` with `gitwt.WithTimeout` before dispatch. It left
herdr untouched, and herdr is the other subprocess a verb shells out
to.

**The hang.** `rt.herdr` is `herdrRunner`, which is `herdr.Exec`
([herdr.go](../internal/herdr/herdr.go)) — a bare
`exec.Command("herdr", …)` with no bound. `gatherFleet` →
`observeHolds` ([main.go](../cmd/frit/main.go)) calls
`herdr.List(rt.herdr)` for every **held** plan, to read whether the
bound session is gone. `frit release` and `frit claim` target a held
lane, so they take that path; `frit board` and `frit who` call
`herdr.List(rt.herdr)` directly. If the local herdr socket wedges, any
of these blocks forever, and because the block is before any output,
nothing is printed — the reported symptom.

**The pattern already exists, on the wrong type.**
[presence.WithTimeout](../internal/presence/timeout.go) bounds a
`herdr.ExecFunc` (`func(name, args…)`) for the multi-host board
fan-out, and [read.go](../internal/presence/read.go) wires it in. But
the single-host `rt.herdr` is a `herdr.Runner` (`func(args…)`), a
different signature, and no decorator bounds it. So the multi-host
path is safe and the single-host path is not.

**Reuse first.** No new machinery kind: this mirrors
[gitwt.WithTimeout](../internal/gitwt/git.go) — a buffered channel and
a goroutine race `time.After(d)`, bounding the wait not the process
(the orphaned herdr is reclaimed at exit; killing it via a context is
the same named follow-on the git plan left). It belongs in the
`herdr` package beside its `Runner` type, next to the existing
`presence.WithTimeout` it parallels. The flag mirrors `--git-timeout`
exactly: a `time.Duration` with an `env:` tag, resolved through the
same four-place precedence, and rejected up front when non-positive.

**Scope.** One decorator, one flag, one seam line. This changes no
report document, so no JSON golden moves; it adds a global flag, not a
verb, so no skill ships with it — the same as `--git-timeout`, which
shipped without one.

## Tasks

1. Add `herdr.WithTimeout`, a `Runner` decorator that fails a stalled
   call after a deadline, and wire `rt.herdr` through it at the
   dispatch seam behind a new `--herdr-timeout` flag, rejected when
   non-positive.

## Phase 1: A bounded herdr runner fails a stalled call instead of hanging

`herdr.WithTimeout(run Runner, d) Runner` returns a runner that passes
a prompt call through unchanged. If the wrapped call has not returned
within `d`, it returns a timeout error instead. Wiring `rt.herdr`
through it at the seam, behind `--herdr-timeout` (default 60s), makes
every herdr-reading verb finish against a wedged socket instead of
hanging.

**RED.** Two failing tests.

- A decorator unit test in
  [herdr_test.go](../internal/herdr/herdr.go)'s package: a `Runner`
  that blocks on a channel, wrapped with a tiny bound, returns a
  "timed out" error within roughly the deadline rather than blocking;
  a fast `Runner` passes its bytes and nil error straight through.
  Model it on `gitwt`'s `WithTimeout` test.
- A wiring test in [main_test.go](../cmd/frit/main_test.go), modelled
  on `TestGitTimeoutFlagReachesTheGitRunner`: run a herdr-reading verb
  (`who`) with `--herdr-timeout 1ns` against a runner whose herdr
  read would otherwise be seen, and assert it returns promptly with
  presence absent — the 1ns bound loses the race against every herdr
  call, so the verb completes with no live pane rather than hanging.
  Add `TestHerdrTimeoutMustBePositive` mirroring the git one: a `0s`
  bound exits non-zero and names `--herdr-timeout must be positive`.

**GREEN.** Four changes.

- Add `WithTimeout(run Runner, d time.Duration) Runner` to
  [herdr.go](../internal/herdr/herdr.go), the buffered-channel/select
  shape `gitwt.WithTimeout` uses, worded for herdr.
- Add the `HerdrTimeout` field to the `cli` struct in
  [main.go](../cmd/frit/main.go): `default:"60s"`,
  `env:"FRIT_HERDR_TIMEOUT"`, help "Fail a stalled herdr call after
  this.", beside `GitTimeout`. The longer `FRIT_HERDR_TIMEOUT` env name
  leaves no room for `GitTimeout`'s "after this long." wording under the
  120-column `lll` cap, so the help is trimmed.
- Reject a non-positive `HerdrTimeout` up front, in the same block
  that rejects `GitTimeout`.
- Wrap `rt.herdr = herdr.WithTimeout(rt.herdr, c.HerdrTimeout)` at the
  seam, right after the `rt.git` / `rt.gitPipe` wrapping.

Gate: build frit; the RED tests pass; `go test ./internal/herdr/...`
and `go test ./cmd/frit/...` are clean;
`go run ./cmd/frit who --herdr-timeout 1ns` returns at once rather
than hanging; `go test ./...`, `go vet ./...`,
`golangci-lint run` and `mdsmith check .` stay clean.

## Execution

One phase: the decorator, the flag and the seam wiring land together,
because a decorator with no seam bounds nothing and a flag with no
decorator has nothing to pass. The gate is the built frit returning
promptly under a 1ns bound — the whole point of the change.

| Phase                  | Design | Implement | Gate that catches a wrong answer                                                       |
| ---------------------- | ------ | --------- | -------------------------------------------------------------------------------------- |
| 1 Bounded herdr runner | opus   | sonnet    | built frit `who --herdr-timeout 1ns` returns at once with presence absent, not hanging |

## Acceptance Criteria

- [x] `herdr.WithTimeout` returns a timeout error when the wrapped
      runner has not returned within the bound, and passes a prompt
      call's output and error through unchanged
- [x] `rt.herdr` is wrapped at the dispatch seam, so `release`,
      `claim`, `board` and `who` all finish against a wedged herdr
      instead of hanging with nothing printed
- [x] `--herdr-timeout` (`FRIT_HERDR_TIMEOUT`, default 60s) is
      accepted, and a non-positive value is rejected up front naming
      the flag
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
