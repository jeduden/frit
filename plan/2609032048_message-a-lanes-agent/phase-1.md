---
n: 1
title: frit message sends an operator's text to a live lane
status: "✅"
result: false
---
Give frit an agent-aware `message` verb that sends arbitrary text to a
lane's live agent through herdr, working or idle. This is the capability
the whole plan turns on — without a way to ask the agent, the output in
Phase 2 has nowhere to route — so it is the proving slice.

**Assumes.** The send primitive and the pane lookup already exist:

- [`herdr.Prompt`](../../internal/herdr/dispatch.go) writes arbitrary
  text to a pane by id. `nudge` already calls it, passing only the phase
  prompt `dispatch.Command`.
- [`liveLaneFor`](../../cmd/frit/dispatch.go) finds the live pane on a
  plan's lane and carries its `PaneID`; `nudge` already uses it, and
  handles herdr-unreachable and presence-unknown around it.
- [`nudge`](../../cmd/frit/dispatch.go) is the shape to follow, with one
  deliberate difference: `nudge` refuses any lane that is not idle,
  because it would collide with a working phase. `message` carries the
  operator's own words, so it must reach a **working** agent too — that
  is the whole point of asking "are you in a PR?".

**Value.** Today no verb sends an operator's text to a working agent.
`open` sends nothing; `nudge` sends only the phase prompt and only to an
idle lane. So a supervisor who sees an ambiguous held lane cannot ask
the agent what is true, and falls back to inferring from refs — which
git cannot settle for work open as a PR. This is the gap that, on
2026-09-03, sent a reader to `frit yield` and a hand-land on plan
2609021315 while its PR #151 was open. After this phase, asking is one
verb.

**RED.** Add unit tests for the new command, leading with the working
lane:

- a `message <id> "text"` on a live lane whose agent is **working**
  names the target pane and, under a dry run, shows the text it would
  send; under `--go` it calls `herdr.Prompt` with that exact text. Assert
  it does not refuse the lane for being non-idle — the divergence from
  `nudge`.
- an **idle** lane behaves the same, so both live statuses are covered.
- a plan with no live lane is refused; a configured host left unread
  reports presence-unknown, not an absent lane — mirroring `nudge`'s
  refusals through the same `liveLaneFor` returns.

Each fails today: there is no `message` command to run. Commit the red.

**GREEN.** Add the `message` command beside `nudge` in
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go) and register it in the
kong CLI. It resolves the selector, finds the lane through `liveLaneFor`,
carries the same herdr-unreachable and presence-unknown handling, and
sends the operator's text with `herdr.Prompt` under `--go`, dry-running
otherwise. Reuse the report shape: either extend
[`NudgeDoc`](../../internal/report/dispatch.go) or add a sibling doc that
carries target, text, sent, refused and problems, built once so the
table and `--json` render from one model (the JSON Contract). Do not
gate on idle. Every new function keeps its dedicated unit test.

**Ship the skill.** Per the Shipping Skills rule in CLAUDE.md, the verb
ships with its thin skill front in this same change. Fold `message` into
[plan-drive](../../.claude/skills/plan-drive/SKILL.md), the skill that
already fronts driving a lane from outside (raise, nudge, escalate):
messaging the agent is the rung for "the lane's state is unclear — ask
it". Follow the JSON Contract for the example command, and gate on the
claim, not the copy: run the built `frit message` and confirm its output
matches what the skill says, since lint and the dogfood-match test pass
on a false claim.

**Guard the edges.** `nudge`'s idle-only refusal is unchanged — only
`message` reaches a working lane. `open` still sends nothing. A lane with
no pane is refused, not sent to. The table and JSON renderers agree.

**Gate.** Built `frit message <id> "hi"` on a live lane names the target
pane and shows the text under a dry run, sends it under `--go`, and does
so on a working lane as well as an idle one; the plan-drive skill claim
runs green against the built binary; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Record the report shape chosen
(extended `NudgeDoc` or a sibling) and why, the exact verb name and flag
surface, and how Phase 2 should reach the "a live pane attends" fact at
the refusal and report sites — through `liveLaneFor`, or a shared helper
this phase introduced.
