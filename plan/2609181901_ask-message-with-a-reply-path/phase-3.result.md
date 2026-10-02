---
n: 3
title: Silence reads as no answer, never as gone
status: "✅"
result: true
summary: >-
  Every ask remedy now reads frit message <id> --ask, the board's ask
  line says no reply is not evidence, and board and who print each
  lane's ask state beneath their tables; who --json carries ask_state
  and answer.
---
# Phase 3 result

## Handoff

The `(dead)` advice no longer sends an operator to a check that
cannot answer. The ask command every remedy shares now carries
`--ask`, so the board's ask line, the discovery card's `ask` and
start's deserted refusal all tell the agent a reply is wanted. The
board's line adds that no reply is not evidence the lane is gone.

Beneath the `board` and `who` tables, a pending ask reads as asked
with no reply yet, saying silence is not evidence; an answered one
prints its answer. One helper renders both, from the fields `--json`
carries. `who --json` now has `ask_state` and `answer` on every lane,
read from the lane's own checkout. A lane on another host, or with no
plan, reads `none`; a record frit cannot read is a problem.

**Verified by.** Unit tests for the command text, the lines, the who
model and its read, committed red first. S91 now runs the remedy as
the refusal names it, `--ask` included, and checks the pane receives
the envelope rather than the bare text. No new scenario row: the
table lines present fields C13 already proves.

**Plan close.** Every acceptance criterion is met. What stays out of
scope, as the plan said from the start: an ask to a lane on another
host is refused, since its reply would land where this host never
reads.
