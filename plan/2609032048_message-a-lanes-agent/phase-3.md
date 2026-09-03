---
n: 3
title: S90 runs the ask-the-agent state under godog
status: "🔲"
result: false
---
Pin the whole route under godog, so the 2026-09-03 misread cannot
return. A held lane has a live working pane and unlanded work. frit's
output routes to messaging the agent, and the message reaches the pane.

**Assumes.** Phases 1 and 2 have landed: `frit message` exists and the
ambiguous-lane output names it. The cross-layer step file and the
herdr-fake vocabulary that plans 2609021314 and 2609031939 built are in
place, and S89 is the highest cross-layer id, so S90 is the next free
one. Phase 2's handoff names the exact state to reproduce.

**Value.** A unit test proves each site in isolation; the misread was a
composition — herdr's live-pane monitoring, git's unlanded reading, and
the refusal wording read together. A cross-layer row runs those layers
as one and holds the route in place against future edits to any single
site.

**RED.** Document row S90 in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md)
in the cross-layer matrix, describing the state: a held lane, bound
session gone or rotated, a live agent working on it, work pushed and
unlanded. Add a `@S90` scenario to
[features/cross-layer.feature](../../features/cross-layer.feature) — not
`@pending` — that stands the state up through the herdr fake and asserts
the observable end: frit's survey and refusal route the reader to `frit
message`, and a `frit message --go` reaches the pane. The bijection test
in [internal/scenario](../../internal/scenario) fails first for a
documented row with no live scenario; write the scenario to satisfy it.
Commit the red.

**GREEN.** Implement the scenario's steps against the existing
cross-layer step definitions, adding only what S90 needs and reusing the
herdr-fake and message-send vocabulary from Phases 1 and 2. Drive it to
green.

**Guard the edges.** S90 runs for real, not skipped. The
scenario-to-doc bijection stays exact — no orphaned row, no undocumented
scenario. Existing cross-layer rows are untouched.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S90:'` reports S90
PASS, not SKIP; `go test ./internal/scenario` bijection stays green; `go
test ./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean; `mdsmith check .` passes.

Write the handoff to `phase-3.result.md`. Confirm S90 is live and the
route it pins, and flip `plan.md`'s `status:` to `✅` with `mdsmith fix
PLAN.md`.
