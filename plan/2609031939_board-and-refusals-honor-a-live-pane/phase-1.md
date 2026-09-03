---
n: 1
title: board does not call an attended lane dead
status: "🔲"
result: false
---
Make `frit board` stop rendering `dead: true` for a held lane whose
bound session herdr confirms gone but whose branch a live pane still
attends. This is the field that flips a reader's whole model, so it is
the proving slice.

**Assumes.** board renders each held plan through
[`AddPlan`](../../internal/report/board.go), which copies `p.Dead`
straight into `BoardPlan.Dead` and takes the live pane's `agent` and
`status` as separate arguments. `p.Dead` is `discovery.Plan.Dead`, set
by `observeHolds` from herdr's session check — "the bound session is
confirmed gone". The live pane is what
[`liveLaneFor`](../../cmd/frit/dispatch.go) finds and what the board
verb already reads to fill `agent`/`agent_status` (empty when no pane
is live on the lane). So at render time board has both facts in hand:
whether the bound session is gone, and whether a pane attends now.

**Value.** Read together, `dead: true` and `agent: claude,
agent_status: idle` are a contradiction, and "dead" wins — a lane
idling between phases looks gone, and the reader reaches for a
teardown. After this phase, board reports an attended lane as
attended: the live pane the reader can trust is not shadowed by a
`dead` flag that means only "the original session rotated away".

**RED.** Add a unit test where `board` is exercised —
[cmd/frit/board_test.go](../../cmd/frit/board_test.go) for the verb, or
the report type's own golden test if the reconciliation lives in
`AddPlan`. Build a held plan with `discovery.Plan.Dead` true, the
bound session gone. Render it with a non-empty `agent` and an
`agent_status` of "idle", the live idle pane. Assert the result does
not present that lane as dead. This fails today: `Dead` copies straight
through, whatever pane attends. Commit the red.

**Decide the field shape, against the JSON Contract.** The contract
(CLAUDE.md) requires every key present and no key nulled. Choose, and
record the choice in the handoff:

- gate the rendered `dead` on the live pane — `dead` is true only when
  the bound session is gone *and* no pane attends, so `dead: true`
  means "nobody is here", the plain reading; or
- keep `dead` as the identity fact and add a companion boolean that
  says the lane is attended, so no single field overclaims.

board already prints `agent`/`agent_status`, so a consumer has the raw
attended fact either way. The question is which shape reads honestly
and keeps every documented consumer whole. Prefer the smaller contract
change that removes the contradiction.

**GREEN.** Reconcile the render with the live pane at the one place
`AddPlan` builds the `BoardPlan`, reusing the `agent` argument already
passed in — no second herdr read. Do not touch `discovery.Plan.Dead`
or `observeHolds`: the decision logic still reads the identity fact.

**Guard the edges.** A held lane with the bound session gone and **no**
live pane still renders exactly as before — `dead: true`, the takeover
candidate it is. A live *bound* lane (not dead) is unchanged. Confirm
the table renderer and the JSON renderer agree, since both build from
the one model (the JSON Contract's rule). Every changed function keeps
its dedicated unit test.

**Gate.** The new render test passes; a lane with no pane still reads
dead; `go test ./...` and `go tool -modfile=tools/go.mod golangci-lint
run` are clean.

Write the handoff to `phase-1.result.md`. Record the field-shape
decision and its contract rationale. Name the exact reconciliation site
and how Phase 2's refusals should read the same live-pane fact —
whether through `liveLaneFor` at the refusal site, or a shared helper
this phase introduced.
