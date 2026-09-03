---
n: 1
title: no report calls an attended lane dead
status: "🔲"
result: false
---
Make every frit report stop rendering `dead: true` for a held lane
whose bound session herdr confirms gone but whose branch a live pane
still attends — working or idle. This is the field that flips a
reader's whole model, so it is the proving slice.

**Assumes.** Two render paths carry a `dead` field, each straight from
`discovery.Plan.Dead`:

- board, through [`AddPlan`](../../internal/report/board.go), which
  copies `p.Dead` into `BoardPlan.Dead` and already takes the live
  pane's `agent` and `status` as arguments — both facts in hand at
  render time.
- the discovery card, through `cardOf` in
  [discovery.go](../../internal/report/discovery.go), which backs
  `ready`, `pick` **and** `find` from one site and does **not** carry
  the agent, so its render has no live-pane fact to consult yet.

`discovery.Plan.Dead` is set by `observeHolds` from herdr's session
check — "the bound session is confirmed gone". The live pane is what
[`liveLaneFor`](../../cmd/frit/dispatch.go) finds; the board verb
already reads it, and the discovery gather can read it the same way,
one herdr query, to fill the fact `cardOf` lacks. orphans is out of
scope. It says "dead" both as a `StaleHold.Dead` field and as the
membership-based `Deserted` category. Listing a bound-session-gone lane
for cleanup is a defensible thing for a teardown verb to do — a
separate question from the survey reports a person reads to decide.

**Value.** Read together, `dead: true` and `agent: claude,
agent_status: working` are a contradiction, and "dead" wins — a lane an
agent is actively working looks gone, and the reader reaches for a
teardown. This was observed on 2026-09-03: plan 2609021313 reported
`dead: true, agent: claude, agent_status: working` in board and `dead:
true` in `ready` while an agent finished it and landed it. After this
phase every report shows an attended lane as attended.

**RED.** Add unit tests at each render site, and lead with the
observed working-agent case:

- board: in [cmd/frit/board_test.go](../../cmd/frit/board_test.go) (or
  the report type's golden test if the reconciliation lives in
  `AddPlan`), build a held plan with `discovery.Plan.Dead` true and
  render it with a non-empty `agent` and `agent_status` of "working".
  Assert the result does not present that lane as dead. Add an "idle"
  case too, so both live statuses are covered.
- the discovery render: exercise `cardOf` (through `ready`, and
  confirm the same for `pick` and `find`, which share it) for the same
  held, bound-session-gone plan with a live pane on its lane, and
  assert it does not report `dead: true`. This is the render that today
  carries `dead` with no agent beside it.

Each fails today: `Dead` copies straight through, whatever pane
attends. Commit the red.

**Decide the field shape, against the JSON Contract.** The contract
(CLAUDE.md) requires every key present, no key nulled, and both
renderings built from one model. Choose, and record the choice in the
handoff:

- gate the rendered `dead` on the live pane — `dead` is true only when
  the bound session is gone *and* no pane attends, so `dead: true`
  means "nobody is here", the plain reading; or
- keep `dead` as the identity fact and add a companion boolean that
  says the lane is attended, so no single field overclaims.

board already prints `agent`/`agent_status`; the discovery card does
not. So whichever shape wins, the live-pane fact must be plumbed into
the discovery gather and card, not guessed. Prefer the smaller contract
change that removes the contradiction. Apply it once at `cardOf`, so
`ready`, `pick` and `find` cannot drift, rather than as per-doc gates.

**GREEN.** Reconcile both renders with the live pane, reusing the one
herdr read the fleet gather already does — no per-report second query.
Plumb the live-pane fact through to `cardOf`, which lacks it today and
backs `ready`, `pick` and `find` at once; board already has the `agent`
argument. Do not touch `discovery.Plan.Dead` or `observeHolds`: the
decision logic still reads the identity fact.

**Guard the edges.** A held lane with the bound session gone and **no**
live pane still renders exactly as before — `dead: true`, the takeover
candidate it is — in board and the discovery card. A live *bound* lane
(not dead) is unchanged. Confirm each report's table renderer and JSON
renderer agree, since both build from the one model (the JSON
Contract's rule). `find` picks up the fix through the shared `cardOf`;
confirm its render, in scope by that path, reads right too. Every
changed function keeps its dedicated unit test.

**Gate.** The new render tests pass for board and the discovery card
(`ready`/`pick`/`find`), on both the working and idle pane; a lane with
no pane still reads dead everywhere; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Record the field-shape
decision and its contract rationale. Name the shared reconciliation
site and how the live-pane fact was plumbed into the agent-less
reports. Say how Phase 2's refusals should read the same fact — through
`liveLaneFor` at the refusal site, or the shared helper this phase
introduced.
