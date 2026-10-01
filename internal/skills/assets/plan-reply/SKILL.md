---
name: plan-reply
description: >-
  Answer a frit ask: when a message in this session says "frit: a
  reply is wanted", record the answer with frit reply, which the
  asker reads off their board. Trigger on an incoming frit ask, "a
  reply is wanted", "load the plan-reply skill".
---
# plan-reply

A supervisor asked this lane a question with `frit message --ask`.
Answer it; frit carries the answer back to their board.

## Method

1. Read the question: the text before "frit: a reply is wanted".
2. Answer from what this lane knows: the phase it is on, whether the
   work is pushed, any open PR and its link, and what blocks it. Keep
   it to one line, with no double quotes.
3. Run it from this lane's worktree:
   `{{frit}} reply "<answer>" --json`
4. `recorded: true` means the asker can read it. A non-empty
   `refused` names why not: with no ask pending, it was already
   answered.
5. Resume the work you were doing.

## Notes

- `reply` writes one local file: no pane, no ref, no network. `frit
  skills` pre-approves it in `.claude/settings.json`, so it needs no
  operator sign-off.
- Never answer with `frit message`: that is a send, gated on the
  operator.
- Off the lane's branch, pass `--plan <id>`.
