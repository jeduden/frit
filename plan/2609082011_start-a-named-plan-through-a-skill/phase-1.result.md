---
n: 1
title: Ship and prove the named-start skill
status: "✅"
result: true
summary: >-
  plan-start ships in the bundle; a named start reaches it from
  plan-pick and plan-drive; C7 and C8 prove the installed command
  starts the selected plan regardless of rank and refuses without
  choosing another. All eight acceptance criteria are met.
---

## Handoff

Every acceptance criterion is met:

- The canonical `plan-start` skill is in the bundle
  (`internal/skills/assets/plan-start/SKILL.md`), naming
  `{{frit}} start <selector> --go --json` as its command, describing
  the selector as required, and telling the reader to branch on
  `prompt_dispatched` and `pane` and never fall back to another plan
  or invoke `/plan-phase` itself.
- `plan-pick`'s and `plan-drive`'s own skills now route a named fresh
  start to `plan-start`: pick is for choosing unspecified work,
  drive's own `start` rung is scoped to resuming a lane whose
  checkout already holds a token, and a fresh named claim points at
  `plan-start` in both.
- `docs/development.md`'s skill-bundle paragraph now lists
  `plan-start`, and both the canonical assets and frit's own
  dogfooded `.claude/skills` were regenerated through
  `frit skills --via "go run ./cmd/frit" --force`, so the two trees
  do not drift.
- C7 and C8 in `features/commands.feature` are no longer `@pending`.
  Their steps live in `cmd/frit/bdd_commands_test.go`'s existing
  command-scenario section: they build a real frit binary once,
  install the skill bundle with that binary as the invocation, read
  the installed skill's own command line back out of its `SKILL.md`,
  and run it as a real subprocess — against a throwaway `herdr` on
  `$PATH` that actually stands up the git worktree it is asked for —
  so the scenario proves the shipped instructions work, not a
  hand-rolled stand-in for them. C7 starts the lower-ranked plan 7
  while plan 8, ranked above it, stays unheld with no agent; C8 asks
  for a dependency-blocked plan 7 while plan 8 is ready, and neither
  plan gains a hold or an agent.
- `go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run`
  and `mdsmith check .` all pass.

No deviation from the phase's spec: the CLI verb `frit start` needed
no change, only the skill layer that fronts it and the routing prose
in the two skills that used to send every fresh claim to `pick`
alone.

The plan carries one phase; this closes it, so `status:` moves
🔲 → ✅ on this same commit, with every Acceptance Criterion checked.

Nothing outstanding. The next session may start clear from
`go run ./cmd/frit show 2609082011`, which will read a finished plan.
