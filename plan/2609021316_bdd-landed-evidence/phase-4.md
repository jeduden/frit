---
n: 4
title: S59 runs for real, and the doctor gap is recorded
status: "🔳"
result: false
---
Convert S59, the plan's last row, from `@pending` into a passing
scenario. It is the row the doc itself hands to `doctor`: a plan's
status flipped to ✅ by hand, with no real landing evidence behind it,
still reads as done — so a dependent naming it in `depends-on` reads
as ready too. This phase asserts that observable as it exists today
and records `internal/doctor`'s own missing check as a finding, not a
fix. Closing it closes the plan.

**Assumes.** `discovery.Ready` in
[ready.go](../../internal/discovery/ready.go) is the rule `frit
ready` in [main.go](../../cmd/frit/main.go) answers with.
`doneByRepo` there marks a plan done purely by its own file's
`status: "✅"` — no ancestry check, no landed-evidence read of any
kind. `allDone` then reads a dependent's `depends-on` list against
that map. Phase 2's and phase 3's own handoffs already confirmed
`internal/doctor`'s checks remain goal, schema, execution-row, tier,
id-sync and phase-n-sync; none of them is an early-✅ check.
`commitPlan(t, repo, id, status, title, deps, body)` in
[discovery_test.go](../../cmd/frit/discovery_test.go) already builds
a plan file carrying a `depends-on` edge, the shape this row's Given
needs directly.

**Value.** The gap the matrix names is now a standing, executable
observation rather than a claim in a plan document: a hand-flipped ✅
with nothing behind it already unblocks its dependents today, and
that stays true or the build fails, until `doctor` grows the check
that would catch it. Closing S59 is the plan's own Acceptance
Criteria met: no row in the section is still `@pending`.

**RED.** Drop `@pending` from S59 in
[landed-evidence.feature](../../features/landed-evidence.feature) and
write its Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S59:'`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenario, in the matrix's own terms. Given a repository with plan
59 hand-flipped to ✅ — `commitPlan` writes it done directly, no lease
ever acquired, no branch ever merged — and plan 60 naming 59 in its
own `depends-on`. When `frit ready` runs. Then plan 60 is listed as
ready. That is the exact shape S59's own title names, and the exact
observable `internal/doctor`'s missing check would need to catch
instead.

**GREEN.** Extend `cmd/frit/bdd_landed_evidence_test.go` with a
`ready report.ReadyDoc` field on `landedEvidenceState`, a Given
wrapping two `commitPlan` calls, a When driving `emit(..., "ready",
"--root", root)`, and a Then scanning `le.ready.Plans` for the
dependent's own id. Every step function ships with a unit test of its
own, per CLAUDE.md.

**Guard the edges.** The Then must check the dependent's id is
present, not merely that the ready list is non-empty — a scenario
that only proves "something is ready" would not pin this row's own
claim. The Given must genuinely leave plan 59 unheld and unmerged —
`commitPlan` alone, no `claim.Acquire` — so the row cannot pass merely
because a lease made 60 unready for an unrelated reason. A scenario
that only passes by weakening an assertion is a finding, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S59:'` passes,
reported PASS, not SKIP. `go test ./cmd/frit -run TestFeatures`
(every row in the section) is green, none `@pending`. `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-4.result.md`. Confirm every row named in
plan.md's Goal — S54, S59, S79..S85, S87 — is PASS under `go test
./cmd/frit -run TestFeatures/S`, none SKIP, and that this closes the
plan: tick the Acceptance Criteria, flip `plan.md`'s own `status` to
✅, and run `mdsmith fix PLAN.md`.
