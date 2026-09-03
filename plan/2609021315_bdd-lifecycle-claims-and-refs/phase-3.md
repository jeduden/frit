---
n: 3
title: Plan deleted while claimed runs for real
status: "✅"
result: false
---
Convert S52 (plan deleted while claimed) from `@pending` into a
passing scenario. S58 (released before the PR merges) is not this
phase's — it is doc-by-argument, per plan.md's own task split, and
phase 4's own.

**Assumes.** `claim.Scavenge` and `claim.ParkUnlanded` are already
proven, by phase 2 and by `TestScavengeParksUnlandedWorkThenDeletes`
in [lease_test.go](../../internal/claim/lease_test.go). Both park
unlanded work to a rescue ref before deleting. The evidence prompting
the call does not matter: the CAS never inspects the marker kind, only
that the observed tip still matches the remote's current one. What
S52 needs is not a new mechanism but a new Given: a lease still live
(never released), whose plan file is deleted from main while the
lease stands, with unlanded work committed onto the lease branch so
there is something for the park to rescue. As phase 1 and phase 2 both
confirmed, no code in this repository names "plan-gone" evidence
today. Nothing decides on its own that a deleted plan file should
trigger a scavenge, and nothing waits out the matrix's own "after a
window" half of S52's outcome. This phase drives `claim.Scavenge`
directly against the live tip, the same way phase 2 drove it directly
against a released tip. Its own handoff states plainly that the
evidence-detection wiring remains out of scope, exactly as plan.md's
Context already names it.

**Value.** The last SCAV row on the drivable side of this half: after
this phase, `claim.Scavenge`'s "park first" promise is checked against
every shape the matrix's outcome column claims for it — a released
tip (phase 2) and a live one whose backing plan is simply gone (this
phase). A regression that let a scavenge silently drop unlanded work
instead of parking it — on any tip, not only a released one — fails
the build.

**RED.** Drop `@pending` from S52 in
[lifecycle.feature](../../features/lifecycle.feature) and write its
Given/When/Then:

```gherkin
@S52
Scenario: plan deleted while claimed
  Given plan 7 is claimed and carries unlanded work
  When the plan file is deleted from main and pushed
  And the ref is scavenged by evidence
  Then origin carries no plan/7 ref
  And the rescue ref carries the unlanded work
```

Run `go test ./cmd/frit -run 'TestFeatures/S52_'`: strict mode reports
the new steps undefined and the subtest fails. That is the red —
commit it, with this phase file itself at `status: "🔳"`.

**GREEN.** Extend `cmd/frit/bdd_lifecycle_test.go`:

- A Given, `planIsClaimedAndCarriesUnlandedWork`, builds a claimable
  plan the way `planIsDoneAndItsLeaseIsReleased` does, but never
  releases: it acquires the lease with `claim.Acquire`, then checks
  out the plan's own work ref (`claim.Branch`), commits a file and
  pushes — the same shape `workOn` gives
  `TestScavengeParksUnlandedWorkThenDeletes`, written locally since
  `workOn` itself is unexported from `internal/claim`'s own test
  file. The pushed tip is recorded on `lifecycleState` for the
  scavenge step to CAS against.
- A When, `thePlanFileIsDeletedFromMainAndPushed`, removes the plan's
  own markdown file from main and pushes — the "plan-gone" fact the
  matrix names as S52's own evidence, reusing `planFileMatches` the
  way S50's rename step already does.
- A When, `theRefIsScavengedByEvidence`, drives `claim.Scavenge`
  directly against the recorded tip and records the `Scavenged.Rescue`
  ref it returns. Distinct step text from phase 2's
  `theReleasedRefIsScavengedByEvidence` — this tip was never
  released, only abandoned — but the same underlying call.
- The existing Then, `originCarriesNoPlanRef`, is reused unchanged.
- A new Then, `theRescueRefCarriesTheUnlandedWork`, checks the
  recorded rescue ref is non-empty and that `ls-remote origin
  <rescue>` contains the tip that was parked — the park half of the
  matrix's own "PARK first", checked directly rather than inferred
  from the delete alone.

Every new step function ships with a dedicated unit test of its own,
per CLAUDE.md. Each follows the file's own pattern: a refusal when an
earlier step in the chain was skipped.

**Guard the edges.** The rescue-ref Then must read `ls-remote` back
and check the parked tip is actually reachable from it, not merely
that `Scavenged.Rescue` came back non-empty — a scavenge that named a
rescue ref but pushed the wrong tip, or none, must still fail this
check. The delete Then (`originCarriesNoPlanRef`) already reads
origin directly, not the CLI's own wording, so it needs no change. A
scenario that only passes by weakening either assertion is a finding,
not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S52_'` passes, PASS,
not SKIP. `go test ./cmd/frit -run TestFeatures` (every section landed
so far) stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-3.result.md`. Say what S58 needs from
`Release` and a second `Acquire` — plan.md's own phase 4 — and whether
this phase's own fixture pattern (a live, unreleased lease with real
work on it) has anything left to say to it, or whether phase 2's
handoff on that question already stands.
