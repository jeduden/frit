---
n: 1
title: pick --go's walk advances past a live top lane, at the verb level
status: "🔲"
result: false
---
Make `pick --go` treat a live-lane pre-flight refusal as a candidate
to skip rather than a stall, so the walk starts the next ready plan.
This is the whole behavior fix; Phase 2 only pins it in the matrix.

**Assumes.** [`(*pickCmd).start`](../../cmd/frit/main.go) walks
`discovery.Candidates(res.Plans)`. Per candidate it calls
[`buildStart(..., reattach=false)`](../../cmd/frit/start.go). It
continues to the next only when the returned `lost` bool is true.
[`startResolved`](../../cmd/frit/start.go) is the explicit `start
<id>` path. It calls `buildStart(..., reattach=true)` and discards
`lost`. Inside `buildStart`,
[`startLiveLaneRefusal`](../../cmd/frit/start.go) returns a non-nil
refusal doc when [`liveLaneFor`](../../cmd/frit/dispatch.go) finds a
live pane on one of the plan's hold branches (#126). Today
`buildStart` returns that doc with `lost` false.
`TestPickGoAdvancesPastALiveHold` shows a *bound* live hold is a lost
race through `mintOrTakeOver`'s veto, and the walk advances. The
pre-flight is the one live-lane path that does not.

**Value.** A fleet with ready work stops stalling behind a lane
somebody is already running. Before this phase, a live top pick freezes
`pick --go` on a refusal; after it, the walk starts the next ready plan,
and the operator's explicit `start <id>` still learns the lane is busy.

**RED.** Add a unit test to
[cmd/frit/pick_test.go](../../cmd/frit/pick_test.go) — name it for the
behavior, e.g. `TestPickGoAdvancesPastALiveTopLaneToTheNextCandidate`.
Build a fleet with two startable plans: the top-ranked one's hold
branch carrying a live herdr pane (the `liveLeaseFixture` +
`withHerdr` shape `TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane`
uses), and a second, lower-ranked plan held by nobody and cleanly
startable (a plain ready plan the herdr fake shows no lane for). Run
`pick --go`. Assert the second plan is the one started — its branch or
id in the output, `started`/`prompt_dispatched` true for it — and that
the output does not refuse on the first. This fails today: the walk
halts on the live top lane and refuses. Commit the red.

**GREEN.** In `buildStart`, return the live-lane pre-flight refusal
with `lost` equal to `!reattach`: a skip in the pick --go walk
(`reattach` false), an unchanged surfaced refusal for explicit `start
<id>` (`reattach` true, where `lost` is discarded). This is the single
behavior line; adjust the `buildStart` doc-comment so the `lost`
contract now names the live-lane refusal alongside the lost race.

**Reframe the pin.** One test pins the old stall:
`TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane`. Its fixture
had one candidate. Split its intent in two:

- The pick --go walk over a fleet whose *only* candidate is a live
  lane now reports `nothing startable`, the same answer
  `TestPickGoAdvancesPastALiveHold` gives — update this test (or add a
  sibling) to assert that, not a refusal.
- The #126 refusal that *must* stay surfaced belongs to explicit
  `start <id>`: assert an explicit `start <id>` on the live lane still
  renders the refusal naming the pane and its branch, and takes over
  nothing. Reuse the same `liveLeaseFixture`.

**Guard the edges.** Every changed or added function keeps its
dedicated unit test (CLAUDE.md). These must still pass unchanged, so
the fix does not disturb the bound-hold advance, the no-reattach
promise, or the explicit-start refusal wording:

- `TestPickGoAdvancesPastALiveHold`
- `TestPickGoDoesNotReattachAHeldLane`
- `TestStartGoRefusalOfALiveLaneCarriesInJSON`
- `TestLiveLaneRefusalNamesThePaneAndItsBranch`

**Gate.** The new advance test passes; the reframed explicit-start
refusal test passes; `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Record the exact `buildStart`
return that changed and the final shape of the reframed pin. Say what
step vocabulary S88 will need from
`bdd_identity_and_cross_layer_test.go` — a live pane on a plan's lane,
a second ready plan, `pick --go` started the second — and which of
those steps already exist there versus need adding.
