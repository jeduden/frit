---
n: 3
title: A doctor check catches a skipped handoff
status: "✅"
result: true
summary: >-
  frit doctor now reports a "handoff" finding for a phase recorded
  done in a plan still in progress whose handoff has no readable
  trace: no "## Handoff" in a single-file plan, none in a directory
  plan's own phase-N.result.md. A plan already done or superseded is
  exempt, since nothing resumes into it — a 34-plan survey of this
  repository confirmed every already-`✅` plan stays clean under that
  rule. The one live gap, plan 2609021554's phase-1 result closing
  with bold prose, is fixed.
---
## Handoff

Landed as scoped, in two pieces.

`internal/planmeta/resume.go` gained `HandoffOf`, exported, a thin
wrapper over the existing unexported `handoffOf` — no new parsing, the
same walk `Resume` already relies on.

`internal/doctor/doctor.go` gained `checkHandoff` and its folder-plan
half `checkFolderHandoff`, wired into `scanFile` alongside the existing
`checkPhaseNumberSync` call. A plan already `StatusDone` or
`StatusSuperseded` returns early — a 34-plan survey of every plan.md on
disk found 34 fully-`✅` plans carrying no `## Handoff` at all and zero
would have been falsely flagged. For a folder plan, each `StatusDone`
phase's own `phase-N.result.md` is read and checked with `HandoffOf`; a
missing file counts as no handoff, one finding per gap, pointing at
that file. For a single-file plan, `## Handoff` is one shared heading
overwritten on every close, so one finding covers the plan once any
done phase exists and `HandoffOf(planBody)` finds nothing, naming the
latest done phase. `"handoff"` joined the `Check` vocabulary doc
comment and `doctorCmd.Help()` in `cmd/frit/main.go`.

RED was `TestScanFlagsADonePhaseWithNoReadableHandoff` in
`internal/doctor/doctor_test.go`: a folder plan still `"🔳"` with one
done phase closing in bold prose. It failed against the old `Scan`,
which reported nothing. `TestScanDoesNotFlagADonePlanWithNoHandoff`
proved the same drift is exempt once the plan itself is `"✅"`, passing
vacuously before the check existed and unchanged after.

The drift named in the spec is fixed: plan 2609021554's phase-1 result
now closes with a `## Handoff` heading carrying the same prose, rather
than a bold `**Handoff.**` lead `handoffOf` never read.

**One nuance the spec did not anticipate.** `frit doctor`'s own repo
discovery scans the *main* worktree, not the branch a plan lane is
running on (`discover.Repos` picks `worktrees[0]`, the checkout at the
repository's git-common-dir). So a built `go run ./cmd/frit doctor`
run from this lane still reports the plan 2609021554 gap until this
plan's commits land on `main` — verified instead by calling
`doctor.Scan` directly against this worktree's own root, which comes
back with no `handoff` finding.

Verified: `go test ./internal/planmeta/... ./internal/doctor/...`, the
full `go test ./...`, `go tool -modfile=tools/go.mod golangci-lint
run`, and `mdsmith check .` are all clean.

**What's still open.** Task 4 has no phase file yet: point
`plan-phase`'s step 4 at `/plan-handoff` for the close, so the two
skills cannot drift. The plan stays 🔳.
