---
id: 2608271957
title: A stalled git network call cannot hang a frit verb
status: "✅"
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
    status: "✅"
  - n: 2
    title: fetchRemote no longer folds a probe failure into "no remote"
    status: "✅"
  - n: 3
    title: A repository git refuses to answer for is named, not dropped
    status: "✅"
  - n: 4
    title: casPush no longer misreports a landed push as lost
    status: "✅"
  - n: 5
    title: release and claim spend one deadline, not one per sequential call
    status: "✅"
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

**Phases 2-5, added post-Phase-1.** `/code-review xhigh --fix` on
Phase 1's own diff fixed what was in scope: gitPipe left unwrapped,
the timeout error dropping which subcommand stalled, no guard on a
non-positive `--git-timeout`. It also named four findings outside
that diff's touched files, deferred at the time. Bounding every git
call surfaced them. None is new — the timeout wrapper just makes each
one reachable, where before a stalled call more often just hung
instead:

- [fetchRemote](../internal/fleet/gather.go)'s own preliminary check
  for whether a remote is configured (`remote get-url`) folds *any*
  error into "not configured", including a probe that failed for a
  real reason. `staleFetch` then never fires, and the run trusts a
  remote-tracking view it never actually confirmed.
- [discover.Repos](../internal/discover/discover.go) silently drops a
  candidate `CommonDir`/`List` errors on — deliberately, so one broken
  checkout does not blind the walk. A bounded local call that stalls
  now takes that same silent path: the whole repository vanishes from
  every command's output with nothing said about why.
- [casPush](../internal/claim/lease.go)'s post-push reconciliation
  reads `remoteHolder` to classify a failed push. If the push actually
  landed — a connection dropped after the transaction committed — but
  that same stalled connection also fails the reconciliation read,
  `remoteHolder` returns `""` and casPush reports a real fault for a
  claim that silently succeeded.
- `release`/`claim` chain several independent sequential network
  calls against the one target repo (the gather's fetch, an
  `ls-remote`, the push, a post-failure `ls-remote`), each
  independently re-armed with the full `--git-timeout`. Against a
  fully stalled remote the total wait is a multiple of the deadline,
  not the deadline itself.

## Tasks

1. Bound every git subprocess with a deadline so a stalled call fails
   fast instead of hanging (Phase 1).
2. Stop folding a `remote`-probe failure into "not configured" so
   `fetchRemote` skips only a genuinely absent remote (Phase 2).
3. Surface a repository `discover.Repos` could not read instead of
   letting it vanish from every command's output (Phase 3).
4. Stop `casPush` misreporting a landed-but-unconfirmable push as a
   lost race (Phase 4).
5. Bound `release` and `claim`'s sequential network calls to one
   overall deadline instead of each re-arming the full
   `--git-timeout` (Phase 5).
6. Emit per-repo progress during the gather's fetch so a slow or
   stalled fetch is visible, silent under `--json` (shaped after
   Phase 1).
7. Fan the serial per-repo fetches out concurrently so a large root's
   gather is bounded by the slowest single fetch (shaped after
   Phase 1).
8. Scope single-plan mutation verbs (release, claim, yield) to fetch
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

## Phase 2: fetchRemote no longer folds a probe failure into "no remote"

**RED.** In [internal/fleet](../internal/fleet), add `fetch_test.go`
with a fake `gitwt.Runner`. Assert `fetchRemote` returns the probe's
own error when the `remote` listing call itself fails (today it
returns `nil` and silently skips the fetch); assert it still skips
without error when the listing succeeds and does not name the
remote; assert it still fetches, and returns the fetch's own result,
when the listing does name it.

**GREEN.** Replace the `remote get-url <remote>` probe — which cannot
tell "not configured" from "the probe itself failed" by error alone —
with `remote` (no args), the stable one-name-per-line listing, and
check membership. A listing failure now propagates as `fetchRemote`'s
own error, which flows into the existing `staleFetch` path exactly
like a failed fetch already does.

**Gate.** `go test ./internal/fleet/...`; no behavioral gate beyond
the tests, since the failure mode is a local probe, not a network one.

## Phase 3: A repository git refuses to answer for is named, not dropped

**RED.** In [internal/discover](../internal/discover), add a test
that a candidate whose `CommonDir` or `List` call errors is reported
back rather than silently omitted. `Repos` gains a second return
value alongside `[]Repo` — a slice naming each skipped candidate's
directory and error — so a caller can choose to surface it. Update
`Repos`'s doc comment: it no longer fails the candidate silently, it
reports and skips.

**GREEN.** Two changes, since the new return value ripples to every
caller:

- Change `discover.Repos`'s signature to
  `(repos []Repo, skipped []Skipped, err error)`, with `Skipped{Dir,
  Err}` a new small type.
- Update every call site. [fleet.Gather](../internal/fleet/gather.go)
  turns each skipped candidate into a `Problem`, the same channel a
  fetch failure already reports through, so it reaches `board`,
  `ready`, `plans` and the rest for free. The other call sites in
  [cmd/frit](../cmd/frit) (repos, reap, and the other bare
  `discover.Repos` reads) that have no `Problems` doc to carry it
  discard the second value explicitly — this phase does not thread a
  new diagnostic channel through every command that has no home for
  one yet.

