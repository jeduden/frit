---
id: 2608212236
title: A --go dispatch reads as a handoff, not a to-do
status: "🔳"
summary: >-
  After `frit start`/`pick --go`, the output reuses the dry-run's
  recipe shape: an `agent:` and `prompt:` line an orchestrating agent
  misreads as its own next command, when a separate lane agent is
  already running that prompt in its pane. Make the dispatch read as a
  completed handoff in both renderings: the text relabels the prompt
  as running and closes with the caller's real next action, and the
  JSON carries a `handoff` field a consumer keys on instead of
  re-deriving the state.
model: sonnet
depends-on: []
---
# A --go dispatch reads as a handoff, not a to-do

## Goal

An agent reading a `--go` dispatch understands that another agent is
already running the phase in its pane. So it reports the lane and
moves on, rather than running the prompt itself.

## Context

The incident: after `frit pick --go`, an orchestrating agent read the
printed `prompt: /plan-phase ...` line as its own next command and ran
the phase in the main checkout, duplicating the lane agent's work.

The cause is one printer. [printStart](../cmd/frit/start.go) renders
`start` the same shape whether it is a dry run or a live `--go`
dispatch: `agent:` then `prompt:`, a recipe the reader is invited to
execute. Under `--go` that recipe has already been handed to a spawned
agent — herdr started it and frit sent it the prompt (`MarkStarted`
records the pane) — but nothing in the output says so. The dry-run
path even ends with `run again with --go to execute`, reinforcing the
reader-executes reading; the started path just stops.

Reuse, not a new printer: the text fix is inside the existing
`printStart` and gated by the `doc.Started` branch already there. The
JSON model is [StartDoc](../internal/report/dispatch.go), which
already tracks `Started`, `Prompt` and `Pane`. A consumer *could*
derive "the prompt is not mine" from `started && !refused`, but that
re-implements the policy the text states in prose. Phase 2 adds a
`handoff` field that names the one axis a consumer acts on — the JSON
mirror of the text's closing line — set by the same `MarkStarted` /
`Refuse` methods that already carry the state, so no new state is
introduced and the "every key always present" contract holds.

`pick --go` prints through this same `printStart`, so both verbs are
fixed by the one change.

## Tasks

1. Text: under `--go`, relabel the prompt as running and close with
   the caller's next action.
2. JSON: add an always-present `handoff` field a consumer keys on.

## Phase 1: the text dispatch names the running agent

Under a live `--go` dispatch (`doc.Started`), the output must read as
a handoff to another agent, not a recipe for the reader. The
prompt line is relabelled `running:` — it is dispatched and executing,
not a suggestion — and a closing line states the caller's real next
action: the lane agent is running the phase in its pane; do not run
it here; watch with `frit open <id>` or move on with `frit board`.
The dry-run and refusal paths are unchanged.

RED, against the `run(...)`/`out.String()` idiom the start tests use
(extend `TestStartGoRunsTheEscalation`, add a dry-run counter-case):

- A `--go` dispatch: the output has `running:` not `prompt:`, names
  the pane, and carries the "don't run it here" directive with the
  plan id.
- A `--go` dispatch: the output does not carry `run again with --go`.
- A dry run: still `prompt:` and still `run again with --go to
  execute`; no "don't run it here" line.

GREEN: the `doc.Started` branch in
[printStart](../cmd/frit/start.go) prints `running:` and the closing
directive; the dry-run branch below it is untouched.

Gate: the three RED cases pass; `go test ./...` and `mdsmith check .`
are clean; the report golden files still hold.

## Phase 2: the JSON dispatch carries a handoff field

`StartDoc` gains an always-present `handoff` string a consumer keys
on. It is `"running"` when the prompt was dispatched to a spawned
agent now executing it. It is `"preview"` for a dry run the caller
would cause with `--go`. It is `"none"` when the escalation was
refused and nothing runs. The methods that already carry the state
set it: default `"preview"` in `NewStart`, `"running"` in
`MarkStarted` (so the resumed path gets it too), `"none"` in
`Refuse`. So no new state enters the model.

RED, with the golden idiom in
[golden_test.go](../internal/report/golden_test.go) plus a focused
unit test on the three transitions:

- A fresh `StartDoc`: `handoff` is `"preview"`.
- After `MarkStarted`: `handoff` is `"running"`.
- After `Refuse`: `handoff` is `"none"`.
- The `start.json` golden carries `handoff` and every key stays
  present.

GREEN: add the `Handoff` field with its `json:"handoff"` tag and set
it in `NewStart`, `MarkStarted` and `Refuse` in
[dispatch.go](../internal/report/dispatch.go).

Gate: the unit cases pass; `start.json` is re-recorded with `go test
./internal/report -update` and the diff read; `go test ./...` and
`mdsmith check .` are clean.

## Execution

Tier is per phase. The design is settled here, so both phases
implement from written assertions; both are output-formatting changes
guarded by string and golden assertions.

| Phase          | Design | Implement | Gate that catches a wrong answer                                      |
| -------------- | ------ | --------- | --------------------------------------------------------------------- |
| 1 text handoff | opus   | sonnet    | `--go` shows `running:` and the directive; dry run keeps its recipe   |
| 2 json handoff | opus   | sonnet    | handoff is running/preview/none across the three states; golden holds |

## Acceptance Criteria

- [ ] A `--go` dispatch shows `running:` and a "don't run it here"
      directive naming the plan id
- [ ] A `--go` dispatch does not invite the reader to re-run with
      `--go`
- [ ] The dry-run and refusal text is unchanged
- [ ] `StartDoc` JSON always carries `handoff`, valued running for a
      live dispatch, preview for a dry run, none for a refusal
- [ ] `pick --go` and `start --go` are both fixed by the one printer
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
