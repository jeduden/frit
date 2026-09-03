---
n: 2
title: The verb-level reap rows of landed evidence run for real
status: "✅"
result: true
summary: >-
  S79, S81 and S82 drop `@pending` and run for real in
  `cmd/frit/bdd_landed_evidence_test.go`. S79 reuses phase 1's own
  push, scavenge and both Then steps verbatim, adding only a Given
  that links a second worktree onto the plan's branch and a Then
  confirming the scavenge still parked a rescue — proving
  `claim.Scavenge`'s `checkedOut` guard through this section's own
  vocabulary rather than only at the lease-API level. S81 and S82 are
  CLI-level: a shared "a fleet-wide `reap --go` runs" When drives the
  real binary over a fleet root each Given builds, mirroring
  `bdd_host_death_and_races_test.go`'s own CLI pattern. S81 pins that
  a session-bound hold reads as confirmed gone with no matching herdr
  answer, and only a herdr answer naming that exact session alive
  turns the same hold into a live-lease refusal — proven both ways, not
  assumed. S82 reuses `strandedCheckout`, `landPlan` and `addOrigin`
  and checks the rescue ref carries the checkout's own tip, not merely
  that some rescue ref exists.
---
## Handoff

**S79's own proof, mirrored rather than reinvented.** The scenario
adds two steps only: `aWorktreeStandsOnBranch`, which links a second
worktree of the holder's own clone onto the plan's branch, and
`theWorkIsParkedToARescueRef`, the mirror of phase 1's own
`nothingIsParked` for a row whose tip is genuinely unlanded. Every
other step — the push, the scavenge, "origin's work ref for the plan
is gone", "the local work ref still resolves at its tip" — is phase
1's own, unchanged. Reusing them is what proves S79 exercises the same
"gone" and "still resolves" claims S54 already pinned, not a
third, silently drifted copy.

**A finding phase 2's own spec got wrong before the fixture was run,
corrected here rather than left standing.** The spec's first draft
assumed `holdRefusal`'s "held by a live lease" wording could not tell
a herdr-confirmed live session apart from a merely fresh, unmatured
claim, and planned to record that as a gap. Running S81's fixture
through an empty herdr fake instead of the alive one disproved it: the
same session-bound hold reads as confirmed gone and reap drops it,
never refuses it. A session bound at `Acquire` that no herdr answer
names is read as abandoned, the same evidence `deadHold`'s own
mismatched-session fixture already relies on elsewhere. Only a herdr
answer that actually names the bound session alive produces S81's own
refusal. `TestHerdrFakeConfirmsBoundSessionAliveIsLoadBearing` pins
both directions over the same Given, so this stays proven rather than
reverting to the same wrong assumption later. Phase 2's own plan text
was corrected in place to match; no code changed, only the spec's own
claim about wording.

**No other finding.** S82's Then reads the rescue ref's own content,
not merely its presence, so a park from the wrong tip would still fail
it; `TestTheCheckoutsCommitIsParkedToTheRescueRefWantsTheExactTip`
pins that a foreign object at the same rescue address is not read as
this row's own park. Every new step function carries its own guard
test, per CLAUDE.md; `TestLandedEvidenceStepsRefuseAMachineTheyNeverMet`
now covers the two S79 steps that name a machine.

**What S80 and S87 need.** Both run over `Gather`, not `reap`, so this
phase's CLI pattern is only half-reusable: the shared "a fleet-wide
`reap --go` runs" When has no counterpart, since S80 and S87 are read
verbs — `board` or `ready` — not a mutating one. What does carry over
is the shape of a CLI-level Given that records a fleet root in
`landedEvidenceState` and a Then that reads `out`/`errb` back, plus
`landedDeletedClone` in
[main_test.go](../../cmd/frit/main_test.go), already built and already
exercised once by `TestFetchFlagReachesTheReadWalk` for exactly S87's
own claim (`--fetch` reads the plan off the board; `--no-fetch` reads
it stale and held). S80's own mechanism,
`laggingDefaultBranch` in
[gather.go](../../internal/fleet/gather.go), is already unit-pinned by
`TestGatherReportsALocalDefaultBranchLaggingItsFetchedRemote` in
[gather_test.go](../../internal/fleet/gather_test.go); the CLI-level
row only needs a read verb run over that same shape and a Then reading
the reported problem's text off `BoardDoc.Problems` or the plain-table
`--json` output.

**What S59 needs.** `frit ready` in
[main.go](../../cmd/frit/main.go) is the read verb —
`discovery.Ready` in [ready.go](../../internal/discovery/ready.go)
is the readiness rule. `doneByRepo` there marks a plan done purely by
its own file's `status: "✅"`, with no landed-evidence check at all, so
a dependent plan naming a hand-flipped one in `depends-on` already
reads as ready today. `internal/doctor` still carries no early-✅
check — its checks remain goal, schema, execution-row, tier, id-sync
and phase-n-sync, confirmed unchanged by grepping its own function
list. S59's own scenario asserts exactly this observable and records
the gap, per plan.md's own Context.

All tests are green:
`go test ./cmd/frit -run 'TestFeatures/S(79|81|82):'` reports three
PASS, none SKIP; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are clean.