**Gate.** `go test ./...` covers the new return value at the unit
level (a fake `Runner` erroring on `CommonDir`) and through
`fleet.Gather` (the repo appears as a `Problem`, not silently absent).

## Phase 4: casPush no longer misreports a landed push as lost

**RED.** In [internal/claim](../internal/claim), extend the
`casPush` tests. Add a fake `Runner` where the push itself errors,
and the follow-up `remoteHolder` read (`ls-remote`) *also* errors —
standing in for the same stalled connection dropping both calls.
Today that reads as `now == ""`, folded into "a real fault". Assert
instead that `casPush` reports the read failure distinctly from a
genuinely absent ref, so a caller can tell "unconfirmed" apart from
"confirmed absent".

**GREEN.** `casPush` already has the tool for this.
[remoteHolderErr](../internal/claim/claim.go) keeps a read fault
apart from a confirmed-absent ref; `remoteHolder` folds them for its
other callers, by design. That folding is documented as failing
safe, because a retry there re-attempts the same push cleanly.
`casPush` is different: a retry there mints a *new* marker commit, so
it cannot land on top of a push that already succeeded. Switch
`casPush`'s reconciliation to `remoteHolderErr`. When the read itself
fails — not merely reads "absent" — return that error as an
unconfirmed-push fault, not a plain one. The caller's message can
then say "the push may have landed; check before retrying" instead
of inviting a blind retry.

**Gate.** Unit tests on `casPush` cover this; the failure mode
requires two calls against the same connection to fail independently,
which is not reproducible against a real remote deterministically, so
it is not a behavioral gate.

## Phase 5: release and claim spend one deadline, not one per sequential call

**RED.** In [internal/gitwt](../internal/gitwt), add a test for
`WithDeadline(run Runner, deadline time.Time) Runner`: two sequential
calls through the same wrapped runner, the first one slow enough to
spend most of the deadline. The second call gets only what is left,
not a fresh full duration — assert it fails once the shared deadline
is exhausted, where the existing `WithTimeout` (a fixed duration
re-armed on every call) would let it through. This is the mechanism a
timing-based end-to-end test cannot pin deterministically; the
compounding-latency claim itself is the behavioral gate below, not an
automated test. `WithDeadline` does not exist yet: the test fails to
compile.

**GREEN.** Add `WithDeadline`. It shares `raceTimeout` with
`WithTimeout`, but computes each call's remaining budget from
`time.Until(deadline)`, not a fixed duration. A call made after the
budget is spent returns immediately, without starting. `release`,
`claim` and `yield` each still open with the fleet-wide
`gatherFleet`, which legitimately wants every repository's fetch
bounded independently — leave that on the existing per-call
`WithTimeout`. Right after it returns, reassign `rt.git` to
`WithDeadline(gitwt.Exec, time.Now().Add(c.GitTimeout))` for what
follows: the single repository's own lease work (a pre-push read, the
push, a retry's read), which now shares one clock instead of each
call getting a fresh `--git-timeout`.

**Gate (behavioral, against built frit).** Claim a plan against a
reachable remote, retarget `origin` to an unreachable one, then run
`go run ./cmd/frit --git-timeout 3s release <id>` and time it.
Confirm it still returns promptly, not hanging. Pinning the exact
per-call multiplier this way is unreliable — a sandboxed network often
fails an unroutable address fast rather than stalling out the full
bound. The unit tests above are what pin `WithDeadline`'s shared
budget deterministically; this gate only confirms the wiring does not
regress Phase 1's no-hang guarantee.

## Execution

| Phase | Tier   | Gate                                                                                                                |
| ----- | ------ | ------------------------------------------------------------------------------------------------------------------- |
| 1     | sonnet | Built frit against an unreachable remote returns within the deadline with the fetch named stale, not hanging.       |
| 2     | sonnet | `go test ./internal/fleet/...` passes.                                                                              |
| 3     | sonnet | `go test ./...` passes; a repo `discover.Repos` cannot read appears as a `fleet.Problem`, not a silent omission.    |
| 4     | sonnet | `go test ./internal/claim/...` passes.                                                                              |
| 5     | sonnet | Built frit: `release` against a fully stalled remote with `--git-timeout 3s` returns in roughly 3s, not a multiple. |

## Acceptance Criteria

- [x] `gitwt.WithTimeout` bounds a `Runner`; a fake blocking past the
      deadline returns an error within ~`d`, a fast fake passes
      through unchanged.
- [x] `rt.git` is wrapped so every git call in a run is bounded, with
      the deadline resolvable from `--git-timeout` / `FRIT_GIT_TIMEOUT`
      and a default that never trips a healthy call.
- [x] Against an unreachable remote, a fetching verb returns within
      the deadline and names the fetch stale rather than hanging.
- [x] `fetchRemote` skips a remote only when the listing genuinely
      does not name it; a listing failure surfaces as `fetchRemote`'s
      own error.
- [x] A repository `discover.Repos` could not read is named, not
      silently dropped, and reaches `fleet.Gather`'s `Problems`.
- [x] `casPush` tells a confirmed-absent ref apart from a
      reconciliation read that itself failed.
- [x] `release`/`claim` against a fully stalled remote return within
      roughly one `--git-timeout`, not a multiple of it.
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
