---
n: 2
title: "Reachable without a new fixture: S45, S49, S60, S73 run real"
status: "✅"
result: true
summary: >-
  S45, S49, S60 and S73 drop `@pending` and run as real scenarios,
  each a fresh combination of Phase 1's own fixtures: a live bound
  session vetoes a second agent's `start --go`, renewing the holder's
  own lease rather than losing it to a seizure; a holder string equal
  to this very host proves nothing once its token is gone, so
  `start` refuses the hold as unprovable exactly as it would a
  stranger's, never resuming it; an unreachable herdr at claim time
  releases the fresh lease its own worktree stand-up could not
  complete, and the next claim, herdr healthy again, mints clean at
  the following epoch with no takeover window waited; and an agent
  that starts before its own prompt call fails is still torn down by
  the failed handoff's unwind, a release marker landing on the
  branch it never got to work. `identityAndCrossLayerState` gains a
  `rec *herdrCalls` field so S45 and S73 can prove what herdr was
  actually asked, not just read the verb's own text; the one existing
  step touched is `holdsPlanBoundToASession`, additively, to also
  record the root S45 needs to run `start --go` against.
---
## Handoff

**No new fixture, as Phase 1 predicted.** All four rows were reachable
by recombining what already existed: `holdsPlanBoundToASession` and
`theWindowHasMaturedForPlan` (S45), `heldLaneOwnedBy` plus `dropToken`
(S49), and the single-repo claim shape S61 already proved safe for a
local `rev-parse` read-back (S60). Only S73 needed a genuinely new
herdr fake — `agent`+`prompt` failing instead of `agent`+`start` — and
even that is one branch changed in `TestStartUnwindTearsDownTheLaneOnA
FailedHandoff`'s own closure, not a new mechanism.

**The one existing step this phase touched.** `holdsPlanBoundToASession`
(Phase 1's own S61 Given) did not record the root it built the
repository under, because S61's own `machineClaimsPlan` step derives
root itself from the clone's path. S45 reuses Phase 1's
`thisMachineRunsStartGoForPlan`, which does need `st.root`, so the
Given step now also stores it. The change is additive — one more
field write, nothing removed or renamed — and S61's own scenario,
which never reads `st.root`, is unaffected: it still passes unchanged.

**`herdrCalls` reaches a step for the first time.** S45 and S73 both
read `rec.verb(...)` rather than the verb's own text output alone:
S45 does not need it (the veto is fully provable through git and the
run's own text), but S73 does — a failed prompt and a failed agent
start both exit non-zero with a release marker on the branch, and
only the recorded calls tell them apart. `theAgentStartsButItsPrompt
Fails` builds its own `*herdrCalls`, stored on the section's own
state, rather than reaching into `start_test.go`'s `startHerdr` and
patching one of its branches — that fake answers `agent`+`start` with
no recording of its own, so a bespoke closure was the smaller change.

**No finding.** Every assertion in all four rows passed on the first
shape tried; nothing was weakened to reach green. The `S60` row's
matrix summary calls the mechanism `RESUME`, but the code path is a
plain re-acquire at the next epoch, not a self-resume — no token
survives the failed stand-up for anything to resume from
(`unwindFailedStandUp`'s own docstring says as much: "no lane ever
persisted a token for it"). The scenario asserts the observable
behavior (`claimed plan 7`, `epoch:   2`) rather than the mechanism
label, so this is a documentation-terminology note, not a product
gap — worth a look if `docs/research/lease-protocol.md` is ever swept
for accuracy, but out of this plan's scope to touch.

**What the remaining rows will still need.** S46, S47, S63, S72, S76
and S77, then S62, S65, S66 and S74, are untouched by this phase:

- S46 (a reused path carrying no token) and S76 (a dead session
  resumes on the marker's lane and its token) both turn on the
  resume-from-outside path — `startResume`, `laneTokenResumeTip`,
  `dropToken` — which this phase still never called, exactly as
  Phase 1's handoff said.
- S47 (a failed handoff leaves a release marker and names the path)
  is the closest sibling to S73 this phase wrote: the difference is a
  teardown that itself fails (`TestStartUnwindNamesWhatTeardownLeft
  BehindWhenItFails`'s own shape, `worktree`+`remove` erroring) rather
  than a clean one, so the release marker's own presence is not new
  ground but naming what was left behind is.
- S77 (a deserted lane on its own host refusing an unpushed suffix)
  needs `yield`, which no row in this file has driven yet — still the
  first one to, as Phase 1's handoff also said.
- S63 and S72 both race two machines through the verbs rather than
  the lease API directly, and neither this phase's four rows nor
  Phase 1's needed that shape; `cloneRepoIntoRoot`
  (`bdd_process_death_test.go`) remains the nearest existing pattern,
  still borrowed by nothing in this file.
- S62, S65, S66 and S74 are the observation-and-boundary rows Phase 1
  scoped out entirely; none of this phase's fixtures reaches them
  either, since they turn on the staleness window resetting, a herdr
  restart's pane loss, an NFS-shared clone, and two repositories
  sharing one plan id — territory `resetWindow`
  (`bdd_process_death_test.go`'s own S7) is the nearest existing
  pattern for the first of the four.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S(45|48|
49|60|61|64|73|86):'` reports eight PASS, none SKIP; `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are clean;
`go test ./internal/scenario` (the bijection gate) stays green.
