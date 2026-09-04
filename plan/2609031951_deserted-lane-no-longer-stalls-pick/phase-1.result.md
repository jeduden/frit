---
n: 1
title: pick --go's walk advances past a deserted top lane
status: "✅"
result: true
summary: >-
  buildStart's startRefusal gate is now a skip in pick --go's walk,
  matching the sibling live-lane gate.
---

## Handoff

**The change.** In [`buildStart`](../../cmd/frit/start.go), the
`startRefusal` gate's return changed from

```go
if doc := startRefusal(...); doc != nil {
    return doc, false, nil
}
```

to

```go
if doc := startRefusal(...); doc != nil {
    return doc, !reattach, nil
}
```

`startRefusal` renders one refusal doc regardless of which internal
check fired it — `desertedRefusal`, `parkFirstRefusal` or
`claimRefusal` — so this one line covers all three uniformly: `pick
--go` (`reattach` false) now reads `lost` true and walks past any of
them; an explicit `start <id>` (`reattach` true) still reads `lost`
false and the caller sees the refusal, unchanged. The `buildStart` doc
comment now names the readiness gate alongside the live-lane gate in
the `lost` contract.

**No existing pin to reframe.** There was no prior unit test asserting
`pick --go` halting on a deserted or unmatured top candidate — the
counterpart 2609031211 found and reframed for the live-lane gate did
not exist here, so nothing needed reframing. Three tests were added
fresh to `cmd/frit/pick_test.go`, all reusing the `heldLaneOwnedBy` +
`startHerdr` fixtures:

- `TestPickGoAdvancesPastADesertedTopLaneToTheNextCandidate` — a
  deserted top lane (herdr confirms the bound session gone) with an
  unparked suffix (a local commit past the hold's persisted tip, so
  `parkFirstRefusal` fires) and a free second candidate: `pick --go`
  starts the second plan and never refuses on the first. Red before
  the change, green after.
- `TestPickGoAdvancesPastTheOnlyDesertedLaneToNothingStartable` — the
  same fixture with no second candidate: `pick --go` reports `nothing
  startable`, not a refusal (mirrors
  `TestPickGoAdvancesPastTheOnlyLiveLaneToNothingStartable`). Covers
  the plan's second Acceptance Criterion; already green once the walk
  change landed, since the classification is uniform across gates.
- `TestStartRefusesADesertedTopLaneWithAnUnparkedSuffix` — the same
  shape, but with the local token dropped (`dropToken`) so an explicit
  `start <id>` cannot resolve a reattach either: it still meets the
  plain `parkFirstRefusal` and renders "deserted hold: its branch
  carries an unparked suffix; run `frit yield 7` to park it first".
  Passed unchanged before and after — it pins that `reattach true`
  keeps `lost` false regardless of the new classification.

**Edges confirmed, no new test needed.**
`TestPickGoRefusesADivergingLocalBranch` (the fault path,
`return nil, false, err`) and `TestPickGoAdvancesPastALiveHold` plus
2609031211's live-lane walk test (the sibling gate) all still pass
unchanged — none of them reach the `startRefusal` branch this phase
touched. A same-host, same-session reattach over an unparked suffix
(`TestStartRefusesAReattachOverAnUnparkedSuffix`, pre-existing) also
stays untouched: that shape resolves `rs.Reattach true` and never
reaches `startRefusal` at all, so it says nothing about this change —
that is why `TestStartRefusesADesertedTopLaneWithAnUnparkedSuffix`
above deliberately drops the token, to force the fixture through the
gate that changed.

**Gate.** `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are both clean.

**What S90 needs from the cross-layer step file.** Already present and
reusable as-is:

- `pick --go runs`
  ([`pickGoRuns`](../../cmd/frit/bdd_identity_and_cross_layer_test.go))
- `plan (\d+) is the one started` (`planIsTheOneStarted`)
- `plan (\d+) is not refused on` (`planIsNotRefusedOn`)
- `plan (\d+) is ready and held by nobody` (S88's own second-candidate
  Given, `planIsReadyAndHeldByNobody`)

Still missing: a Given step that plants a deserted, park-first-refused
top candidate reachable from outside its own lane — S77's own Given
lines (`this machine holds plan 7 in a lane bound to a session, with
its token persisted` + `a takeover bound to a session at a new epoch
lands on plan 7` + `herdr shows no agent on the lane`) build exactly
that shape today, but S77's scenario only exercises them from inside
the lane via `start --go`. S90 can compose the same three Given
clauses with S88's `plan 8 is ready and held by nobody` and the
existing `pick --go runs` / `plan 8 is the one started` / `plan 7 is
not refused on` Then steps — no new step function should be needed,
only a new `@S90` scenario in
[features/cross-layer.feature](../../features/cross-layer.feature)
composing existing Given/When/Then lines, plus the matching row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md).
