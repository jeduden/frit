---
n: 2
title: Plan authoring names its BDD coverage
status: "✅"
result: true
summary: CLAUDE.md, plan/proto.md and plan-new all name the S93-worked BDD decision
---
## Handoff

Plan authoring can no longer skip the BDD question by never asking
it. Three places now require the decision, all pointing at the same
worked example and procedure:

- [CLAUDE.md](../../CLAUDE.md)'s "Plan Maintenance" gained a bullet: a
  plan or phase touching the lease protocol decides whether it needs
  an `@S<n>` scenario and states that in the phase's own spec, per
  [docs/development.md](../../docs/development.md)'s executable
  scenario matrix, worked example S93.
- [plan/proto.md](../proto.md)'s "Phases and the Execution table"
  convention gained the matching bullet — and its embedded copy at
  [internal/scaffold/assets/proto.md](../../internal/scaffold/assets/proto.md)
  was re-synced, since `TestShippedProtoMatchesRepo` pins the two
  equal byte-for-byte.
- [the plan-new SKILL.md](../../internal/skills/assets/plan-new/SKILL.md)
  gained step 3, "Decide BDD coverage," between "Reuse first" and
  "Phase 1 is a proving slice"; steps 4–9 renumbered. The dogfood copy
  was regenerated with
  `go run ./cmd/frit skills . --force --via "go run ./cmd/frit"` —
  `git diff --stat .claude/skills/` touched only
  `plan-new/SKILL.md`.

**RED.** Editing the canonical `SKILL.md` alone, before regenerating
the dogfood copy, failed
`go test ./internal/skills -run TestDogfoodCopiesMatchCanonical` with
"has drifted from" — confirming that test is the real gate.

**Sizing.** The new step first pushed `## Method` to 45 lines and the
token budget to 676, past `.mdsmith.yml`'s caps (40 lines, 650
tokens). Trimmed the new step's wording and tightened step 2
("Reuse first") to fit.

**Gate.** Reading `.claude/skills/plan-new/SKILL.md` back confirms the
new step names S93 and `docs/development.md`.
`go test ./internal/skills -run TestDogfoodCopiesMatchCanonical` is
green. `mdsmith check .` is clean across the whole repo (288 files,
zero failures). `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.

**What Phase 3 inherits.** The BDD-coverage rule currently points
only at the lease-protocol matrix — the one scenario home that
exists. Phase 3 widens that home for a command behavior that is not a
lease-protocol scenario; it does not need to revisit CLAUDE.md,
plan/proto.md or plan-new again unless the new home changes how the
decision is stated.
