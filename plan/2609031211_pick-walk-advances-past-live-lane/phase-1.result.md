---
n: 1
title: pick --go's walk advances past a live top lane, at the verb level
status: "✅"
result: true
summary: >-
  `buildStart`'s live-lane pre-flight (#126) now returns its refusal
  with `lost` equal to `!reattach`: a skip in `pick --go`'s walk, an
  unchanged surfaced refusal for explicit `start <id>`. A live top lane
  with a free next candidate no longer stalls the fleet.
---
## Handoff

**The one-line fix**, in `buildStart`
([cmd/frit/start.go](../../cmd/frit/start.go)):

```go
liveDoc, liveProbs, liveHerdrErr := startLiveLaneRefusal(
    c, rt, res, plan, phase, doGo, rs)
if liveDoc != nil {
    return liveDoc, !reattach, nil   // was: return liveDoc, false, nil
}
```

`reattach` is `buildStart`'s own flag — true only for `startResolved`
(explicit `start <id>`), false for `pick --go`'s walk — so the change
touches nothing on the explicit path; `startResolved` already discards
the bool. The doc comment above `buildStart` now names the live-lane
refusal alongside the lost race as the two things `lost` covers.

**The reframed pin**, in [cmd/frit/pick_test.go](../../cmd/frit/pick_test.go):

- `TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane` is renamed
  `TestPickGoAdvancesPastTheOnlyLiveLaneToNothingStartable` and now
  asserts `nothing startable` with no `"refused"` substring, the same
  answer `TestPickGoAdvancesPastALiveHold` gives for a lost race —
  still built on `liveLeaseFixture`, still confirms
  `worktree create` never ran and the lease tip is untouched.
- A new `TestPickGoAdvancesPastALiveTopLaneToTheNextCandidate` adds a
  second, freely-startable plan (id 8, `commitPlan` on the same repo,
  no push needed — discovery reads it off the local branch, same as
  `TestPickGoAdvancesPastALostRace`) underneath the live-held plan 7.
  It asserts plan 8 is started (`agent start plan-8`, `started plan
  8`) and plan 7 never is, with no `"refused"` in the output. Its herdr
  fake wraps `liveLeaseFixture`'s runner in a new test helper,
  `freshDispatchAfterLiveLaneQuery`: the fixture's runner only scripts
  the `agent list` answer the live-lane pre-flight reads on plan 7 and
  returns nothing for every other verb, so the helper layers in
  `startHerdr`'s `worktree create` / `pane current` answers for the
  real fresh dispatch plan 8 goes on to run, while still recording
  every call through the fixture's own `*herdrCalls`.
- The explicit-`start <id>` side of the split needed no new test:
  `TestStartGoRefusesWhenALiveAgentAlreadyHoldsTheLane` in
  [cmd/frit/start_test.go](../../cmd/frit/start_test.go) already drives
  `start 7 --go` against the same `liveLeaseFixture` and asserts the
  #126 refusal, unchanged by this phase.

Every guard-list test named in the phase spec —
`TestPickGoAdvancesPastALiveHold`, `TestPickGoDoesNotReattachAHeldLane`,
`TestStartGoRefusalOfALiveLaneCarriesInJSON`,
`TestLiveLaneRefusalNamesThePaneAndItsBranch` — passes unchanged.
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are both clean.

**What S88 (phase 2) will need from
`cmd/frit/bdd_identity_and_cross_layer_test.go`.** That file's
registrars carry no `pick --go` driver at all today, and
no "a second ready plan" step — every existing scenario in the section
drives `start` or `claim` on a single named plan. S88's Given/When/Then
needs three things, none of which exist there yet:

1. A live pane on a plan's own hold branch, matured, session-less —
   the exact shape `liveLeaseFixture`/`liveLeaseLane` build for the
   unit tests above. The closest existing BDD relative is
   `aHeldLaneHoldingPlanWhoseMarkerNamesAsHolder` +
   `herdrConfirmsTheSessionIsLive` in this same file, but that pair
   builds a *held* claim marker with a bound session, not the
   live-but-unbound lane #126 (and this plan's fix) is about; it is a
   pattern to imitate, not a step to reuse directly.
   `cmd/frit/bdd_host_death_and_races_test.go`'s `liveLaneHerdr` builds
   the live-but-unbound shape dynamically (off a real first `start
   --go`, in a different section, for S32) — its
   parsing-based `worktree create`/`pane current`/`agent list` wiring
   is reusable code, but S88's pane must be live *before* `pick --go`
   ever runs, seeded the way `liveLeaseFixture` seeds it, not raised by
   a first call.
2. A second, freely-startable ready plan in the same fleet — no
   existing step in this file builds a two-plan fixture; every
   scenario here is single-plan. Needs adding.
3. A driver step running `pick --go` and an assertion step naming
   which plan started — neither exists in this file; every driver here
   is `start`, `claim`, or `release`. Needs adding, following
   `thisHostStartsPlan`'s shape (run the CLI for real, capture
   `cs.out`/`cs.errb`, read the tip back off origin).

Phase 2 owns writing these three steps, the `@S88` scenario in
[features/cross-layer.feature](../../features/cross-layer.feature), and
the matrix row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md).

**Addendum, post-landing.** `/code-review --fix` found a real gap in
this phase's own change: a skipped candidate's live-lane refusal doc
can carry problems beyond the gather's own — an unread configured
host, an unreachable herdr — and `pc.start`'s `continue` on `lost`
dropped the whole doc, taking those problems with it. Neither the
eventual success doc nor `nothing startable` picked them back up, so
an operator running with `--hosts` configured could lose a real
warning silently. Fixed in `cmd/frit/main.go`: `pc.start` now carries
forward each skipped candidate's own problems, past the deterministic
prefix `carriedProblemCount` isolates, into whichever doc the walk
ends up rendering. `TestPickGoCarriesAProblemFromTheLiveLaneCheckOfA
SkippedCandidate` and `TestCarriedProblemCountMatchesWhatCarryProblems
Keeps` pin it, in `cmd/frit/pick_test.go`.
