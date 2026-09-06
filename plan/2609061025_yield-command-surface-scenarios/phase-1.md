---
n: 1
title: Yield on a plan nobody holds is a clean no-op
status: "🔲"
result: false
---
Prove `frit yield`'s clean no-op with a command scenario. A `frit
yield` on a plan nobody holds parks nothing and refuses nothing — the
honest no-op, distinct from a refusal. The scenario drives the real
command and fixes the yield command-surface step vocabulary later
phases reuse.

**BDD coverage.** This phase adds one `C<n>` scenario to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature).

**Assumes.** `yieldNothingLocal` in
[cmd/frit/yield.go](../../cmd/frit/yield.go) reports a yield with no
local work ref to park. When the plan is unheld, it is the clean
no-op: parks nothing, refuses nothing. `C1` in
[features/commands.feature](../../features/commands.feature) is the
`release` no-op, its steps in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go) —
the near-sibling this scenario mirrors for `yield`.

**Value.** The one yield outcome a developer meets most — nothing to
do, said honestly — gets its first end-to-end proof. Later yield
command scenarios copy this one's shape.

**RED.** Allocate the next free `C<n>` after merging `main`. Add its
row and a tagged scenario:

```gherkin
Scenario: yield on a plan nobody holds is a clean no-op
  Given a plan nobody holds and no local work ref for it
  When frit yield is run for that plan
  Then yield parks nothing and refuses nothing
```

Bind its steps in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go),
reusing the command world. The scenario fails until the steps exist,
not because yield is wrong.

**GREEN.** Write the step definitions that build the unheld-plan
fixture, run the built `frit yield`, and assert nothing was parked and
nothing refused. No change to
[cmd/frit/yield.go](../../cmd/frit/yield.go).

**Guard the edges.** Add the `C<n>` row and its tag together for the
bijection gate. Reuse an existing step's text rather than a second
definition, so godog's strict mode does not fail. Read the outcome
from the command's own output, not an internal call.

**Gate.** Against the built frit: the scenario runs and passes. `go
test ./internal/scenario` is green. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
