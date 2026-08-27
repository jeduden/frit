---
id: 2608271957
title: A stalled git network call cannot hang a frit verb
status: "🔲"
summary: >-
  release acts on one plan but first runs a full fleet gather that,
  with --fetch on by default, does one serial git fetch per repo — and
  no git subprocess has any timeout, so a single stalled fetch or push
  blocks the whole command indefinitely ("a minute and counting").
  Bound every git subprocess with a deadline so a stall fails fast: a
  timed-out fetch degrades to the local view and names the staleness, a
  timed-out push errors cleanly. Then surface and shrink the gather —
  per-repo progress during the fetch, fetches fanned out concurrently,
  and single-plan mutations scoped to their target repo.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: A bounded git runner fails a stalled call instead of hanging
    status: "🔲"
---
# A stalled git network call cannot hang a frit verb

## Goal

No frit verb hangs on a stalled git subprocess. A git network call
that does not return within a bound fails fast. A fetch degrades to the
local view; a push errors cleanly. So `frit release` returns in seconds
rather than "a minute and counting."

## Context

Reproduced by inspection this session (2026-08-27). `frit release`
ends one lane's lease on one plan. That is a single
`git push --force-with-lease`
([internal/claim/lease.go](../internal/claim/lease.go), `casPush`).
But [releaseCmd.Run](../cmd/frit/release.go) runs a **full fleet
gather first** ([gatherFleet](../cmd/frit/main.go) →
[fleet.Gather](../internal/fleet/gather.go)). Two properties of that
gather make it slow. One of them makes it hang:

1. `--fetch` defaults to **true**
   ([the Fetch flag](../cmd/frit/main.go)), so the gather runs
   `git fetch --prune --quiet <remote>` once per repository under the
   root, serially ([fetchRemote](../internal/fleet/gather.go)).
   Worktrees collapse to one repo, so for a one-repo fleet this is one
   fetch plus the push, ~4s; the cost scales with the repo count.
2. No git subprocess has any timeout.
   [gitwt.Exec / gitwt.ExecPipe](../internal/gitwt/git.go) use plain
   `exec.Command` + `cmd.Run()` — no context, no deadline, and SSH
   with no `ConnectTimeout`. Every network call — the per-repo fetch,
   the release push, a contested-push
   [ls-remote](../internal/claim/claim.go) — can block forever.

Steady-state is a few seconds; "a minute and counting" is a **stalled
network call** with nothing to bound it. This plan bounds it, then
surfaces and shrinks the gather.

**Reuse first.**
[presence.WithTimeout](../internal/presence/timeout.go) already bounds
a slow herdr runner with exactly this shape — a goroutine, a buffered
channel, a `select` on `time.After` — and documents the tradeoff (it
bounds the wait, not the process). Phase 1 mirrors it for `gitwt`
rather than inventing a second timeout. The seam is one line:
`rt.git` is assigned `gitwt.Exec` at
[the runtime construction](../cmd/frit/main.go); wrapping it there
bounds every git call for the whole run at one site.
[reap's progress writer](../cmd/frit/reap.go) — an `io.Writer` to
stderr, `io.Discard` under `--json` — is the pattern the later
progress lever copies, so the JSON contract (stdout is the whole
report) is kept. [freshBase](../internal/claim/claim.go) fetches a
single branch into `FETCH_HEAD` and is the spirit of the scope lever.

**Why a decorator, not `CommandContext`.** A `Runner` is a closure, so
a decorator is unit-testable red/green with a blocking fake and no
real git. Like `presence.WithTimeout` it bounds the wait rather than
killing the process; the abandoned git is orphaned when frit exits,
which is what the user sees end. Killing the child (via
`exec.CommandContext`) is a named follow-on, not Phase 1.

**Scope.** This plan does not change `--fetch`'s default or the
mutating verbs' own `freshBase`; it bounds, surfaces, parallelizes,
and scopes the existing gather.

## Tasks

1. Bound every git subprocess with a deadline so a stalled call fails
   fast instead of hanging (Phase 1).
2. Emit per-repo progress during the gather's fetch so a slow or
   stalled fetch is visible, silent under `--json` (shaped after
   Phase 1).
3. Fan the serial per-repo fetches out concurrently so a large root's
   gather is bounded by the slowest single fetch (shaped after
   Phase 1).
4. Scope single-plan mutation verbs (release, claim, yield) to fetch
   only their target repo rather than the whole fleet (shaped after
   Phase 1).

## Phase 1: A bounded git runner fails a stalled call instead of hanging

**RED.** In [internal/gitwt](../internal/gitwt), add
`gitwt_timeout_test.go`. Assert `WithTimeout(run, d)` wrapping a fake
`Runner` that blocks past `d` returns a non-nil error within roughly
`d` (not after the fake unblocks), and that a fake returning before `d`
passes its output and error through unchanged. Model the test on
[presence's timeout test](../internal/presence). The test fails to
compile: `WithTimeout` does not exist.

**GREEN.** Add the decorator, then wrap the runner at the seam.

- Add `WithTimeout(run Runner, d time.Duration) Runner` to
  [internal/gitwt/git.go](../internal/gitwt/git.go), mirroring
  [presence.WithTimeout](../internal/presence/timeout.go): a buffered
  channel, a goroutine running `run`, a `select` on `time.After(d)`
  returning a `"git: timed out after %s"` error. Document the
  bounds-the-wait-not-the-process tradeoff as presence does, and link
  the `CommandContext` upgrade as a follow-on.
- At [the runtime construction](../cmd/frit/main.go) wrap the runner:
  `git: gitwt.WithTimeout(gitwt.Exec, gitTimeout)`. `gitTimeout` is a
  generous backstop (a network fetch/push is seconds; a local op is
  ms), resolvable like frit's other settings — a `--git-timeout` flag
  with `env:"FRIT_GIT_TIMEOUT"` and a default (60s) that never trips a
  healthy call. Leave `gitPipe` on the batch path unwrapped, or wrap
  it too if the same deadline fits; a batch read is local, so the
  network bound is what matters.

**Gate (behavioral, against built frit).** Point a throwaway checkout's
`origin` at an unreachable SSH host and run
`go run ./cmd/frit --git-timeout 3s release <id>` (or any fetching
verb) inside it. Confirm it returns within a few seconds with the
fetch named as failed/stale — not "a minute and counting." Lint alone
does not prove this; the run does.

## Execution

| Phase | Tier   | Gate                                                                                                          |
| ----- | ------ | ------------------------------------------------------------------------------------------------------------- |
| 1     | sonnet | Built frit against an unreachable remote returns within the deadline with the fetch named stale, not hanging. |

## Acceptance Criteria

- [ ] `gitwt.WithTimeout` bounds a `Runner`; a fake blocking past the
      deadline returns an error within ~`d`, a fast fake passes
      through unchanged.
- [ ] `rt.git` is wrapped so every git call in a run is bounded, with
      the deadline resolvable from `--git-timeout` / `FRIT_GIT_TIMEOUT`
      and a default that never trips a healthy call.
- [ ] Against an unreachable remote, a fetching verb returns within
      the deadline and names the fetch stale rather than hanging.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
