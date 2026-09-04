---
n: 2
title: Resume surfaces a single-file plan's Handoff heading
status: "✅"
result: true
summary: >-
  resumeFromLedger now calls handoffOf(planBody) and carries a
  single-file plan's top-level `## Handoff` heading as the open
  phase's HandoffIn, the same field a directory plan's result file
  already filled — frit phase surfaces it identically either way.
---
## Handoff

The fix landed exactly where the phase spec named it:
`resumeFromLedger` in `internal/planmeta/resume.go` now calls the
existing `handoffOf(planBody)` and threads its result into the
bundle's `HandoffIn`. No new parsing seam — `handoffOf` already walked
a parsed document's top-level children for a level-2 `## Handoff`
heading, exactly the same top level `sectionText` and
`derivePhasesFromHeadings` read, and `plan.md`'s front matter is
already stripped from that AST before the walk starts.

RED was `TestResumeFromLedgerSurfacesThePlansOwnHandoffHeading` in
`internal/planmeta/resume_test.go`: a ledger plan body (`dir` `""`)
carrying a top-level `## Handoff` section between its done phase 1 and
open phase 2, asserting `Resume`'s bundle carries that text as
`HandoffIn`. It failed against the old `resumeFromLedger`, which never
read the section.

Two doc comments claimed a ledger plan carries no handoff at all —
`resumeFromLedger`'s own in `resume.go`, and `printPhase`'s in
`cmd/frit/main.go`. Both are narrowed: a ledger plan still carries no
notes or result path of its own, but its handoff now travels.

`TestResumeFallsBackToTheLedgerWhenNoPhaseFiles`'s existing
`assert.Empty(t, got.HandoffIn)` still passes unchanged: that test's
plan.md carries no `## Handoff` section, so `handoffOf` still reports
`ok == false` and the field stays empty — this phase never widened
what counts as a handoff, only where a ledger plan's is read from.

Verified: `go test ./internal/planmeta/...` (the new case plus every
existing Resume test), the full `go test ./...`, `go tool
-modfile=tools/go.mod golangci-lint run`, and `mdsmith check .` are
all clean.

**What's still open.** Tasks 3–4 have no phase files yet: a doctor
check for a phase recorded done whose handoff is missing in its
plan's shape (and fixing the existing drift in plan 2609021554's
phase-1 result, which closes with bold prose `handoffOf` does not
read), and pointing plan-phase's step 4 at `/plan-handoff`. The plan
stays 🔳.
