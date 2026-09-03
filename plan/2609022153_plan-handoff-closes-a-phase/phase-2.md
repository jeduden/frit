---
n: 2
title: Resume surfaces a single-file plan's Handoff heading
status: "🔳"
result: false
---
Task 2. `resume.go`'s `resumeFromLedger` builds every single-file
plan's bundle. It carries no handoff today. The doc comment on
`Resume` says so outright: a flat or inline-section plan "resumes
unchanged" — without one. `frit phase` on a single-file plan never
shows the previous phase's context, even once `plan-handoff` (phase 1)
writes a `## Handoff` heading into `plan.md`.

**Assumes.** `handoffOf` in
[internal/planmeta/resume.go](../../internal/planmeta/resume.go)
already finds a level-2 `## Handoff` heading. It walks a parsed
document's top-level children and returns the section's text — the
same function a directory plan's result file uses. `markdown.Parse`
strips `plan.md`'s own front-matter block before that walk starts. So
calling `handoffOf` on `planBody` inside `resumeFromLedger` sees the
same top-level headings `sectionText` already walks. No new parsing
seam.

**RED.** In
[internal/planmeta/resume_test.go](../../internal/planmeta/resume_test.go),
add a ledger-plan body with `dir` `""`, mirroring
`TestResumeWithNoDirectoryUsesTheLedger`. Its `plan.md` carries a
top-level `## Handoff` section between a `phases:`-derived done phase
and the still-open one. Assert `Resume`'s bundle carries that
section's text as `HandoffIn`. Confirm it fails first: today's
`resumeFromLedger` never reads the section, so `HandoffIn` comes back
empty.

**GREEN.** In `resumeFromLedger`, call `handoffOf(planBody)`. Set the
bundle's `HandoffIn` from its result. It stays empty when the plan
carries no such heading — that keeps
`TestResumeFallsBackToTheLedgerWhenNoPhaseFiles`'s existing
empty-`HandoffIn` case passing. Two doc comments still claim a ledger
plan carries no handoff: `Resume`'s own, and `printPhase`'s in
[cmd/frit/main.go](../../cmd/frit/main.go). The latter says a
ledger-resumed plan "carries no handoff, notes or result path of its
own" — narrow both to notes and result path, since printPhase already
renders `HandoffIn` whenever it is non-empty.

**Gate.** `go test ./internal/planmeta/...` covers the new case.
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
stay clean. `mdsmith check .` clean.
