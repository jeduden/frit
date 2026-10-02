---
n: 1
title: An ask and its reply, end to end
status: "✅"
result: true
summary: >-
  message --ask sends an envelope and records a pending ask, reply
  answers it from the lane as a local write, and board --json reads
  none, pending or answered; C13 runs the loop through the built frit.
  A real session answered once the reply was a permission rule, but
  plan-reply's allowed-tools did not grant it, so that criterion stays
  open.
---
# Phase 1 result

## Handoff

A supervisor can now ask a lane a question and read the answer off
the board. `frit message <id> --ask "<text>"` wraps the text in a
one-line envelope: a reply is wanted, load `plan-reply`, or run
`frit reply "<answer>"`. A dry run shows it; `--go` records a pending
ask, then sends. `frit reply "<answer>"` from the lane infers the plan
from its branch and records the answer. It gathers nothing, fetches
nothing and reaches no herdr, so it moves no ref and writes no pane.
With nothing pending it refuses and says so. `board --json` carries
`ask_state` (`none`, `pending`, `answered`) and `answer` on every row.

**Where it deviates from the spec.** In five places:

- The board keys are `ask_state` and `answer`. `ask` already carries
  the remedy command, and changing its meaning would break consumers.
- The record is `frit/ask-<id>.json` under the repository's common
  git dir, not the per-worktree dir `claim.TokenPath` uses. The main
  checkout and a linked lane have different git dirs; only the common
  dir is one file for both.
- `reply` names another plan with `--plan <id>`, not a slug. Resolving
  a slug needs the fleet gather, which fetches.
- `--ask` refuses a lane on another host by name, since its reply
  would land in a checkout this host never reads.
- The ask is recorded before the send, so a fast reply finds it, and
  withdrawn if the send fails.

**Verified by.** Unit tests for the record, the report model and the
verbs, committed red first. C13 drives the built frit with a fake
herdr on `$PATH`: ask from the main checkout, reply from a linked
lane, then `board --json` reads it answered. It also checks the reply
left the herdr log and every ref untouched. Golden JSON changes are
additive only. The new package joins the CI coverage gate at 100%.

**The approval claim, tried in real sessions.** Claude Code 2.1.286,
print mode, default permissions, in a fixture lane with the bundle
installed at the default bare `frit`. Print mode denies, rather than
prompts, any tool not pre-approved.

1. Envelope alone: the agent tried to load `plan-reply`, and the
   Skill load itself was denied. Its reply was then denied too. It
   drafted the right answer and asked for approval.
2. Skill tool pre-allowed: the skill loaded, and the agent ran its
   exact example, `frit reply "…" --json`. It was denied anyway. The
   skill's `allowed-tools: Bash(frit reply:*)` granted nothing.
3. Control, no skill, `Bash(frit reply:*)` given as a session
   permission rule: the reply ran with no denial, and `board --json`
   read `answered` with the agent's text.

So the pattern is right and the loop closes with a real agent, but a
skill's front matter is not where the grant takes effect. The
approval criterion stays unticked. Interactive mode was not tried; it
would prompt the operator, which is the sign-off this plan removes.

**For phase 2.** The owner decides where the `Bash(frit reply:*)`
rule lives. One way is for `frit skills` to write it into the
repository's `.claude/settings.json` allow list, beside the skill.
The other is a documented one-line operator step. The Skill load
needs the same answer. The planned work remains: render the ask state
in the `board` and `who` tables, and make the `(dead)` advice point at
`--ask` and say silence is not evidence. `plan-drive` already says so.
