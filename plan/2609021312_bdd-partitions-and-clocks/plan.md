---
id: 2609021312
title: The partition and clock scenarios run under godog
status: "✅"
summary: >-
  The lease-protocol matrix's "Partitions" section, S20..S25, and its
  "Clocks" section, S33..S36, are declared in features/partitions
  .feature and features/clocks.feature but every row is still
  @pending: declared, skipped, proving nothing. This plan writes each
  of the ten as a real Given/When/Then over the lease API, a git
  runner that fails like a cut network, the observation window driven
  on an explicit clock, and the cmd/frit fixtures, bound in one step
  file for both sections, so a regression in what the doc promises
  under a partition or a wrong clock fails the build. It stands alone:
  no other conversion plan is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The partition and clock scenarios run under godog

## Goal

Every row of the matrix's partition section (S20..S25) and clock
section (S33..S36) is a passing godog scenario. None is tagged
`@pending`. A regression in any promise the two sections make — a cut
holder that stops rather than pushes, a push that landed while the
client saw an error, a window that reads tip change and never a
timestamp — fails `go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real,
and [partitions.feature](../../features/partitions.feature) and
[clocks.feature](../../features/clocks.feature) declare S20..S25 and
S33..S36 with `@pending`. `TestFeatures` skips each one. These are the
rows the doc leans on hardest for its safety story — CAS decides, not
a clock, not a hostname — and today nothing executes a single one.

**What already exists, and is reused.** The lease API in
[internal/claim](../../internal/claim) covers the ref side: `Acquire`,
`Renew`, `Resume`, `Release`, `Takeover`, `Yield`, `RemoteTip`,
`ReadToken`, `OwnAdvance`. Every one takes a `gitwt.Runner`, the
`func(dir string, args ...string) ([]byte, error)` type in
[internal/gitwt/git.go](../../internal/gitwt/git.go). A partition is
therefore a Runner that wraps `gitwt.Exec` and fails `push`, `fetch`
and `ls-remote`. The CAS already classifies that shape.
[caspush_test.go](../../internal/claim/caspush_test.go) pins a push
whose confirming read also fails as `UnconfirmedPushError`. It pins a
push that errored yet landed as a win. The observation window is pure
and clocked by its caller. `discovery.Observe(window, tip, now, sMax)`
and `discovery.StaleHold(window, now, t, sMax)` in
[internal/discovery/stale.go](../../internal/discovery/stale.go) take
an explicit `now`. [internal/observe](../../internal/observe) persists
a window with `Save`, `Load`, `Key` and `Path`; `isolate` points that
store at a throwaway cache. `seedWindow` in
[claim_test.go](../../cmd/frit/claim_test.go) already writes one with
chosen times. Commit dates are faked the way
[lease_test.go](../../internal/claim/lease_test.go) does, through
`GIT_AUTHOR_DATE` and `GIT_COMMITTER_DATE`. The vocabulary in
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) already says
"holds the lease", "commits work it never pushes", "takes the lease
over", "comes back and renews", "the renewal is fenced" and "yield
parks". Those steps are reused as they stand. `claimableRepo` and
`cloneAgain` build the origin-and-clone pair. Unit tests already pin
S21's mechanism; `grep -rn "S21" cmd/frit internal` finds the resume
tests. Each scenario mirrors the unit test's fixture rather than
inventing one.

**The seam that does not exist.** The verbs build their runtime at one
place: `run` in [main.go](../../cmd/frit/main.go) sets `rt.git` from
`gitwt.WithTimeout` and offers no injection. A unit test wraps the
Runner only by building `&runtime{git: ...}` by hand and calling a
verb's function directly — `unwindGit` in
[start_test.go](../../cmd/frit/start_test.go) is the pattern. A
scenario that must go through the CLI can cut the network with git
alone instead: point `remote.origin.url` at a path that does not
exist, or split `url` from `pushurl` for the asymmetric row.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Lease API plus a failing Runner plus an explicit clock, drivable
  now: S20 (a cut holder's renewal comes back unconfirmed and moves
  nothing; the observer's window matures on the unchanged tip; the
  takeover lands; on heal the holder is fenced and yields), S21 (a
  push that landed while the client saw an error is still the lane's
  own advance, and resume takes it at the same epoch), S25 (a release
  from the holder's own recorded tip is a CAS; fenced, it deletes
  nothing and origin still holds the takeover), S33 and S34 (a frozen
  or backward-stepping commit clock changes no decision: the window
  resets on tip change and matures on the observer's own elapsed
  time; `%ct` on the markers is what misleads).
- Verb-level, over a dead remote URL: S22 (the board carries the
  observed-at age and mutates no ref), S23 (`observeHolds` called
  directly with explicit times voids every window whose gap exceeded
  S_max, so no takeover fires on heal), S24 (a failed fetch degrades
  to a Problem and never corrupts what the board already knows a
  renewal landed elsewhere).
- Lease API plus an explicit clock, drivable without any of the
  above: S35. Phase 2's own research found the pure discovery
  functions cannot make a window mature "early" from a single clock
  jump — any gap that size just voids it, same as a partition. The
  row instead reuses S20's own maturation loop for an ordinary
  takeover, then proves `claim.TakeoverCount`'s backoff through a
  real chain: the same span that reads stale under the bare takeover
  window does not once one takeover marker has backed the threshold
  off.
- Resolved by argument, asserted as an observable: S36 (two hosts
  whose clocks differ by years each observe the same tip; each
  window's span is on its own clock and both converge on the
  tip-change rule; no marker timestamp enters any decision).

**Convention for a row the doc resolves by argument.** S33 says
"timestamps are decoration". The scenario asserts the observable: two
markers carry the same commit date, their SHAs differ, and the window
over the second reads `Samples: 1` with no void. Every such row
becomes an assertion about what a verb, the store or the remote
shows, never a comment.

**Where the steps go.** Both sections' steps live in one new
`cmd/frit/bdd_partitions_and_clocks_test.go`, appended to the step
registry from `init` exactly as `bdd_lease_test.go` is. This plan
never edits `bdd_test.go`, and touches no feature file but its two.
The sibling conversion plans — process death, host death and races,
storage, identity and cross-layer, the two lifecycle halves — each
own their file the same way, so all land in any order. A step text
this file defines that another already has fails as ambiguous under
godog's strict mode; the fix is to reuse the existing step.

**Out of scope.** No change to the lease protocol or to any verb. A
scenario that cannot be made to pass without changing behaviour is a
finding, parked in the handoff with the row it concerns, not a fix
made here.

## Tasks

1. Phase 1 (proving slice): the five rows the lease API, a partition
   Runner and an explicit clock can drive — S20, S21, S25, S33, S34 —
   written and passing in `bdd_partitions_and_clocks_test.go`, the
   file registered, the partition Runner and the clocked observer
   fixed for later rows. Driven red by dropping `@pending`: strict
   mode fails the undefined steps.
2. Later phases, shaped by Phase 1's handoff: the verb-level rows S22,
   S23, S24, S35 over a hand-built runtime, `observeHolds` with
   explicit times and a dead or split remote URL; then S36, the
   cross-host skew row asserted as two clocked observers converging.

## Execution

| Phase | Title                                                                          | Tier   | Gate                                                                                                                                                         |
| ----- | ------------------------------------------------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | The five lease-level partition and clock rows run for real                     | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(20\|21\|25\|33\|34):'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean |
| 2     | The three verb-level partition rows and the far-forward clock row run for real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(22\|23\|24\|35):'` passes with no SKIP; `go test ./...` and golangci-lint clean                                     |
| 3     | The cross-host clock skew row runs for real, closing the matrix's ten          | sonnet | `go test ./cmd/frit -run 'TestFeatures/S36:'` passes with no SKIP; `go test ./...` and golangci-lint clean                                                   |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| --- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The five lease-level partition and clock rows run for real](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
|     | ↳      | S20, S21, S25, S33 and S34 are real Given/When/Then scenarios in features/partitions.feature and features/clocks.feature, none @pending, all bound in a new cmd/frit/bdd_partitions_and_clocks_test.go that appends its own registrar and reuses bdd_lease_test.go's vocabulary rather than redefining it. A partition Runner fails push, fetch and ls-remote; a second Runner shape runs the real push and fails only the client's own read of it, for the row where the push landed under an error. The observation window is a pure discovery.Window advanced on a clock each step chooses, never time.Now. Every step function carries its own unit test, including the two Runner wrappers.                                                    |
| 2   | ✅     | [The three verb-level partition rows and the far-forward clock row run for real](phase-2.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
|     | ↳      | S22, S23, S24 and S35 are real Given/When/Then scenarios, none @pending, all bound in the existing cmd/frit/bdd_partitions_and_clocks_test.go via a second registrar kept apart only for the file's own lint budget. S22 and S24 go through the real board verb against a second, breakable checkout — fetchRemote/staleFetch's own degrade, never a fake Runner. S23 drives observeHolds directly against a synthetic fleet and an explicit clock, no repository at all. S35 corrects Phase 1's own suggestion of an early-firing window — the pure discovery functions cannot produce one — and instead proves the takeover backoff through a real chain. Every new step function carries its own unit test.                                      |
| 3   | ✅     | [The cross-host clock skew row runs for real, closing the matrix's ten](phase-3.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
|     | ↳      | S36 is a real Given/When/Then scenario, no longer @pending, bound in a third registrar in the existing cmd/frit/bdd_partitions_and_clocks_test.go. pcState carries two new maps, hostWindows and hostClocks, keyed by host rather than the singular pair every earlier row in this file shares. Each host matures its own window independently, on a clock skewed years from the other's, through the same maturation loop S20's own observerWatchesTipGoStale already uses; the Then step reads each host's StaleHold only against its own recorded clock, so convergence is proven by construction, never asserted as prose. Every new step function carries its own unit test. All ten rows this plan opened — S20-S25, S33-S36 — pass together. |
<?/catalog?>

## Acceptance Criteria

- [x] No scenario in `features/partitions.feature` or
      `features/clocks.feature` carries `@pending`; `go test
      ./cmd/frit -run TestFeatures/S` reports S20..S25 and S33..S36 as
      PASS, none as SKIP
- [x] Every step is bound in
      `cmd/frit/bdd_partitions_and_clocks_test.go` or reused from
      `bdd_lease_test.go`; `bdd_test.go` is untouched
- [x] Each scenario asserts an observable — a verb's result, origin's
      refs, a marker, a stored observation — never a comment
- [x] No scenario reads the wall clock into a decision: every window
      is observed on a time the step chose
- [x] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
