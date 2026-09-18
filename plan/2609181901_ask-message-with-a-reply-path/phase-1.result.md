---
n: 1
title: An ask and its reply, end to end
status: "✅"
result: true
summary: An ask now travels to a lane and its answer back, and board --json reports it; the plan-reply skill's pre-approval did not hold under default permissions.
---
## Handoff

Phase 1 landed the loop on one host, with one gate element unmet: the
approval claim. Read that section first.

**What works.** A supervisor asks with `message --ask --go`. The lane
answers with `reply`. `board --json` then reports the ask as `answered`
with the answer text. C13 proves it against the built frit, driven by
godog with a fake herdr on `$PATH`, and it runs without skipping. It
installs the bundled plan-reply skill with the built frit as its
invocation, runs the skill's own example command from the lane's
checkout, and checks the output matches what the skill promises. It
also confirms the reply touched no pane and moved no ref. `reply` takes
no `--go`, and with no ask pending it refuses and says why.

**Acceptance criteria met.** The dry-run envelope, the local reply, the
`board --json` ask state, the envelope naming the skill and the raw
command, and one host only. On one host only: `--ask` into a lane on
another host is refused, since its reply could never be read here. The
full Go tests, lint and `mdsmith check .` are clean.

The shape later phases copy:

- The ask is one file per plan beside the lane's token, written before
  the send and removed if the send fails. A new ask supersedes an
  answered one; an answer is never overwritten.
- `board --json` gained `ask_state` (`none`, `pending`, `answered`) and
  `ask_answer` on every row. The existing `ask` key is unchanged: it is
  still the advice command, not the state.
- The state is read only for a lane frit sees live on this host. A lane
  with no live pane reports `none` even if a record exists.
- The board and `who` tables do not render the state yet. That, the
  `(dead)` advice wording and plan-drive's ladder are the later phases
  the plan expected.

**The approval claim did not hold.** The gate asked for a real Claude
Code session, given the envelope under default permissions, with the
bundle installed in a fixture lane. The acceptance criterion for the
skill's pre-approval stays unticked. Evidence, from Claude Code 2.1.277
with haiku, project settings only so no user allow rule leaked in:

- Interactive, in tmux: the session loaded nothing until an operator
  approved `Use skill "plan-reply"?`. So the harness prompts before the
  skill's `allowed-tools` can apply at all.
- `claude -p`, default permissions: the Skill call was denied, and the
  agent answered in prose without recording anything.
- `claude -p` with only `Skill(plan-reply)` allowed: the skill loaded,
  but `frit reply "..." --json` was still denied. That held for both
  the one-line and the YAML-list form of `allowed-tools`.
- Control: the same run with the CLI allow rule `Bash(frit reply:*)`
  recorded the answer with no denial. The pattern matches the command
  and its quoting, so the pattern is not the fault.

I did not observe the interactive Bash step after the skill was
approved. The auto-mode classifier denied my driving that nested
session's approval prompt, and I did not work around it. That
interactive case, and whether `allowed-tools` is honored in an
interactive session but not in `-p`, is the open question.

**Decision for the owner before the next phase.** Two routes, neither
taken here. Ship a `.claude/settings.json` fragment with the bundle
that allows `Skill(plan-reply)` and `Bash(<frit> reply:*)`, which
leaves the skill's own pre-approval as documentation only. Or confirm
interactively that `allowed-tools` works there and treat `-p` as the
outlier. The first changes what `frit skills` writes, so it is not
mine to add unasked. Either way the envelope's raw `frit reply` command
still needs an operator's sign-off in a repository without the rule.

Side effect to know: trusting the fixture folder in the interactive
session recorded a trust entry for a scratchpad path in the user's
Claude Code config.
