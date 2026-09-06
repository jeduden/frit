---
n: 1
title: The yield-honesty behavior gets scenario S93
status: "✅"
result: true
summary: S93 lands — matrix row, tagged scenario, dedicated steps — all gates green
---
## Handoff

S93 now catalogs the yield-honesty behavior plan 2609052054 landed
with unit tests alone: a distant host that never fetched a plan's ref
runs `frit yield` against another lane's live hold and is refused
honestly, naming the foreign hold, carrying "wait for the takeover
window" as its way out, touching neither origin's lease tip nor
writing a rescue ref.

**RED.** Added the S93 row to the Host-death section of
[lease-protocol.md](../../docs/research/lease-protocol.md) with no
`@S93` scenario tagged; `go test ./internal/scenario` failed on the
bijection gate, proving the gate is what forces the scenario to exist
(commit b011da4).

**GREEN.** Added the `@S93` scenario to
[host-death.feature](../../features/host-death.feature) and its own
steps in
[the host-death steps](../../cmd/frit/bdd_host_death_and_races_test.go):
`hasNeverFetchedLease` and `holdsLeaseUnseenBy` set up the topology —
this host's clone predates the foreign holder's clone, so the foreign
holder's later `claim.Acquire` mints and pushes the work ref only to
origin, never into this host's local git state; `thisHostYieldsPlan`
drives the real `frit yield` CLI; the four Then steps assert the
refusal, its way out, the untouched origin tip and the absent rescue
ref. Registration split into a new `registerYieldHonesty` to keep
`registerHostDeathAndRaces` under the linter's `funlen`.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/^S93:'`,
`go test ./internal/scenario`, `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith check .`
are all green. Every other S-scenario, including the S16/S20
fenced-yield pair, stayed green and untouched.

**What Phase 2 inherits.** S93 is the concrete worked example Phase
2's plan-authoring instruction points at: a matrix row plus a tagged
`@S<n>` scenario, added in the same shape as every other cross-host
lease behavior. Nothing here touched `plan/proto.md`, `CLAUDE.md` or
the `plan-new` skill — that is Phase 2's own work.
