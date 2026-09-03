---
n: 1
title: pick --go's walk advances past a deserted top lane
status: "🔲"
result: false
---
Make `pick --go` treat a `startRefusal` on a candidate as one to skip
rather than a stall, so the walk starts the next ready plan. This is
the whole behavior fix; Phase 2 only pins it in the matrix.

**Assumes.** [`(*pickCmd).start`](../../cmd/frit/main.go) walks
`discovery.Candidates(res.Plans)`. Per candidate it calls
[`buildStart(..., reattach=false)`](../../cmd/frit/start.go). It
continues to the next only when the returned `lost` bool is true.
[`startResolved`](../../cmd/frit/start.go) is the explicit `start
<id>` path. It calls `buildStart(..., reattach=true)` and discards
`lost`.
Inside `buildStart`, [`startRefusal`](../../cmd/frit/start.go) returns
a non-nil refusal doc for a deserted hold
([`desertedRefusal`](../../cmd/frit/start.go)), an unparked suffix
([`parkFirstRefusal`](../../cmd/frit/start.go)) or an unmatured
takeover (`claimRefusal`). Today `buildStart` returns that doc with
`lost` false, and the walk halts. Plan 2609031211 already made the
sibling gate, `startLiveLaneRefusal`, return `!reattach` for the same
reason. This phase does the same for the `startRefusal` gate.

**Value.** A fleet with ready work stops stalling behind a deserted
top pick. Before this phase, a deserted hold on the top-ranked plan
refuses every `pick --go` while lower plans sit startable; after it,
the walk passes over the deserted candidate and starts the next, and
the operator's explicit `start <id>` still learns the lane needs a
`frit yield` first.

**RED.** Add a unit test to
[cmd/frit/pick_test.go](../../cmd/frit/pick_test.go) — name it for the
behavior, e.g. `TestPickGoAdvancesPastADesertedTopLaneToTheNextCandidate`.
Build a fleet with two candidates: the top-ranked one a deserted hold
with an unparked suffix on its own lane — the shape
`desertedRefusal`/`parkFirstRefusal` fire on (held, herdr-confirmed
bound session gone, unmatured window, a local commit past the pushed
tip) — and a second, lower-ranked plan held by nobody and cleanly
startable. Run `pick --go`. Assert the second plan is the one started,
and the output does not refuse on the first. This fails today: the walk
halts on the deserted top candidate. Commit the red.

Search for an existing pin first. A test may already assert `pick
--go` halting on a deserted or unmatured top candidate — the
counterpart to `TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane`
that 2609031211 reframed. If so, reframe it the same way. The
single-candidate case now reports `nothing startable`, and the
surfaced refusal moves to an explicit `start <id>` assertion.

**GREEN.** In `buildStart`, return the `startRefusal` gate's refusal
doc with `lost` equal to `!reattach`: a skip in the pick --go walk
(`reattach` false), an unchanged surfaced refusal for explicit `start
<id>` (`reattach` true, where `lost` is discarded). This mirrors
2609031211's one-line change to the live-lane gate; adjust the
`buildStart` doc-comment so the `lost` contract names both refusal
gates.

**Guard the edges.** These must still pass unchanged:

- `TestPickGoRefusesADivergingLocalBranch` — a diverging local branch
  is a fault: `pick --go` still exits non-zero and stands nothing up.
  It travels the `return nil, false, err` path, not the refusal path,
  so it is untouched. Confirm it.
- `TestPickGoAdvancesPastALiveHold` and 2609031211's live-lane walk
  test — the sibling gate's behavior is unchanged.
- An explicit `start <id>` on the deserted hold still renders the
  "deserted hold … run `frit yield <id>`" refusal. Assert it, reusing
  the deserted-hold fixture.

Every changed or added function keeps its dedicated unit test.

**Gate.** The new advance test passes; the reframed explicit-start
refusal test passes; the diverging-branch fault still exits non-zero;
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are clean.

Write the handoff to `phase-1.result.md`. Record the exact `buildStart`
return that changed and the final shape of any reframed pin. Say what
step vocabulary S90 will need from the cross-layer step file — a
deserted top candidate, a free next plan, `pick --go` started the next
— and which of those steps already exist there versus need adding.
