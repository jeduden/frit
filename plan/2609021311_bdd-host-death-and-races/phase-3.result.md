---
n: 3
title: S29, the release-vs-loser's-read race, runs for real
status: "✅"
result: true
summary: >-
  S29 drops `@pending` and passes as a real godog scenario. Its one new
  step, `claimsRacingARelease`, wraps `gitwt.Runner` for exactly one
  claimant's Acquire: the wrapper lies "absent" on that call's first
  `ls-remote`, letting Acquire attempt a real push against a ref its
  own read should have shown live, and releases the real holder's
  lease the instant that push fails for real — landing the race
  squarely inside `casPush`'s own reconciliation read.
---
## Handoff

**The design held exactly as specified, first try.** Every step of the
plan.md and phase-3.md analysis — the fresh-claim-only path into
`casPush`, the single `ls-remote` call `remoteHolder` makes,
`fetchedMarker`'s own fallback fetch bringing in an object the loser's
clone never fetched, `heldError` copying a release marker's `Kind`,
`Holder` and `Epoch` verbatim into the `HeldError` it returns — matched
the running code with no correction needed. The scenario passed on the
first `go test` run after implementation.

**One simplification the implementation made over the phase spec.**
The spec's wrapper sketch gated the injected release on "a push that
just failed"; the shipped `racingReleaseRunner` gates on that plus its
own `released` flag, so a second push in the same scenario — box-b's
own later retry, which reuses the *unwrapped* `gitwt.Exec` and so never
enters this wrapper at all — was never actually at risk. The flag is
kept anyway: a wrapper that could double-release on a retried Acquire
within its own scope would be a footgun for a future scenario that
reused it, not a defect in this one.

### What S32 still needs

Unchanged from phase 2's own handoff, refined by one confirmed
mechanical wrinkle this phase's own research turned up while tracing
`cmd/frit/start.go`: herdr's `worktree create` RPC never creates a
real git worktree itself — `standUpLane` delegates entirely to herdr
and never runs `git worktree add` locally. A stateful fake that only
remembers the `--cwd` argument and echoes it back in `agent list`
therefore hands the second `start` call's own presence read a path
nothing has checked out — `herdr.Resolve`'s `rev-parse
--show-toplevel` finds no repository there, and the live-lane match
silently fails to fire.

The fake must therefore, on intercepting `worktree create`, also
perform a real `git clone --branch <plan-branch> <origin-url>
<the --cwd path>` as a side effect — cloning from the same origin the
scenario's own `claimableRepo` set up — before answering with the fake
`pane_id`. Only then does a second, same-host `start --go` call's own
`liveLaneFor` walk find a real worktree on the plan's real branch at
that path, and `startLiveLaneRefusal` has something genuine to refuse
against.

The scenario's other precondition, confirmed by tracing `buildStart`
and `claimRefusal` together: `startLiveLaneRefusal` is reachable at
all only when `discovery.Ready` already includes the plan despite
`p.Held` being true — the S76 exception for a *matured* hold. A fresh,
unmatured claim on the same host would refuse earlier, via the
ordinary "already held" wording, before `startLiveLaneRefusal` ever
ran. So S32's second `start --go` call needs the first's window
seeded matured (`seedWindow`, the same step `theHoldsTakeoverWindowHasMatured`
already provides) before it runs — read back via `claim.RemoteTip`
into `w.lease.Tip` first, since the first call went through the CLI,
not the lease API, and left `w.lease` unset.

`go test ./...`, `go test ./internal/scenario` (the bijection gate),
`go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith check .`
are all clean.
