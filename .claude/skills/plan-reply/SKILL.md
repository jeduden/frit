---
name: plan-reply
description: >-
  Answer an incoming frit ask: a message saying a reply is wanted and
  naming this skill. Record the answer with frit reply, from the lane's
  own checkout. Trigger on "A reply is wanted", "answer the ask", "frit
  reply".
allowed-tools: Bash(go run ./cmd/frit reply:*)
---
# plan-reply

A supervisor asked this lane's agent a question with `frit message
--ask`. The answer is a local record the supervisor's frit reads, not a
second message.

## Reply

1. Answer from what you know of the lane: its branch, whether the work
   is pushed, whether a PR is open, what you are doing now.
2. Run `go run ./cmd/frit reply "<answer>" --json` from this lane's checkout. It
   takes the plan from the checkout; a plan id after the answer names
   it explicitly.
3. Read the result: `{"command": "reply", "plan": 7, "question": "...",
   "answer": "..."}` means the answer is recorded. Stop there.

## Notes

- It writes one file beside the lane's token. It sends nothing to a
  pane, moves no ref and needs no `--go`.
- A refusal that says no ask is pending means nothing waits for an
  answer: the ask never arrived or is already answered. Do not send
  the answer another way.
- Never ask the operator to approve or relay the reply.
