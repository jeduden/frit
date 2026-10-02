---
n: 2
title: The reply is pre-approved where the harness reads it
status: "✅"
result: false
---
Phase 1 showed a skill's `allowed-tools` grants nothing: a real
session denied both the `plan-reply` load and `frit reply`, while the
same `Bash(frit reply:*)` pattern as a permission rule let the reply
land. The owner chose the permission rule. `frit skills` writes it.

RED, in order, each failing on today's code:

1. `skills.Install` adds `Bash(<invoke> reply:*)` and
   `Skill(plan-reply)` to `permissions.allow` in the repository's
   `.claude/settings.json`, creating the file when absent. The
   invocation follows `--via`, as the skills' own commands do.
2. An existing settings file is merged, never clobbered: other keys
   and other allow rules survive, a rule already present is not
   repeated, and a second install changes nothing. A settings file
   that is not JSON, or whose `permissions` or `allow` has the wrong
   shape, refuses the install before any file is written.
3. The written settings path is reported with the skill paths.

GREEN: the merge lives in `internal/skills` beside `Install`, which
validates the settings before writing a skill and writes them after.
`plan-reply` drops its `allowed-tools`: it grants nothing, and a
reader would trust it. frit's own `.claude/settings.json` regenerates
with `--via "go run ./cmd/frit"`, and a dogfood test pins its rules.
[development.md](../../docs/development.md) says where the grant
lives and why.

BDD coverage: no new row. The behavior is one local file written by
`frit skills`, with no lease and no second host. Unit tests pin the
merge. C13 already covers the reply itself.

Gate: rerun Phase 1's real-session check with only the envelope as
the prompt, default permissions, and the bundle installed by the
built `frit skills`. The reply must land with no denial, and
`board --json` must read it answered. Record the run in the phase
result. Then run the Go tests, lint and `mdsmith check .`.
