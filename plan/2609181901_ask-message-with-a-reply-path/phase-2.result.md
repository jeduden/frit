---
n: 2
title: The reply is pre-approved where the harness reads it
status: "✅"
result: true
summary: >-
  frit skills merges Bash(<via> reply:*) and Skill(plan-reply) into the
  repository's .claude/settings.json allow list. A real session given
  only the envelope answered with no denial, once its workspace was
  trusted; the board read it answered.
---
# Phase 2 result

## Handoff

A lane now answers an ask with no operator sign-off. `frit skills`
merges two rules into the repository's `.claude/settings.json` allow
list: the reply command under the `--via` invocation, and loading
`plan-reply`. Other keys and rules survive, a rule already present is
not repeated, and a second install writes nothing. A settings file
frit cannot merge into stops the install before any skill lands. The
settings path is reported with the skill paths when it changed.
`plan-reply` dropped its `allowed-tools`, since it granted nothing.
frit's own settings now carry the `go run ./cmd/frit` form, and a
dogfood test pins them.

**Verified by.** Unit tests for the grants, the merge, the refusals
and the dogfood settings, committed red first. No new BDD row: the
behavior is one local file `frit skills` writes, with no lease and no
second host; C13 already covers the reply.

**The gate, in real sessions.** Claude Code 2.1.286, print mode,
default permissions, no flags. The fixture lane had the bundle
installed by the built `frit skills`, at the default bare `frit`. The
prompt was only the envelope `message --ask --go` sent.

1. Untrusted lane: the reply was denied. The harness said why: it
   ignores a project's allow rules until that workspace's trust
   prompt is accepted.
2. Parent directory trusted, lane not: still denied. Trust did not
   carry down to the new lane directory.
3. Lane trusted: the agent loaded the skill, ran `frit reply`, met no
   denial, and `board --json` read `answered` with its text.

**What that means for a lane.** An agent started interactively in a
new lane meets Claude Code's trust prompt first, and accepting it is
what makes these rules count. A print-mode session in a directory
never trusted still denies the reply; the operator trusts it once.

**For phase 3.** Render the ask state in the `board` and `who` tables.
Make the `(dead)` advice point at `--ask` and say silence is not
evidence. `plan-drive` already says so.
