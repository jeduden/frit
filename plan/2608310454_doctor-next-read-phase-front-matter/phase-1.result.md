---
n: 1
title: Doctor validates a ledger-free folder plan from phase front matter
status: "✅"
---
## Handoff

Doctor now validates a ledger-free folder plan's Execution rows.
`planmeta.PhasesFromDir(dir, planBody)` assembles a folder plan's phases
from its `phase-*.md` `{n, title, status}` front matter, ordered like
`phaseSpecNumbers`, each enriched with its `## Execution` row via the
existing `attachExecutionRows`, so `HasExecutionRow`, `Tier` and `Gate`
read exactly as for a ledger phase. `parsePhaseFile` is the phase-file
counterpart of `Parse`. `doctor.scanFile` calls `PhasesFromDir` only
when `Parse` left `Plan.Phases` empty and the file is a folder plan's
`plan.md`, so the ledger wins where present.

Phase 2 inherits: `PhasesFromDir` is the seam to feed `Resume` /
`FirstOpenPhase` so `frit next`/`frit phase` find the open phase of a
ledger-free folder plan from phase-file `status`, not only the
`## Handoff` marker. `Resume` already holds `dir`; reuse `PhasesFromDir`
there rather than a second walk. Keep every existing resume test green —
the ledger and the `## Handoff` done-signal must still work where they
are what a plan uses.
