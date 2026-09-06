---
n: 2
title: A plan mid-flight or already done raises no drift
status: "🔲"
result: false
---
Prove `frit drift`'s restraint with a command scenario. A plan still
in progress whose work has not reached `main` raises no drift signal
of its own — no landed flag, no last-phase flag — and a plan already
marked done is not in drift's report at all. The scenario drives the
real command over both shapes in one repository.

**BDD coverage.** This phase adds one `C<n>` scenario to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature), reusing
the drift steps the sibling plan and this plan's own phase 1
established.

**Assumes.** `Unfinished()` in
[internal/planmeta](../../internal/planmeta) is the filter `drift`'s
`Run` in [cmd/frit/drift.go](../../cmd/frit/drift.go) checks before a
plan is walked at all — a done plan never reaches `driftRowFor` in the
first place. A plan whose only evidence is its own creation commit,
with no merge and no matching tip content, reads `Landed: false` and
`LastPhaseCommit: false` — the same shape
`TestDriftReportsLandedAndNamingCommits`'s "Plan 200" and
`TestDriftIgnoresADonePlan` already prove at the unit level.

**Value.** A developer trusts `frit drift` not to cry wolf only if its
silence is proven the same way its signal is — end to end, over the
real command, not inferred from the unit tests alone.

**RED.** Allocate the next free `C<n>` after merging `main`. Add its
row and a tagged scenario:

```gherkin
Scenario: a plan mid-flight or already done raises no drift
  Given a plan mid-flight whose work has not merged into main
  And a plan already marked done
  When frit drift is run
  Then drift raises nothing for the mid-flight plan
  And drift does not list the done plan
```

Reuse `frit drift is run`; bind only the four new steps. The scenario
fails until they exist, not because drift is wrong.

**GREEN.** Write the step definitions: the mid-flight `Given` writes a
plan file and commits it on its own, with no branch merged and no tip
content to match; the done `Given` adds a second plan file, status
`✅`, to the same repository, tracking its id apart from the world's
own `planID` so both `Then` steps read the right row. No change to
[cmd/frit/drift.go](../../cmd/frit/drift.go).

**Guard the edges.** Add the `C<n>` row and its tag together for the
bijection gate. Reuse `driftRowFor` and, where a `Then` step needs the
whole document rather than one row (the done plan must be absent, not
merely un-landed), factor its `json.Unmarshal` out into a small shared
helper rather than duplicating it.

**Gate.** Against the built frit: the scenario runs and passes. `go
test ./internal/scenario` is green. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.

Write the handoff to phase-2.result.md
