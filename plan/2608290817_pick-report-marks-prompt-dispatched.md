---
id: 2608290817
title: The dispatch report marks its prompt already dispatched
status: "🔳"
summary: >-
  After `frit start`/`pick --go`, the JSON still carries a bare
  `prompt` field that reads as "run this yourself", identical in shape
  whether the prompt was already sent into the lane's pane or not. A
  consumer keying on `prompt` can double-drive the lane. The text and a
  `handoff` field already name the handoff, and `next_action` already
  hands over `frit open <id>` — but `prompt` stays ambiguous. Add an
  always-present `prompt_dispatched` boolean, kept in sync with
  `handoff`, so the `prompt` string is unambiguously informational once
  it has been sent.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: the JSON marks the prompt as already dispatched
    status: "🔲"
---
# The dispatch report marks its prompt already dispatched

## Goal

A consumer parsing `frit pick --go --json` can tell the `prompt` string
was already sent into the pane. So it never re-runs the phase against a
worktree another agent already drives.

## Context

Issue [#86](https://github.com/jeduden/frit/issues/86). Plan
[2608212236](2608212236_dispatch-reads-as-a-handoff.md) already relabels
the text as `running:` under `--go` and added a `handoff` field. Since
then a derived `next_action` was added too: it hands over `frit open
<id>` once the handoff is running. Together they signal a live handoff.

The residual gap is the `prompt` field itself. In
[StartDoc](../internal/report/dispatch.go) `Prompt` is a bare string in
every state — preview, running and none carry it identically. Its own
doc comment concedes the point: `next_action` is "the verb a consumer
runs instead of the dispatched Prompt ... where Prompt is still the
recipe to run". A consumer that keys on `prompt` rather than `handoff`
reads the same instruction whether or not it was already dispatched, and
double-drives the lane.

Reuse, not a new axis: `setHandoff` in
[dispatch.go](../internal/report/dispatch.go) is already the one writer
that keeps `Handoff` and `NextAction` in step. A `prompt_dispatched`
boolean derived there — true only for `HandoffRunning` — cannot disagree
with the handoff it mirrors, and no new state enters the model. The text
renderer is unchanged; it already reads as a handoff.

## Tasks

1. Add an always-present `prompt_dispatched` boolean to `StartDoc`, set
   through `setHandoff` so it tracks the handoff.

## Phase 1: the JSON marks the prompt as already dispatched

`StartDoc` gains an always-present `prompt_dispatched` boolean. It is
`true` exactly when `Handoff` is `HandoffRunning` — the prompt was sent
into the pane — and `false` for a preview or a refusal, where `prompt`
is still the caller's recipe. It is written only through `setHandoff`,
beside `NextAction`, so the three cannot part. The `Prompt` field's doc
comment gains a line pointing at `prompt_dispatched` for the "is this
mine to run" question.

RED, with the golden idiom in
[golden_test.go](../internal/report/golden_test.go) plus a focused unit
test on the three transitions:

- A fresh `StartDoc`: `prompt_dispatched` is `false`.
- After `MarkStarted`: `prompt_dispatched` is `true`.
- After `Refuse`: `prompt_dispatched` is `false`.
- `start-running.json` carries `prompt_dispatched: true`; `start.json`
  and `start-wholeplan.json` carry `false`; every key stays present.

GREEN: add a `PromptDispatched bool` field to `StartDoc`, tagged
`json:"prompt_dispatched"`. Set it inside `setHandoff` from
`handoff == HandoffRunning`, in
[dispatch.go](../internal/report/dispatch.go).

Gate: the unit cases pass; the three `start*.json` goldens are
re-recorded with `go test ./internal/report -update` and the diff read
to confirm `prompt_dispatched` is true only under a running handoff;
`go test ./...` and `mdsmith check .` are clean.

## Execution

Tier is per phase. The design is settled here: one output-formatting
field guarded by a focused unit test and the golden files.

| Phase         | Design | Implement | Gate that catches a wrong answer                                     |
| ------------- | ------ | --------- | -------------------------------------------------------------------- |
| 1 prompt flag | opus   | sonnet    | prompt_dispatched is true only under a running handoff; goldens hold |

## Acceptance Criteria

- [ ] `StartDoc` JSON always carries `prompt_dispatched`
- [ ] `prompt_dispatched` is `true` for a running handoff, `false` for a
      preview or a refusal
- [ ] `prompt_dispatched` is written only through `setHandoff`, so it
      cannot disagree with `handoff` or `next_action`
- [ ] `pick --go` and `start --go` are both fixed, since both render
      through the one `StartDoc`
- [ ] The text rendering is unchanged
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
