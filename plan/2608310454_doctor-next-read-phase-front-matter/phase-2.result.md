---
n: 2
title: next and phase find a ledger-free folder plan's open phase from status
status: "✅"
---
## Handoff

`frit next` and `frit phase` now find a ledger-free folder plan's open
phase from its `phase-*.md` status. `Resume` reads each phase file's own
`status` as the done-signal (a phase at ✅ or ⛔ is done), reusing phase
1's `parsePhaseFile` through a `phaseFileStatus` helper; a phase file
with no front-matter status falls back to the result file's `## Handoff`
marker, so a plan written before the convention resumes as before. The
open phase's bundle keeps only the phase file's prose as its spec — the
front matter is the ledger, not the brief. A shared `folderPlanPhases`
helper in `cmd/frit` fills a folder plan's `plan.Phases` from
`PhasesFromDir` when `planmeta.Parse` left it empty, wired into
`laneOverride` (next, show) and the `phase` command, so `NextPhase`
points at the first phase whose status is not ✅.

No regression: ledgered and `## Handoff`-driven phase files read
unchanged. The ledger wins where present — `folderPlanPhases` fills in
only when `Parse` left `Plan.Phases` empty and the plan is a folder
plan's `plan.md`; a flat plan keeps its own reading. Every prior resume,
phase and next test stays green.

Plan complete: all acceptance criteria met. This is the last phase, so
plan.md `status` flips to ✅.
