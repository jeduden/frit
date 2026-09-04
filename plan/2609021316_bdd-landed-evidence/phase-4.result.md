---
n: 4
title: S59 runs for real, and the doctor gap is recorded
status: "✅"
result: true
summary: >-
  S59 drops `@pending` and runs for real: a plan hand-flipped to ✅
  through `commitPlan` alone — no lease, no merge — already unblocks
  a dependent naming it, confirmed against `frit ready`'s own
  `discovery.Ready`. `internal/doctor` still carries no early-✅
  check, exactly as phases 2 and 3 already found; this phase is the
  first to turn that fact into an executable scenario rather than a
  claim. Every row the plan's Goal names — S54, S59, S79..S85, S87 —
  is now PASS under godog, none `@pending`, closing the plan.
---
## Handoff

**The row itself.** `aRepositoryWithPlanHandFlippedAndPlanDependingOnIt`
writes both plans with `commitPlan` alone, never `claim.Acquire` —
`TestARepositoryWithPlanHandFlippedAndPlanDependingOnItBuildsBothPlans`
pins that neither carries a lease branch, so the row cannot pass
because a lease made the dependent unready for an unrelated reason.
`planIsListedAsReady` checks the dependent's own id is present, not
merely that the ready list is non-empty, pinned by
`TestPlanIsListedAsReadyWantsTheSpecificID` against a ready list
naming an unrelated plan.

**No new finding.** Phases 2 and 3 already established that
`internal/doctor`'s checks are goal, schema, execution-row, tier,
id-sync and phase-n-sync, with nothing that reads a dependency's own
landing evidence. This phase adds no new fact about the gap; it turns
the existing one into a scenario that fails the build the day
`doneByRepo` or an early-✅ check changes this behavior.

**The plan's own Acceptance Criteria.** Every row
`features/landed-evidence.feature` declares —
S54, S59, S79, S80, S81, S82, S83, S84, S85, S87 — is PASS under `go
test ./cmd/frit -run TestFeatures/S`, none SKIP, none carrying
`@pending`. Every step is bound in `cmd/frit/bdd_landed_evidence_test.go`
or reused from `bdd_lease_test.go`; `bdd_test.go` and
`features/lifecycle.feature` are untouched, confirmed by this plan's
own diff across all four phases. Each scenario asserts an observable —
a verb's result, origin's refs, a rescue ref, a reported problem —
never a comment. The one finding this plan's own rows expose, S59's
own `internal/doctor` gap, is recorded here and in phases 2 and 3's
handoffs, not fixed. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S59:'`
reports one PASS, not SKIP; the whole section (`TestFeatures/S5`
through `TestFeatures/S8`, matching every landed-evidence row) carries
no FAIL and no SKIP among this plan's own ten rows; `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are clean.
