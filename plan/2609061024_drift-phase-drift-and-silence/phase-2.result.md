---
n: 2
title: A plan mid-flight or already done raises no drift
status: "✅"
result: true
summary: C5 proves drift's restraint end to end — quiet for unmerged work, silent on a done plan.
---
## Handoff

`frit drift`'s restraint now has its own command scenario, alongside
its signal. `C5` — "a plan mid-flight or already done raises no
drift" — landed in
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature), steps
bound in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
One repository carries both shapes: a plan still in progress whose
only evidence is its own creation commit — no branch merged, no tip
content matching `main` — and a second plan already marked done. The
scenario drives the built `frit drift --json` and asserts, from that
output alone, that the mid-flight plan's row carries neither the
landed nor the last-phase flag, and that the done plan has no row at
all. No change to [cmd/frit/drift.go](../../cmd/frit/drift.go) — both
shapes were already proven at the unit level by
`TestDriftReportsLandedAndNamingCommits`'s "Plan 200" and
`TestDriftIgnoresADonePlan`; this phase gives them their first
end-to-end scenario.

`driftRowFor`'s json-decoding was factored into a shared `driftDocFor`
so the done-plan `Then` step — which needs the whole row set to prove
an id's *absence*, not one row's contents — reuses it rather than a
second `json.Unmarshal`. `commandState` grew a `doneID` field, kept
apart from the world's own `planID` since this is the first scenario
to need two plan ids in play at once.

RED landed first (catalog row and tagged scenario, steps undefined,
`TestFeatures/^C5:` failing on 4 undefined steps), then GREEN (the
step bindings and the `driftDocFor` extraction). Gate is green:
`TestFeatures/^C5:` passes over the built frit,
`go test ./internal/scenario` confirms the bijection, `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are clean.

**What the next phase inherits.** `driftDocFor` is available to any
scenario that needs to reason over the whole row set rather than one
plan's row. The plan's own Tasks section also names a commit naming
two plan ids being attributed to both — untouched by this phase, not
named in the plan's Acceptance Criteria, and left as a candidate for a
fresh plan rather than a phase 3 here. `C6` is the next free id. With
both Acceptance Criteria proven, this closes the plan.
