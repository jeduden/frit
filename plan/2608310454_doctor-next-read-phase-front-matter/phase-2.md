---
n: 2
title: next and phase find a ledger-free folder plan's open phase from status
status: "✅"
---
Move the open-phase reading onto the phase files. `frit next` and
`frit phase` must find and report the open phase of a ledger-free folder
plan from each `phase-*.md`'s own `status`, not only the result file's
`## Handoff` marker — the reading `plan-new` now writes for.

Assumes: phase 1 landed. So `planmeta.PhasesFromDir` assembles a folder
plan's phases from `phase-*.md` front matter, each enriched with its
`## Execution` row. But
[internal/planmeta/resume.go](../../internal/planmeta/resume.go) still
decides a phase is done by the result file's `## Handoff` heading. And
[cmd/frit/main.go](../../cmd/frit/main.go)'s `laneOverride` and `phase`
fill `plan.Phases` from `planmeta.Parse` alone. That is empty for a
ledger-free folder plan, so `NextPhase` finds nothing.

Value: a ledger-free folder plan — the shape `plan-new` now writes —
reports its open phase from `frit next` and `frit phase`, so an executor
can resume it. Without this, next reports no phase and phase falls
through to the wrong one.

RED. Two failing tests. First, `Resume`: a folder plan whose
`phase-1.md` front matter says `status: ✅` but carries no
`phase-1.result.md`, and whose `phase-2.md` says `status: 🔲`, resumes at
phase 2 — at HEAD `Resume` sees no `## Handoff` for phase 1 and returns
it as open. Second, the shared `next`/`phase` phase-assembly helper: a
ledger-free folder plan's phases are read from its directory when the
plan.md ledger is empty, so `FirstOpenPhase` points at the first phase
whose `status` is not ✅ — empty at HEAD.

GREEN. In `Resume`, read each `phase-*.md`'s own `status` (reusing
phase 1's `parsePhaseFile`) as the done-signal: a phase at ✅ or ⛔ is
done, and a phase with no front-matter status falls back to the
`## Handoff` marker so a ledgerless phase file with a handoff still
resumes as before. Add a `cmd/frit` helper that fills a folder plan's
`plan.Phases` from `PhasesFromDir` when `planmeta.Parse` left it empty,
and wire it into `laneOverride` and `phase` so `frit next` and
`frit phase` report the open phase.

Do not change how a ledgered or `## Handoff`-driven phase file reads.
Keep every existing resume test green — the ledger and the `## Handoff`
done-signal must still work where they are what a plan uses.

Gate: the new resume and phase-assembly tests fail at HEAD and pass
after; every existing planmeta and cmd test stays green; `go test ./...`
passes and `go tool -modfile=tools/go.mod golangci-lint run` is clean.
