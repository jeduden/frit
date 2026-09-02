---
n: 1
title: The five lease-level partition and clock rows run for real
status: "✅"
result: true
summary: >-
  S20, S21, S25, S33 and S34 are real Given/When/Then scenarios in
  features/partitions.feature and features/clocks.feature, none
  @pending, all bound in a new cmd/frit/bdd_partitions_and_clocks_test.go
  that appends its own registrar and reuses bdd_lease_test.go's
  vocabulary rather than redefining it. A partition Runner fails push,
  fetch and ls-remote; a second Runner shape runs the real push and
  fails only the client's own read of it, for the row where the push
  landed under an error. The observation window is a pure
  discovery.Window advanced on a clock each step chooses, never
  time.Now. Every step function carries its own unit test, including
  the two Runner wrappers.
---
## Handoff

### What is green

`go test ./cmd/frit -run 'TestFeatures/S(20|21|25|33|34):'` reports
all five as PASS, none SKIP. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run ./...` are clean.

- **S20** (worker partitioned mid-work): a `partitionRunner` wraps
  `gitwt.Exec` and fails `push`/`fetch`/`ls-remote`; "box-a"'s renewal
  under it reports `UnconfirmedPushError` and origin's tip does not
  move. An observer folds looks at that tip into a `discovery.Window`
  on a clock the step advances itself, past `DefaultTakeoverWindow`;
  `StaleHold` reads true. "box-b" takes the lease over (the existing,
  unconditional `claim.Takeover` — the window only demonstrates why an
  operator would call it, per the phase's own framing). On heal,
  "box-a"'s renewal is fenced naming "box-b", suggests yield, and
  yield parks its unpushed work.
- **S21** (push landed during partition): needed a real lane
  (`git worktree add`), since `Acquire` runs before the worktree
  exists and `persistToken` is a no-op then — see finding below. A
  `landedButUnconfirmedRunner` runs the real push through `gitwt.Exec`,
  discards the (successful) result, and returns a synthetic error; it
  also fails `ls-remote`, so `casPush` cannot reconcile and reports
  unconfirmed even though the push in fact landed. On heal,
  `claim.OwnAdvance` accepts origin's tip from the persisted token, and
  `claim.Resume` lands a beat at the same epoch.
- **S25** (stale unwind delete after heal): reuses S20's cut→stale→
  takeover setup, then "box-a" releases from its own recorded tip
  post-heal; the release is fenced naming "box-b" (the fence assertion
  itself is `bdd_lease_test.go`'s own, called directly — not
  redefined), origin still holds the takeover, and the work ref is
  still readable — there is no unleased delete a fenced release could
  have fired.
- **S33** (frozen clock): `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` pinned
  via `t.Setenv` for the subtest; two renewals mint two beats sharing
  one `%ct` but distinct SHAs (the marker's nonce, not the clock, keeps
  them apart — `internal/claim/lease_test.go`'s
  `TestAcquireMarkersNeverShareASHA` already pins this at the unit
  level). Sampling both beats resets the window to one sample with no
  void, since `discovery.Observe` keys staleness on tip change, not
  time; `StaleHold` reads false both moments after the last sample and
  a year after it.
- **S34** (clock steps backward): the second beat is dated years before
  the first; the tip still moved, the window still resets the same way,
  and `%ct` on the tip is smaller than on its parent — the date
  misleads only a human reading the log.

### Findings

- **`persistToken` is a no-op until the worktree exists.** `Acquire`
  is called before `herdr` (or, here, the test) builds the lane
  worktree, so its `persistToken` call silently does nothing —
  intentional, documented at `persistToken`'s own site. S21 could not
  reuse the shared `holdsTheLease` step (its lane is the placeholder
  `"/lanes/<holder>"`, never a real directory) for this reason; it
  needed its own `holdsTheLeaseInARealLane` step plus one ordinary,
  healthy renewal before arming the partition, to get a persisted
  token onto disk at all. This is existing, correct production
  behavior, not a defect — flagged here only because it cost a
  detour discovering it, and a later phase driving `Resume`/`OwnAdvance`
  through a real lane will hit the same thing.
- **The phase's own gate regex does not match.** `go test ./cmd/frit
  -run 'TestFeatures/S(20|21|25|33|34)_'` (trailing underscore, as
  written in this phase's spec) matches nothing:
  `TestFeatures` names each subtest `"<id>: <title>"`
  (`bdd_test.go`'s `t.Run(sc.ID+": "+sc.Name, ...)`), and `go test`'s
  `-run` turns the space into `_` but leaves the colon — so the real
  subtest name is `S20:_worker_partitioned_mid-work`, not
  `S20_worker...`. The colon form,
  `'TestFeatures/S(20|21|25|33|34):'`, is what actually selects the
  five. Worth fixing in whichever plan or skill text next quotes this
  gate shape — every later `bdd_*` phase's own gate line has the same
  bug waiting in it.

### What the later rows need

- **S22, S23, S24** (verb-level, `main.go`'s `run`): none can be
  driven through the lease API alone — they need a hand-built
  `&runtime{git: ...}` the way `unwindGit` in
  `cmd/frit/start_test.go` builds one, since `run` wires `rt.git` from
  `gitwt.WithTimeout` with no injection seam. `observeHolds(res, rt,
  now)` in `main.go` already takes an explicit `now` (see
  `cmd/frit/main.go:1229`), so S23's "everyone partitioned, all
  windows void on heal" can seed several `discovery.Window`s (via
  `observe.Save`/`seedWindow`'s pattern in `claim_test.go`) and assert
  none matures into a takeover. S24's asymmetric row (push ok, fetch
  fails) needs a Runner that fails only `fetch`/`ls-remote`, the
  mirror image of this phase's `partitionRunner` — trivial to build
  from the same shape. A dead or split remote is simpler for S22 and
  S24 than a fake Runner where the row is really about the CLI
  surface: point `remote.origin.url` at a path that does not exist for
  a full cut, or set `pushurl` apart from `url` for the row that must
  push but never fetch.
- **S35** (clock steps far forward): reuses this phase's
  `partitionRunner`/`landedButUnconfirmedRunner` shapes not at all —
  it is about an observer's own clock jumping ahead, which this
  phase's `observerWatchesTipGoStale`/`observerSamplesTheCurrentTip`
  steps already demonstrate the primitive for (advance `s.clock` by an
  arbitrary jump instead of `DefaultSampleGap`). The row's promise is
  that an early-firing takeover is still safe (CAS decides) and that
  `TakeoverCount` backs the threshold off — `claim.TakeoverCount` is
  already exported and unit-tested; a verb-level scenario would seed
  one or more takeover markers and assert the count, not just call the
  function directly.
- **S36** (cross-host clock skew): should hold two independent
  `discovery.Window`/clock pairs in the section state — not one, as
  this phase's `pcState` does — one host's clock offset by years from
  the other's. Both observe the same origin tip, each folds samples on
  its own clock via `discovery.Observe`, and the assertion is that both
  windows reach the same `StaleHold` answer despite the skew: nothing
  compares one host's timestamp against the other's, only each host's
  own elapsed time against its own last sample. `pcState` would need a
  second `window`/`clock` pair (or a small map keyed by host) rather
  than the single pair this phase's five rows never needed two of.

### What changed, in the source's own terms

New file: `cmd/frit/bdd_partitions_and_clocks_test.go`. `pcState` is
this section's own state via the existing `section[T]` mechanism — no
field was added to `world`, and neither `bdd_lease_test.go` nor
`bdd_test.go` was touched. It carries a `gitwt.Runner` override per
machine (absent means `gitwt.Exec`), which machines are cut off, each
machine's real lane path where one was built, the chain of tips this
section's own renewals produced, and one `discovery.Window` plus one
explicit clock. 25 new step functions are registered, each with its
own unit test (35 new test functions in total, including the two
Runner wrappers' own).
