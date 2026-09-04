---
id: 2609032048
title: frit can message a lane's agent, so an ambiguous lane asks
status: "🔳"
summary: >-
  Git alone cannot tell a deserted, unlanded lane apart from one whose
  work is pushed and sitting in an open PR while its agent finishes the
  merge. On 2026-09-03 a reader read the second for the first and
  reached for `frit yield` and a hand-landing on plan 2609021315 — whose
  work was already open as a PR awaiting CI. herdr already monitors the
  agent on a lane and knows when it works or idles, but frit has no verb
  to send that agent a question: `open` sends nothing, `nudge` sends only
  the phase prompt and only to an idle lane. This plan makes frit
  agent-aware in the one way that resolves the ambiguity — a `message`
  verb that sends arbitrary text to a lane's live agent through herdr,
  working or idle — and routes the ambiguous-state output there. A held
  lane a live pane attends, whose disposition git cannot classify, names
  `frit message` as the way to ask the agent, not `frit yield` or a
  hand-land. A cross-layer matrix row, S90, runs the state under godog.
model: sonnet
depends-on: [2609031939]
---
# frit can message a lane's agent, so an ambiguous lane asks

## Goal

A held lane can look abandoned when it is not. Its work may be pushed
and open as a PR; its agent may be mid-merge. Refs alone cannot tell the
two apart. frit resolves the ambiguity by asking the agent, not by
reaching for a teardown or a hand-land. It gains an agent-aware
`message` verb that sends text to a lane's live agent through herdr,
working or idle. The deserted refusals and survey reports route a reader
to that verb when git cannot classify the lane's work.

## Context

**The misread, observed.** On 2026-09-03 `frit pick --go` refused plan
2609021315 with "deserted hold: its branch carries an unparked suffix;
run `frit yield 2609021315` to park it first", and `frit board` showed
it `dead: true`. A reader yielded the hold and prepared to rebase the
worktree onto main and land it by hand. The work was already pushed to
`refs/heads/plan/2609021315` on origin and open as PR #151, its CI still
running. Nothing in frit's output carried that fact, and nothing pointed
the reader at the one source that had it: the agent on the lane, which
was alive and finishing a merge.

**Why git cannot settle it.** frit's landed-evidence reads origin's
default branch, through [`lanes`](../../internal/lanes/lanes.go), and a
plan whose work is open as a PR is not yet merged there — so it reads
"unlanded", exactly as a truly abandoned local lane does. frit has no
awareness of pull requests, and adding it would couple frit to GitHub
and break the git-only, shell-out-and-parse-porcelain rule in CLAUDE.md.
The honest resolution is not for frit to guess the PR: it is to ask the
agent, which knows. "Message the agent is the only way."

**herdr already monitors the agent; frit cannot yet speak to it.** herdr
reports the agent on a pane and whether it works or idles —
[`Pane.Agent`, `Pane.Presence`](../../internal/herdr/parse.go) — and
frit already reads that in board's `agent`/`agent_status`. What frit
lacks is a verb to send that agent text. [`open`](../../cmd/frit/dispatch.go)
focuses the pane and sends nothing; [`nudge`](../../cmd/frit/dispatch.go)
sends only the phase prompt, `dispatch.Command`, and refuses any lane
that is not idle. Neither lets a supervisor ask a working agent "are you
in a PR — what is your status?". The send primitive already exists:
[`herdr.Prompt`](../../internal/herdr/dispatch.go) writes arbitrary text
to a pane, and [`liveLaneFor`](../../cmd/frit/dispatch.go) already finds
the pane and carries its `PaneID`.

**What is reused.** `message` is `herdr.Prompt` over the pane
`liveLaneFor` finds — no new herdr query and no new send path, only a
verb that carries the operator's own text instead of a phase prompt, and
that does not refuse a working lane the way `nudge` does. Its report
follows [`NudgeDoc`](../../internal/report/dispatch.go): a target pane, a
dry-run by default, `--go` to send, a refusal when no live lane is found.
The routing in Phase 2 layers on plan 2609031939, which makes the same
refusals lead with resume instead of yield for an attended lane; this
plan adds the "ask the agent" remedy beside it. The godog row reuses the
cross-layer step file and herdr-fake vocabulary and takes the next free
id, S90.

