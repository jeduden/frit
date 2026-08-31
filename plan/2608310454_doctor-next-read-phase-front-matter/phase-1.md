---
n: 1
title: Doctor validates a ledger-free folder plan from phase front matter
status: "✅"
result: false
---
Prove the reading moves to the phase files on the smallest slice.
Assemble a ledger-free folder plan's phases from `phase-*.md` front
matter, and let `frit doctor` catch a phase with no `## Execution` row —
the check it silently skips today.

Assumes: plan 2608310418 has landed, so `phase-*.md` files carry
`{n, title, status}` front matter and `plan-new` writes no ledger for a
new plan. [internal/planmeta/plan.go](../../internal/planmeta/plan.go)
builds `Plan.Phases` from the ledger or `## Phase N` headings only.
[internal/planmeta/resume.go](../../internal/planmeta/resume.go) already
globs a directory's `phase-*.md` in `phaseSpecNumbers`.

Value: a ledger-free folder plan — the shape `plan-new` now writes —
regains doctor's Execution-row validation. Without this, doctor is blind
to exactly the plans the new default produces.

RED. Add a test fixture: a folder plan with no `phases:` ledger, whose
`phase-*.md` files carry front matter, and whose `## Execution` table is
missing a row for one phase. Assert `frit doctor` (via
[internal/doctor](../../internal/doctor)) reports the missing-Execution
finding for it. At HEAD the finding is absent — `Plan.Phases` is empty
for a ledger-free folder plan, so doctor walks nothing — so the test
fails.

GREEN. Add a dir-aware phase assembly in
[internal/planmeta](../../internal/planmeta) that parses each
`phase-*.md`'s front-matter `{n, title, status}` into `Phase` entries,
reusing the `phaseSpecNumbers` glob. Feed it into the plan model so
`Plan.Phases` is populated for a ledger-free folder plan, and wire
`doctor.Scan` to use it (it already holds each plan's path, hence the
directory). A plan that carries a ledger keeps reading from the ledger —
the ledger wins where present, front matter fills in only its absence.

Do not change how a ledgered plan reads. If the seam forces a discovery
rewrite rather than a dir-aware read beside the existing walk, stop and
report — the reuse note says the walk already exists.

Gate: the new doctor test fails at HEAD and passes after; a ledgered
plan's existing doctor and resume tests stay green; `go test ./...`
passes and `go tool -modfile=tools/go.mod golangci-lint run` is clean.
