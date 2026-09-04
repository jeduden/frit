---
n: 3
title: A doctor check catches a skipped handoff
status: "🔳"
result: false
---
Task 3. A phase can still close with its status flipped to done and no
readable handoff behind it — `plan-phase` step 4 today only reminds an
executor, and nothing checks the result afterward. Add a `frit doctor`
check that reports a phase recorded done whose handoff is missing in
its plan's shape: no readable `## Handoff` in a single-file plan's
`plan.md`, none in a directory plan's own `phase-N.result.md`. Fix the
one real instance this already caught: the phase-1 result under
[plan 2609021554](../2609021554_gather-reports-progress-and-status/plan.md)
closes with bold prose, `**Handoff.**`, that `handoffOf` does not read
as the heading it names.

**Assumes.** `handoffOf` in
[internal/planmeta/resume.go](../../internal/planmeta/resume.go) is
unexported. doctor cannot call it as-is. Export a thin wrapper, the
way `SpecFileName`/`ResultFileName` already wrap `specFileName`/
`resultFileName` for `checkPhaseNumberSync`. Name it `HandoffOf(source
[]byte) (text string, ok bool)`. That way doctor never re-derives the
`## Handoff` walk.

A plan already `✅` or `⛔` needs no lingering handoff, since nothing
resumes into a finished plan. A scripted survey of every plan on disk
today found 34 fully-`✅` plans with no top-level `## Handoff` at all,
and zero readable-drift inside any of them. Every one of those 34 is a
plan whose own `status:` is already `✅`. Scoping the check to plans
still in progress (not `✅`/`⛔`) keeps all 34 clean. It still catches
the one live gap: plan 2609021554, `status: "🔳"`, phase 1 done, no
readable handoff.

**RED.** In
[internal/doctor/doctor_test.go](../../internal/doctor/doctor_test.go),
add `TestScanFlagsADonePhaseWithNoReadableHandoff`. It builds a folder
plan, `status: "🔳"`, with one `phase-1.md` at `status: "✅"` and a
`phase-1.result.md` that closes with bold prose instead of a `##
Handoff` heading — mirroring the real plan 2609021554 drift. Assert
`Scan` reports one finding, `Check == "handoff"`, naming phase 1 and
`phase-1.result.md`. Add a second case alongside it,
`TestScanDoesNotFlagADonePlanWithNoHandoff`: the same shape, but the
plan's own `status:` is `✅`. Assert `Scan` reports nothing, proving the
34-plan survey's exemption holds. Confirm both fail first. No `handoff`
check exists yet, so the first case reports nothing where it should
report one, and the second already passes vacuously.

**GREEN.** Four changes:

- `internal/planmeta/resume.go`: add `HandoffOf`, exported, wrapping
  `handoffOf` verbatim — same doc comment, delegated body.
- `internal/doctor/doctor.go`: add `checkHandoff(p planmeta.Plan, rel,
  dir string, isFolder bool) ([]Finding, error)`. Returns early when
  `p.Status` is `StatusDone` or `StatusSuperseded`. For a folder plan,
  walk `p.Phases` (already populated by `scanFile`'s existing
  `PhasesFromDir` fallback); for each `StatusDone` phase, read
  `filepath.Join(dir, planmeta.ResultFileName(string(ph.N)))` — a
  missing file counts as no handoff, any other read error is
  propagated — and add a `"handoff"` `Finding` when
  `planmeta.HandoffOf` reports `ok == false`, pointing at that
  `phase-N.result.md`. For a single-file plan, `## Handoff` is one
  shared heading, not one per phase: if any phase is `StatusDone` and
  `planmeta.HandoffOf(planBody)` reports `ok == false`, add one
  `"handoff"` finding pointing at the plan file itself, naming the
  latest done phase. Wire the call into `scanFile` alongside the
  existing `checkPhaseNumberSync` call, gated the same way on
  `plans.IsFolderPlanFile(rel)` for the directory branch and running
  unconditionally for the single-file case.
- Add `"handoff"` to the `Finding.Check` vocabulary doc comment and to
  `doctorCmd.Help()` in
  [cmd/frit/main.go](../../cmd/frit/main.go), alongside `phase-n-sync`.
- Fix the drift in
  [plan 2609021554's phase-1.result.md](
  ../2609021554_gather-reports-progress-and-status/phase-1.result.md).
  Replace the closing `**Handoff.**` paragraph with a `## Handoff`
  heading carrying the same prose, so `handoffOf` reads it and a real
  `frit doctor` run over this repository comes back clean on the new
  check.

**Gate.** `go test ./internal/planmeta/... ./internal/doctor/...`
covers both. Full `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` stay clean. `mdsmith
check .` clean. A built `go run ./cmd/frit doctor` over this
repository reports no `handoff` finding.
