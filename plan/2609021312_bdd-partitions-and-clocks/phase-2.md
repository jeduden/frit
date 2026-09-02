---
n: 2
title: The three verb-level partition rows and the far-forward clock row run for real
status: "🔲"
result: false
---
Convert the four rows Phase 1's handoff flagged as needing more than
the lease API: S22, S23, S24 (Partitions), S35 (Clocks). S22 and S24
go through the real CLI's `board`; S23 calls `observeHolds` directly;
S35 stays at the lease-API level Phase 1 proved, adding the takeover
backoff Phase 1 never exercised.

**Assumes.** What Phase 1's handoff recorded, plus:

- `fetchRemote`/`staleFetch` in
  [gather.go](../../internal/fleet/gather.go) are not fatal: a failed
  `git fetch` becomes a `Problem` ("could not fetch ...; may be
  stale") only once a `refs/remotes/<remote>/*` ref already exists to
  call stale, and `leaseTips` falls back to whichever ref it has. S22
  and S24 need only a real, breakable `remote.origin.url` — no fake
  `gitwt.Runner`.
- `observeHolds(res *fleet.Result, rt *runtime, now time.Time)`
  ([main.go](../../cmd/frit/main.go)) mutates each `discovery.Plan`'s
  `Stale`/`StaleFor`/`Voided` via `discovery.Observe`/`StaleHold`,
  reading window and sample gap off `res.Coords[p.Repo]`
  (`staleClock` defaults when absent); `claim.TakeoverCount` only
  runs when a coord is present. A hand-built `fleet.Result{Plans:
  ...}` with no `Coords` drives S23 with no repository and no Runner.
- `report.BoardDoc` ([board.go](../../internal/report/board.go))
  carries `StaleSeconds` per plan and `Problems` per repo;
  `board`'s `Run` fetches by default and writes it via
  `report.WriteJSON`; `emit` in
  [json_test.go](../../cmd/frit/json_test.go) already decodes it.
- `claim.TakeoverCount(repoDir, planID, base, tip, run)` in
  [lease.go](../../internal/claim/lease.go) is the backoff S35
  drives end to end, not a fresh mechanism.
- `seedWindow` in [claim_test.go](../../cmd/frit/claim_test.go)
  writes a matured `observe.State` entry; reused as-is for S22.
- `cloneAgain` in [bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go)
  clones into its own `t.TempDir()`; S22/S24 need a clone under a
  shared `--root` instead, `initRepo`'s own layout — a new helper.

**Value.** `board` is the verb an operator actually reads under a
partition; nothing in Phase 1 touched it. S22 and S24 pin that a read
failure degrades to a caveat, never a wrong answer, and never touches
a mutation happening elsewhere. S23 pins the line that keeps a
recovering fleet from stampeding: an outage voids every window rather
than reading its wall-clock gap as one long stale span. S35 closes
Phase 1's gap: the backoff that damps a wrongly-early takeover is
`TakeoverCount`, driven through a real chain, not the bare function.

**RED.** Drop `@pending` from S22, S23, S24 in
[partitions.feature](../../features/partitions.feature) and S35 in
[clocks.feature](../../features/clocks.feature); write each
Given/When/Then below. `go test ./cmd/frit -run
'TestFeatures/S(22|23|24|35):'` reports the new steps undefined —
commit that red.

```gherkin
@S22
Scenario: observer partitioned
  Given "box-a" holds the lease for plan 22
  And an observer has already watched "box-a"'s tip for a while
  When the network cuts the observer off from origin
  And the observer reads the board
  Then the board reports "box-a"'s observed-at age
  And the board reports the observer's fetch as unreachable
  And origin's tip has not moved

@S23
Scenario: everyone partitioned, origin up
  Given several held plans were each observed a while ago
  And the gap since each one's last sample exceeds the sample-gap bound
  When the fleet is observed again, now that origin is reachable
  Then every window resets to one sample
  And no plan reads its takeover window matured

@S24
Scenario: asymmetric: push ok, fetch fails
  Given "box-a" holds the lease for plan 24 in a real lane
  When "box-a" renews its lease
  Then the renewal is a plain win
  When an observer clones origin, catching up with the renewed tip
  And the network cuts the observer off from origin
  And the observer reads the board
  Then the board reports the observer's fetch as unreachable
  And the board still reports "box-a" held at the renewed tip

@S35
Scenario: clock steps far forward
  Given "box-a" holds the lease for plan 35
  When an observer watches "box-a"'s tip go stale
  Then the window reads the hold stale
  When "box-b" takes the lease over
  Then origin holds the takeover
  When a further observer watches "box-b"'s tip mature by the same span
  Then that span does not read stale once the takeover count backs the threshold off
```

**GREEN.** Every step appends to `registerPartitionsAndClocks`, in
the same `bdd_partitions_and_clocks_test.go` file Phase 1 opened
under [cmd/frit](../../cmd/frit). Grow `pcState` only for a field two
rows actually share.

- **S22.** `w.holdsTheLease` (reused) puts "box-a" on a real origin.
  `seedWindow(w.t, "atlas", 22, w.lease.Tip, 3*time.Hour)` matures the
  observer's window before anything is cut, so the assertion is not a
  race against the test's own runtime. A new helper,
  `observerClone(t, root, name, originURL string) string`, clones into
  `filepath.Join(root, name)` — the sibling-of-`root` layout `board
  --root root` walks, unlike `cloneAgain`'s own `t.TempDir()`. Cutting
  it off is `git remote set-url origin <nonexistent path>` on that
  checkout alone. Read the board with `run([]string{"board", "--root",
  observerRoot, "--json"}, ...)`, decoded into `report.BoardDoc` the
  way `emit` does. Assert `StaleSeconds` is at least the seeded span
  (a lower bound: real time moved a little further by read time),
  `Problems` names the repo with a `"could not fetch"` message, and
  `claim.RemoteTip` on "box-a"'s own checkout still equals
  `w.lease.Tip`.
- **S24.** `w.holdsTheLeaseInARealLane` then `w.renewsItsLease`
  (reused) land a real renewal; assert `w.err == nil` — a "plain win"
  Then step this row is first to need; check no existing step already
  covers the positive case before adding one. Clone the observer only
  after the renewal, straight off "box-a"'s repo (so it already
  carries the fresh tip with no fetch needed), then cut it off and
  read the board as S22 does. `BoardPlan` carries no tip field, so
  "held at the renewed tip" is proven by cloning after the renewal,
  not by a JSON field; assert `Held: true` for `ID: 24` plus S22's
  Problem check.
