---
id: 2608260639
title: A dispatched handoff carries the consumer's next action, not a bare prompt
status: "✅"
summary: >-
  After `frit pick --go`/`start --go`, the JSON still carries a bare
  `prompt` field that reads as "run this yourself", though the prompt
  was already dispatched into the lane's pane. A consumer keying on
  `prompt` double-drives the lane. Add an always-present `next_action`
  to `StartDoc`: when a handoff is running it carries the caller's real
  next verb (`frit open <id>`), leaving `prompt` informational; empty
  otherwise, so the dry-run recipe is unchanged.
model: sonnet
depends-on: [2608212236]
---
# A dispatched handoff carries the consumer's next action, not a bare prompt

## Goal

A consumer parsing `--json` after a `--go` dispatch is handed the
verb it should run next: look at the lane. It no longer infers one
from a `prompt` string that was already sent. So it never runs the
phase a second time against a worktree another agent is driving.

## Context

Issue [#86](https://github.com/jeduden/frit/issues/86). The prior plan
[2608212236](2608212236_dispatch-reads-as-a-handoff.md) fixed the
*table*: under `--go`, [printStart](../cmd/frit/start.go) relabels the
prompt `running:` and closes with a "do not run it here — watch with
frit open <id>" directive. It also added the `handoff` field to
[StartDoc](../internal/report/dispatch.go), valued `running` on a live
dispatch.

The residual gap is in the JSON alone. Its `prompt` key still carries
the exact text already sent into the pane, presented flat and adjacent
to `handoff: "running"` with no stated relationship. The obvious
reading of `prompt` ("here is what to run next") contradicts the fact
that it was already dispatched. A consumer that acts on `prompt`
double-drives the lane — two sessions mutating one worktree corrupts
any check either runs.

`handoff: "running"` names the *state* but not the caller's *action*.
The text already states that action in its closing line; the prior
plan framed `handoff` as "the JSON mirror of the text's closing line",
but it mirrored only the state, not the verb the line hands over. This
plan finishes that mirror: the JSON carries the caller's real next
action the way the text does.

Design: add an always-present `next_action` string to `StartDoc`. When
`handoff` is `running`, it carries `frit open <id>` — the safe verb, a
look at the running lane, which is exactly what the text's directive
offers first. It stays empty for `preview` and `none`, where the raw
`prompt` is still the recipe the reader is invited to run, so the
dry-run and refusal JSON are unchanged. `prompt` is kept always present
(the table's `running:` line and the golden depend on it); `next_action`
is added beside it, not carved out of it.

Rejected: a `prompt_dispatched: true` boolean. It only restates what
`handoff == "running"` already says — "not yours" — without handing the
consumer the verb that is. `next_action` says what to do, not merely
what not to do, and matches the JSON contract's habit of carrying the
consumer's action rather than making it re-derive one.

`pick --go` renders through this same `StartDoc`/`renderStart`, so both
verbs are fixed by the one field.

## Tasks

1. Add an always-present `next_action` field to `StartDoc`, set to
   `frit open <id>` when a handoff is running and empty otherwise.
2. Re-record the `start.json` golden and pin the field across the
   preview / running / none states.
3. Extend the same field to `OpenDoc` (see the extension below): name
   `frit start <id>` for a laneless plan whose presence was read, and
   make `next_action` a drift-proof projection on both documents.

## Phase 1: the JSON dispatch carries the consumer's next action

`StartDoc` gains an always-present `next_action` string a consumer runs
instead of the dispatched `prompt`. It is empty in `NewStart` (a
`preview` dry run) and on `Refuse` (`none`), where `prompt` is still the
recipe. `MarkStarted` — the one method that flips `handoff` to
`running` — sets it to `frit open <id>` composed from `d.Plan.ID`, so
the resumed dispatch (which also calls `MarkStarted`) carries it too. No
new state enters the model; the field is derived from what
`MarkStarted` already knows.

RED, with the three-transition unit idiom in
[dispatch_test.go](../internal/report/dispatch_test.go) beside
`TestStartHandoffTracksTheThreeTransitions`, plus the golden idiom in
[golden_test.go](../internal/report/golden_test.go):

- A fresh `StartDoc` (preview): `next_action` is `""`.
- After `MarkStarted` on plan 7: `next_action` is `frit open 7`.
- After `Refuse`: `next_action` is `""`.
- The `start.json` golden carries `next_action` and every key stays
  present (the golden is a dry run, so its value is `""`).

GREEN: add the `NextAction string \`json:"next_action"\`` field to
`StartDoc` in [dispatch.go](../internal/report/dispatch.go) and set it
inside `MarkStarted` with `fmt.Sprintf("frit open %d", d.Plan.ID)`.

Gate: the unit cases pass; `start.json` is re-recorded with `go test
./internal/report -update` and the diff read (only `next_action`, valued
`""`, is added); to confirm the running value end to end, build frit and
run `pick --go` against a claimable repo (or read the `MarkStarted`
assertion) and see `next_action` is `frit open <id>` while `prompt`
holds the dispatched text; `go test ./...` and `mdsmith check .` are
clean.

## Execution

One phase for `StartDoc`. The design is settled above; the phase
implements from written assertions and is guarded by unit and golden
assertions, with a built-binary check that the running value composes
the plan id. The extension below carries the same field to `open`.

| Phase              | Design | Implement | Gate that catches a wrong answer                                                 |
| ------------------ | ------ | --------- | -------------------------------------------------------------------------------- |
| 1 json next_action | opus   | sonnet    | next_action is `frit open <id>` when running, `""` on preview/none; golden holds |

## Extension: open carries the same next action

The plan's title is about a *dispatched handoff*, and `open` is the
sibling rung with the same gap: its JSON reported a resolved plan but
never the verb a consumer should run when no lane was live. This
extension carries `next_action` to `OpenDoc` too, so the whole handoff
family answers the one question consistently.

`open`'s `next_action` is `frit start <id>` when no lane is live and
presence was read. That is the next rung: `nudge` refuses a laneless
plan, and `open` has nothing to raise. It is empty when a lane was
focused — watch it, do not escalate. It is empty too when presence could
not be read, an unreachable herdr or an unread host, because a lane may
run behind the gap. A carried repo-read problem (`AddProblem`) does not
clear it. Only `PresenceUnknown` does. So an unrelated broken checkout
under `--all` never suppresses the rung. `start` stays the authority on
whether the rung proceeds. For a done or blocked plan it reports its own
refusal. That is why `open` names the rung rather than reimplementing
readiness.

On both documents `next_action` is a projection of its discriminant, not
lockstep state. `StartDoc.setHandoff` is the sole writer of `handoff`
and `next_action` together. `OpenDoc.refreshNextAction` reprojects from
`Focused` and presence. So the field can never lag the facts.
`printOpen` surfaces the rung as a `start it with …` line. So `open`'s
table gained a line — this extension is not JSON-only.

## Acceptance Criteria

- [x] `StartDoc` JSON always carries `next_action`
- [x] `next_action` is `frit open <id>` on a live `--go` dispatch and
      empty on a dry run or a refusal
- [x] `prompt` stays present and unchanged in every state
- [x] `pick --go` and `start --go` are both fixed by the one field
- [x] The `start.json` golden is re-recorded and every key stays present
- [x] `OpenDoc` JSON always carries `next_action`, `frit start <id>` when
      no lane is live and presence was read, empty on a focused lane or
      unread presence
- [x] A carried repo-read problem does not clear `open`'s `next_action`;
      only an unread presence does
- [x] `next_action` is a projection of its discriminant on both docs, so
      it cannot drift (`setHandoff`, `refreshNextAction`)
- [x] The `open` / `open-nolane` / `start-running` goldens pin the field
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
