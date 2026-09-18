---
n: 2
title: The bundle allows the reply in project settings
status: "✅"
result: true
summary: A lane with the bundle installed in a trusted folder replies with no prompt; an untrusted folder still prompts, because Claude Code ignores its project allow rules.
---
## Handoff

Phase 2 made the approval real, with one limit found on the way. The
approval criterion is ticked, and the limit is written into the plan.

**What works.** `frit skills` now also merges `Skill(plan-reply)` and
`Bash(<invoke> reply:*)` into the repository's `.claude/settings.json`.
It keeps every other key and rule, does nothing when both are present,
and refuses a settings file that is not JSON before it writes a skill.
The command lists the file among those it wrote. C14 proves it against
the built frit, on a settings file that already carries a plugin and
rules, and it runs without skipping. frit's own `.claude` is
regenerated, and a test holds it to the bundle. The full Go tests, lint
and `mdsmith check .` are clean.

**The real-session check.** Claude Code 2.1.277 with haiku, `claude
-p` under default permissions, project settings only, the phase 1
envelope as the prompt, the bundle installed by the built frit:

- Trusted folder with the rule: the skill loaded, the reply was
  recorded and no tool was denied.
- Same folder with the settings file removed, as a control: the Skill
  call was denied and nothing was recorded.
- Untrusted folder with the same file: denied, and the record stayed
  empty. The same two rules passed through `--settings` or
  `--allowedTools` worked there. So Claude Code ignores a project's
  allow rules until the operator trusts the folder, which keeps a
  repository from granting itself permissions.

That trust limit is the plan's third stated limit, and
[development.md](../../docs/development.md) says it. A lane frit opens
in a folder the operator has worked in is normally trusted. A fresh
checkout nobody opened still prompts.

**Not observed.** The interactive case. Every run above is `claude
-p`. Phase 1 saw an interactive session prompt for the Skill before
any rule applied, but not with this rule present, and the auto-mode
classifier blocked driving that prompt. An operator who wants it
confirmed can install the bundle in a trusted repository and give a
session the envelope.

**Left for a later phase.** The `(dead)` advice still does not say an
unanswered ask is not evidence, and the board and `who` tables do not
render the ask state. That criterion stays unticked, and plan-drive's
ladder does not use the pending state yet. No opt-out flag for the
settings write was added, since the owner did not ask for one. It is
the first follow-up if the widening bothers a repository.

Side effect to know: this session rewrote a scratchpad fixture under
the earlier session's directory, which the user's Claude Code config
already lists as trusted.
