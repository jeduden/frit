---
n: 1
title: A merged plan drift reports as landed, named by its commit
status: "🔲"
result: false
---
Prove `frit drift`'s core signal with a command scenario. A plan whose
work merged into `main`, but whose status still says in progress,
drift reports as landed. It names the commit that carries the plan's
id. The scenario drives the real command and fixes the drift step
vocabulary later phases reuse.

**BDD coverage.** This phase adds one `C<n>` scenario to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature). It is
the worked example every later drift scenario copies.

**Assumes.** `driftCmd.Run` in
[cmd/frit/drift.go](../../cmd/frit/drift.go) walks each not-done plan
and reports whether its work landed and which commits name its id.
`landed` reads the ancestor-merge signal, or a squash-merge content
match. `commitsNaming` and `bucketByID` attribute a commit to a plan
by the id runs in its subject. `C1` in
[features/commands.feature](../../features/commands.feature) is the
one command scenario so far, its steps in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
The lease world's `holdsTheLease` vocabulary and plan fixtures are
shared; a command scenario threads the same world.

**Value.** The verb a developer trusts to tell landed work from a
stale status gets its first end-to-end proof. Every later drift
scenario — squash merge, marker-only, `--json` — copies this one's
shape rather than inventing a world.

**RED.** Allocate the next free `C<n>` against the catalog after
merging `main`. Add its row to
[command-scenarios.md](../../docs/research/command-scenarios.md) and a
tagged scenario to
[features/commands.feature](../../features/commands.feature):

```gherkin
Scenario: a plan whose work merged is reported as landed
  Given a plan in progress whose work has merged into main
  When frit drift is run
  Then drift reports the plan's work has landed
  And drift names the commit that carries the plan's id
```

Bind its steps in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go),
reusing the shared world. The scenario fails until the steps exist,
not because drift is wrong — drift already reports this correctly.

**GREEN.** Write the step definitions that build the fixture, run the
built `frit drift`, and assert the landed report and the named commit.
No change to [cmd/frit/drift.go](../../cmd/frit/drift.go): this phase
writes a scenario over code that already passes.

**Guard the edges.** The bijection gate fails a `C<n>` tag with no row
or a row with no tag, so add both together. godog runs strict: reuse
an existing step's text rather than a second definition of it. Keep
the drift assertions to what `drift` actually prints, read from the
command's own output, not an internal call.

**Gate.** Against the built frit: the new scenario runs rather than
skips, and passes. `go test ./internal/scenario` is green — the
`C<n>` maps to its row and back. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
