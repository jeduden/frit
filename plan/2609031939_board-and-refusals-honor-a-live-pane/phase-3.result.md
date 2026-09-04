---
n: 3
title: S89 runs the attended-lane state end-to-end under godog
status: "✅"
result: true
summary: >-
  Cross-layer matrix row S89 documents "bound session gone, pane still
  attends"; its @S89 scenario in features/cross-layer.feature runs the
  state end-to-end — board and ready render the lane attended through
  a real `--json` read, and start's refusal names the pane and leads
  with resume — reusing Phase 1's and Phase 2's own fixtures and
  wording rather than inventing a third vocabulary.
---
## Handoff

Landed as scoped; the scenario passed on its first GREEN run.

**The row, as landed.** S89 sits in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md)'s
"Cross-layer: herdr and frit disagree" table, right after S88:
"bound session gone, pane still attends" — "board and ready render the
lane attended, not dead; start's deserted refusal names the pane,
leading with resume — `frit yield` only the fallback (plan 2609031939)
(RESUME, YIELD)". The first draft ran 45 words in the outcome cell and
tripped `mdsmith`'s 30-word table-cell rule; the trimmed wording above
is what actually landed, unchanged in substance.

**The scenario.** `@S89` in
[features/cross-layer.feature](../../features/cross-layer.feature)
reuses S77's own Given fixture verbatim — `this machine holds plan 7
in a lane bound to a session, with its token persisted` and `a
takeover bound to a session at a new epoch lands on plan 7` — and adds
one new Given, `herdr shows a live pane on the lane`, in place of
S77's `herdr shows no agent on the lane`. Three When/Then pairs follow
from that one setup, matching the phase spec's three observables:
`frit board --json reports plan 7` / `the board does not mark plan 7
dead, and shows the live pane`; `frit ready --json lists plan 7` /
`ready does not mark plan 7 dead either`; and the existing `the lane
runs start --go for plan 7` / a new `start refuses, naming the pane
and leading with resume`.

**Step vocabulary added**, all in
`cmd/frit/bdd_identity_and_cross_layer_test.go`, registered by a new
`registerAttendedLaneIdentityAndCrossLayer` (kept separate so no
registrar trips golangci-lint's funlen):

- `herdrShowsALivePaneOnTheLane` — the one new Given, faking a pane at
  the lane's own cwd under a pane id distinct from the bound session,
  so herdr's session check still confirms the bound session gone
  while its pane list shows this new one live.
- `fritBoardJSONReportsPlan` / `fritReadyJSONListsPlan` — run
  `board --json` / `ready --json` from the fleet root without
  resetting the herdr fake a Given step armed, decode into
  `report.BoardDoc` / `report.ReadyDoc`, and keep the plan's own row
  (`identityAndCrossLayerState.boardRow` / `.readyRow`, both new
  fields) for the following Then.
- `theBoardDoesNotMarkPlanDeadAndShowsTheLivePane` /
  `readyDoesNotMarkPlanDeadEither` — assert `Dead` is false on the
  kept row; the board check also asserts `Agent` is non-empty, so a
  row that simply lost its Dead flag without actually carrying the
  pane fact would still fail.
- `startRefusesNamingThePaneAndLeadingWithResume` — asserts the
  refusal contains `refused`, the pane id `wLive:p1`, and that
  `frit open` appears before `frit yield` in the string — the same
  ordering check `TestResumeRefusalNamesThePaneAndLeadsWithOpen`
  (phase 2) already pins on `resumeRefusal` directly, now proven
  through the whole verb.

Every new step function has its own unit test:
`TestAttendedLaneIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition`
(each step refuses on a missing precondition rather than reading a
zero value as real) and
`TestAttendedLaneIdentityAndCrossLayerReadBacksWantTheirExactShape`
(each Then reads the exact shape it needs, built by hand rather than
through a live run).

**No step collided.** All six step texts are new; none is redefined
from another section's file, so strict mode raised no ambiguity.

**Guard the edges, confirmed.** `@S89` is the scenario's only S tag.
`go test ./cmd/frit -run 'TestFeatures/S89:'` reports PASS, not SKIP.
`go test ./internal/scenario` (the bijection gate) stays green — the
row and the tag matched from the first commit that added both, in
Phase 3's own RED step.

**Acceptance Criteria.** All met: `board`/`ready`/`pick`/`find --json`
no longer call an attended lane dead (Phase 1); an explicit `start
<id>` refusal for an attended lane names the pane and leads with
resume (Phase 2); an unattended dead lane still reads and refuses
exactly as before (both phases' guard tests); `frit orphans` is
untouched; S89 is documented and runs for real under godog (this
phase). This was the plan's last phase — its front-matter `status`
moves 🔳 → ✅ in this same commit, alongside the acceptance-criteria
checkboxes.

Verified: `go test ./cmd/frit -run 'TestFeatures/S89:'` (PASS, no
SKIP), `go test ./internal/scenario`, the full `go test ./...`, `go
tool -modfile=tools/go.mod golangci-lint run`, and `mdsmith check .`
are all clean.
