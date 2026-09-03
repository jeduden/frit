---
n: 2
title: S88 runs for real under godog
status: "✅"
result: true
summary: >-
  Matrix row S88, "a live top lane in pick's walk", is documented in
  the lease protocol's "Cross-layer" section and runs for real as a
  `@S88` godog scenario in `features/cross-layer.feature`, bound in
  `cmd/frit/bdd_identity_and_cross_layer_test.go`. The bijection gate
  and the whole suite stay green.
---
## Handoff

**The row, as landed** (`docs/research/lease-protocol.md`, "Cross-layer:
herdr and frit disagree"):

> S88 · a live top lane in pick's walk · `pick --go` treats the
> live-lane pre-flight refusal (#126) as a candidate to skip: it
> advances to the next ready plan; an explicit `start <id>` meets the
> refusal (plan 2609031211)

**The scenario, as landed** (`features/cross-layer.feature`):

```gherkin
@S88
Scenario: a live top lane in pick's walk
  Given plan 7's hold branch already carries a live herdr pane
  And plan 8 is ready and held by nobody
  When pick --go runs
  Then plan 8 is the one started
  And plan 7 is not refused on
```

**Step vocabulary added** to
`cmd/frit/bdd_identity_and_cross_layer_test.go`, in a new
`registerPickWalkIdentityAndCrossLayer` registrar (appended to `init`
alongside the section's four others):

- `plansHoldBranchAlreadyCarriesALiveHerdrPane` — the Given. Reuses
  `liveLeaseFixture` outright (the same fixture
  `cmd/frit/pick_test.go`'s unit-level pin builds), layering
  `freshDispatchAfterLiveLaneQuery`'s `worktree create` / `pane
  current` answers on top so the walk's real fresh dispatch onto plan
  8 can still run. Like every other fixture in this file built on
  `heldLaneOwnedBy`/`liveLeaseFixture`, it only ever mints plan 7's
  lease and refuses any other id.
- `planIsReadyAndHeldByNobody` — the second Given, new: `commitPlan`
  on the same repo the live-lane step built, nothing more. No existing
  step in this section built a second plan; every prior scenario here
  is single-plan.
- `pickGoRuns` — the When, new: this section's first `pick --go`
  driver. Follows `thisHostStartsPlan`'s shape (`bdd_host_death_and_
  races_test.go`) — run the CLI for real, capture `out`/`errb`/`code`
  onto `identityAndCrossLayerState`.
- `planIsTheOneStarted` — a Then, new: asserts `"started plan <id>"`
  in the captured output, so a walk that started the wrong plan, or
  nothing, fails it. This is the identity assertion the phase spec
  called for, not merely "a start happened".
- `planIsNotRefusedOn` — a Then, new: asserts no `"refused"` substring
  anywhere in the output, the same "no refusal at all" shape the
  unit-level pin (`TestPickGoAdvancesPastALiveTopLaneToTheNextCandidate`)
  asserts.

No existing step text in this file or `bdd_lease_test.go` was reused
or redefined; strict mode reported no ambiguity. `identityAndCrossLayerState`
gained no new field — `root`, `repo`, `held`, `rec`, `out`, `errOut`
and `code` (already there) cover everything the five steps need.

Two new unit tests, following this file's own
`TestPhaseN...RefuseTheirMissingPrecondition` /
`...ReadBacksWantTheirExactShape` convention, cover the five step
functions: `TestPickWalkIdentityAndCrossLayerStepsRefuseTheirMissing
Precondition` and `TestPickWalkIdentityAndCrossLayerReadBacksWantTheir
ExactShape`.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S88:'` reports S88
PASS, not SKIP. `go test ./internal/scenario` (the bijection gate)
stays green. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean. `mdsmith check .` passes.

This closes the plan: both phases landed, all six acceptance criteria
met.
