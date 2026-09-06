---
n: 1
title: A plan whose final phase landed surfaces as drift
status: "✅"
result: true
summary: C4 proves drift's phase-level signal end to end — a commit naming the plan's final phase.
---
## Handoff

`frit drift`'s phase-level signal now has its first command scenario.
`C4` — "a plan whose final phase landed surfaces as drift" — landed in
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature), steps
bound in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
The fixture builds a plan still marked in progress with a two-phase
ledger, whose last-phase commit already sits on `main`; the scenario
drives the built `frit drift --json` and asserts, from that output
alone, that the row's `LastPhaseCommit` flag reads true. No change to
[cmd/frit/drift.go](../../cmd/frit/drift.go) — this phase wrote a
scenario over code that already passed.

The fixture is `lastPhasePlanRepo` in
[cmd/frit/drift_test.go](../../cmd/frit/drift_test.go), extracted so
`TestDriftFlagsALastPhaseCommit`'s own positive case and C4's setup
step share one git sequence rather than drifting apart on what "the
last phase landed" means — the dedup the sibling plan's own close
called out as a follow-up worth doing up front here.

RED landed first (catalog row and tagged scenario, steps undefined,
`TestFeatures/^C4:` failing on 2 undefined steps), then GREEN (the
step bindings and the shared fixture). Gate is green:
`TestFeatures/^C4:` passes over the built frit,
`go test ./internal/scenario` confirms the bijection, `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are clean.

**What the next phase inherits.** `driftRowFor` and `frit drift is
run` are reused unchanged. A later phase's silent-case scenario — a
plan mid-flight whose work has not merged, or a plan already done —
needs only its own `Given` fixture and `Then` wording, plus a `phase-2.md`
this plan does not yet carry; the Execution table and Acceptance
Criteria below still name that work as open. `C5` is the next free id.
