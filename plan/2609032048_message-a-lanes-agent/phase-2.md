---
n: 2
title: the ambiguous-lane output routes to message
status: "✅"
result: false
---
When git cannot classify a held lane's work and a live pane attends it,
the output a reader consults points at `frit message`. So "ask the
agent" stands where "yield and land" misled.

**Assumes.** Plan 2609031939 has landed: the deserted refusals and the
`dead` render already reconcile with the live pane, and the refusals lead
with resume instead of yield for an attended lane. This phase adds the
"ask the agent" remedy beside that resume — it does not re-do the
reconciliation. The live-pane fact is in hand at the refusal sites
through [`liveLaneFor`](../../cmd/frit/dispatch.go), and Phase 1's handoff
names how the render sites reach it.

**Value.** Even a lane correctly shown as attended leaves a reader
asking "is this abandoned or in flight?" — and git cannot answer, because
work open as a PR reads unlanded. The one source that can answer is the
agent, now reachable with `frit message`. Naming that verb in the output
turns the reader from inferring to asking. This is the remedy missing on
2026-09-03: the refusal named `frit yield`, the reader tore down and
prepared to hand-land, and the PR was open the whole time.

**RED.** Add unit tests, leading with the observed case:

- an explicit `start <id>` (or the deserted refusal path) on a held lane
  a live pane attends, work unlanded, names `frit message <id>` as a way
  to ask the agent its status, and does not lead with `frit yield` or a
  hand-land. It may name resume too — 2609031939's wording — but the ask
  remedy is present.
- the survey report a reader reads to decide — board and/or the
  discovery card for the same lane — carries the "ask the agent" pointer
  or the fact a consumer needs to offer it, without breaking the JSON
  Contract (every key present, built from one model).
- a held lane with **no** live pane names no `frit message` remedy: there
  is no agent to ask, so its deserted reading and yield remedy stand
  unchanged.

Each fails today: the refusal names yield, and no report mentions
messaging. Commit the red.

**GREEN.** At the refusal site in
[cmd/frit/start.go](../../cmd/frit/start.go), where 2609031939 already
consults the live pane, add the `frit message` remedy for the attended,
unlanded case. Decide, against the JSON Contract, whether the survey
reports carry a machine-readable pointer or a rendered hint, and apply it
at the shared site (`cardOf` for the discovery reports) so `ready`,
`pick` and `find` cannot drift. Gate every branch on the live-pane fact,
so a no-pane lane is untouched. Reuse Phase 1's report vocabulary for the
remedy text. Every changed function keeps its dedicated unit test.

**Guard the edges.** A no-pane deserted lane reads and refuses exactly as
before. A live *bound* lane is unchanged. The remedy names the real verb
and selector, so a reader can run it verbatim. Table and JSON renderers
agree.

**Gate.** The new tests pass: an attended, unlanded lane names `frit
message` and does not lead with `frit yield`; a no-pane lane is
unchanged. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

Write the handoff to `phase-2.result.md`. Record where the remedy is
emitted (refusal, report, or both), the field or wording added, and the
exact scenario Phase 3 should reproduce end-to-end so S91 pins the whole
route from state to "ask the agent".