**Why depends-on 2609031939.** That plan reconciles the `dead` render
and the deserted refusals with the live pane, editing the same start.go
refusal region this plan's Phase 2 extends. Landing it first keeps this
plan's edits clean and its S-id from colliding with its S89.

**Out of scope.** No change to the lease protocol, to landed-evidence,
or to `discovery.Plan.Dead`. frit gains no pull-request awareness: it
stays git-only and points at the agent, which is the source of truth for
in-flight work. A lane with no live pane is unchanged — there is no agent
to ask, and its deserted reading and yield remedy stand.

## Tasks

1. Phase 1 (proving slice): a `frit message <selector> "<text>"` verb
   sends arbitrary text to a lane's live agent through `herdr.Prompt`,
   whether the agent works or idles, dry-run unless `--go`, reporting the
   target pane and the text; and it ships its thin skill front in the
   same change.
2. Phase 2: the deserted refusals and the survey reports route a reader
   to `frit message` for a held lane a live pane attends whose work git
   cannot classify — so "ask the agent" stands where "yield and land"
   misled, layered on plan 2609031939's resume-first refusal.
3. Phase 3: document the state as cross-layer matrix row S90 and run it
   end-to-end under godog — the output routes to messaging the agent and
   the message reaches the pane — so the misread cannot return.

## Execution

| Phase | Title                                        | Tier   | Gate                                                                                                                                        |
| ----- | -------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | frit message sends text to a live lane       | sonnet | Built `frit message <id>` on a working lane names the pane, shows the text dry, sends under `--go`; skill claim runs; suite and lint clean  |
| 2     | the ambiguous-lane output routes to message  | sonnet | Unit test: an attended, unlanded lane names `frit message` and not `frit yield`; a no-pane lane is unchanged; suite and lint clean          |
| 3     | S90 runs the ask-the-agent state under godog | sonnet | `TestFeatures/S90:` passes no SKIP; `go test ./internal/scenario` bijection green; `go test ./...` and lint clean; `mdsmith check .` passes |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                         |
| --- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [frit message sends an operator's text to a live lane](phase-1.md)                                                                                                                                                                                                                                                                            |
|     | ↳      | `frit message <id> "<text>"` sends arbitrary text to a plan's live lane through `herdr.Prompt`, working or idle — the one deliberate divergence from `nudge`, which refuses a busy lane. Dry-run by default, `--go` to send, reporting a sibling `report.MessageDoc` built the same way `NudgeDoc` is. The skill front rides the same change. |
| 2   | 🔲     | [the ambiguous-lane output routes to message](phase-2.md)                                                                                                                                                                                                                                                                                     |
| 3   | 🔲     | [S90 runs the ask-the-agent state under godog](phase-3.md)                                                                                                                                                                                                                                                                                    |
<?/catalog?>

## Acceptance Criteria

- [x] `frit message <selector> "<text>"` sends the given text to the
      live agent on the plan's lane through `herdr.Prompt`, whether the
      agent is working or idle; it dry-runs by default and sends only
      under `--go`, naming the target pane in both cases
- [x] `frit message` refuses a plan with no live lane, and reports
      presence-unknown rather than an absent lane when a configured host
      goes unread — matching how `nudge` withholds its action
- [x] The `message` verb ships with its thin skill front in the same
      change, per the Shipping Skills rule; the skill's example command
      follows the JSON Contract
- [ ] For a held lane a live pane attends whose work is unlanded, the
      deserted refusal and the survey report name `frit message` as the
      way to ask the agent, and do not lead with `frit yield` or a
      hand-landing
- [ ] A held lane with no live pane is unchanged: no `frit message`
      remedy is offered, and its deserted reading and yield remedy stand
- [ ] frit gains no pull-request awareness and no new remote read: the
      resolution is the agent, and the send reuses `herdr.Prompt` over
      the pane `liveLaneFor` already finds
- [ ] Cross-layer matrix row S90 is documented in
      [docs/research/lease-protocol.md](../../docs/research/lease-protocol.md),
      and a `@S90` scenario in
      [features/cross-layer.feature](../../features/cross-layer.feature)
      reproduces the state and runs for real, not `@pending`
- [ ] `go test ./cmd/frit -run 'TestFeatures/S90:'` reports S90 PASS,
      not SKIP; `go test ./internal/scenario` stays green
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