- **S23.** No git repository. Build `[]discovery.Plan{{Repo: "atlas",
  ID: 23, HoldTip: "tip-a", Held: true}, ...}` — synthetic tips
  suffice, since `discovery.Observe` only compares them for change.
  Seed each window via `observe.Save` directly (not `seedWindow`,
  which reads real `time.Now()`): `First`/`Last` off `w.pc().clock`,
  `Last` older than `DefaultSampleGap` by over a minute, `Samples: 9`.
  Call `observeHolds(&fleet.Result{Plans: plans}, &runtime{git:
  gitwt.Exec}, w.pc().clock)` directly, no `Coords`. Assert every
  plan's `Stale` is `false` and its window is a fresh, one-sample
  span.
- **S35.** `discovery.Observe`/`StaleHold` take whatever `now` a
  caller passes and never see wall-clock time at all, so a single
  large jump cannot demonstrate an "early" maturity: the gap since the
  last sample would itself exceed S_max and void the window on the
  spot (the same rule any partition trips). What actually varies with
  a fast clock is real time elapsed under one logical "T", not sample
  count — indistinguishable from ordinary maturation at this pure
  layer. This row's real, testable content is what happens once one
  is already live: `w.holdsTheLease`, then the existing
  `observerWatchesTipGoStale("box-a")` matures a window exactly as
  S20/S25 do — reused, not reinvented. `w.takesTheLeaseOver("box-b")`
  and `w.originHoldsTheTakeover` (both reused) are "CAS makes it
  safe": nothing here needs a new assertion, a takeover is already
  proven safe by every other row that does one. The backoff is the
  new part: `observerWatchesTipGoStale("box-b")` called a second time
  matures a fresh window on `w.taken.Tip` of the same shape (it always
  builds from a bare `Window{}`, so calling it again is calling it
  fresh, not extending the first). A new Then step reads
  `claim.TakeoverCount(repo, int64(w.planID), "origin/main",
  w.taken.Tip, gitwt.Exec)` — `k` should be `1`, the marker "box-b"'s
  own takeover minted — and computes `threshold :=
  time.Duration(k+1) * discovery.DefaultTakeoverWindow`, the same
  formula `observeHolds` uses. It asserts `StaleHold` true against the
  bare `DefaultTakeoverWindow` (the span really did mature — otherwise
  the row proves nothing) and false against the backed-off `threshold`
  — the pair, not either alone, is what pins the damping.

**Guard the edges.** Assert "box-a"'s own remote is untouched, not
just that its tip did not move — a shared-config bug should not pass
by accident. S23's plans need `Held: true`; re-check `observeHolds`
before assuming which field gates it, rather than trusting this
prose. S35's backoff Then must assert both halves of the pair above —
a check that only asserts "not stale under the backed-off threshold"
passes vacuously if the window never matured at all.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(22|23|24|35):'`
passes, all four PASS, none SKIP. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write `phase-2.result.md`. Record any finding this phase's own
research overturned from Phase 1's handoff. Say what S36 needs: two
independent `discovery.Window`/clock pairs on the same tip, converging
on the same `StaleHold` despite years of skew — a lease-API row like
S35, not a verb-level one.
