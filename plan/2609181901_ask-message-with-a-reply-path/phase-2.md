---
n: 2
title: The bundle allows the reply in project settings
status: "✅"
result: false
---
Phase 1's real-session check showed the plan-reply skill's own
`allowed-tools` does not pre-approve the reply: the harness prompts
before it loads the skill, and under `claude -p` the Bash step was
still denied after the skill loaded. A project allow rule for the same
command worked. The owner chose that route. `frit skills` also writes
the rule beside the skills, so a responder in a repository with the
bundle replies with no operator sign-off.

RED, in order, each failing on today's code:

1. `skills.Install` into a repository with no `.claude/settings.json`
   writes one whose `permissions.allow` lists `Skill(plan-reply)` and
   `Bash(<invoke> reply:*)`, with `<invoke>` the `--via` value, and
   reports the path among those written.
2. Into a repository whose `.claude/settings.json` already carries
   other keys and other allow rules, it adds only the two missing
   rules and keeps every other key and rule. A second run changes
   nothing and does not report the file. A file that is not valid JSON
   is refused with the path named, and nothing else is written.
3. The Bash rule is the `allowed-tools` value the plan-reply skill
   ships, so the two never drift.
4. frit's own `.claude/settings.json` carries both rules for
   `go run ./cmd/frit`, guarded like the dogfooded skill copies.

GREEN: `Install` builds the merged settings before it writes a skill,
so a refusal leaves the repository untouched. It edits only
`permissions.allow`. Every other key survives, though a rewritten
file's keys come back sorted and indented. The rules widen a
repository's permissions, so the command's output names the file. Do
not add an opt-out flag unasked.

Decision on the plan's own text: its Context says the skill's
`allowed-tools` pre-approves the reply. Phase 1 disproved that, so
this phase rewrites that paragraph and the acceptance criterion to say
the project rule approves it and `allowed-tools` documents the intent.

BDD coverage: `@C14`, command-level, one host, no lease race. `frit
skills` run from the built frit into a repository with an existing
settings file leaves that file's own keys and lists the reply rule.
Bind it in `cmd/frit/bdd_ask_test.go` beside C13 on the shared world.
No `@S<n>` applies.

Gate: C14 runs without skipping against the built frit. Regenerate
frit's own `.claude` with `--via "go run ./cmd/frit"`. Then repeat
phase 1's real-session check with the bundle installed by the built
frit, under default permissions, and record whether the reply lands
with no permission prompt. If it still prompts, say so and do not tick
the approval criterion. Then run the full Go tests, lint and `mdsmith
check .`.
