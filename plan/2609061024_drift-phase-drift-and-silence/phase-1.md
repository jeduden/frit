---
n: 1
title: A plan whose final phase landed surfaces as drift
status: "✅"
result: false
---
Prove `frit drift`'s phase-level signal with a command scenario. A
multi-phase plan whose last phase's commit sits on `main`, while its
status still says in progress, drift reports as naming the final
phase. The scenario drives the real command.

**BDD coverage.** This phase adds one `C<n>` scenario to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature), reusing
the drift steps the sibling plan established.

**Assumes.** `lastPhaseNumber` in
[cmd/frit/drift.go](../../cmd/frit/drift.go) is a plan's
highest-numbered phase. `namesLastPhase` reports whether some commit's
subject names that phase. The sibling plan 2609061023 has already
bound the drift step vocabulary in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go)
and added the first `C<n>` row.

**Value.** The phase-close signal a developer reads before flipping a
multi-phase plan's status gets its first end-to-end proof. The silent
cases in later phases copy this one's shape.

**RED.** Allocate the next free `C<n>` after merging `main`. Add its
row and a tagged scenario:

```gherkin
Scenario: a plan whose final phase landed surfaces as drift
  Given a multi-phase plan in progress whose last phase's commit is on main
  When frit drift is run
  Then drift reports that a commit names the plan's final phase
```

Reuse the sibling's drift steps; bind only the new ones. The scenario
fails until its steps exist, not because drift is wrong.

**GREEN.** Write the step definitions that build the multi-phase plan
fixture, land a commit naming its final phase, run the built `frit
drift`, and assert the report. No change to
[cmd/frit/drift.go](../../cmd/frit/drift.go).

**Guard the edges.** Add the `C<n>` row and its tag together for the
bijection gate. Reuse an existing step's text rather than redefine it,
so godog's strict mode does not fail on an ambiguous step.

**Gate.** Against the built frit: the scenario runs and passes. `go
test ./internal/scenario` is green. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
