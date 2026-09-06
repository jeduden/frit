---
n: 2
title: Yielding one's own live lane is refused toward release
status: "✅"
result: false
---
Prove `frit yield`'s live-lane refusal with a command scenario.
Yielding the plan this very lane holds — freshly claimed, nothing
fenced — is refused, and the refusal points at `release` rather than
silently discarding the lease. This is the C3 no-op's sibling: same
command, the other single-host outcome `StillHeldError` computes.

**BDD coverage.** This phase adds one `C<n>` scenario to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[features/commands.feature](../../features/commands.feature).

**Assumes.** `claim.Yield` in
[internal/claim/lease.go](../../internal/claim/lease.go) compares the
remote lease branch's tip against this lane's own local tip of the
same branch; when they match, nothing is fenced — this lane is the
live holder — and it returns `StillHeldError` rather than parking.
`yieldError` in [cmd/frit/yield.go](../../cmd/frit/yield.go) turns
that into a refusal naming `release` as the way out. The unit test
`TestYieldRefusesTheCurrentHolder` in
[cmd/frit/yield_test.go](../../cmd/frit/yield_test.go) already proves
this at the Go level: claim the plan, then yield it in the same repo,
same lane, no divergence in between.

**Not `S93`.** `S93` is a distant host refusing a hold it never
fetched — two hosts, a lease it cannot read. This is one host,
holding its own freshly-minted lease, offered back to itself. No
lease race, so its home is `command-scenarios.md`, not
`lease-protocol.md`.

**Value.** The refusal a developer meets most after the C3 no-op —
"you still hold this, use `release`" — gets its first end-to-end
proof, and the wording that keeps `yield` from ever discarding a live
lease is pinned against the real command, not just the unit test.

**RED.** Merge `main`, allocate the next free `C<n>` (`C6` as of this
writing — confirm against the landed catalog), add its row, and a
tagged scenario:

```gherkin
Scenario: yielding one's own live lane is refused toward release
  Given a plan freshly claimed by this lane
  When it is yielded
  Then yield refuses it, naming release as the way out
```

Bind its steps in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go),
reusing the command world and phase 1's `it is yielded` step
unchanged. The scenario fails until the new steps exist, not because
yield is wrong.

**GREEN.** Write the step definitions. The Given runs the real `frit
claim` CLI against a `claimableRepo` fixture, mirroring
`TestClaimMintsAPickablePlan`'s herdr fake, which also satisfies
claim's worktree-create call. It leaves the plan's lease branch at
origin's own tip — no divergence, the live-holder shape `claim.Yield`
reads as still held. The Then reads the refusal off the command's own
stdout: it contains `refused` and names `release`. It also confirms
nothing was parked (no rescue ref on origin), the same restraint
`TestYieldRefusesTheCurrentHolder` pins at the Go level.

**Guard the edges.** Add the `C<n>` row and its tag together for the
bijection gate. Reuse `it is yielded` rather than a second step
definition, so godog's strict mode does not fail. Read the outcome
from the command's own output, not an internal call. No change to
[cmd/frit/yield.go](../../cmd/frit/yield.go) or
[internal/claim/lease.go](../../internal/claim/lease.go).

**Gate.** Against the built frit: the scenario runs and passes. `go
test ./internal/scenario` is green. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
