---
n: 1
title: A merged plan drift reports as landed, named by its commit
status: "✅"
result: true
summary: C2 proves drift's core signal end to end — landed, named by its commit.
---
## Handoff

`frit drift`'s core reconciliation now has its first command scenario.
`C2` — "a plan whose work merged is reported as landed" — landed in
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature), steps
bound in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
The fixture builds a plan still marked in progress whose hold branch
merges into `main` by an ordinary merge commit; the scenario drives
the built `frit drift --json` and asserts, from that output alone,
that the row reads landed and its commits carry the plan's own
creation commit ("plan 100"). No change to
[cmd/frit/drift.go](../../cmd/frit/drift.go) — this phase wrote a
scenario over code that already passed.

RED landed first (catalog row and tagged scenario, steps undefined,
`TestFeatures/^C2:` failing on 4 undefined steps), then GREEN (the
step bindings, reusing `commandState` and the shared world). Gate is
green: `TestFeatures/^C2:` passes over the built frit,
`go test ./internal/scenario` confirms the bijection, `go test ./...`
and `go tool -modfile=tools/go.mod golangci-lint run` are clean.

**What the next phase inherits.** The drift step vocabulary now lives
in `bdd_commands_test.go`: `aPlanInProgressWhoseWorkHasMergedIntoMain`,
`fritDriftIsRun`, and a `driftRowFor` helper that decodes drift's own
`--json` output into `report.DriftRow` for a `Then` step to assert on.
A later phase's squash-merge or marker-only scenario reuses `frit
drift is run` and `driftRowFor` rather than inventing new plumbing,
and only needs its own `Given` fixture and `Then` wording. `C3` is the
next free id.
